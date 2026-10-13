package httpapi

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"

	"mousehole/backend/internal/asset"
	"mousehole/backend/internal/authorization"
	"mousehole/backend/internal/command"
	"mousehole/backend/internal/policy"
	"mousehole/backend/internal/scopecommand"
	"mousehole/backend/internal/validation"

	"github.com/go-chi/chi/v5"
)

// Scopes 是资产、授权与静态决策传输层需要的明确接口；不含执行入口。
type Scopes interface {
	Accept(context.Context, string, string, string, scopecommand.Request) (scopecommand.Receipt, bool, error)
	Get(context.Context, string, string, string) (scopecommand.Receipt, error)
	PreviewAssets(context.Context, string, string, asset.Import) (asset.Preview, error)
	ListAssets(context.Context, string, string, scopecommand.ListOptions) (scopecommand.Page[asset.Asset], error)
	ListVersions(context.Context, string, string, scopecommand.ListOptions) (scopecommand.Page[authorization.Version], error)
	GetVersion(context.Context, string, string, string) (authorization.Version, error)
	GetPolicy(context.Context, string, string) (scopecommand.Policy, error)
	ListPrechecks(context.Context, string, string, scopecommand.ListOptions) (scopecommand.Page[policy.Decision], error)
	GetPrecheck(context.Context, string, string, string) (policy.Decision, error)
}

func (s *server) scopeRoutes(r chi.Router) {
	r.Route("/projects/{project_id}", func(r chi.Router) {
		r.Use(s.scopeBoundary)
		r.Get("/assets", s.scopeRead)
		r.With(s.requireCSRF).Post("/assets/preview", s.scopePreview)
		r.With(s.requireCSRF).Put("/scope-commands/{command_id}", s.scopeAccept)
		r.Get("/scope-commands/{command_id}", s.scopeRead)
		r.Get("/authorization-versions", s.scopeRead)
		r.Get("/authorization-versions/{version_id}", s.scopeRead)
		r.Get("/policy", s.scopeRead)
		r.Get("/prechecks", s.scopeRead)
		r.Get("/prechecks/{decision_id}", s.scopeRead)
	})
}
func (s *server) scopeBoundary(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !command.ValidID(chi.URLParam(r, "project_id")) {
			s.scopeError(w, r, scopecommand.ErrInvalidRequest)
			return
		}
		for _, key := range []string{"command_id", "version_id", "decision_id"} {
			value := chi.URLParam(r, key)
			if value != "" && !command.ValidID(value) {
				s.scopeError(w, r, scopecommand.ErrInvalidRequest)
				return
			}
		}
		if s.scopes == nil {
			s.scopeError(w, r, scopecommand.ErrUnavailable)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *server) scopeBody(w http.ResponseWriter, r *http.Request, out any) bool {
	media, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || media != "application/json" {
		s.fail(w, r, 415, "unsupported_media_type", "请求必须使用 application/json。")
		return false
	}
	body, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 256*1024))
	if e != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(e, &tooLarge) {
			s.fail(w, r, 413, "request_too_large", "请求体不能超过 256 KiB。")
		} else {
			s.scopeError(w, r, scopecommand.ErrInvalidRequest)
		}
		return false
	}
	if e = scopecommand.DecodeJSON(body, out); e != nil {
		s.scopeError(w, r, e)
		return false
	}
	return true
}
func (s *server) scopeAccept(w http.ResponseWriter, r *http.Request) {
	if !command.ValidID(chi.URLParam(r, "command_id")) {
		s.scopeError(w, r, scopecommand.ErrInvalidRequest)
		return
	}
	var request scopecommand.Request
	if !s.scopeBody(w, r, &request) {
		return
	}
	receipt, created, e := s.scopes.Accept(r.Context(), currentPrincipal(r).ActorID, chi.URLParam(r, "project_id"), chi.URLParam(r, "command_id"), request)
	if e != nil {
		s.scopeError(w, r, e)
		return
	}
	status := 200
	if created {
		status = 201
	}
	s.logger.InfoContext(r.Context(), "范围命令已受理", "event", "scope_command_accepted", "request_id", requestID(r), "command_id", receipt.CommandID, "kind", receipt.Kind, "created", created)
	s.json(w, r, status, receipt)
}
func (s *server) scopePreview(w http.ResponseWriter, r *http.Request) {
	var request asset.Import
	if !s.scopeBody(w, r, &request) {
		return
	}
	preview, e := s.scopes.PreviewAssets(r.Context(), currentPrincipal(r).ActorID, chi.URLParam(r, "project_id"), request)
	if e != nil {
		s.scopeError(w, r, e)
		return
	}
	s.json(w, r, 200, preview)
}

