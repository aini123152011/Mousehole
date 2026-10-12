package command

import (
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	// DevelopmentActorID 是本地开发适配器唯一的合成身份，不是生产凭据。
	DevelopmentActorID = "00000000-0000-4000-8000-000000000001"
	// DevelopmentProjectID 限定本切片可访问的唯一合成项目。
	DevelopmentProjectID = "00000000-0000-4000-8000-000000000002"
	// CreateTaskDraft 是本切片唯一支持的命令，不产生目标执行行为。
	CreateTaskDraft = "create_task_draft"
)

var (
	// ErrInvalidRequest 表示参数或命令类型不满足合同。
	ErrInvalidRequest = errors.New("命令参数无效")
	// ErrForbidden 表示身份或项目不在本地开发边界内。
	ErrForbidden = errors.New("无权访问该项目")
	// ErrNotFound 表示当前尚未读取到回执，不证明并发提交一定失败。
	ErrNotFound = errors.New("暂未查询到命令")
	// ErrConflict 表示幂等键已经绑定不同参数或身份。
	ErrConflict = errors.New("命令 ID 已绑定不同参数")
	// ErrUnavailable 包括提交结果未知；调用者必须保留原 ID 查询。
	ErrUnavailable = errors.New("数据库暂不可用或受理结果待确认")
)

var canonicalUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// Request 是 OpenAPI 命令正文，HTTP 层拒绝未知字段后交由服务验证业务约束。
type Request struct {
	Kind    string  `json:"kind"`
	Payload Payload `json:"payload"`
}

// Payload 保存合成草稿参数；标题在受理前只做首尾 Unicode 空白规范化。
type Payload struct {
	Title string `json:"title"`
}

// Receipt 是持久化受理快照；accepted 与 draft 均不表示发生过目标行为。
type Receipt struct {
	CommandID  string `json:"command_id"`
	ProjectID  string `json:"project_id"`
	Kind       string `json:"kind"`
	State      string `json:"state"`
	TaskID     string `json:"task_id"`
	TaskTitle  string `json:"task_title"`
	TaskState  string `json:"task_state"`
	Version    string `json:"version"`     // 十进制业务版本，避免浏览器整数精度损失。
	AcceptedAt string `json:"accepted_at"` // 原始受理时间，UTC RFC3339。
}

// ValidID 只接受标准带连字符的小写 UUID，禁止隐式改写客户端幂等键。
func ValidID(value string) bool { return canonicalUUID.MatchString(value) }

// NormalizeRequest 统一标题边界，并拒绝未支持命令及不合法 UTF-8 文本。
func NormalizeRequest(request Request) (Request, error) {
	request.Payload.Title = strings.TrimSpace(request.Payload.Title)
	length := utf8.RuneCountInString(request.Payload.Title)
	if request.Kind != CreateTaskDraft || !utf8.ValidString(request.Payload.Title) || length < 1 || length > 120 || strings.ContainsRune(request.Payload.Title, 0) {
		return Request{}, ErrInvalidRequest
	}
	return request, nil
}

func validateAccess(actorID, projectID, commandID string) error {
	if !ValidID(projectID) || !ValidID(commandID) {
		return ErrInvalidRequest
	}
	if actorID != DevelopmentActorID || projectID != DevelopmentProjectID {
		return ErrForbidden
	}
	return nil
}
