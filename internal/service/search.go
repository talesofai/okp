package service

import (
	"database/sql/driver"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/talesofai/okp/internal/access"
	"github.com/talesofai/okp/internal/model"
	"github.com/talesofai/okp/internal/store"
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
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	MatchReason string        `json:"-"` // 内部使用，不暴露给用户
}

// SearchParams 搜索参数
type SearchParams struct {
	UserID   string
	Query    string
	Domain   string
	Type     string
	Tags     []string
	Scenario string
	Filters  map[string]string // frontmatter 任意字段过滤，如 {"sender":"kjx","group":"feishu-worldbuild"}
	Sort     string            // updated_at:desc(默认) | updated_at:asc | date:desc | date:asc | title:asc
	Limit    int
	Offset   int
}

// Search 执行概念搜索。
func Search(params SearchParams) ([]SearchResult, int64, error) {
	if params.Limit <= 0 {
		params.Limit = 50
	}
	if params.Limit > 200 {
		params.Limit = 200
	}

	q := store.DB.Model(&model.Concept{}).Scopes(access.ScopeReadableConcepts(params.UserID))

	// 结构过滤
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

	// frontmatter 任意字段过滤（走 GIN 索引的 @> containment）
	if len(params.Filters) > 0 && !store.IsSQLite {
		for k, v := range params.Filters {
			q = q.Where("frontmatter->>? = ?", k, v)
		}
	}

	// 文本查询
	textSearch := false
	if params.Query != "" {
		query := params.Query
		if store.IsSQLite {
			like := "%" + query + "%"
			q = q.Where("(id = ? OR id LIKE ? OR title LIKE ? OR description LIKE ? OR CAST(frontmatter AS TEXT) LIKE ?)",
				query, query+"%", like, like, like)
			textSearch = true
		} else if looksLikePath(query) {
			q = q.Where("id = ? OR id LIKE ?", query, query+"%")
		} else {
			for _, w := range splitQuery(query) {
				like := "%" + w + "%"
				// title/description/id/frontmatter 子串命中 + 标题 trigram
				q = q.Where(`(
					title ILIKE ? OR description ILIKE ? OR id ILIKE ?
					OR frontmatter::text ILIKE ?
					OR similarity(title, ?) > 0.3
				)`, like, like, like, like, w)
			}
			textSearch = true
		}
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 向量搜索：有 query 且有 API key 时，并行进行
	if params.Query != "" && !store.IsSQLite && embedAPIKey != "" {
		vecResults, _ := vectorSearch(params.Query, params)
		if len(vecResults) > 0 {
			// 字符命中按精确度排序后再取 limit
			q = applyExactnessOrder(q, params.Query)
			q = q.Offset(params.Offset).Limit(params.Limit)
			var trgmConcepts []model.Concept
			_ = q.Find(&trgmConcepts).Error

			merged := mergeResults(trgmConcepts, vecResults, params.Query, params.Tags, params.Limit)
			return merged, int64(len(merged)), nil
		}
	}

	// 排序
	q = applySort(q, params.Sort, textSearch, params.Query)
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
			CreatedAt:   c.CreatedAt,
			UpdatedAt:   c.UpdatedAt,
			MatchReason: matchReason(c, params.Query, params.Tags),
		}
	}
	return results, total, nil
}

// applyExactnessOrder：子串精确命中优先于纯 trigram 模糊
func applyExactnessOrder(q *gorm.DB, query string) *gorm.DB {
	if store.IsSQLite || query == "" {
		return q
	}
	like := "%" + query + "%"
	return q.Clauses(clause.OrderBy{
		Expression: clause.Expr{
			SQL: `CASE
				WHEN title ILIKE ? THEN 0
				WHEN description ILIKE ? THEN 1
				WHEN frontmatter::text ILIKE ? THEN 2
				WHEN id ILIKE ? THEN 3
				ELSE 4
			END ASC, similarity(title, ?) DESC`,
			Vars: []interface{}{like, like, like, like, query},
		},
	})
}

// applySort 决定排序策略：
//   - 有文本搜索时：精确子串优先，再 similarity
//   - sort 显式指定时按参数
//   - 默认 updated_at DESC
func applySort(q *gorm.DB, sort string, textSearch bool, query string) *gorm.DB {
	if textSearch && !store.IsSQLite && query != "" && sort == "" {
		return applyExactnessOrder(q, query)
	}

	switch sort {
	case "updated_at:asc":
		return q.Order("updated_at ASC")
	case "date:desc":
		// frontmatter->>'date' 是 YYYY-MM-DD，字母序 = 时间序
		return q.Order("frontmatter->>'date' DESC NULLS LAST")
	case "date:asc":
		return q.Order("frontmatter->>'date' ASC NULLS LAST")
	case "title:asc":
		return q.Order("title ASC")
	default:
		if textSearch && !store.IsSQLite && query != "" {
			return applyExactnessOrder(q, query)
		}
		return q.Order("updated_at DESC")
	}
}

