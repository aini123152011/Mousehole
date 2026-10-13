package policy

import (
	"mousehole/backend/internal/authorization"
	"mousehole/backend/internal/target"
	"mousehole/backend/internal/validation"
	"slices"
	"time"
)

// Parameters 只有 TCP 操作可携带显式端口；目录决定参数而非用户声明等级。
type Parameters struct {
	Port *int `json:"port,omitempty"`
}

// Request 是静态目录输入，不要求目标先成为资产。
type Request struct {
	AuthorizationVersionID string     `json:"authorization_version_id"`
	Operation              string     `json:"operation"`
	Target                 string     `json:"target"`
	Parameters             Parameters `json:"parameters"`
}

// RuntimeCheck 描述后续执行仍需验证的条件；required 不代表本切片已执行。
type RuntimeCheck struct {
	Code    string `json:"code"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// Decision 冻结检查时刻和 revision，不能当作查询时仍有效的许可。
type Decision struct {
	ID                     string         `json:"id"`
	ProjectID              string         `json:"project_id"`
	CommandID              string         `json:"command_id"`
	AuthorizationVersionID string         `json:"authorization_version_id"`
	Operation              string         `json:"operation"`
	CanonicalTarget        string         `json:"canonical_target"`
	Parameters             Parameters     `json:"parameters"`
	Outcome                string         `json:"outcome"`
	ReasonCodes            []string       `json:"reason_codes"`
	Message                string         `json:"message"`
	MatchedEntryID         *string        `json:"matched_entry_id"`
	CheckedAt              string         `json:"checked_at"`
	ContentHash            string         `json:"content_hash"`
	GlobalRevision         string         `json:"global_revision"`
	ProjectRevision        string         `json:"project_revision"`
	AlgorithmVersion       string         `json:"algorithm_version"`
	PolicyVersion          string         `json:"policy_version"`
	CatalogVersion         string         `json:"catalog_version"`
	RuntimeChecks          []RuntimeCheck `json:"runtime_checks"`
}

// Normalize 验证固定目录及参数；未知操作和额外参数在受理前拒绝。
func Normalize(r Request) (Request, error) {
	kind := ""
	switch r.Operation {
	case "dns_lookup":
		kind = "domain"
		if r.Parameters.Port != nil {
			return r, validation.New("parameters.port", "DNS 操作不接受端口参数。")
		}
	case "tcp_connect":
		kind = "domain"
		if _, err := target.Normalize("ip", r.Target); err == nil {
			kind = "ip"
		}
		if r.Parameters.Port == nil || *r.Parameters.Port < 1 || *r.Parameters.Port > 65535 {
			return r, validation.New("parameters.port", "TCP 须有显式合法端口。")
		}
	case "http_metadata":
		kind = "url"
		if r.Parameters.Port != nil {
			return r, validation.New("parameters.port", "HTTP 端口由 URL 确定，不接受额外参数。")
		}
	default:
		return r, validation.New("operation", "未知静态预检查操作。")
	}
	v, err := target.Normalize(kind, r.Target)
	r.Target = v
	if err != nil {
		return r, validation.At("target", err)
	}
	return r, nil
}

// Evaluate 只使用已持锁的数据库快照与一次权威时刻，不消耗预算。
func Evaluate(r Request, v authorization.Version, active *string, denies []authorization.Deny, now time.Time) Decision {
	d := Decision{AuthorizationVersionID: v.ID, Operation: r.Operation, CanonicalTarget: r.Target, Parameters: r.Parameters, Outcome: "deny", ReasonCodes: []string{}, CheckedAt: now.UTC().Format(time.RFC3339Nano), ContentHash: v.ContentHash, AlgorithmVersion: target.AlgorithmVersion, PolicyVersion: "policy-v1", CatalogVersion: "precheck-v1", RuntimeChecks: runtimeChecks(r)}
	reject := func(code, message string) Decision { d.ReasonCodes = []string{code}; d.Message = message; return d }
	kind := "domain"
	port := 0
	hostKind, host := "", ""
	if r.Operation == "tcp_connect" {
		port = *r.Parameters.Port
		if _, err := target.IP(r.Target); err == nil {
			kind = "ip"
		}
	}
	if r.Operation == "http_metadata" {
		kind = "url"
		hostKind, host, port = target.URLHost(r.Target)
	}
	if target.Reserved(kind, r.Target) {
		return reject("platform_reserved", "目标属于平台保留或元数据地址")
	}
	for _, deny := range denies {
		if deny.Active && (target.Matches(deny.Kind, deny.Value, deny.IncludeSubdomains, kind, r.Target) || kind == "url" && target.Matches(deny.Kind, deny.Value, deny.IncludeSubdomains, hostKind, host)) {
			return reject("scope_denied", "命中当前紧急禁止规则："+deny.ID)
		}
	}
	if v.State != "active" || active == nil || *active != v.ID {
		return reject("version_"+v.State, "该版本不是当前生效版本")
	}
	display := authorization.DisplayState(v.State, v.Content, now)
	if display == "expired" {
		return reject("version_expired", "授权已到期")
	}
	if display == "not_yet_valid" {
		return reject("version_not_yet_valid", "授权尚未进入有效时窗")
	}
	for i, entry := range v.Content.Entries {
		if !target.Matches(entry.Kind, entry.Value, entry.IncludeSubdomains, kind, r.Target) || !slices.Contains(entry.Operations, r.Operation) {
			continue
		}
		if port != 0 && !authorization.HasPort(entry.Ports, port) {
			continue
		}
		if (kind == "domain" && r.Operation == "tcp_connect" || kind == "url" && hostKind == "domain") && len(entry.ResolvedIPCIDRs) == 0 {
			continue
		}
		matched := v.EntryIDs[i]
		d.MatchedEntryID = &matched
		d.Outcome = "requires_runtime_checks"
		d.ReasonCodes = []string{"static_scope_matched"}
		d.Message = "静态规则匹配，执行条件尚未验证"
		return d
	}
	return reject("no_complete_scope_match", "没有条目同时匹配目标、操作、端口与解析约束")
}

func runtimeChecks(r Request) []RuntimeCheck {
	checks := []RuntimeCheck{{"dns_cname_chain", "required", "实际 DNS/CNAME 链须重新检查"}, {"final_ip_and_denies", "required", "每个最终 IPv4/IPv6、解析 CIDR 与当前禁止须检查"}, {"connection_redirect_policy", "required", "真实连接目标、重定向禁止与凭据禁止须检查"}, {"trusted_adapter_semantics", "required", "可信适配器须证明非破坏性语义"}, {"current_authorization_membership", "required", "执行时重验当前授权、时窗与成员权限"}, {"lease_fencing_ttl", "required", "lease、fencing 与 TTL 须由执行层校验"}, {"budget_rate_window", "required", "实际速率、剩余预算和有效时窗须检查"}, {"execution_audit_available", "required", "执行审计与证据写入须可用"}}
	if r.Operation == "tcp_connect" {
		if _, err := target.IP(r.Target); err == nil {
			checks[0].Status = "not_applicable"
		}
	}
	if r.Operation == "http_metadata" {
		kind, _, _ := target.URLHost(r.Target)
		if kind == "ip" {
			checks[0].Status = "not_applicable"
		}
	}
	if r.Operation == "dns_lookup" {
		checks[2].Status = "not_applicable"
	}
	return checks
}
