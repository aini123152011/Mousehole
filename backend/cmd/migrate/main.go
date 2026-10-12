// Command migrate 显式执行编号迁移；API 启动永远不代替此步骤。
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"mousehole/backend/db/migrations"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func run() error {
	if os.Getenv("MOUSEHOLE_MODE") != "development" {
		return fmt.Errorf("迁移必须显式设置 MOUSEHOLE_MODE=development")
	}
	if len(os.Args) != 2 || (os.Args[1] != "up" && os.Args[1] != "status") {
		return fmt.Errorf("用法：go run ./cmd/migrate up 或 status；不提供自动删除数据入口")
	}
	databaseURL := os.Getenv("MOUSEHOLE_DATABASE_URL")
	if databaseURL == "" {
		return fmt.Errorf("必须设置 MOUSEHOLE_DATABASE_URL")
	}
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("数据库连接配置无效")
	}
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.Files)
	if err != nil {
		return fmt.Errorf("无法加载编号迁移")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if os.Args[1] == "status" {
		statuses, err := provider.Status(ctx)
		if err != nil {
			return fmt.Errorf("无法读取迁移状态，请检查数据库和权限")
		}
		for _, status := range statuses {
			fmt.Printf("%s\t%s\n", status.Source.Path, status.State)
		}
		return nil
	}
	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("迁移失败；事务迁移已回滚，请检查数据库与编号迁移约束")
	}
	fmt.Printf("迁移完成，应用 %d 个编号版本。\n", len(results))
	return nil
}
