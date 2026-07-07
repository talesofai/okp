package model

import (
	"crypto/sha256"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ── JSONMap: jsonb 字段的通用类型 ──────────────────────────

// JSONMap 是 map[string]any 的别名，实现 GORM 的 Scanner/Valuer 接口
// 用于 jsonb 字段（Frontmatter, Provenance）
type JSONMap map[string]any

func (m JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(m)
}

func (m *JSONMap) Scan(value any) error {
	if value == nil {
		*m = make(JSONMap)
		return nil
	}
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("JSONMap.Scan: expected []byte or string, got %T", value)
	}
	if len(data) == 0 {
		*m = make(JSONMap)
		return nil
	}
	*m = make(JSONMap)
	return json.Unmarshal(data, m)
}

// ── StringSlice: text[] 字段的通用类型 ──────────────────────

type StringSlice []string

func (s StringSlice) Value() (driver.Value, error) {
	if len(s) == 0 {
		return "{}", nil // PG 空数组字面量
	}
	// PG text[] 格式: {elem1,elem2}，元素中的特殊字符需转义
	parts := make([]string, len(s))
	for i, v := range s {
		// 转义: 反斜杠、引号、逗号
		escaped := strings.ReplaceAll(v, "\\", "\\\\")
		escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
		parts[i] = "\"" + escaped + "\""
	}
	return "{" + strings.Join(parts, ",") + "}", nil
}

func (s *StringSlice) Scan(value any) error {
	if value == nil {
		*s = StringSlice{}
		return nil
	}
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("StringSlice.Scan: expected []byte or string, got %T", value)
	}
	if len(data) == 0 || string(data) == "{}" || string(data) == "[]" || string(data) == "null" {
		*s = StringSlice{}
		return nil
	}

	// 先尝试 JSON 格式 [...]
	if data[0] == '[' {
		return json.Unmarshal(data, s)
	}

	// PG text[] 格式: {"elem1","elem2"} -> 手动解析
	if data[0] == '{' {
		*s = parsePGArray(string(data))
		return nil
	}

	return json.Unmarshal(data, s)
}

// parsePGArray 解析 PostgreSQL text[] 字面量 (如 {"a","b,c"})
func parsePGArray(raw string) []string {
	// 去掉首尾大括号
	if len(raw) < 2 || raw[0] != '{' || raw[len(raw)-1] != '}' {
		return nil
	}
	inner := raw[1 : len(raw)-1]
	if inner == "" {
		return []string{}
	}

	var result []string
	var current []byte
	i := 0
	for i < len(inner) {
		ch := inner[i]
		if ch == '"' {
			// 引号包裹的元素
			i++ // skip opening quote
			for i < len(inner) {
				if inner[i] == '\\' && i+1 < len(inner) {
					current = append(current, inner[i+1])
					i += 2
				} else if inner[i] == '"' {
					i++ // skip closing quote
					break
				} else {
					current = append(current, inner[i])
					i++
				}
			}
			result = append(result, string(current))
			current = nil
			// skip comma
			if i < len(inner) && inner[i] == ',' {
				i++
			}
		} else if ch == ',' {
			// 无引号元素结束
			result = append(result, string(current))
			current = nil
			i++
		} else {
			current = append(current, ch)
			i++
		}
	}
	// 最后一个无引号元素
	if len(current) > 0 {
		result = append(result, string(current))
	}
	return result
}

// ── Concept — 主干表 ───────────────────────────────────────

// Concept 对应 OKF concept，每个领域知识一行。
// ID 路径式：'fandom/genshin-impact/characters/klee'
type Concept struct {
	ID          string      `gorm:"primaryKey;type:text" json:"id"`
	Domain      string      `gorm:"type:text;not null;index:idx_domain_type_status,priority:1" json:"domain"`
	Type        string      `gorm:"type:text;not null;index:idx_domain_type_status,priority:2" json:"type"`
	Title       string      `gorm:"type:text" json:"title,omitempty"`
	Description string      `gorm:"type:text" json:"description,omitempty"`
	Tags        StringSlice `gorm:"type:text[];default:'{}'" json:"tags,omitempty"`
	Frontmatter JSONMap     `gorm:"type:jsonb;default:'{}'" json:"frontmatter,omitempty"`
	Body        string      `gorm:"type:text" json:"body,omitempty"`
	Resource    string      `gorm:"type:text" json:"resource,omitempty"`
	Status      string      `gorm:"type:text;default:'draft';index:idx_domain_type_status,priority:3" json:"status"`
	Provenance  JSONMap     `gorm:"type:jsonb;not null" json:"provenance"`
	ContentHash string      `gorm:"type:text" json:"content_hash,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// ComputeHash 计算 concept 内容指纹（幂等重导入用）。
// 散列范围：body + resource + frontmatter + title + description
func (c *Concept) ComputeHash() string {
	payload := c.Body + c.Resource + c.Title + c.Description
	if c.Frontmatter != nil {
		b, _ := json.Marshal(c.Frontmatter)
		payload += string(b)
	}
	h := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(h[:])
}

// BeforeCreate GORM hook: 自动计算 content_hash 和填充默认值
func (c *Concept) BeforeCreate(tx *gorm.DB) error {
	if c.Status == "" {
		c.Status = "draft"
	}
	if c.Tags == nil {
		c.Tags = StringSlice{}
	}
	if c.Frontmatter == nil {
		c.Frontmatter = make(JSONMap)
	}
	if c.Provenance == nil {
		c.Provenance = make(JSONMap)
	}
	c.ContentHash = c.ComputeHash()
	if c.Provenance["imported_at"] == nil {
		c.Provenance["imported_at"] = time.Now().UTC().Format(time.RFC3339)
	}
	return nil
}

// BeforeUpdate GORM hook: 更新时重新计算 hash
func (c *Concept) BeforeUpdate(tx *gorm.DB) error {
	c.ContentHash = c.ComputeHash()
	return nil
}

// ── Link — 有向关系 ────────────────────────────────────────

// Link 记录 concept 之间的引用关系。to_id 允许不存在（OKF 断链语义）。
type Link struct {
	ID        uint      `gorm:"primaryKey" json:"-"`
	FromID    string    `gorm:"type:text;not null;index:idx_from" json:"from_id"`
	ToID      string    `gorm:"type:text;not null;index:idx_to" json:"to_id"`
	Context   string    `gorm:"type:text" json:"context,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ── Revision — 变更历史 ─────────────────────────────────────

// Revision 记录 concept 每次变更的完整快照，替代 git 历史。
type Revision struct {
	ID        uint      `gorm:"primaryKey" json:"-"`
	ConceptID string    `gorm:"type:text;not null;index:idx_concept_rev,priority:1" json:"concept_id"`
	Rev       int       `gorm:"not null;index:idx_concept_rev,priority:2" json:"rev"`
	Content   JSONMap   `gorm:"type:jsonb;not null" json:"content"` // concept 完整快照
	Actor     string    `gorm:"type:text" json:"actor"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"at"`
}
