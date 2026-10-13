package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"mousehole/backend/internal/command"
	"mousehole/backend/internal/identity"
	"mousehole/backend/internal/validation"
)

// Commands 是传输层需要的最小业务接口；真实实现由 command.Service 提供。
type Commands interface {
	Accept(context.Context, string, string, string, command.Request) (command.Receipt, bool, error)
	Get(context.Context, string, string, string) (command.Receipt, error)
}

// Identities 是传输层需要的登录、会话与 CSRF 能力。
type Identities interface {
	BeginLogin(context.Context) (identity.LoginStart, error)
	CompleteLogin(context.Context, string, string, string) (identity.LoginResult, error)
	Authenticate(context.Context, string) (identity.Principal, error)
	RevokeSession(context.Context, string) error
	CSRFToken(string) string
	ValidateCSRF(string, string) error
}

// APIError 是与 OpenAPI 一致的公开错误，不包含底层异常或完整请求。
type APIError struct {
	Code             string             `json:"code"`
	Message          string             `json:"message"`
	RequestID        string             `json:"request_id"`
	ValidationErrors []validation.Issue `json:"validation_errors,omitempty"`
}

// SessionResponse 是浏览器内存中的当前会话投影；不含 Cookie 或 OIDC token。
type SessionResponse struct {
	ActorID     string                   `json:"actor_id"`
	DisplayName string                   `json:"display_name"`
	Email       *string                  `json:"email"`
	Projects    []identity.ProjectAccess `json:"projects"`
	CSRFToken   string                   `json:"csrf_token"`
	ExpiresAt   string                   `json:"expires_at"`
}

type server struct {
	cfg        Config
	commands   Commands
	identities Identities
	logger     *slog.Logger
	scopes     Scopes
}
type requestIDKey struct{}
type principalKey struct{}
type sessionTokenKey struct{}

// NewHandler 校验配置并创建路由；受保护路由只从服务端会话取得 actor。
func NewHandler(cfg Config, commands Commands, identities Identities, logger *slog.Logger, scopes ...Scopes) (http.Handler, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if commands == nil || identities == nil {
		return nil, errors.New("命令与身份服务不能为空")
	}
	if logger == nil {
		logger = slog.Default()
	}
	s := &server{cfg: cfg, commands: commands, identities: identities, logger: logger}
	if len(scopes) > 0 {
		s.scopes = scopes[0]
	}
	router := chi.NewRouter()
	router.Use(s.boundary)
	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		s.json(w, r, http.StatusOK, map[string]string{"status": "ok"})
	})
	router.Route("/api/v1", func(r chi.Router) {
		r.Get("/auth/login", s.login)
		r.Get("/auth/callback", s.callback)
		r.Group(func(r chi.Router) {
			r.Use(s.requireSession)
			r.Get("/session", s.session)
			r.With(s.requireCSRF).Post("/auth/logout", s.logout)
			s.scopeRoutes(r)
			r.Route("/projects/{project_id}/commands/{command_id}", func(r chi.Router) {
				r.Use(s.commandBoundary)
				r.With(s.requireCSRF).Put("/", s.accept)
				r.Get("/", s.get)
			})
		})
	})
	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		s.fail(w, r, http.StatusNotFound, "route_not_found", "请求路径不存在。")
	})
	router.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		s.fail(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "该路径不支持此请求方法。")
	})
	return router, nil
}