// Sample 随机采样 concepts
func Sample(userID, domain, typ string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 5
	}
	if limit > 50 {
		limit = 50
	}

	q := store.DB.Model(&model.Concept{}).Scopes(access.ScopeReadableConcepts(userID))
	if domain != "" {
		q = q.Where("domain = ?", domain)
	}
	if typ != "" {
		q = q.Where("type = ?", typ)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}
	if total == 0 {
		return []SearchResult{}, nil
	}

	// 随机 offset + ORDER BY RANDOM() 双保险
	offset := 0
	if total > int64(limit) {
		offset = int(rand.Int63n(total - int64(limit)))
	}

	var concepts []model.Concept
	if err := q.Order("RANDOM()").Offset(offset).Limit(limit).Find(&concepts).Error; err != nil {
		return nil, err
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
			CreatedAt:   c.CreatedAt,
			UpdatedAt:   c.UpdatedAt,
			MatchReason: "sample",
		}
	}
	return results, nil
}

// ── 辅助函数 ─────────────────────────────────────────────────

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

// vectorSearch 用向量进行语义搜索（包括跨语言）
func vectorSearch(query string, params SearchParams) ([]model.Concept, error) {
	vecs, err := embedText([]string{query})
	if err != nil || len(vecs) == 0 {
		return nil, err
	}
	vecStr := floatsToVecStr(vecs[0])

	// 有 embedding 就参与检索；不严格依赖 embed_status，避免批量导入后 status 未回写导致只命中几条
	q := store.DB.Model(&model.Concept{}).
		Scopes(access.ScopeReadableConcepts(params.UserID)).
		Where("embedding IS NOT NULL")
	if params.Domain != "" {
		q = q.Where("domain = ?", params.Domain)
	}
	if params.Type != "" {
		q = q.Where("type = ?", params.Type)
	}
	for _, tag := range params.Tags {
		q = q.Where("? = ANY(tags)", tag)
	}
	if params.Scenario != "" {
		q = q.Where("frontmatter->>'scenario' = ?", params.Scenario)
	}
	for k, v := range params.Filters {
		q = q.Where("frontmatter->>? = ?", k, v)
	}

	var concepts []model.Concept
	err = q.Order(fmt.Sprintf("embedding <=> '%s'::vector", vecStr)).
		Limit(params.Limit).
		Find(&concepts).Error
	return concepts, err
}

// mergeResults 合并 trgm 和向量结果。
// 规则：
//  1. 先放字符精确命中（title/description/frontmatter 含 query）
//  2. 再放其余 trgm 结果
//  3. 最后补 vector 语义结果
func mergeResults(trgm []model.Concept, vec []model.Concept, query string, tags []string, limit int) []SearchResult {
	seen := map[string]bool{}
	results := []SearchResult{}
	q := strings.ToLower(strings.TrimSpace(query))

	containsQuery := func(c model.Concept) bool {
		if q == "" {
			return false
		}
		if strings.Contains(strings.ToLower(c.Title), q) ||
			strings.Contains(strings.ToLower(c.Description), q) ||
			strings.Contains(strings.ToLower(c.ID), q) {
			return true
		}
		if c.Frontmatter != nil {
			for _, v := range c.Frontmatter {
				if s, ok := v.(string); ok && strings.Contains(strings.ToLower(s), q) {
					return true
				}
				// 非 string 也转一下
				if strings.Contains(strings.ToLower(fmt.Sprint(v)), q) {
					return true
				}
			}
		}
		for _, t := range c.Tags {
			if strings.Contains(strings.ToLower(t), q) {
				return true
			}
		}
		return false
	}

	appendOne := func(c model.Concept) {
		if seen[c.ID] {
			return
		}
		seen[c.ID] = true
		results = append(results, SearchResult{
			ID: c.ID, Domain: c.Domain, Type: c.Type,
			Title: c.Title, Description: c.Description,
			Tags: []string(c.Tags), Frontmatter: c.Frontmatter,
			CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
		})
	}

	// 1) exact substring from trgm set
	for _, c := range trgm {
		if containsQuery(c) {
			appendOne(c)
		}
	}
	// 2) remaining trgm
	for _, c := range trgm {
		appendOne(c)
	}
	// 3) vector supplements only if we still have room;
	//    and only when query has no/weak exact hits, still useful for recall.
	for _, c := range vec {
		appendOne(c)
	}

	if len(results) > limit {
		return results[:limit]
	}
	return results
}

// ── 领域清单 ─────────────────────────────────────────────────

