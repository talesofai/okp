package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	chimw "github.com/go-chi/cors"

	"github.com/talesofai/okp/internal/model"
	auth "github.com/talesofai/okp/internal/middleware"
	"github.com/talesofai/okp/internal/service"
	"github.com/talesofai/okp/internal/store"
)


// NewRouter 构建 HTTP 路由（chi）。
func NewRouter() http.Handler {
	r := chi.NewRouter()

	// 全局中间件
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(chimw.Handler(chimw.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "PUT", "POST", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"X-Total-Count"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	r.Use(auth.Auth)

	// API v1 — concept ID 使用 / 分隔符（OKF 原生格式，如 fandom/genshin-impact/characters/klee）
	// concepts 用 catch-all /* 捕获含 / 的多段 ID
	r.Patch("/api/v1/concepts/*", patchConcept)  // 仅更新 status（不覆盖其他字段）
	r.Post("/api/v1/concepts:batch", batchUpsert)
	r.Get("/api/v1/concepts", listConcepts)
	r.Put("/api/v1/concepts/*", upsertConcept)
	r.Get("/api/v1/concepts/*", getConcept)

	// links 独立资源: /api/v1/links/{id-with-slashes}
	r.Get("/api/v1/links/*", getConceptLinks)
	r.Put("/api/v1/links/*", putConceptLinks)

	r.Get("/api/v1/domains", listDomains)
	r.Get("/api/v1/domains/{domain}/export", exportDomain)
	r.Get("/api/v1/health", healthCheck)

	// Admin
	r.Put("/api/v1/admin/users/{uuid}", updateUserRole)

	return r
}

// ── 响应辅助 ────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("writeJSON 失败", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// ── 端点 ────────────────────────────────────────────────────

func healthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// PUT /api/v1/concepts/{id}
func upsertConcept(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "*")
	var c model.Concept
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeError(w, http.StatusBadRequest, "请求体 JSON 解析失败: "+err.Error())
		return
	}
	c.ID = id

	result, validationErrs, err := service.PutConcept(&c)
	if err != nil {
		slog.Error("upsert 失败", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "写入失败: "+err.Error())
		return
	}
	if len(validationErrs) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error":  "校验失败（L1 硬门禁）",
			"detail": validationErrs,
		})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// PATCH /api/v1/concepts/{id} — 仅更新 status，不覆盖其他字段
func patchConcept(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "*")
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Status != "draft" && body.Status != "accepted" {
		writeError(w, http.StatusBadRequest, "status must be 'draft' or 'accepted'")
		return
	}
	if err := store.DB.Model(&model.Concept{}).Where("id = ?", id).Update("status", body.Status).Error; err != nil {
		writeError(w, http.StatusNotFound, "concept 不存在: "+id)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": id, "status": body.Status})
}

// POST /api/v1/concepts:batch
func batchUpsert(w http.ResponseWriter, r *http.Request) {
	var concepts []model.Concept
	if err := json.NewDecoder(r.Body).Decode(&concepts); err != nil {
		writeError(w, http.StatusBadRequest, "请求体 JSON 解析失败: "+err.Error())
		return
	}

	type batchResult struct {
		ID     string `json:"id"`
		Status string `json:"status"` // "created" | "updated" | "skipped" | "error"
		Error  string `json:"error,omitempty"`
	}
	results := make([]batchResult, 0, len(concepts))

	for _, c := range concepts {
		result, validationErrs, err := service.PutConcept(&c)
		br := batchResult{ID: c.ID}
		if err != nil {
			br.Status = "error"
			br.Error = err.Error()
		} else if len(validationErrs) > 0 {
			br.Status = "error"
			br.Error = "校验失败: " + validationErrs[0].Message
		} else {
			br.Status = "created"
		}
		_ = result
		results = append(results, br)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total":   len(concepts),
		"results": results,
	})
}

