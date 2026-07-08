package service

import (
	"fmt"
	"strings"

	"github.com/talesofai/okp/internal/model"
	"github.com/talesofai/okp/internal/store"
	"gorm.io/gorm/clause"
)

// ── 校验 ────────────────────────────────────────────────────

// ValidationError 教学化校验错误（L1 硬门禁）。
// 必须包含具体修复建议，而不是通用错误码。
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Fix     string `json:"fix"` // 告诉调用方怎么改
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s (fix: %s)", e.Field, e.Message, e.Fix)
}

// ValidatePut 校验 upsert 请求（L1 硬门禁）。
// 返回 nil 表示通过；返回 error 列表表示校验失败。
func ValidatePut(c *model.Concept) []ValidationError {
	var errs []ValidationError

	if strings.TrimSpace(c.ID) == "" {
		errs = append(errs, ValidationError{
			Field:   "id",
			Message: "concept id 不能为空",
			Fix:     "使用路径式 ID，如 'fandom/genshin-impact/characters/klee'",
		})
	}
	if strings.TrimSpace(c.Domain) == "" {
		errs = append(errs, ValidationError{
			Field:   "domain",
			Message: "domain 不能为空",
			Fix:     "设置为知识域名称，如 'fandom'、'art-style'",
		})
	}
	if strings.TrimSpace(c.Type) == "" {
		errs = append(errs, ValidationError{
			Field:   "type",
			Message: "type 不能为空（OKF 唯一必需字段）",
			Fix:     "设置为概念类型，如 'Character'、'ArtStyle'、'Playbook'",
		})
	}
	if len(c.Provenance) == 0 {
		errs = append(errs, ValidationError{
			Field:   "provenance",
			Message: "provenance 不能为空（数据溯源必填）",
			Fix:     `填写 {"source":"...","agent":"...","raw_ref":"..."}`,
		})
	}
	if c.Provenance["source"] == nil || c.Provenance["source"] == "" {
		errs = append(errs, ValidationError{
			Field:   "provenance.source",
			Message: "provenance.source 必填（数据来源）",
			Fix:     `如 "fandom-crawl"、"manual"、"agent-import"`,
		})
	}
	if c.Provenance["agent"] == nil || c.Provenance["agent"] == "" {
		errs = append(errs, ValidationError{
			Field:   "provenance.agent",
			Message: "provenance.agent 必填（写入方标识）",
			Fix:     `如 "kb-import/0.1"、"okp-cli"`,
		})
	}

	// description 限长（供 agent 的"卡片目录"用），超长截断
	if len(c.Description) > 500 {
		c.Description = c.Description[:497] + "..."
	}

	// id 格式校验（OKF 风格：/ 分隔的路径）
	if strings.Contains(c.ID, " ") {
		errs = append(errs, ValidationError{
			Field:   "id",
			Message: "id 不能包含空格",
			Fix:     "使用 '-' 或 '_' 替代空格",
		})
	}
	if strings.HasPrefix(c.ID, "/") || strings.HasSuffix(c.ID, "/") {
		errs = append(errs, ValidationError{
			Field:   "id",
			Message: "id 不能以 / 开头或结尾",
			Fix:     "使用 'domain/type/name' 格式，如 'fandom/genshin-impact/characters/klee'",
		})
	}
	if strings.Contains(c.ID, "//") {
		errs = append(errs, ValidationError{
			Field:   "id",
			Message: "id 不能包含连续的 //",
			Fix:     "检查路径分隔符，去掉多余的 /",
		})
	}

	return errs
}

// ── CRUD ─────────────────────────────────────────────────────

// PutConcept upsert 一个 concept。走完整 L1 校验 → 去重 → 写入/更新路径。
func PutConcept(c *model.Concept) (*model.Concept, []ValidationError, error) {
	// L1 校验
	if errs := ValidatePut(c); len(errs) > 0 {
		return nil, errs, nil
	}

	db := store.DB

	// 去重：search-before-insert（同 domain+type 下 trgm 标题相似度）
	existing, isDup := checkDuplicate(c)
	if isDup && existing != nil && existing.ID != c.ID {
		errs := []ValidationError{{
			Field:   "title",
			Message: fmt.Sprintf("疑似重复：与已有 concept '%s'（%s）标题高度相似", existing.ID, existing.Title),
			Fix:     "检查是否与已有概念重复；若确实不同，请修改标题以区分；若为同一概念，使用已有 id",
		}}
		return nil, errs, nil
	}

	// 检查是否已存在（幂等 re-import）
	var prev model.Concept
	result := db.First(&prev, "id = ?", c.ID)
	if result.Error == nil {
		// 已存在 → 检查 content_hash
		if prev.ContentHash == c.ComputeHash() {
			// 内容未变 → skip
			return &prev, nil, nil
		}
		// 内容变化 → 更新
		c.CreatedAt = prev.CreatedAt
		if err := db.Save(c).Error; err != nil {
			return nil, nil, err
		}
		// 记录 revision
		saveRevision(c, "update")
		return c, nil, nil
	}

	// 新创建或更新
	if err := db.Save(c).Error; err != nil {
		return nil, nil, err
	}
	saveRevision(c, "create")
	return c, nil, nil
}

