// Package migrations 保存进入版本控制的数据库迁移。
package migrations

import "embed"

//go:embed *.sql
var Files embed.FS