type DomainInfo struct {
	Domain       string    `json:"domain"`
	ConceptCount int64     `json:"concept_count"`
	HasReadme    bool      `json:"has_readme"`
	Visibility   string    `json:"visibility"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type nullableTime struct {
	time.Time
	Valid bool
}

func (t nullableTime) Value() (driver.Value, error) {
	if !t.Valid {
		return nil, nil
	}
	return t.Time, nil
}

func (t *nullableTime) Scan(value any) error {
	t.Valid = false
	t.Time = time.Time{}
	if value == nil {
		return nil
	}
	if parsed, ok := value.(time.Time); ok {
		t.Time = parsed
		t.Valid = true
		return nil
	}

	var raw string
	switch v := value.(type) {
	case string:
		raw = v
	case []byte:
		raw = string(v)
	default:
		return fmt.Errorf("cannot scan %T as timestamp", value)
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	} {
		parsed, err := time.Parse(layout, raw)
		if err == nil {
			t.Time = parsed
			t.Valid = true
			return nil
		}
	}
	return fmt.Errorf("cannot parse timestamp %q", raw)
}

type domainInfoRow struct {
	Domain           string       `gorm:"column:domain"`
	ConceptCount     int64        `gorm:"column:concept_count"`
	HasReadme        bool         `gorm:"column:has_readme"`
	Visibility       string       `gorm:"column:visibility"`
	MetaCreatedAt    nullableTime `gorm:"column:meta_created_at"`
	MetaUpdatedAt    nullableTime `gorm:"column:meta_updated_at"`
	ConceptCreatedAt nullableTime `gorm:"column:concept_created_at"`
	ConceptUpdatedAt nullableTime `gorm:"column:concept_updated_at"`
}

func domainInfoTimes(row domainInfoRow) (time.Time, time.Time) {
	var created, updated time.Time
	addEarliest := func(value nullableTime) {
		if !value.Valid || value.Time.IsZero() {
			return
		}
		if created.IsZero() || value.Time.Before(created) {
			created = value.Time
		}
	}
	addLatest := func(value nullableTime) {
		if !value.Valid || value.Time.IsZero() {
			return
		}
		if updated.IsZero() || value.Time.After(updated) {
			updated = value.Time
		}
	}

	addEarliest(row.MetaCreatedAt)
	addEarliest(row.ConceptCreatedAt)
	addLatest(row.MetaUpdatedAt)
	addLatest(row.ConceptUpdatedAt)

	// Older domain_meta rows may not have a meaningful created_at after the
	// column is added. The first known update is the best available fallback.
	if created.IsZero() {
		created = updated
	}
	if updated.IsZero() {
		updated = created
	}
	return created, updated
}

// ListDomains 列出所有领域。
// 包含：有 concept 的 domain + 只有 README 的 domain（concept_count=0）。
// q 为空时返回全部；不为空时用 trgm 模糊匹配领域名。
func ListDomains(userID, q string) ([]DomainInfo, error) {
	// FULL JOIN：concept 聚合 + domain_meta，两边都覆盖，同时聚合领域时间。
	query := `
		SELECT
			COALESCE(c.domain, m.domain) AS domain,
			COALESCE(c.concept_count, 0) AS concept_count,
			(m.domain IS NOT NULL) AS has_readme,
			COALESCE(m.visibility, 'public') AS visibility,
			m.created_at AS meta_created_at,
			m.updated_at AS meta_updated_at,
			c.concept_created_at,
			c.concept_updated_at
		FROM (
			SELECT domain,
				COUNT(*) AS concept_count,
				MIN(created_at) AS concept_created_at,
				MAX(updated_at) AS concept_updated_at
			FROM concepts GROUP BY domain
		) c
		FULL JOIN domain_meta m ON c.domain = m.domain
		WHERE (
			COALESCE(m.visibility, 'public') <> 'private'
			OR EXISTS (
				SELECT 1 FROM domain_members dmbr
				WHERE dmbr.domain = COALESCE(c.domain, m.domain)
				AND dmbr.user_id = ?
				AND dmbr.role IN ('host', 'writer', 'reader')
			)
		)`

	args := []interface{}{userID}
	if q != "" && !store.IsSQLite {
		query += ` AND (similarity(COALESCE(c.domain, m.domain), ?) > 0.1
				OR COALESCE(c.domain, m.domain) ILIKE ?)`
		args = append(args, q, "%"+q+"%")
	} else if q != "" {
		query += ` AND COALESCE(c.domain, m.domain) LIKE ?`
		args = append(args, "%"+q+"%")
	}
	query += ` ORDER BY concept_count DESC`

	var rows []domainInfoRow
	if err := store.DB.Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	domains := make([]DomainInfo, len(rows))
	for i, row := range rows {
		createdAt, updatedAt := domainInfoTimes(row)
		domains[i] = DomainInfo{
			Domain:       row.Domain,
			ConceptCount: row.ConceptCount,
			HasReadme:    row.HasReadme,
			Visibility:   row.Visibility,
			CreatedAt:    createdAt,
			UpdatedAt:    updatedAt,
		}
	}
	return domains, nil
}