// GetConcept 按 ID 获取 concept。
func GetConcept(id string) (*model.Concept, error) {
	var c model.Concept
	if err := store.DB.First(&c, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

// ── 去重 ────────────────────────────────────────────────────

// checkDuplicate 在同 domain+type 下检查重复。
// PG 模式使用 trgm similarity，SQLite 模式使用精确 title 匹配。
func checkDuplicate(c *model.Concept) (*model.Concept, bool) {
	if c.Title == "" {
		return nil, false
	}

	var candidates []model.Concept

	if store.IsSQLite {
		// SQLite：精确 title 匹配
		store.DB.Where("domain = ? AND type = ? AND title = ? AND id != ?", c.Domain, c.Type, c.Title, c.ID).
			Limit(3).Find(&candidates)
		if len(candidates) > 0 {
			return &candidates[0], true
		}
		return nil, false
	}

	// PG：trgm similarity
	store.DB.Where("domain = ? AND type = ? AND id != ?", c.Domain, c.Type, c.ID).
		Clauses(clause.OrderBy{
			Expression: clause.Expr{SQL: "similarity(title, ?) DESC", Vars: []interface{}{c.Title}},
		}).
		Limit(3).
		Find(&candidates)

	for i := range candidates {
		var sim float64
		store.DB.Raw("SELECT similarity(?, ?)", candidates[i].Title, c.Title).Scan(&sim)
		if sim > 0.7 {
			return &candidates[i], true
		}
	}
	return nil, false
}

// ── Revision ─────────────────────────────────────────────────

func saveRevision(c *model.Concept, action string) {
	var maxRev int
	store.DB.Model(&model.Revision{}).
		Where("concept_id = ?", c.ID).
		Select("COALESCE(MAX(rev), 0)").
		Scan(&maxRev)

	rev := model.Revision{
		ConceptID: c.ID,
		Rev:       maxRev + 1,
		Actor:     fmt.Sprintf("%v", c.Provenance["agent"]),
		Content: model.JSONMap{
			"id":          c.ID,
			"domain":      c.Domain,
			"type":        c.Type,
			"title":       c.Title,
			"description": c.Description,
			"tags":        c.Tags,
			"frontmatter": c.Frontmatter,
			"body":        c.Body,
			"resource":    c.Resource,
			"provenance":  c.Provenance,
			"action":      action,
		},
	}
	store.DB.Create(&rev)
}

// ── Links ────────────────────────────────────────────────────

// GetLinks 获取 concept 的出链和反向引用。
// GetLinks 获取 concept 的出链和反向引用，支持分页。
func GetLinks(id string, limit, offset int) (outgoing []model.Link, backlinks []model.Link, totalOut int64, totalBack int64, err error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	db := store.DB.Model(&model.Link{})
	if err := db.Where("from_id = ?", id).Count(&totalOut).Limit(limit).Offset(offset).Find(&outgoing).Error; err != nil {
		return nil, nil, 0, 0, err
	}
	if err := db.Where("to_id = ?", id).Count(&totalBack).Limit(limit).Offset(offset).Find(&backlinks).Error; err != nil {
		return nil, nil, 0, 0, err
	}
	return outgoing, backlinks, totalOut, totalBack, nil
}

// PutLinks 全量替换 concept 的出链（先删后插）。
func PutLinks(fromID string, linkPairs []struct {
	ToID    string `json:"to_id"`
	Context string `json:"context,omitempty"`
}) error {
	tx := store.DB.Begin()

	if err := tx.Where("from_id = ?", fromID).Delete(&model.Link{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	for _, lp := range linkPairs {
		link := model.Link{
			FromID:  fromID,
			ToID:    lp.ToID,
			Context: lp.Context,
		}
		if err := tx.Create(&link).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit().Error
}
