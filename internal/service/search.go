package service

import (
	"strings"

	"github.com/talesofai/okp/internal/model"
	"github.com/talesofai/okp/internal/store"
	"gorm.io/gorm/clause"
)

// SearchResult 搜索结果项
type SearchResult struct {
	ID          string        `json:"id"`
	Domain      string        `json:"domain"`
	Type        string        `json:"type"`
	Title       string        `json:"title,omitempty"`
	Description string        `json:"description,omitempty"`
	Tags        []string      `json:"tags,omitempty"`
	Frontmatter model.JSONMap `json:"frontmatter,omitempty"`
	MatchReason string        `json:"match_reason"`
}

// SearchParams 搜索参数
type SearchParams struct {
	Query    string
	Domain   string
	Type     string
	Tags     []string
	Scenario string
	Limit    int
	Offset   int
}

// Search 执行概念搜索。
//   - 路径式（含2个以上 /） → 精确 ID 或前缀
//   - 多词查询（含空格）→ 拆词后每词 ILIKE AND
//   - 单词/短语 → trgm 相似度 + ILIKE
//   - 无 query → 结构过滤
func Search(params SearchParams) ([]SearchResult, int64, error) {
	db := store.DB

	if params.Limit <= 0 {
		params.Limit = 50
	}
	if params.Limit > 200 {
		params.Limit = 200
	}

	q := db.Model(&model.Concept{})

	if params.Domain != "" {
		q = q.Where("domain = ?", params.Domain)
	}
	if params.Type != "" {
		q = q.Where("type = ?", params.Type)
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

	// 文本查询
	textSearch := false
	if params.Query != "" {
		query := params.Query
		if store.IsSQLite {
			like := "%" + query + "%"
			q = q.Where("(id = ? OR id LIKE ? OR title LIKE ? OR description LIKE ?)",
				query, query+"%", like, like)
			textSearch = true
		} else if looksLikePath(query) {
			q = q.Where("id = ? OR id LIKE ?", query, query+"%")
		} else {
			words := splitQuery(query)
			for _, w := range words {
				like := "%" + w + "%"
				q = q.Where("(title ILIKE ? OR description ILIKE ? OR similarity(title, ?) > 0.2)",
					like, like, w)
			}
			textSearch = true
		}
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if textSearch && !store.IsSQLite && params.Query != "" {
		words := splitQuery(params.Query)
		if len(words) == 1 {
			q = q.Clauses(clause.OrderBy{
				Expression: clause.Expr{
					SQL:  "similarity(title, ?) DESC",
					Vars: []interface{}{words[0]},
				},
			})
		} else {
			q = q.Order("updated_at DESC")
		}
	} else {
		q = q.Order("updated_at DESC")
	}

	q = q.Offset(params.Offset).Limit(params.Limit)

	var concepts []model.Concept
	if err := q.Find(&concepts).Error; err != nil {
		return nil, 0, err
	}

	results := make([]SearchResult, len(concepts))
	for i, c := range concepts {
		results[i] = SearchResult{
			ID:          c.ID,
			Domain:      c.Domain,
			Type:        c.Type,
			Title:       c.Title,
			Description: c.Description,
			Tags:        []string(c.Tags),
			Frontmatter: c.Frontmatter,
			MatchReason: matchReason(c, params.Query, params.Tags),
		}
	}

	return results, total, nil
}

func splitQuery(q string) []string {
	raw := strings.Fields(q)
	if len(raw) == 0 {
		return []string{q}
	}
	return raw
}

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

func looksLikePath(q string) bool {
	return len(q) >= 5 && strings.Count(q, "/") >= 2
}

// ── 领域清单 ─────────────────────────────────────────────────

type DomainInfo struct {
	Domain       string `json:"domain"`
	ConceptCount int64  `json:"concept_count"`
}

func ListDomains() ([]DomainInfo, error) {
	var domains []DomainInfo
	err := store.DB.Model(&model.Concept{}).
		Select("domain, count(*) as concept_count").
		Group("domain").
		Order("concept_count DESC").
		Scan(&domains).Error
	return domains, err
}
