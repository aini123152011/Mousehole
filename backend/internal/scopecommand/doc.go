// Package scopecommand 协调本切片的数据库幂等受理、业务、审计与 outbox。
// 不改旧草稿账本；冻结回执只表达历史事务结果，不触发外部执行。
package scopecommand
