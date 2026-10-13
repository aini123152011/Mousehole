package authorization

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"mousehole/backend/internal/asset"
	"mousehole/backend/internal/target"
	"mousehole/backend/internal/validation"
)

// Entry 是一个完整范围；不得跨条目拼接 operation、端口或解析约束。
type Entry struct {
	Kind              string   `json:"kind"`
	Value             string   `json:"value"`
	IncludeSubdomains bool     `json:"include_subdomains"`
	Operations        []string `json:"operations"`
	Ports             []int32  `json:"ports"`
	ResolvedIPCIDRs   []string `json:"resolved_ip_cidrs"`
}

// Budget 的单位为请求总数、并发数及每秒请求数；本切片只保存上限，不消费预算。
type Budget struct {
	MaxRequests          string `json:"max_requests"`
	MaxConcurrency       string `json:"max_concurrency"`
	MaxRequestsPerSecond string `json:"max_requests_per_second"`
}

// Content 是保存后禁止编辑的规范内容；时间 UTC，有效区间左闭右开。
type Content struct {
	Title             string  `json:"title"`
	BasisRef          string  `json:"basis_ref"`
	Contact           string  `json:"contact"`
	ValidFrom         string  `json:"valid_from"`
	ExpiresAt         string  `json:"expires_at"`
	AlgorithmVersion  string  `json:"algorithm_version"`
	PolicyVersion     string  `json:"policy_version"`
	CatalogVersion    string  `json:"catalog_version"`
	MaxOperationLevel string  `json:"max_operation_level"`
	Budget            Budget  `json:"budget"`
	RedirectPolicy    string  `json:"redirect_policy"`
	CredentialPolicy  string  `json:"credential_policy"`
	Entries           []Entry `json:"entries"`
}

// Version 的 EntryIDs 与规范内容的 Entries 同序，不能当作权限凭证。
type Version struct {
	ID           string   `json:"id"`
	ProjectID    string   `json:"project_id"`
	Sequence     string   `json:"sequence"`
	ContentHash  string   `json:"content_hash"`
	Content      Content  `json:"content"`
	EntryIDs     []string `json:"entry_ids"`
	State        string   `json:"state"`
	DisplayState string   `json:"display_state"`
	CreatedAt    string   `json:"created_at"`
}

// Deny 是当前可撤销禁止规则；nil 项目代表全企业，正文修改需新规则。
type Deny struct {
	ID                string  `json:"id"`
	ProjectID         *string `json:"project_id"`
	Kind              string  `json:"kind"`
	Value             string  `json:"value"`
	IncludeSubdomains bool    `json:"include_subdomains"`
	Reason            string  `json:"reason"`
	Active            bool    `json:"active"`
	CreatedAt         string  `json:"created_at"`
}

// DenyInput 是管理员提交的禁止正文，不接受身份或状态字段。
type DenyInput struct {
	Kind              string `json:"kind"`
	Value             string `json:"value"`
	IncludeSubdomains bool   `json:"include_subdomains"`
	Reason            string `json:"reason"`
}

// NormalizeDeny 与授权共用规范化，但 deny 不包含可放宽内置保留地址的语义。
func NormalizeDeny(d DenyInput) (DenyInput, error) {
	v, e := target.Normalize(d.Kind, d.Value)
	if e != nil {
		return d, e
	}
	if d.Kind == "url" {
		return d, fmt.Errorf("禁止 URL 使用 url_exact 类型")
	}
	d.Value = v
	if d.IncludeSubdomains && (d.Kind != "domain" || !target.SubdomainsAllowed(v)) {
		return d, fmt.Errorf("泛子域范围不能使用裸公共后缀")
	}
	d.Reason, e = asset.Text(d.Reason, 1, 500)
	return d, e
}

