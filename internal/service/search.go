package service

import (
	"github.com/talesofai/okp/internal/model"
	"github.com/talesofai/okp/internal/store"
	"gorm.io/gorm/clause"
)

// SearchResult 搜索结果项，包含 concept 摘要 + 匹配原因（agent 可据此调整查询）。
type SearchResult struct {
	ID          string   `json:"id"`
	Domain      string   `json:"domain"`
	Type        string   `json:"type"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Status      string   `json:"status"`
	MatchReason string   `json:"match_reason"` // "id_exact" | "title_trgm" | "description_like" | "tag_match"
}

// SearchParams 搜索参数
type SearchParams struct {
	Query    string   // 自由文本
	Domain   string   // 限定 domain
	Type     string   // 限定 type
	Tags     []string // 限定 tags（AND）
	Status   string   // 限定 status，默认 "accepted"
	Scenario string   // frontmatter 内的 scenario 字段
	Limit    int      // 默认 50
	Offset   int
}

// Search 执行概念搜索。服务端 query planning：
//   - 路径式 → 精确 ID 匹配
//   - 短实体名（≤15 字符）→ trgm 子串 + 相似度
//   - 长查询 → 全文（trgm similarity，后续可升级 pgroonga）
//   - 无 query → 结构过滤（domain/type/tag/scenario）
func Search(params SearchParams) ([]SearchResult, int64, error) {
	db := store.DB

	if params.Limit <= 0 {
		params.Limit = 50
	}
	if params.Limit > 200 {
		params.Limit = 200
	}
	if params.Status == "" {
		// 默认不过滤 status，所有 status 都能搜到
	}

	q := db.Model(&model.Concept{})

	// 结构过滤
	if params.Domain != "" {
		q = q.Where("domain = ?", params.Domain)
	}
	if params.Type != "" {
		q = q.Where("type = ?", params.Type)
	}
	// Status 过滤（空 = 不过滤）
	if params.Status != "" {
		q = q.Where("status = ?", params.Status)
	}
	if len(params.Tags) > 0 {
		if store.IsSQLite {
			for _, tag := range params.Tags {
				q = q.Where("tags LIKE ?", "%\""+tag+"\"%")
			}
		} else {
			for _, tag := range params.Tags {
				q = q.Where("? = ANY(tags)", tag)
			}
		}
	}
	if params.Scenario != "" {
		q = q.Where("frontmatter->>'scenario' = ?", params.Scenario)
	}

	// 文本查询 → query planning
	if params.Query != "" {
		query := params.Query
		if store.IsSQLite {
			// SQLite：简单 LIKE（无 trgm）
			like := "%" + query + "%"
			q = q.Where("(id = ? OR id LIKE ? OR title LIKE ? OR description LIKE ?)",
				query, query+"%", like, like)
		} else if looksLikePath(query) {
			// 路径式 → 精确 ID
			q = q.Where("id = ? OR id LIKE ?", query, query+"%")
		} else if len([]rune(query)) <= 15 {
			// 短实体名 → trgm 子串 + 相似度排序
			q = q.Where(
				"(title % ? OR similarity(title, ?) > 0.3 OR description ILIKE ?)",
				query, query, "%"+query+"%",
			)
		} else {
			// 长查询 → trgm similarity（后续可升级 pgroonga）
			q = q.Where(
				"(title % ? OR similarity(title, ?) > 0.2 OR description ILIKE ?)",
				query, query, "%"+query+"%",
			)
		}
	}

	// 计数
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 排序（结构优于文本）
	if params.Query != "" && !store.IsSQLite && !looksLikePath(params.Query) {
		q = q.Clauses(clause.OrderBy{
			Expression: clause.Expr{SQL: "similarity(title, ?) DESC", Vars: []interface{}{params.Query}},
		})
	} else {
		q = q.Order("updated_at DESC")
	}

	// 分页
	q = q.Offset(params.Offset).Limit(params.Limit)

	var concepts []model.Concept
	if err := q.Find(&concepts).Error; err != nil {
		return nil, 0, err
	}

	// 转换为 SearchResult + 添加 match_reason
	results := make([]SearchResult, len(concepts))
	for i, c := range concepts {
		results[i] = SearchResult{
			ID:          c.ID,
			Domain:      c.Domain,
			Type:        c.Type,
			Title:       c.Title,
			Description: c.Description,
			Tags:        []string(c.Tags),
			Status:      c.Status,
			MatchReason: matchReason(c, params.Query, params.Tags),
		}
	}

	return results, total, nil
}

// matchReason 生成人类可读的匹配原因（帮助 agent 决定是否收窄查询）。
func matchReason(c model.Concept, query string, tags []string) string {
	if query != "" && c.ID == query {
		return "id_exact"
	}
	if query != "" {
		return "text_match"
	}
	if len(tags) > 0 {
		return "tag_match"
	}
	return "filter_match"
}

// LooksLikePath 判断 query 是否像路径式 ID（包含 /）
// OKP concept ID 格式: domain/type/slug（OKF 原生）
func looksLikePath(q string) bool {
	return len(q) > 3 && q[0] != ' ' && containsAny(q, "/")
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
	}
	return false
}

// ── 领域清单 ─────────────────────────────────────────────────

// DomainInfo 领域信息
type DomainInfo struct {
	Domain      string `json:"domain"`
	ConceptCount int64 `json:"concept_count"`
}

// ListDomains 列出所有 domain 及其 concept 数量，支持搜索和分页。
func ListDomains(q string, limit, offset int) ([]DomainInfo, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	db := store.DB.Model(&model.Concept{})
	if q != "" {
		db = db.Where("domain ILIKE ?", "%"+q+"%")
	}

	var total int64
	// 子查询：每个 domain 一条
	subQuery := store.DB.Model(&model.Concept{}).
		Select("domain").
		Group("domain")
	if q != "" {
		subQuery = subQuery.Where("domain ILIKE ?", "%"+q+"%")
	}
	if err := subQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var domains []DomainInfo
	err := db.
		Select("domain, count(*) as concept_count").
		Group("domain").
		Order("concept_count DESC").
		Limit(limit).
		Offset(offset).
		Scan(&domains).Error
	return domains, total, err
}