func (s *server) boundary(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		id, err := uuid.NewRandom()
		if err != nil {
			r = r.WithContext(context.WithValue(r.Context(), requestIDKey{}, uuid.Nil.String()))
			w.Header().Set("X-Request-ID", uuid.Nil.String())
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "无法生成请求标识。")
			return
		}
		ctx := context.WithValue(r.Context(), requestIDKey{}, id.String())
		ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		r = r.WithContext(ctx)
		w.Header().Set("X-Request-ID", id.String())
		defer func() {
			if recover() != nil {
				s.logger.ErrorContext(ctx, "请求处理异常", "event", "request_panicked", "request_id", id.String())
				s.fail(w, r, http.StatusInternalServerError, "internal_error", "服务内部异常，请保留原命令 ID 查询确认。")
			}
		}()
		origin := r.Header.Get("Origin")
		if r.Host != s.cfg.publicHost() || len(r.Header.Values("Origin")) > 1 || (origin != "" && origin != s.cfg.WebOrigin) {
			s.fail(w, r, http.StatusForbidden, "origin_forbidden", "请求来源不符合部署配置。")
			return
		}
		w.Header().Add("Vary", "Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", s.cfg.WebOrigin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
		}
		if r.Method == http.MethodOptions {
			method := r.Header.Get("Access-Control-Request-Method")
			if origin == "" || (method != http.MethodGet && method != http.MethodPost && method != http.MethodPut) || !allowedHeaders(r.Header.Get("Access-Control-Request-Headers")) {
				s.fail(w, r, http.StatusForbidden, "origin_forbidden", "跨来源预检不符合部署协议。")
				return
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token")
			w.Header().Add("Vary", "Access-Control-Request-Method")
			w.Header().Add("Vary", "Access-Control-Request-Headers")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func allowedHeaders(value string) bool {
	if value == "" {
		return true
	}
	for _, header := range strings.Split(value, ",") {
		switch strings.ToLower(strings.TrimSpace(header)) {
		case "content-type", "x-csrf-token":
		default:
			return false
		}
	}
	return true
}

func (s *server) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookies := r.CookiesNamed(s.cfg.sessionCookieName())
		if len(cookies) != 1 || cookies[0].Value == "" {
			s.identityError(w, r, identity.ErrUnauthenticated)
			return
		}
		principal, err := s.identities.Authenticate(r.Context(), cookies[0].Value)
		if err != nil {
			s.identityError(w, r, err)
			return
		}
		ctx := context.WithValue(r.Context(), principalKey{}, principal)
		ctx = context.WithValue(ctx, sessionTokenKey{}, cookies[0].Value)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *server) requireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		values := r.Header.Values("X-CSRF-Token")
		if len(values) != 1 || len(r.Header.Values("Origin")) != 1 || r.Header.Get("Origin") != s.cfg.WebOrigin ||
			s.identities.ValidateCSRF(sessionToken(r), values[0]) != nil {
			s.identityError(w, r, identity.ErrCSRFInvalid)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *server) commandBoundary(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !command.ValidID(chi.URLParam(r, "project_id")) || !command.ValidID(chi.URLParam(r, "command_id")) {
			s.domainError(w, r, command.ErrInvalidRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	start, err := s.identities.BeginLogin(r.Context())
	if err != nil {
		s.identityError(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: s.cfg.loginCookieName(), Value: start.BrowserBinding, Path: "/", Expires: start.ExpiresAt,
		MaxAge: int(time.Until(start.ExpiresAt).Seconds()), HttpOnly: true, Secure: s.cfg.secureCookies(), SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, start.AuthorizationURL, http.StatusFound)
}

func (s *server) callback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	_, hasOIDCError := query["error"]
	bindings := r.CookiesNamed(s.cfg.loginCookieName())
	s.clearLoginCookie(w)
	if len(bindings) != 1 || bindings[0].Value == "" || len(query["state"]) != 1 || len(query["code"]) != 1 || hasOIDCError {
		s.callbackFailure(w, r, identity.ErrInvalidRequest)
		return
	}
	result, err := s.identities.CompleteLogin(r.Context(), query.Get("state"), bindings[0].Value, query.Get("code"))
	if err != nil {
		s.callbackFailure(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: s.cfg.sessionCookieName(), Value: result.SessionToken, Path: "/", Expires: result.ExpiresAt,
		MaxAge: int(time.Until(result.ExpiresAt).Seconds()), HttpOnly: true, Secure: s.cfg.secureCookies(), SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, s.cfg.WebOrigin+result.ReturnPath, http.StatusSeeOther)
}

func (s *server) session(w http.ResponseWriter, r *http.Request) {
	principal := currentPrincipal(r)
	s.json(w, r, http.StatusOK, SessionResponse{
		ActorID: principal.ActorID, DisplayName: principal.DisplayName, Email: principal.Email,
		Projects: principal.Projects, CSRFToken: s.identities.CSRFToken(sessionToken(r)),
		ExpiresAt: principal.AbsoluteExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	if err := s.identities.RevokeSession(r.Context(), sessionToken(r)); err != nil {
		s.identityError(w, r, err)
		return
	}
	s.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) accept(w http.ResponseWriter, r *http.Request) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		s.fail(w, r, http.StatusUnsupportedMediaType, "unsupported_media_type", "请求必须使用 application/json。")
		return
	}
	request, err := decodeRequest(w, r)
	if err != nil {
		var sizeError *http.MaxBytesError
		if errors.As(err, &sizeError) {
			s.fail(w, r, http.StatusRequestEntityTooLarge, "request_too_large", "请求体不能超过 16 KiB。")
		} else {
			s.domainError(w, r, command.ErrInvalidRequest)
		}
		return
	}
	receipt, created, err := s.commands.Accept(r.Context(), currentPrincipal(r).ActorID, chi.URLParam(r, "project_id"), chi.URLParam(r, "command_id"), request)
	if err != nil {
		s.domainError(w, r, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	s.logger.InfoContext(r.Context(), "任务草稿命令已受理", "event", "command_accepted", "request_id", requestID(r), "project_id", receipt.ProjectID, "command_id", receipt.CommandID, "created", created)
	s.json(w, r, status, receipt)
}

func (s *server) get(w http.ResponseWriter, r *http.Request) {
	receipt, err := s.commands.Get(r.Context(), currentPrincipal(r).ActorID, chi.URLParam(r, "project_id"), chi.URLParam(r, "command_id"))
	if err != nil {
		s.domainError(w, r, err)
		return
	}
	s.json(w, r, http.StatusOK, receipt)
}

func (s *server) domainError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, command.ErrInvalidRequest):
		s.fail(w, r, http.StatusBadRequest, "invalid_request", "命令参数无效，请检查小写 UUID、命令类型及 1–120 字标题。")
	case errors.Is(err, command.ErrForbidden):
		s.fail(w, r, http.StatusForbidden, "project_forbidden", "当前身份无权访问该项目或执行此操作。")
	case errors.Is(err, command.ErrNotFound):
		s.fail(w, r, http.StatusNotFound, "command_not_found", "暂未查询到命令，请保留原命令 ID 再次查询。")
	case errors.Is(err, command.ErrConflict):
		s.fail(w, r, http.StatusConflict, "idempotency_conflict", "该命令 ID 已用于不同参数，请查询原命令。")
	case errors.Is(err, command.ErrUnavailable):
		s.fail(w, r, http.StatusServiceUnavailable, "service_unavailable", "服务暂不可用，受理结果待确认，请保留原命令 ID 查询。")
	default:
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "服务内部异常，请保留原命令 ID 查询确认。")
	}
}

func (s *server) identityError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, identity.ErrUnauthenticated):
		s.fail(w, r, http.StatusUnauthorized, "unauthenticated", "请先使用企业身份登录。")
	case errors.Is(err, identity.ErrSessionExpired):
		s.clearSessionCookie(w)
		s.fail(w, r, http.StatusUnauthorized, "session_expired", "会话已失效，请重新登录。")
	case errors.Is(err, identity.ErrCSRFInvalid):
		s.fail(w, r, http.StatusForbidden, "csrf_invalid", "请求防伪校验失败，请刷新页面后重试。")
	case errors.Is(err, identity.ErrIdentityNotProvisioned):
		s.fail(w, r, http.StatusForbidden, "identity_not_provisioned", "该企业身份尚未开通平台权限。")
	case errors.Is(err, identity.ErrInvalidRequest):
		s.fail(w, r, http.StatusBadRequest, "invalid_request", "登录请求无效或已经使用，请重新发起登录。")
	case errors.Is(err, identity.ErrUnavailable):
		s.fail(w, r, http.StatusServiceUnavailable, "service_unavailable", "身份服务暂不可用，请稍后重试。")
	default:
		s.fail(w, r, http.StatusInternalServerError, "internal_error", "身份服务内部异常。")
	}
}