// NormalizeContent 排序集合并绑定全部语义字段；失败不产生 candidate。
func NormalizeContent(c Content) (Content, error) {
	var err error
	for _, field := range []struct {
		p    *string
		max  int
		name string
	}{{&c.Title, 120, "title"}, {&c.BasisRef, 500, "basis_ref"}, {&c.Contact, 200, "contact"}} {
		*field.p, err = asset.Text(*field.p, 1, field.max)
		if err != nil {
			return c, validation.At(field.name, err)
		}
	}
	from, e := time.Parse(time.RFC3339Nano, c.ValidFrom)
	until, ue := time.Parse(time.RFC3339Nano, c.ExpiresAt)
	if e != nil {
		return c, validation.New("valid_from", "开始时间须为 RFC3339 格式。")
	}
	if ue != nil || !from.Before(until) {
		return c, validation.New("expires_at", "结束时间须为 RFC3339 且晚于开始时间。")
	}
	c.ValidFrom = from.UTC().Format(time.RFC3339Nano)
	c.ExpiresAt = until.UTC().Format(time.RFC3339Nano)
	for _, rule := range []struct{ field, value, expected string }{{"algorithm_version", c.AlgorithmVersion, target.AlgorithmVersion}, {"policy_version", c.PolicyVersion, "policy-v1"}, {"catalog_version", c.CatalogVersion, "precheck-v1"}, {"max_operation_level", c.MaxOperationLevel, "non_destructive"}, {"redirect_policy", c.RedirectPolicy, "deny"}, {"credential_policy", c.CredentialPolicy, "none"}} {
		if rule.value != rule.expected {
			return c, validation.New(rule.field, "规则版本、等级或执行约束须使用当前固定目录。")
		}
	}
	counts := []string{c.Budget.MaxRequests, c.Budget.MaxConcurrency, c.Budget.MaxRequestsPerSecond}
	numbers := make([]int64, 3)
	for i, v := range counts {
		numbers[i], err = strconv.ParseInt(v, 10, 64)
		if err != nil || numbers[i] < 1 || strconv.FormatInt(numbers[i], 10) != v {
			return c, validation.New("budget."+[]string{"max_requests", "max_concurrency", "max_requests_per_second"}[i], "预算须为不超过 BIGINT 上限的正整数字符串。")
		}
	}
	if numbers[1] > numbers[0] {
		return c, validation.New("budget.max_concurrency", "并发上限不能大于请求总量。")
	}
	if len(c.Entries) < 1 || len(c.Entries) > 100 {
		return c, validation.New("entries", "授权范围须为 1–100 条。")
	}
	entries := map[string]Entry{}
	for entryIndex, entry := range c.Entries {
		field := fmt.Sprintf("entries.%d", entryIndex)
		if entry.Kind != "domain" && entry.Kind != "ip" && entry.Kind != "cidr" && entry.Kind != "url_exact" {
			return c, validation.New(field+".kind", "范围类型须为 domain、ip、cidr 或 url_exact。")
		}
		entry.Value, err = target.Normalize(entry.Kind, entry.Value)
		if err != nil {
			return c, validation.At(field+".value", err)
		}
		if entry.IncludeSubdomains && (entry.Kind != "domain" || !target.SubdomainsAllowed(entry.Value)) {
			return c, validation.New(field+".include_subdomains", "泛子域授权不能使用裸公共后缀。")
		}
		if len(entry.Operations) < 1 || len(entry.Operations) > 3 || len(entry.Ports) > 32 || len(entry.ResolvedIPCIDRs) > 32 {
			return c, validation.New(field, "范围操作、端口或解析 CIDR 数量无效。")
		}
		entry.Operations = uniqueStrings(entry.Operations)
		for i, cidr := range entry.ResolvedIPCIDRs {
			entry.ResolvedIPCIDRs[i], err = target.Normalize("cidr", cidr)
			if err != nil {
				return c, validation.At(fmt.Sprintf("%s.resolved_ip_cidrs.%d", field, i), err)
			}
		}
		entry.ResolvedIPCIDRs = uniqueStrings(entry.ResolvedIPCIDRs)
		seen := map[int32]bool{}
		for portIndex, port := range entry.Ports {
			if port < 1 || port > 65535 {
				return c, validation.New(fmt.Sprintf("%s.ports.%d", field, portIndex), "端口须为 1–65535。")
			}
			seen[port] = true
		}
		entry.Ports = []int32{}
		for port := range seen {
			entry.Ports = append(entry.Ports, port)
		}
		sort.Slice(entry.Ports, func(i, j int) bool { return entry.Ports[i] < entry.Ports[j] })
		for _, operation := range entry.Operations {
			switch operation {
			case "dns_lookup":
				if entry.Kind != "domain" {
					return c, validation.New(field+".operations", "DNS 预检查须为域名范围。")
				}
			case "tcp_connect":
				if entry.Kind == "url_exact" || len(entry.Ports) == 0 || entry.Kind == "domain" && len(entry.ResolvedIPCIDRs) == 0 {
					return c, validation.New(field+".operations", "TCP 范围须有显式端口，域名另须解析 CIDR。")
				}
			case "http_metadata":
				if entry.Kind != "url_exact" {
					return c, validation.New(field+".operations", "HTTP 预检查只支持精确 URL 范围。")
				}
				kind, _, port := target.URLHost(entry.Value)
				if !HasPort(entry.Ports, port) || kind == "domain" && len(entry.ResolvedIPCIDRs) == 0 {
					return c, validation.New(field+".operations", "HTTP 范围须声明 URL 端口与域名解析 CIDR。")
				}
			default:
				return c, validation.New(field+".operations", "未知预检查操作。")
			}
		}
		encoded, _ := json.Marshal(entry)
		entries[string(encoded)] = entry
	}
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	c.Entries = []Entry{}
	for _, key := range keys {
		c.Entries = append(c.Entries, entries[key])
	}
	return c, nil
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	for _, v := range values {
		seen[v] = true
	}
	out := make([]string, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// Hash 固定结构顺序编码规范语义内容，不含数据库 ID 或可变状态。
func Hash(value any) string {
	encoded, _ := json.Marshal(value)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

// HasPort 判断显式端口集合，不提供任意端口缺省。
func HasPort(ports []int32, port int) bool {
	for _, v := range ports {
		if int(v) == port {
			return true
		}
	}
	return false
}

// DisplayState 在读取时计算时窗状态，不靠后台改写不可变内容。
func DisplayState(state string, c Content, now time.Time) string {
	if state == "revoked" || state == "superseded" {
		return state
	}
	from, _ := time.Parse(time.RFC3339Nano, c.ValidFrom)
	until, _ := time.Parse(time.RFC3339Nano, c.ExpiresAt)
	if !now.Before(until) {
		return "expired"
	}
	if now.Before(from) {
		return "not_yet_valid"
	}
	return state
}
