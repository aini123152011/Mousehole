// Package command 管理草稿命令的校验、幂等受理与恢复查询。
// PostgreSQL 保存唯一权威状态；受理、草稿、审计和 outbox 同事务提交。
// 本包不访问目标、不调用模型、不调度工作流，也不消费 outbox。
package command
