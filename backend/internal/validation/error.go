package validation

import "errors"

// Issue 指向已知输入字段；Message 仅由校验代码构造，不能直接复制外部错误。
type Issue struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Error 保留受控字段说明，由命令层另行包装稳定的 invalid_request 类别。
type Error struct{ Issues []Issue }

// Error 返回第一条受控说明供本地诊断，HTTP 仍仅通过 Details 提取明确字段。
func (e *Error) Error() string {
	if len(e.Issues) > 0 {
		return e.Issues[0].Message
	}
	return "输入字段未满足校验规则"
}

// New 使用开发时确定的字段路径和中文说明构造输入错误。
func New(field, message string) error {
	return &Error{Issues: []Issue{{Field: field, Message: message}}}
}

// Details 仅提取受控类型，任意普通错误不会成为公开文案。
func Details(err error) []Issue {
	var v *Error
	if !errors.As(err, &v) {
		return nil
	}
	return append([]Issue(nil), v.Issues...)
}

// At 给受控错误添加已知字段路径；未分类错误只使用固定兜底说明。
func At(prefix string, err error) error {
	issues := Details(err)
	if len(issues) == 0 {
		return New(prefix, "该字段未满足输入约束，请检查格式和取值。")
	}
	for i := range issues {
		if issues[i].Field == "" {
			issues[i].Field = prefix
		} else if prefix != "" {
			issues[i].Field = prefix + "." + issues[i].Field
		}
	}
	return &Error{Issues: issues}
}