// GET /api/v1/concepts
func listConcepts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	params := service.SearchParams{
		Query:    q.Get("q"),
		Domain:   q.Get("domain"),
		Type:     q.Get("type"),
		Tags:     q["tag"], // 支持 ?tag=a&tag=b
		Status:   q.Get("status"),
		Scenario: q.Get("scenario"),
		Limit:    parseIntDefault(q.Get("limit"), 50),
		Offset:   parseIntDefault(q.Get("offset"), 0),
	}

	results, total, err := service.Search(params)
	if err != nil {
		slog.Error("搜索失败", "error", err)
		writeError(w, http.StatusInternalServerError, "搜索失败: "+err.Error())
		return
	}

	w.Header().Set("X-Total-Count", itoa(total))
	writeJSON(w, http.StatusOK, results)
}

// GET /api/v1/concepts/{id}
func getConcept(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "*")
	c, err := service.GetConcept(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "concept 不存在: "+id)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// GET /api/v1/concepts/{id}/links
func getConceptLinks(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "*")
	outgoing, backlinks, err := service.GetLinks(id)
	if err != nil {
		slog.Error("获取链接失败", "error", err)
		writeError(w, http.StatusInternalServerError, "获取链接失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"concept_id": id,
		"outgoing":   outgoing,
		"backlinks":  backlinks,
	})
}

// PUT /api/v1/concepts/{id}/links
func putConceptLinks(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "*")

	var body struct {
		Links []struct {
			ToID    string `json:"to_id"`
			Context string `json:"context,omitempty"`
		} `json:"links"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "请求体 JSON 解析失败: "+err.Error())
		return
	}

	if err := service.PutLinks(id, body.Links); err != nil {
		slog.Error("更新链接失败", "error", err)
		writeError(w, http.StatusInternalServerError, "更新链接失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GET /api/v1/domains
func listDomains(w http.ResponseWriter, r *http.Request) {
	domains, err := service.ListDomains()
	if err != nil {
		slog.Error("获取领域列表失败", "error", err)
		writeError(w, http.StatusInternalServerError, "获取领域列表失败")
		return
	}
	writeJSON(w, http.StatusOK, domains)
}

// GET /api/v1/domains/{domain}/export
func exportDomain(w http.ResponseWriter, r *http.Request) {
	domain := chi.URLParam(r, "domain")
	outDir := r.URL.Query().Get("out")
	if outDir == "" {
		outDir = "./okp-export"
	}

	bundlePath, err := service.ExportDomain(domain, outDir)
	if err != nil {
		if strings.Contains(err.Error(), "无 accepted concept") {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			slog.Error("导出失败", "error", err)
			writeError(w, http.StatusInternalServerError, "导出失败: "+err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"domain": domain,
		"path":   bundlePath,
	})
}

// ── 工具函数 ────────────────────────────────────────────────

func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return def
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	s := ""
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}

// ── Admin ──────────────────────────────────────────────────

// PUT /api/v1/admin/users/{uuid}
// Body: {"role": "writer"} or {"role": "reader"}
// 仅静态 API token 用户可调用
func updateUserRole(w http.ResponseWriter, r *http.Request) {
	uuid := chi.URLParam(r, "uuid")
	if uuid == "" {
		writeError(w, http.StatusBadRequest, "uuid required")
		return
	}

	// 仅 OKP_API_TOKEN 可以管理角色
	if auth.AuthTypeFromContext(r) != "token" {
		writeError(w, http.StatusForbidden, "admin endpoints require OKP_API_TOKEN")
		return
	}

	var body struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Role != "reader" && body.Role != "writer" {
		writeError(w, http.StatusBadRequest, "role must be 'reader' or 'writer'")
		return
	}

	if err := store.DB.Model(&model.User{}).Where("uuid = ?", uuid).Update("role", body.Role).Error; err != nil {
		slog.Error("更新用户角色失败", "uuid", uuid, "error", err)
		writeError(w, http.StatusInternalServerError, "update failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"uuid": uuid, "role": body.Role})
}
