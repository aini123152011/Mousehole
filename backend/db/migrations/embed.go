// Package migrations 提供嵌入二进制的编号迁移，只由显式迁移命令和隔离测试调用。
package migrations

import "embed"

// Files 保存 goose SQL 迁移；API 启动不读取或执行这些文件。
//
//go:embed *.sql
var Files embed.FS