func scopeOptions(r *http.Request, assets bool) (scopecommand.ListOptions, error) {
	o := scopecommand.ListOptions{}
	values, e := parseQuery(r)
	if e != nil {
		return o, e
	}
	for key, list := range values {
		if len(list) != 1 {
			return o, scopecommand.ErrInvalidRequest
		}
		switch key {
		case "page_size":
			o.PageSize, e = strconv.Atoi(list[0])
			if e != nil || o.PageSize < 1 || o.PageSize > 100 {
				return o, scopecommand.ErrInvalidRequest
			}
		case "cursor":
			o.Cursor = list[0]
		case "kind":
			if !assets {
				return o, scopecommand.ErrInvalidRequest
			}
			o.Kind = list[0]
		case "search":
			if !assets {
				return o, scopecommand.ErrInvalidRequest
			}
			o.Search = list[0]
		default:
			return o, scopecommand.ErrInvalidRequest
		}
	}
	return o, nil
}
func parseQuery(r *http.Request) (url.Values, error) {
	values, e := url.ParseQuery(r.URL.RawQuery)
	if e != nil {
		return nil, scopecommand.ErrInvalidRequest
	}
	return values, nil
}

func (s *server) scopeRead(w http.ResponseWriter, r *http.Request) {
	// 子路由参数在匹配最终 handler 后才可见，不能仅依赖父路由 middleware。
	for _, key := range []string{"command_id", "version_id", "decision_id"} {
		if value := chi.URLParam(r, key); value != "" && !command.ValidID(value) {
			s.scopeError(w, r, scopecommand.ErrInvalidRequest)
			return
		}
	}
	actor, project := currentPrincipal(r).ActorID, chi.URLParam(r, "project_id")
	var value any
	var e error
	pattern := chi.RouteContext(r.Context()).RoutePattern()
	switch pattern {
	case "/api/v1/projects/{project_id}/assets":
		var o scopecommand.ListOptions
		o, e = scopeOptions(r, true)
		if e == nil {
			value, e = s.scopes.ListAssets(r.Context(), actor, project, o)
		}
	case "/api/v1/projects/{project_id}/authorization-versions":
		var o scopecommand.ListOptions
		o, e = scopeOptions(r, false)
		if e == nil {
			value, e = s.scopes.ListVersions(r.Context(), actor, project, o)
		}
	case "/api/v1/projects/{project_id}/prechecks":
		var o scopecommand.ListOptions
		o, e = scopeOptions(r, false)
		if e == nil {
			value, e = s.scopes.ListPrechecks(r.Context(), actor, project, o)
		}
	case "/api/v1/projects/{project_id}/authorization-versions/{version_id}":
		value, e = s.scopes.GetVersion(r.Context(), actor, project, chi.URLParam(r, "version_id"))
	case "/api/v1/projects/{project_id}/prechecks/{decision_id}":
		value, e = s.scopes.GetPrecheck(r.Context(), actor, project, chi.URLParam(r, "decision_id"))
	case "/api/v1/projects/{project_id}/policy":
		value, e = s.scopes.GetPolicy(r.Context(), actor, project)
	case "/api/v1/projects/{project_id}/scope-commands/{command_id}":
		value, e = s.scopes.Get(r.Context(), actor, project, chi.URLParam(r, "command_id"))
	default:
		e = scopecommand.ErrNotFound
	}
	if e != nil {
		s.scopeError(w, r, e)
		return
	}
	s.json(w, r, 200, value)
}
func (s *server) scopeError(w http.ResponseWriter, r *http.Request, e error) {
	switch {
	case errors.Is(e, scopecommand.ErrInvalidRequest):
		s.json(w, r, 400, APIError{Code: "invalid_request", Message: "范围请求参数无效，请检查字段、规范化预览及范围约束。", RequestID: requestID(r), ValidationErrors: validation.Details(e)})
	case errors.Is(e, scopecommand.ErrForbidden):
		s.fail(w, r, 403, "forbidden", "当前身份无权访问该项目或执行此操作。")
	case errors.Is(e, scopecommand.ErrNotFound):
		s.fail(w, r, 404, "not_found", "暂未查询到对象；命令结果未知时请保留原 ID 查询。")
	case errors.Is(e, scopecommand.ErrConflict):
		s.fail(w, r, 409, "command_conflict", "原命令 ID 已用于不同参数，请查询原回执。")
	case errors.Is(e, scopecommand.ErrRevisionConflict):
		s.fail(w, r, 409, "policy_revision_conflict", "策略已变化，请重新读取修订后确认操作。")
	case errors.Is(e, scopecommand.ErrUnavailable):
		s.fail(w, r, 503, "unavailable", "服务暂不可用，受理结果待确认，请保留原命令 ID 查询。")
	default:
		s.fail(w, r, 500, "internal_error", "服务内部异常，请保留原命令 ID 查询确认。")
	}
}
