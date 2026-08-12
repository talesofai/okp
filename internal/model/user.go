package model

import "time"

// User 记录所有通过认证访问 okp 的用户。
// Role 控制全局权限：reader（默认，只读）/ admin（可管理公开 domain）。
// Per-domain 权限通过 domain_members 表控制（host / writer / reader）。
type User struct {
	UUID        string    `gorm:"primaryKey;type:text" json:"uuid"`
	AuthType    string    `gorm:"type:text;not null" json:"auth_type"`             // "logto" | "execution" | "token" | "cohub_work"
	Role        string    `gorm:"type:text;not null;default:'reader'" json:"role"` // "reader" | "admin"
	Username    string    `gorm:"type:text;default:''" json:"username"`
	DisplayName string    `gorm:"type:text;default:''" json:"display_name"`
	AvatarURL   string    `gorm:"type:text;default:''" json:"avatar_url"`
	LastSeen    time.Time `json:"last_seen_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// DomainMember 记录用户在某个 domain 的角色。
// host: 每个 domain 唯一，由 domain 创建者持有；管理 domain 并可写 concept
// writer: 可写 concept
// reader: private domain 的显式只读成员；公开 domain 不需要成员记录
type DomainMember struct {
	Domain    string    `gorm:"primaryKey;type:text" json:"domain"`
	UserID    string    `gorm:"primaryKey;type:text" json:"user_id"`
	Role      string    `gorm:"type:text;not null;default:'reader'" json:"role"` // "host" | "writer" | "reader"
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
