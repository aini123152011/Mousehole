// Command admin 提供受控身份管理及授权激活、撤销、禁止和原回执恢复入口。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mousehole/backend/internal/identity"
)

const (
	exitSuccess     = 0
	exitInternal    = 1
	exitUsage       = 2
	exitNotFound    = 3
	exitConflict    = 4
	exitUnavailable = 5
)

type commandOutcome struct {
	code    int
	message string
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	result := dispatch(args)
	if result.message != "" {
		output := stderr
		if result.code == exitSuccess {
			output = stdout
		}
		_, _ = fmt.Fprintln(output, result.message)
	}
	return result.code
}

func dispatch(args []string) commandOutcome {
	if len(args) == 0 {
		return usageOutcome("必须指定管理命令")
	}
	if len(args) == 1 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		return commandOutcome{code: exitSuccess, message: commandUsage("")}
	}
	switch args[0] {
	case "provision-user":
		return runProvision(args[1:])
	case "revoke-sessions":
		return runUserMutation("revoke-sessions", args[1:], identity.RevokeAllSessionsWithPool)
	case "disable-user":
		return runUserMutation("disable-user", args[1:], identity.DisableUserWithPool)
	case "revoke-membership":
		return runRevokeMembership(args[1:])
	case "scope":
		return runScope(args[1:], false)
	case "scope-command":
		return runScope(args[1:], true)
	default:
		return usageOutcome("未知管理命令")
	}
}

func runProvision(args []string) commandOutcome {
	flags := newFlagSet("provision-user")
	databaseURL := flags.String("database-url", "", "数据库连接串（必须显式提供）")
	issuer := flags.String("issuer", "", "精确 OIDC issuer")
	subject := flags.String("subject", "", "OIDC subject")
	userID := flags.String("user-id", "", "平台用户 UUID")
	displayName := flags.String("display-name", "", "用户显示名")
	email := flags.String("email", "", "仅作展示资料的邮箱")
	projectID := flags.String("project-id", "", "项目 UUID")
	projectName := flags.String("project-name", "", "项目名称")
	role := flags.String("role", "", "viewer 或 analyst")
	if result, stop := parseFlags(flags, args); stop {
		return result
	}
	if *databaseURL == "" || *issuer == "" || *subject == "" || *userID == "" || *displayName == "" || *projectID == "" || *projectName == "" || *role == "" {
		return commandOutcome{code: exitUsage, message: "身份预置必须显式提供数据库、issuer、subject、用户、项目和角色参数。"}
	}
	if !identity.ValidID(*userID) || !identity.ValidID(*projectID) {
		return commandOutcome{code: exitUsage, message: "用户和项目 ID 必须是标准小写 UUID。"}
	}
	return withDatabase(*databaseURL, func(ctx context.Context, pool *pgxpool.Pool) commandOutcome {
		err := identity.ProvisionWithPool(ctx, pool, identity.ProvisionRequest{
			Issuer: *issuer, Subject: *subject, UserID: *userID, DisplayName: *displayName, Email: *email,
			ProjectID: *projectID, ProjectName: *projectName, Role: *role,
		})
		switch {
		case errors.Is(err, identity.ErrInvalidRequest):
			return commandOutcome{code: exitUsage, message: "身份预置参数无效。"}
		case errors.Is(err, identity.ErrConflict):
			return commandOutcome{code: exitConflict, message: "身份预置参数与既有身份或项目冲突。"}
		case errors.Is(err, identity.ErrUnavailable):
			return commandOutcome{code: exitUnavailable, message: "身份预置数据库操作未能确认。"}
		case err != nil:
			return commandOutcome{code: exitInternal, message: "身份预置失败。"}
		default:
			return commandOutcome{code: exitSuccess, message: "身份与项目成员已预置。"}
		}
	})
}

type userMutation func(context.Context, *pgxpool.Pool, string) (identity.AdminMutationResult, error)

func runUserMutation(name string, args []string, mutate userMutation) commandOutcome {
	flags := newFlagSet(name)
	databaseURL := flags.String("database-url", "", "数据库连接串（必须显式提供）")
	userID := flags.String("user-id", "", "平台用户 UUID")
	if result, stop := parseFlags(flags, args); stop {
		return result
	}
	if *databaseURL == "" || *userID == "" {
		return commandOutcome{code: exitUsage, message: "管理命令必须显式提供数据库和用户 ID。"}
	}
	if !identity.ValidID(*userID) {
		return commandOutcome{code: exitUsage, message: "用户 ID 必须是标准小写 UUID。"}
	}
	return withDatabase(*databaseURL, func(ctx context.Context, pool *pgxpool.Pool) commandOutcome {
		result, err := mutate(ctx, pool, *userID)
		if err != nil {
			return mutationError(err)
		}
		switch name {
		case "revoke-sessions":
			if result.Changed {
				return commandOutcome{code: exitSuccess, message: fmt.Sprintf("已撤销用户会话（%d 个）。", result.RevokedSessions)}
			}
			return commandOutcome{code: exitSuccess, message: "用户会话已处于撤销状态。"}
		case "disable-user":
			if result.Changed {
				return commandOutcome{code: exitSuccess, message: fmt.Sprintf("用户已禁用，并撤销会话（%d 个）。", result.RevokedSessions)}
			}
			return commandOutcome{code: exitSuccess, message: "用户已处于禁用状态。"}
		default:
			return commandOutcome{code: exitInternal, message: "管理命令失败。"}
		}
	})
}

