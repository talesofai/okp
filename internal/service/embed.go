package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/talesofai/okp/internal/model"
	"github.com/talesofai/okp/internal/store"
)

const (
	embedModel     = "text-embedding-3-large"
	embedDim       = 1536 // large 模型分析降维1536，质量仍优于 small
	embedBatchSize = 50
)

var (
	embedAPIBase = envOrDefault("OKP_EMBED_API_BASE", "https://yunwu.ai/v1")
	embedAPIKey  = os.Getenv("OKP_EMBED_API_KEY")
)

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// embedText 返回文本的向量，失败时重试最多3次
func embedText(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	type reqBody struct {
		Model      string   `json:"model"`
		Input      []string `json:"input"`
		Dimensions int      `json:"dimensions,omitempty"`
	}
	type embeddingObj struct {
		Embedding []float32 `json:"embedding"`
	}
	type respBody struct {
		Data []embeddingObj `json:"data"`
	}

	body, _ := json.Marshal(reqBody{Model: embedModel, Input: texts, Dimensions: embedDim})

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt*2) * time.Second)
		}

		req, err := http.NewRequest("POST", embedAPIBase+"/embeddings", bytes.NewReader(body))
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+embedAPIKey)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("HTTP error (attempt %d): %w", attempt+1, err)
			continue
		}
		defer resp.Body.Close()

		respBytes, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			lastErr = fmt.Errorf("API %d: %s", resp.StatusCode, string(respBytes[:min(200, len(respBytes))]))
			continue
		}

		var result respBody
		if err := json.Unmarshal(respBytes, &result); err != nil {
			lastErr = fmt.Errorf("parse error: %w", err)
			continue
		}

		vecs := make([][]float32, len(result.Data))
		for i, d := range result.Data {
			vecs[i] = d.Embedding
		}
		return vecs, nil
	}
	return nil, lastErr
}

// conceptEmbedText 生成 concept 用于 embedding 的文本
// 包含 title + type + tags + description，让向量覆盖元数据和内容
func conceptEmbedText(c *model.Concept) string {
	parts := []string{c.Title}
	// 加入 type 和 domain（如 Character, fandom）
	if c.Type != "" {
		parts = append(parts, c.Type)
	}
	// 加入 tags（如 genshin-impact, pyro, polearm）
	if len(c.Tags) > 0 {
		parts = append(parts, strings.Join([]string(c.Tags), " "))
	}
	// 加入 description
	if c.Description != "" {
		parts = append(parts, c.Description)
	}
	return strings.Join(parts, " — ")
}

// EmbedConcept 为单个 concept 生成并存储向量
func EmbedConcept(id string) error {
	var c model.Concept
	if err := store.DB.First(&c, "id = ?", id).Error; err != nil {
		return err
	}

	text := conceptEmbedText(&c)
	vecs, err := embedText([]string{text})
	if err != nil {
		store.DB.Exec("UPDATE concepts SET embed_status='failed' WHERE id=?", id)
		return fmt.Errorf("embed failed for %s: %w", id, err)
	}

	vec := vecs[0]
	// 存储向量（pgvector 格式 '[f1,f2,...]'）
	vecStr := floatsToVecStr(vec)
	if err := store.DB.Exec(
		"UPDATE concepts SET embedding = ?::vector, embed_status='done' WHERE id=?",
		vecStr, id,
	).Error; err != nil {
		return fmt.Errorf("store embed failed for %s: %w", id, err)
	}
	return nil
}

// EmbedBatch 批量处理 embed_status=pending 或 failed 的 concepts
// 返回 (processed, errors)
func EmbedBatch(domain string, limit int) (int, int) {
	if limit <= 0 {
		limit = 1000
	}

	q := store.DB.Model(&model.Concept{}).
		Where("embed_status IN ('pending','failed')").
		Limit(limit)
	if domain != "" {
		q = q.Where("domain = ?", domain)
	}

	var concepts []model.Concept
	if err := q.Select("id, title, description").Find(&concepts).Error; err != nil {
		slog.Error("embed batch query failed", "error", err)
		return 0, 0
	}

	processed, errors := 0, 0

	// 分批调用 API
	for i := 0; i < len(concepts); i += embedBatchSize {
		end := i + embedBatchSize
		if end > len(concepts) {
			end = len(concepts)
		}
		batch := concepts[i:end]

		texts := make([]string, len(batch))
		for j, c := range batch {
			texts[j] = conceptEmbedText(&c)
		}

		vecs, err := embedText(texts)
		if err != nil {
			slog.Warn("embed batch failed", "start", i, "error", err)
			// 标记为 failed
			ids := make([]string, len(batch))
			for j, c := range batch {
				ids[j] = c.ID
			}
			store.DB.Exec("UPDATE concepts SET embed_status='failed' WHERE id = ANY(?)", ids)
			errors += len(batch)
			continue
		}

		// 逐条更新
		for j, c := range batch {
			vecStr := floatsToVecStr(vecs[j])
			if err := store.DB.Exec(
				"UPDATE concepts SET embedding = ?::vector, embed_status='done' WHERE id=?",
				vecStr, c.ID,
			).Error; err != nil {
				slog.Warn("embed store failed", "id", c.ID, "error", err)
				errors++
			} else {
				processed++
			}
		}

		// 防止 rate limit
		if i+embedBatchSize < len(concepts) {
			time.Sleep(200 * time.Millisecond)
		}
	}

	return processed, errors
}

// EmbedStats 返回 embedding 状态统计
func EmbedStats() map[string]int64 {
	type row struct {
		Status string
		Count  int64
	}
	var rows []row
	store.DB.Model(&model.Concept{}).
		Select("embed_status as status, count(*) as count").
		Group("embed_status").
		Scan(&rows)
	result := map[string]int64{}
	for _, r := range rows {
		result[r.Status] = r.Count
	}
	return result
}

// floatsToVecStr 把 []float32 转成 pgvector 格式字符串 '[f1,f2,...]'
func floatsToVecStr(v []float32) string {
	var sb strings.Builder
	sb.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, "%.8f", f)
	}
	sb.WriteByte(']')
	return sb.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// AsyncEmbed 写入 concept 后异步触发 embedding（非阻塞）
func AsyncEmbed(id string) {
	if embedAPIKey == "" {
		return
	}
	go func() {
		if err := EmbedConcept(id); err != nil {
			slog.Warn("async embed failed", "id", id, "error", err)
		}
	}()
}
