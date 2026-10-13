// Package access 在调用方事务内统一锁定用户、项目与成员。
// 不开启独立事务、不缓存权限；读写命令与管理员撤销遵循同一锁序。
package access
