package model

import "time"

// DomainInvite 记录 domain 邀请码。
// CodeHash 存规范化邀请码的 SHA-256 hex；明文 code 只在创建时返回一次。
// 公开 domain 邀请 writer；private domain 可邀请 reader 或 writer。
// 不允许通过邀请授予 host。
type DomainInvite struct {
	ID         string     `gorm:"primaryKey;type:text" json:"id"`
	CodeHash   string     `gorm:"uniqueIndex;type:text;not null" json:"-"`
	Domain     string     `gorm:"type:text;not null;index" json:"domain"`
	Role       string     `gorm:"type:text;not null;default:'writer'" json:"role"` // reader | writer
	CreatedBy  string     `gorm:"type:text;not null" json:"created_by"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	MaxUses    int        `gorm:"not null;default:1" json:"max_uses"`
	UsedCount  int        `gorm:"not null;default:0" json:"used_count"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	LastUsedBy string     `gorm:"type:text" json:"last_used_by,omitempty"`
}
