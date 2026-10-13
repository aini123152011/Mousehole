package scopecommand

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"unicode/utf8"

	"mousehole/backend/internal/asset"
	"mousehole/backend/internal/authorization"
	"mousehole/backend/internal/command"
	"mousehole/backend/internal/policy"
	"mousehole/backend/internal/validation"
)

var (
	// ErrInvalidRequest 表示请求在受理前未满足固定结构和范围合同。
	ErrInvalidRequest = errors.New("范围请求参数无效")
	// ErrForbidden 表示当前活动权限或角色不足。
	ErrForbidden = errors.New("项目权限不足")
	// ErrNotFound 统一隐藏跨项目及跨 actor 对象存在性。
	ErrNotFound = errors.New("暂未查询到对象")
	// ErrConflict 表示原命令 ID 的 principal、kind 或参数不同。
	ErrConflict = errors.New("命令参数冲突")
	// ErrRevisionConflict 表示当前策略已变化，不能覆盖。
	ErrRevisionConflict = errors.New("策略修订冲突")
	// ErrUnavailable 包装数据库或提交未确认的内部原因。
	ErrUnavailable = errors.New("范围服务暂不可用")
)

// Request 是固定 kind 与 payload 的命令包络；未知管理 kind 不开放给 Web。
type Request struct {
	Kind    string          `json:"kind"`
	Payload json.RawMessage `json:"payload"`
}

// Receipt 冻结首次受理时间和业务结果，project_id 为 nil 时表示全局管理命令。
type Receipt struct {
	CommandID  string          `json:"command_id"`
	ProjectID  *string         `json:"project_id"`
	Kind       string          `json:"kind"`
	State      string          `json:"state"`
	AcceptedAt string          `json:"accepted_at"`
	Result     json.RawMessage `json:"result"`
}

// ImportResult 的数量使用十进制字符串；资产仅返回当前项目关联。
type ImportResult struct {
	Type           string        `json:"type"`
	InputCount     string        `json:"input_count"`
	UniqueCount    string        `json:"unique_count"`
	DuplicateCount string        `json:"duplicate_count"`
	ExistingCount  string        `json:"existing_count"`
	Assets         []asset.Asset `json:"assets"`
}

// VersionResult 是新候选版本回执，不意味着版本已生效。
type VersionResult struct {
	Type    string                `json:"type"`
	Version authorization.Version `json:"version"`
}

// PrecheckResult 是纯本地历史决策，不含可执行 allow token。
type PrecheckResult struct {
	Type     string          `json:"type"`
	Decision policy.Decision `json:"decision"`
}

// PolicyChangeResult 记录管理变更后的 revision，不携带用户身份声明。
type PolicyChangeResult struct {
	Type            string `json:"type"`
	ObjectID        string `json:"object_id"`
	GlobalRevision  string `json:"global_revision"`
	ProjectRevision string `json:"project_revision"`
}

// AdminRequest 使用部署受控 CLI 的操作引用；引用不被当作 OIDC 身份。
type AdminRequest struct {
	Kind                   string                   `json:"kind"`
	VersionID              string                   `json:"version_id"`
	DenyID                 string                   `json:"deny_id"`
	ExpectedPolicyRevision string                   `json:"expected_policy_revision"`
	Deny                   *authorization.DenyInput `json:"deny"`
}

// Policy 是项目可见的当前指针与禁止快照，revision 字符串不会丢精度。
type Policy struct {
	ProjectID       string               `json:"project_id"`
	GlobalRevision  string               `json:"global_revision"`
	ProjectRevision string               `json:"project_revision"`
	ActiveVersionID *string              `json:"active_version_id"`
	Denies          []authorization.Deny `json:"denies"`
}

// ListOptions 只支持稳定 seek，搜索输入不进入 cursor 明文。
type ListOptions struct {
	PageSize int
	Cursor   string
	Kind     string
	Search   string
}

// Page 不提供未经计算的 total；next_cursor 为签名的筛选 hash 与定位。
type Page[T any] struct {
	Items      []T     `json:"items"`
	HasMore    bool    `json:"has_more"`
	NextCursor *string `json:"next_cursor"`
	PageCount  string  `json:"page_count"`
}

