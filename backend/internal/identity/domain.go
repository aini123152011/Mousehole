package identity

import (
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// RoleViewer 允许读取项目内当前用户自己的资源。
	RoleViewer = "viewer"
	// RoleAnalyst 在读取能力之外允许创建任务草稿。
	RoleAnalyst = "analyst"
	// CapabilityProjectRead 表示项目基础读取能力。
	CapabilityProjectRead = "project:read"
	// CapabilityTaskDraftCreate 表示创建任务草稿的能力。
	CapabilityTaskDraftCreate = "task_draft:create"
)

var (
	// ErrInvalidRequest 表示身份入口参数不满足公开合同。
	ErrInvalidRequest = errors.New("身份请求参数无效")
	// ErrUnauthenticated 表示没有可用服务端会话。
	ErrUnauthenticated = errors.New("尚未登录")
	// ErrSessionExpired 表示会话已撤销或超过空闲、绝对有效期。
	ErrSessionExpired = errors.New("会话已失效")
	// ErrIdentityNotProvisioned 表示 OIDC 身份未由管理员预置。
	ErrIdentityNotProvisioned = errors.New("身份尚未开通")
	// ErrCSRFInvalid 表示写请求的会话绑定 CSRF 校验失败。
	ErrCSRFInvalid = errors.New("请求防伪校验失败")
	// ErrConflict 表示 bootstrap 参数与既有稳定绑定冲突。
	ErrConflict = errors.New("身份或项目参数与既有记录冲突")
	// ErrAdminTargetNotFound 表示管理命令指定的用户或成员关系不存在。
	ErrAdminTargetNotFound = errors.New("管理目标不存在")
	// ErrUnavailable 表示身份存储或 OIDC 依赖无法确认完成。
	ErrUnavailable = errors.New("身份服务暂不可用")
)

// ProjectAccess 是会话可见的项目与稳定能力投影。
type ProjectAccess struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Role         string   `json:"role"`
	Capabilities []string `json:"capabilities"`
	Version      string   `json:"version"`
}

// Principal 是从服务端会话恢复的当前用户，不包含任何 OIDC token。
type Principal struct {
	ActorID           string          `json:"actor_id"`
	DisplayName       string          `json:"display_name"`
	Email             *string         `json:"email"`
	Projects          []ProjectAccess `json:"projects"`
	AbsoluteExpiresAt time.Time       `json:"-"`
}

// LoginStart 是浏览器跳转登录所需的公开结果；绑定值只进入短期 HttpOnly Cookie。
type LoginStart struct {
	AuthorizationURL string
	BrowserBinding   string
	ExpiresAt        time.Time
}

// LoginResult 是完成 OIDC 回调后创建的本地会话；token 只进入 HttpOnly Cookie。
type LoginResult struct {
	SessionToken string
	ReturnPath   string
	ExpiresAt    time.Time
}

// ProvisionRequest 描述受控 bootstrap 的完整稳定绑定。
type ProvisionRequest struct {
	Issuer      string
	Subject     string
	UserID      string
	DisplayName string
	Email       string
	ProjectID   string
	ProjectName string
	Role        string
}

// AdminMutationResult 描述管理操作是否改变权威状态及其撤销的会话数量。
type AdminMutationResult struct {
	Changed         bool
	RevokedSessions int64
}

// CapabilitiesForRole 返回角色对应的固定能力集合，未知角色默认无能力。
func CapabilitiesForRole(role string) []string {
	switch role {
	case RoleViewer:
		return []string{CapabilityProjectRead}
	case RoleAnalyst:
		return []string{CapabilityProjectRead, CapabilityTaskDraftCreate}
	default:
		return []string{}
	}
}

func validProvision(request ProvisionRequest) bool {
	return request.Issuer != "" && request.Subject != "" && validProfileName(request.DisplayName) &&
		validProfileName(request.ProjectName) && validProfileEmail(request.Email) &&
		(request.Role == RoleViewer || request.Role == RoleAnalyst)
}

func validProfileName(value string) bool {
	value = strings.TrimSpace(value)
	length := utf8.RuneCountInString(value)
	return utf8.ValidString(value) && length >= 1 && length <= 120 && !strings.ContainsRune(value, 0)
}

func validProfileEmail(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	address, err := mail.ParseAddress(value)
	return err == nil && address.Address == value
}

// ValidID 只接受标准带连字符的小写 UUID，避免管理入口隐式改写标识。
func ValidID(value string) bool { return canonicalID(value) }
