package service

import (
	"time"

	"github.com/talesofai/okp/internal/model"
	"github.com/talesofai/okp/internal/store"
)

// LintResult 校验结果
type LintResult struct {
	ConceptID string           `json:"concept_id"`
	Errors    []ValidationError `json:"errors,omitempty"`
	Warnings  []string          `json:"warnings,omitempty"`
}

// LintConcept 对单个 concept 执行本地校验（L2 软门禁）。
// 与 ValidatePut（L1 硬门禁）互补：L2 检查可以容忍的"建议修复"项。
func LintConcept(c *model.Concept) LintResult {
	res := LintResult{ConceptID: c.ID}

	// 硬门禁校验
	if errs := ValidatePut(c); len(errs) > 0 {
		res.Errors = errs
	}

	// 软建议检查
	if c.Title == "" {
		res.Warnings = append(res.Warnings, "建议填写 title（搜索和列表展示用）")
	}
	if c.Description == "" {
		res.Warnings = append(res.Warnings, "建议填写 description（agent 卡片目录用一句话摘要）")
	}
	if c.Body == "" {
		res.Warnings = append(res.Warnings, "body 为空：concept 无正文内容")
	}
	if c.Provenance["raw_ref"] == nil || c.Provenance["raw_ref"] == "" {
		res.Warnings = append(res.Warnings, "建议填写 provenance.raw_ref（原始数据引用，方便溯源）")
	}
	if len(c.Tags) == 0 {
		res.Warnings = append(res.Warnings, "建议添加至少一个 tag（方便跨领域分类检索）")
	}

	return res
}

// ── 夜间 lint（L3 异步门禁）──────────────────────────────────

// LintReport 全局 lint 报告
type LintReport struct {
	Orphans      []string `json:"orphans"`       // 无入链无出链的 concept
	BrokenLinks  []BrokenLink `json:"broken_links"` // 指向不存在 concept 的链接
	Duplicates   []Duplicate `json:"duplicates"`   // 疑似重复（同 domain+type 下 title 高度相似）
	Stale        []string `json:"stale"`          // 超过 90 天未更新的 concept
}

type BrokenLink struct {
	FromID string `json:"from_id"`
	ToID   string `json:"to_id"`
}

type Duplicate struct {
	ID1   string  `json:"id1"`
	ID2   string  `json:"id2"`
	Score float64 `json:"similarity_score"`
}

// RunLint 执行全局 lint 检查（夜间定时任务用）。
func RunLint() (*LintReport, error) {
	db := store.DB
	report := &LintReport{}

	// 断链：to_id 在 concepts 中不存在
	db.Raw(`
		SELECT l.from_id, l.to_id FROM links l
		LEFT JOIN concepts c ON l.to_id = c.id
		WHERE c.id IS NULL
	`).Scan(&report.BrokenLinks)

	// 孤儿：未被任何 link 引用且自身无出链
	db.Raw(`
		SELECT c.id FROM concepts c
		WHERE c.id NOT IN (SELECT DISTINCT from_id FROM links)
		AND c.id NOT IN (SELECT DISTINCT to_id FROM links)
	`).Scan(&report.Orphans)

	// 疑似重复：同 domain+type 下 title 高度相似
	if store.IsSQLite {
		// SQLite：精确同名检查（简化版）
		db.Raw(`
			SELECT a.id as id1, b.id as id2, 1.0 as score
			FROM concepts a
			JOIN concepts b ON a.domain = b.domain AND a.type = b.type AND a.id < b.id
			WHERE a.title = b.title
			LIMIT 100
		`).Scan(&report.Duplicates)
	} else {
		// PG：trgm similarity
		db.Raw(`
			SELECT a.id as id1, b.id as id2, similarity(a.title, b.title) as score
			FROM concepts a
			JOIN concepts b ON a.domain = b.domain AND a.type = b.type AND a.id < b.id
			WHERE similarity(a.title, b.title) > 0.85
			ORDER BY score DESC
			LIMIT 100
		`).Scan(&report.Duplicates)
	}

	// 陈旧概念（90 天未更新）
	if store.IsSQLite {
		ninetyDaysAgo := time.Now().Add(-90 * 24 * time.Hour)
		db.Model(&model.Concept{}).
			Where("updated_at < ?", ninetyDaysAgo).
			Pluck("id", &report.Stale)
	} else {
		db.Model(&model.Concept{}).
			Where("updated_at < NOW() - INTERVAL '90 days'").
			Pluck("id", &report.Stale)
	}

	return report, nil
}
