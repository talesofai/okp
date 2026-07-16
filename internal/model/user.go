package model

import "time"

// User 记录所有通过认证访问 okp 的用户。
// Role 控制全局权限：reader（默认，只读）/ admin（全局管理员，可写任何 domain）。
// Per-domain 的写权限通过 domain_members 表控制（host / writer）。
type User struct {
	UUID      string    `gorm:"primaryKey;type:text" json:"uuid"`
	AuthType  string    `gorm:"type:text;not null" json:"auth_type"` // "logto" | "execution" | "token" | "cohub_work"
	Role      string    `gorm:"type:text;not null;default:'reader'" json:"role"` // "reader" | "admin"
	LastSeen  time.Time `json:"last_seen_at"`
	CreatedAt time.Time `json:"created_at"`
}

// DomainMember 记录用户在某个 domain 的角色。
// host: 管理 domain（README/schema/成员管理），可写 concept
// writer: 可写 concept
// reader: 显式 reader（默认所有人都是 reader，此记录用于显式授权 host/writer）
type DomainMember struct {
	Domain    string    `gorm:"primaryKey;type:text" json:"domain"`
	UserID    string    `gorm:"primaryKey;type:text" json:"user_id"`
	Role      string    `gorm:"type:text;not null;default:'reader'" json:"role"` // "host" | "writer" | "reader"
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
