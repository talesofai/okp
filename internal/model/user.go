package model

import "time"

// User 记录所有通过认证访问 okp 的用户。
// Role 控制权限：reader（默认，只读）/ writer（可写）。
// 管理员直接在数据库里把 role 改成 writer 即可授权，无需重启服务。
type User struct {
	UUID      string    `gorm:"primaryKey;type:text" json:"uuid"`
	AuthType  string    `gorm:"type:text;not null" json:"auth_type"` // "logto" | "execution" | "token"
	Role      string    `gorm:"type:text;not null;default:'reader'" json:"role"`
	LastSeen  time.Time `json:"last_seen_at"`
	CreatedAt time.Time `json:"created_at"`
}