func (s *server) callbackFailure(w http.ResponseWriter, r *http.Request, err error) {
	code, message, status := "invalid_request", "登录请求无效或已经使用，请重新发起登录。", http.StatusBadRequest
	if errors.Is(err, identity.ErrIdentityNotProvisioned) {
		code, message, status = "identity_not_provisioned", "该企业身份尚未开通平台权限，请联系管理员。", http.StatusForbidden
	} else if errors.Is(err, identity.ErrUnavailable) {
		code, message, status = "service_unavailable", "身份服务暂不可用，请稍后重试。", http.StatusServiceUnavailable
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = callbackPage.Execute(w, map[string]string{"Code": code, "Message": message, "RequestID": requestID(r), "LoginURL": s.cfg.PublicURL + "/api/v1/auth/login"})
}

var callbackPage = template.Must(template.New("callback-error").Parse(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>登录未完成</title></head><body><main><h1>登录未完成</h1><p>{{.Message}}</p><p>错误码：<code>{{.Code}}</code></p><p>请求 ID：<code>{{.RequestID}}</code></p><p><a href="{{.LoginURL}}">重新登录</a></p></main></body></html>`))

func (s *server) clearLoginCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: s.cfg.loginCookieName(), Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.cfg.secureCookies(), SameSite: http.SameSiteLaxMode})
}

func (s *server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: s.cfg.sessionCookieName(), Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.cfg.secureCookies(), SameSite: http.SameSiteLaxMode})
}

func currentPrincipal(r *http.Request) identity.Principal {
	principal, _ := r.Context().Value(principalKey{}).(identity.Principal)
	return principal
}

func sessionToken(r *http.Request) string {
	token, _ := r.Context().Value(sessionTokenKey{}).(string)
	return token
}

func requestID(r *http.Request) string {
	id, _ := r.Context().Value(requestIDKey{}).(string)
	return id
}

func (s *server) fail(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	id := requestID(r)
	if status >= 500 {
		s.logger.ErrorContext(r.Context(), "请求未能确认完成", "event", "request_failed", "request_id", id, "code", code)
	}
	s.json(w, r, status, APIError{Code: code, Message: message, RequestID: id})
}

func (s *server) json(w http.ResponseWriter, r *http.Request, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		s.logger.WarnContext(r.Context(), "响应传输中断，客户端需按原命令查询", "event", "response_write_failed", "request_id", requestID(r))
	}
}
