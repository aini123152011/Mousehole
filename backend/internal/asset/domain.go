package asset

import (
	"fmt"
	"mousehole/backend/internal/target"
	"mousehole/backend/internal/validation"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Input 是明确类型的资产输入，value 最长 2048 字节。
type Input struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// Import 是整批导入请求；来源仅为人工声明，不能当作真实性证明。
type Import struct {
	Entries     []Input `json:"entries"`
	Source      string  `json:"source"`
	Notes       string  `json:"notes"`
	ExternalRef string  `json:"external_ref"`
}

// Source 是本项目内的一次来源声明，不含其他项目的导入信息。
type Source struct {
	CommandID   string `json:"command_id"`
	Source      string `json:"source"`
	Notes       string `json:"notes"`
	ExternalRef string `json:"external_ref"`
	CreatedAt   string `json:"created_at"`
}

// Asset 是仅由项目关联入口公开的规范资产。
type Asset struct {
	ID               string   `json:"id"`
	ProjectID        string   `json:"project_id"`
	Kind             string   `json:"kind"`
	CanonicalValue   string   `json:"canonical_value"`
	AlgorithmVersion string   `json:"algorithm_version"`
	CreatedAt        string   `json:"created_at"`
	Sources          []Source `json:"sources"`
}

// PreviewRow 保留行号及中文错误，不返回未受限原始行。
type PreviewRow struct {
	Row            int    `json:"row"`
	Kind           string `json:"kind"`
	CanonicalValue string `json:"canonical_value"`
	Error          string `json:"error"`
}

// Preview 是纯规范化预览；existing_count 仅按当前项目统计。
type Preview struct {
	Rows           []PreviewRow `json:"rows"`
	Valid          bool         `json:"valid"`
	InputCount     string       `json:"input_count"`
	UniqueCount    string       `json:"unique_count"`
	DuplicateCount string       `json:"duplicate_count"`
	ExistingCount  string       `json:"existing_count"`
}

// Text 验证受限人工说明；禁止控制字符，不替调用方记录原输入。
func Text(value string, min, max int) (string, error) {
	// 先检查原文本，避免 TrimSpace 将首尾控制字符静默变为可接受参数。
	if !utf8.ValidString(value) || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return "", validation.New("", "文本不能含控制字符。")
	}
	value = strings.TrimSpace(value)
	n := utf8.RuneCountInString(value)
	if !utf8.ValidString(value) || n < min || n > max || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return "", validation.New("", fmt.Sprintf("文本长度须为 %d–%d 字且不能含控制字符。", min, max))
	}
	return value, nil
}

// Normalize 保留规范化重复条目用于幂等参数绑定；业务写入用 Unique 去重。
func Normalize(request Import) (Import, Preview, error) {
	if len(request.Entries) < 1 || len(request.Entries) > 100 || request.Source != "manual" && request.Source != "api_import" {
		return Import{}, Preview{}, validation.New("entries", "导入须为 1–100 条且来源须为 manual 或 api_import。")
	}
	var err error
	request.Notes, err = Text(request.Notes, 0, 500)
	if err != nil {
		return Import{}, Preview{}, validation.At("notes", err)
	}
	request.ExternalRef, err = Text(request.ExternalRef, 0, 500)
	if err != nil {
		return Import{}, Preview{}, validation.At("external_ref", err)
	}
	issues := []validation.Issue{}
	result := Preview{Rows: []PreviewRow{}, Valid: true, InputCount: fmt.Sprint(len(request.Entries)), ExistingCount: "0"}
	normalized := make([]Input, 0, len(request.Entries))
	seen := map[Input]bool{}
	for i, entry := range request.Entries {
		row := PreviewRow{Row: i + 1, Kind: entry.Kind}
		if entry.Kind != "domain" && entry.Kind != "ip" && entry.Kind != "url" {
			err = validation.New("", "资产只支持 domain、ip 和 url。")
		} else {
			row.CanonicalValue, err = target.Normalize(entry.Kind, entry.Value)
		}
		if err != nil {
			details := validation.Details(validation.At(fmt.Sprintf("entries.%d.value", i), err))
			issues = append(issues, details...)
			row.Error = details[0].Message
			result.Valid = false
		} else {
			entry.Value = row.CanonicalValue
			normalized = append(normalized, entry)
			seen[entry] = true
		}
		result.Rows = append(result.Rows, row)
	}
	result.UniqueCount = fmt.Sprint(len(seen))
	result.DuplicateCount = fmt.Sprint(len(normalized) - len(seen))
	request.Entries = normalized
	if !result.Valid {
		return request, result, &validation.Error{Issues: issues}
	}
	return request, result, nil
}

// Unique 规范条目按自然键稳定排序，避免并发 upsert 锁反转。
func Unique(entries []Input) []Input {
	seen := map[Input]bool{}
	for _, e := range entries {
		seen[e] = true
	}
	out := make([]Input, 0, len(seen))
	for e := range seen {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind == out[j].Kind {
			return out[i].Value < out[j].Value
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}