func runRevokeMembership(args []string) commandOutcome {
	flags := newFlagSet("revoke-membership")
	databaseURL := flags.String("database-url", "", "数据库连接串（必须显式提供）")
	userID := flags.String("user-id", "", "平台用户 UUID")
	projectID := flags.String("project-id", "", "项目 UUID")
	if result, stop := parseFlags(flags, args); stop {
		return result
	}
	if *databaseURL == "" || *userID == "" || *projectID == "" {
		return commandOutcome{code: exitUsage, message: "成员撤销必须显式提供数据库、用户 ID 和项目 ID。"}
	}
	if !identity.ValidID(*userID) || !identity.ValidID(*projectID) {
		return commandOutcome{code: exitUsage, message: "用户和项目 ID 必须是标准小写 UUID。"}
	}
	return withDatabase(*databaseURL, func(ctx context.Context, pool *pgxpool.Pool) commandOutcome {
		result, err := identity.RevokeMembershipWithPool(ctx, pool, *userID, *projectID)
		if err != nil {
			return mutationError(err)
		}
		if result.Changed {
			return commandOutcome{code: exitSuccess, message: "项目成员权限已撤销。"}
		}
		return commandOutcome{code: exitSuccess, message: "项目成员权限已处于撤销状态。"}
	})
}

func newFlagSet(name string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	return flags
}

func parseFlags(flags *flag.FlagSet, args []string) (commandOutcome, bool) {
	if err := flags.Parse(args); errors.Is(err, flag.ErrHelp) {
		return commandOutcome{code: exitSuccess, message: commandUsage(flags.Name())}, true
	} else if err != nil {
		return commandOutcome{code: exitUsage, message: "管理命令参数解析失败。"}, true
	}
	if flags.NArg() != 0 {
		return commandOutcome{code: exitUsage, message: "管理命令不接受位置参数。"}, true
	}
	return commandOutcome{}, false
}

func withDatabase(databaseURL string, action func(context.Context, *pgxpool.Pool) commandOutcome) commandOutcome {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return commandOutcome{code: exitUsage, message: "数据库连接参数无效。"}
	}
	defer pool.Close()
	if err = pool.Ping(ctx); err != nil {
		return commandOutcome{code: exitUnavailable, message: "无法连接数据库。"}
	}
	return action(ctx, pool)
}

func mutationError(err error) commandOutcome {
	switch {
	case errors.Is(err, identity.ErrInvalidRequest):
		return commandOutcome{code: exitUsage, message: "管理命令参数无效。"}
	case errors.Is(err, identity.ErrAdminTargetNotFound):
		return commandOutcome{code: exitNotFound, message: "指定用户或项目成员关系不存在。"}
	case errors.Is(err, identity.ErrConflict):
		return commandOutcome{code: exitConflict, message: "管理命令与既有稳定绑定冲突。"}
	case errors.Is(err, identity.ErrUnavailable):
		return commandOutcome{code: exitUnavailable, message: "管理数据库操作未能确认。"}
	default:
		return commandOutcome{code: exitInternal, message: "管理命令失败。"}
	}
}

func usageOutcome(message string) commandOutcome {
	return commandOutcome{code: exitUsage, message: message + "\n" + commandUsage("")}
}

func commandUsage(name string) string {
	switch name {
	case "provision-user":
		return "用法：mousehole-admin provision-user --database-url <DSN> --issuer <URL> --subject <sub> --user-id <UUID> --display-name <名称> --project-id <UUID> --project-name <名称> --role viewer|analyst"
	case "revoke-sessions":
		return "用法：mousehole-admin revoke-sessions --database-url <DSN> --user-id <UUID>"
	case "disable-user":
		return "用法：mousehole-admin disable-user --database-url <DSN> --user-id <UUID>"
	case "revoke-membership":
		return "用法：mousehole-admin revoke-membership --database-url <DSN> --user-id <UUID> --project-id <UUID>"
	default:
		return "可用命令：provision-user、revoke-sessions、disable-user、revoke-membership、scope activate|revoke|deny-add|deny-revoke、scope-command get"
	}
}
