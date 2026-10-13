// Package authorization 定义不可变授权内容、可变状态及当前禁止规则。
// 内容规范化与 hash 只含语义字段，不含数据库身份；管理变更由同事务命令协调。
package authorization
