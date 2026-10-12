package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"mousehole/backend/internal/command"
)

// Commands 是传输层需要的最小业务接口；真实实现由 command.Service 提供。
type Commands interface {
	Accept(context.Context, string, string, string, command.Request) (command.Receipt, bool, error)
	Get(context.Context, string, string, string) (command.Receipt, error)
}

// APIError 是与 OpenAPI 一致的公开错误，不包含底层异常或完整请求。
type APIError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

type server struct {
	cfg      Config
	commands Commands
	logger   *slog.Logger
}
type requestIDKey struct{}

// NewHandler 校验配置并创建路由；业务路由统一经过来源、身份与项目边界。
func NewHandler(cfg Config, commands Commands, logger *slog.Logger) (http.Handler, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}
	s := &server{cfg: cfg, commands: commands, logger: logger}
	router := chi.NewRouter()
	router.Use(s.boundary)
	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		s.json(w, r, http.StatusOK, map[string]string{"status": "ok"})
	})
	router.Route("/api/v1", func(r chi.Router) {
		r.Use(s.developmentIdentity)
		r.Get("/session", s.session)
		r.Route("/projects/{project_id}/commands/{command_id}", func(r chi.Router) {
			r.Use(s.projectBoundary)
			r.Put("/", s.accept)
			r.Get("/", s.get)
		})
	})
	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		s.fail(w, r, 404, "route_not_found", "请求路径不存在。")
	})
	router.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		s.fail(w, r, 405, "method_not_allowed", "该路径不支持此请求方法。")
	})
	return router, nil
}

func (s *server) boundary(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		id, err := uuid.NewRandom()
		if err != nil {
			// 熵源异常时用零 UUID 标记无法关联的请求，仍保持统一错误合同。
			r = r.WithContext(context.WithValue(r.Context(), requestIDKey{}, uuid.Nil.String()))
			w.Header().Set("X-Request-ID", uuid.Nil.String())
			s.fail(w, r, http.StatusInternalServerError, "internal_error", "无法生成请求标识，请保留原命令 ID 查询确认。")
			return
		}
		ctx := context.WithValue(r.Context(), requestIDKey{}, id.String())
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		r = r.WithContext(ctx)
		w.Header().Set("X-Request-ID", id.String())
		defer func() {
			if recover() != nil {
				s.logger.ErrorContext(ctx, "请求处理异常", "event", "request_panicked", "request_id", id.String())
				s.fail(w, r, 500, "internal_error", "服务内部异常，请保留原命令 ID 查询确认。")
			}
		}()
		// 不信任转发头；显式 Host 可阻止浏览器通过 DNS 重绑定访问开发适配器。
		origin := r.Header.Get("Origin")
		if r.Host != s.cfg.ListenAddr || len(r.Header.Values("Origin")) > 1 || (origin != "" && origin != s.cfg.WebOrigin) {
			s.fail(w, r, 403, "development_only", "请求来源不符合本地开发配置。")
			return
		}
		w.Header().Add("Vary", "Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", s.cfg.WebOrigin)
			w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
		}
		if r.Method == http.MethodOptions {
			method := r.Header.Get("Access-Control-Request-Method")
			if origin == "" || (method != http.MethodGet && method != http.MethodPut) || !allowedHeaders(r.Header.Get("Access-Control-Request-Headers")) {
				s.fail(w, r, 403, "development_only", "跨来源预检不符合本地开发协议。")
				return
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, PUT")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Mousehole-Development")
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
		case "content-type", "x-mousehole-development":
		default:
			return false
		}
	}
	return true
}

func (s *server) developmentIdentity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Mousehole-Development") != "true" || len(r.Header.Values("X-Mousehole-Development")) != 1 {
			s.fail(w, r, 403, "development_only", "业务请求必须携带本地开发标记。")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *server) projectBoundary(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		projectID, commandID := chi.URLParam(r, "project_id"), chi.URLParam(r, "command_id")
		if !command.ValidID(projectID) || !command.ValidID(commandID) {
			s.domainError(w, r, command.ErrInvalidRequest)
			return
		}
		if projectID != command.DevelopmentProjectID {
			s.domainError(w, r, command.ErrForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *server) session(w http.ResponseWriter, r *http.Request) {
	s.json(w, r, 200, map[string]string{"mode": "development", "actor_id": command.DevelopmentActorID, "project_id": command.DevelopmentProjectID, "project_name": "本地演示项目"})
}

func (s *server) accept(w http.ResponseWriter, r *http.Request) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		s.fail(w, r, 415, "unsupported_media_type", "请求必须使用 application/json。")
		return
	}
	request, err := decodeRequest(w, r)
	if err != nil {
		var sizeError *http.MaxBytesError
		if errors.As(err, &sizeError) {
			s.fail(w, r, 413, "request_too_large", "请求体不能超过 16 KiB。")
		} else {
			s.domainError(w, r, command.ErrInvalidRequest)
		}
		return
	}
	receipt, created, err := s.commands.Accept(r.Context(), command.DevelopmentActorID, chi.URLParam(r, "project_id"), chi.URLParam(r, "command_id"), request)
	if err != nil {
		s.domainError(w, r, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	s.logger.InfoContext(r.Context(), "任务草稿命令已受理", "event", "command_accepted", "request_id", r.Context().Value(requestIDKey{}), "project_id", receipt.ProjectID, "command_id", receipt.CommandID, "created", created)
	s.json(w, r, status, receipt)
}

func (s *server) get(w http.ResponseWriter, r *http.Request) {
	receipt, err := s.commands.Get(r.Context(), command.DevelopmentActorID, chi.URLParam(r, "project_id"), chi.URLParam(r, "command_id"))
	if err != nil {
		s.domainError(w, r, err)
		return
	}
	s.json(w, r, http.StatusOK, receipt)
}

func (s *server) domainError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, command.ErrInvalidRequest):
		s.fail(w, r, 400, "invalid_request", "命令参数无效，请检查小写 UUID、命令类型及 1–120 字标题。")
	case errors.Is(err, command.ErrForbidden):
		s.fail(w, r, 403, "project_forbidden", "本地开发身份无权访问该项目。")
	case errors.Is(err, command.ErrNotFound):
		s.fail(w, r, 404, "command_not_found", "暂未查询到命令，请保留原命令 ID 再次查询。")
	case errors.Is(err, command.ErrConflict):
		s.fail(w, r, 409, "idempotency_conflict", "该命令 ID 已用于不同参数，请查询原命令。")
	case errors.Is(err, command.ErrUnavailable):
		s.fail(w, r, 503, "service_unavailable", "服务暂不可用，受理结果待确认，请保留原命令 ID 查询。")
	default:
		s.fail(w, r, 500, "internal_error", "服务内部异常，请保留原命令 ID 查询确认。")
	}
}

func (s *server) fail(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	id, _ := r.Context().Value(requestIDKey{}).(string)
	if status >= 500 {
		s.logger.ErrorContext(r.Context(), "请求未能确认完成", "event", "request_failed", "request_id", id, "code", code)
	}
	s.json(w, r, status, APIError{Code: code, Message: message, RequestID: id})
}

func (s *server) json(w http.ResponseWriter, r *http.Request, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		s.logger.WarnContext(r.Context(), "响应传输中断，客户端需按原命令查询", "event", "response_write_failed", "request_id", r.Context().Value(requestIDKey{}))
	}
}