// DecodeJSON 拒绝重复/未知/大小写错键、缺字段、null 和尾随 JSON。
// 领域服务与 HTTP/CLI 共用，防止参数绑定在不同入口发生歧义。
func DecodeJSON(data []byte, destination any) error {
	if !utf8.Valid(data) {
		return ErrInvalidRequest
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := uniqueJSON(decoder); err != nil {
		return ErrInvalidRequest
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return ErrInvalidRequest
	}
	var raw any
	decoder = json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		return ErrInvalidRequest
	}
	typ := reflect.TypeOf(destination)
	if typ.Kind() != reflect.Pointer || shape(raw, typ.Elem()) != nil {
		return ErrInvalidRequest
	}
	decoder = json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return ErrInvalidRequest
	}
	return nil
}

func uniqueJSON(d *json.Decoder) error {
	token, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			t, e := d.Token()
			if e != nil {
				return e
			}
			k, ok := t.(string)
			if !ok || seen[k] {
				return ErrInvalidRequest
			}
			seen[k] = true
			if e = uniqueJSON(d); e != nil {
				return e
			}
		}
	case '[':
		for d.More() {
			if e := uniqueJSON(d); e != nil {
				return e
			}
		}
	default:
		return ErrInvalidRequest
	}
	_, err = d.Token()
	return err
}
func shape(raw any, typ reflect.Type) error {
	if typ == reflect.TypeFor[json.RawMessage]() {
		if raw == nil {
			return ErrInvalidRequest
		}
		return nil
	}
	if raw == nil {
		return ErrInvalidRequest
	}
	if typ.Kind() == reflect.Pointer {
		return shape(raw, typ.Elem())
	}
	switch typ.Kind() {
	case reflect.Struct:
		object, ok := raw.(map[string]any)
		if !ok {
			return ErrInvalidRequest
		}
		fields := map[string]reflect.StructField{}
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			tag := strings.Split(f.Tag.Get("json"), ",")[0]
			if tag != "" && tag != "-" {
				fields[tag] = f
				_, exists := object[tag]
				if !exists && !strings.Contains(f.Tag.Get("json"), "omitempty") {
					return ErrInvalidRequest
				}
			}
		}
		for key, value := range object {
			f, ok := fields[key]
			if !ok || shape(value, f.Type) != nil {
				return ErrInvalidRequest
			}
		}
	case reflect.Slice:
		values, ok := raw.([]any)
		if !ok {
			return ErrInvalidRequest
		}
		for _, v := range values {
			if shape(v, typ.Elem()) != nil {
				return ErrInvalidRequest
			}
		}
	}
	return nil
}

// NormalizeRequest 生成唯一规范 payload，hash 之外仍以规范 JSON 比较参数。
func NormalizeRequest(r Request) (Request, error) {
	var value any
	switch r.Kind {
	case "import_assets":
		var v asset.Import
		if err := DecodeJSON(r.Payload, &v); err != nil {
			return r, err
		}
		n, _, err := asset.Normalize(v)
		if err != nil {
			return r, fmt.Errorf("%w: %w", ErrInvalidRequest, validation.At("payload", err))
		}
		value = n
	case "create_authorization_version":
		var v authorization.Content
		if err := DecodeJSON(r.Payload, &v); err != nil {
			return r, err
		}
		n, err := authorization.NormalizeContent(v)
		if err != nil {
			return r, fmt.Errorf("%w: %w", ErrInvalidRequest, validation.At("payload", err))
		}
		value = n
	case "precheck_target":
		var v policy.Request
		if err := DecodeJSON(r.Payload, &v); err != nil {
			return r, err
		}
		if !command.ValidID(v.AuthorizationVersionID) {
			return r, ErrInvalidRequest
		}
		n, err := policy.Normalize(v)
		if err != nil {
			return r, fmt.Errorf("%w: %w", ErrInvalidRequest, validation.At("payload", err))
		}
		value = n
	default:
		return r, ErrInvalidRequest
	}
	r.Payload, _ = json.Marshal(value)
	return r, nil
}
