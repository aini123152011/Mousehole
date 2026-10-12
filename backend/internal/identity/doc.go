// Package identity 管理 OIDC 登录、服务端会话、CSRF 与项目访问投影。
// PostgreSQL 是用户、外部身份、成员关系、登录尝试和会话的唯一权威。
// 本包不根据 email、名称或 group claim 自动授权，也不授予目标执行能力。
package identity
