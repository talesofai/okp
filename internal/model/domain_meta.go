package model

import "time"

// DomainMeta 存储每个 domain 的 README 和 frontmatter schema 定义。
// README 由 agent 撰写，schema 从 README YAML frontmatter 解析。
// agent 写入 concept 时按 schema 校验 frontmatter 字段。
type DomainMeta struct {
	Domain     string    `gorm:"primaryKey;type:text" json:"domain"`
	Readme     string    `gorm:"type:text;not null;default:''" json:"readme"`
	Schema     JSONMap   `gorm:"type:jsonb;default:'{}'" json:"schema"`
	Visibility string    `gorm:"type:text;not null;default:'public';index" json:"visibility"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// FrontmatterSchema 从 README YAML frontmatter 解析出的字段定义
type FrontmatterSchema struct {
	Fields map[string]FieldDef `yaml:"fields" json:"fields"`
}

// FieldDef 单个 frontmatter 字段的定义
type FieldDef struct {
	Type        string   `yaml:"type" json:"type"` // string | date | url | int | enum
	Required    bool     `yaml:"required" json:"required"`
	Description string   `yaml:"description" json:"description"`
	Enum        []string `yaml:"enum,omitempty" json:"enum,omitempty"`
}
