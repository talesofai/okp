package service

import (
	"strings"

	"github.com/talesofai/okp/internal/model"
	"github.com/talesofai/okp/internal/store"
	"gorm.io/gorm/clause"
)

// SearchResult 搜索结果项，包含 concept 摘要 + 匹配原因 + frontmatter（供 agent 按字段过滤）
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
	Status   string
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
	if params.Status == "" {
		params.Status = "accepted"
	}

	q := db.Model(&model.Concept{})

	// 结构过滤
	if params.Domain != "" {
		q = q.Where("domain = ?", params.Domain)
	}
	if params.Type != "" {
		q = q.Where("type = ?", params.Type)
	}
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
			// 3段路径式 ID → 精确匹配或前缀
			q = q.Where("id = ? OR id LIKE ?", query, query+"%")
		} else {
			// 文本搜索：拆词，每词 ILIKE AND + trgm
			words := splitQuery(query)
			for _, w := range words {
				like := "%" + w + "%"
				q = q.Where("(title ILIKE ? OR description ILIKE ? OR similarity(title, ?) > 0.2)",
					like, like, w)
			}
			textSearch = true
		}
	}

	// 计数
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 排序
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

// splitQuery 按空白拆词，过滤空串
func splitQuery(q string) []string {
	raw := strings.Fields(q)
	out := make([]string, 0, len(raw))
	for _, w := range raw {
		if w != "" {
			out = append(out, w)
		}
	}
	if len(out) == 0 {
		return []string{q}
	}
	return out
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

// looksLikePath 判断 query 是否是3段 OKF 路径（domain/type/slug）
func looksLikePath(q string) bool {
	if len(q) < 5 {
		return false
	}
	count := strings.Count(q, "/")
	return count >= 2
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
		Where("status = ?", "accepted").
		Group("domain").
		Order("concept_count DESC").
		Scan(&domains).Error
	return domains, err
}
