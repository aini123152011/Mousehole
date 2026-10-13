package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strconv"

	"mousehole/backend/internal/authorization"
	"mousehole/backend/internal/command"
	"mousehole/backend/internal/scopecommand"

	"github.com/jackc/pgx/v5/pgxpool"
)

func runScope(args []string, lookup bool) commandOutcome {
	usage := "用法：scope activate|revoke|deny-add|deny-revoke --database-url <DSN> --project-id <UUID>（全局 deny 改用 --global）--command-id <UUID> --operator-ref <操作引用> --expected-policy-revision <修订>；激活/撤销需 --version-id，新增禁止需 --payload-file，撤销禁止需 --deny-id。查询：scope-command get 使用相同数据库、项目或 --global、命令 ID 和操作引用。成功 stdout 为单个冻结回执 JSON。"
	if len(args) == 0 {
		return commandOutcome{code: exitUsage, message: usage}
	}
	action := args[0]
	if action == "help" || action == "--help" || action == "-h" {
		return commandOutcome{code: exitSuccess, message: usage}
	}
	flags := newFlagSet("scope " + action)
	databaseURL := flags.String("database-url", "", "显式数据库连接串")
	project := flags.String("project-id", "", "项目 UUID")
	global := flags.Bool("global", false, "全企业禁止命名空间")
	id := flags.String("command-id", "", "原命令 UUID")
	operator := flags.String("operator-ref", "", "受控管理操作引用，不作为已认证用户身份")
	expected := flags.String("expected-policy-revision", "", "当前策略十进制修订")
	version := flags.String("version-id", "", "授权版本 UUID")
	denyID := flags.String("deny-id", "", "禁止规则 UUID")
	file := flags.String("payload-file", "", "禁止正文 JSON 文件")
	if result, stop := parseFlags(flags, args[1:]); stop {
		if result.code == exitSuccess {
			result.message = usage
		}
		return result
	}
	if *databaseURL == "" || !command.ValidID(*id) || *operator == "" || (*project == "") == !*global || *project != "" && !command.ValidID(*project) {
		return commandOutcome{code: exitUsage, message: "必须显式提供数据库、命令 ID、操作引用以及唯一的项目或全局范围。"}
	}
	var request scopecommand.AdminRequest
	if lookup {
		if action != "get" || *expected != "" || *version != "" || *denyID != "" || *file != "" {
			return commandOutcome{code: exitUsage, message: usage}
		}
	} else {
		number, e := strconv.ParseInt(*expected, 10, 64)
		if e != nil || number < 1 || strconv.FormatInt(number, 10) != *expected {
			return commandOutcome{code: exitUsage, message: "必须提供有效的十进制策略修订。"}
		}
		request.ExpectedPolicyRevision = *expected
		switch action {
		case "activate", "revoke":
			if *global || !command.ValidID(*version) || *file != "" || *denyID != "" {
				return commandOutcome{code: exitUsage, message: usage}
			}
			request.Kind = "activate_authorization_version"
			if action == "revoke" {
				request.Kind = "revoke_authorization_version"
			}
			request.VersionID = *version
		case "deny-add":
			if *file == "" || *version != "" || *denyID != "" {
				return commandOutcome{code: exitUsage, message: usage}
			}
			f, e := os.Open(*file)
			if e != nil {
				return commandOutcome{code: exitUsage, message: "无法读取禁止正文文件。"}
			}
			body, e := io.ReadAll(io.LimitReader(f, 256*1024+1))
			_ = f.Close()
			if e != nil || len(body) > 256*1024 {
				return commandOutcome{code: exitUsage, message: "禁止正文文件无效或超过 256 KiB。"}
			}
			var deny authorization.DenyInput
			if e = scopecommand.DecodeJSON(body, &deny); e != nil {
				return commandOutcome{code: exitUsage, message: "禁止正文须只包含 kind、value、include_subdomains、reason。"}
			}
			request.Kind = "add_scope_deny"
			request.Deny = &deny
		case "deny-revoke":
			if !command.ValidID(*denyID) || *version != "" || *file != "" {
				return commandOutcome{code: exitUsage, message: usage}
			}
			request.Kind = "revoke_scope_deny"
			request.DenyID = *denyID
		default:
			return commandOutcome{code: exitUsage, message: usage}
		}
	}
	return withDatabase(*databaseURL, func(ctx context.Context, pool *pgxpool.Pool) commandOutcome {
		s := scopecommand.NewService(pool, nil)
		var receipt scopecommand.Receipt
		var e error
		if lookup {
			receipt, e = s.AdminGet(ctx, *project, *id, *operator)
		} else {
			receipt, _, e = s.AdminAccept(ctx, *project, *id, *operator, request)
		}
		if e != nil {
			return scopeMutationError(e)
		}
		encoded, e := json.Marshal(receipt)
		if e != nil {
			return commandOutcome{code: exitInternal, message: "管理回执编码失败。"}
		}
		return commandOutcome{code: exitSuccess, message: string(encoded)}
	})
}

func scopeMutationError(e error) commandOutcome {
	switch {
	case errors.Is(e, scopecommand.ErrInvalidRequest):
		return commandOutcome{code: exitUsage, message: "范围管理参数、状态或有效时窗无效。"}
	case errors.Is(e, scopecommand.ErrNotFound), errors.Is(e, scopecommand.ErrForbidden):
		return commandOutcome{code: exitNotFound, message: "暂未查询到本范围对象或原管理回执。"}
	case errors.Is(e, scopecommand.ErrConflict), errors.Is(e, scopecommand.ErrRevisionConflict):
		return commandOutcome{code: exitConflict, message: "命令参数或策略修订冲突，请查询原回执和当前修订。"}
	case errors.Is(e, scopecommand.ErrUnavailable):
		return commandOutcome{code: exitUnavailable, message: "管理结果尚未确认，请保留原命令 ID 查询。"}
	default:
		return commandOutcome{code: exitInternal, message: "范围管理内部失败，请保留原命令 ID 查询。"}
	}
}
