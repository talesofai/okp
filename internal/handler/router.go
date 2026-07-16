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
	r.Post("/api/v1/concepts:batch", batchUpsert)
	r.Get("/api/v1/concepts", listConcepts)
	r.Get("/api/v1/concepts/sample", sampleConcepts)
	r.Put("/api/v1/concepts/*", upsertConcept)
	r.Get("/api/v1/concepts/*", getConcept)

	// links 独立资源: /api/v1/links/{id-with-slashes}
	r.Get("/api/v1/links/*", getConceptLinks)
	r.Put("/api/v1/links/*", putConceptLinks)

	r.Get("/api/v1/domains", listDomains)
	r.Get("/api/v1/domains/{domain}/export", exportDomain)
	r.Get("/api/v1/domains/{domain}", getDomainMeta)
	r.Put("/api/v1/domains/{domain}", putDomainMeta)
	r.Get("/api/v1/embed/stats", embedStats)
	r.Post("/api/v1/embed/batch", embedBatch)
	r.Get("/api/v1/health", healthCheck)
	r.Get("/api/v1/me", meHandler)

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

// GET /api/v1/me
func meHandler(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r)
	authType := auth.AuthTypeFromContext(r)

	var user model.User
	if err := store.DB.Where("uuid = ?", userID).First(&user).Error; err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"uuid":       user.UUID,
		"auth_type":  authType,
		"role":       user.Role,
		"last_seen":  user.LastSeen,
		"created_at": user.CreatedAt,
	})
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

// POST /api/v1/concepts:batch
func batchUpsert(w http.ResponseWriter, r *http.Request) {
	var concepts []model.Concept
	if err := json.NewDecoder(r.Body).Decode(&concepts); err != nil {
		writeError(w, http.StatusBadRequest, "请求体 JSON 解析失败: "+err.Error())
		return
	}

	results := service.BatchPutConcepts(concepts)

	writeJSON(w, http.StatusOK, map[string]any{
		"total":   len(concepts),
		"results": results,
	})
}

// GET /api/v1/concepts
func listConcepts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// frontmatter filters: ?fm[sender]=kjx&fm[group]=feishu-worldbuild
	filters := map[string]string{}
	for key, vals := range q {
		if strings.HasPrefix(key, "fm[") && strings.HasSuffix(key, "]") && len(vals) > 0 {
			field := key[3 : len(key)-1]
			filters[field] = vals[0]
		}
	}

	params := service.SearchParams{
		Query:    q.Get("q"),
		Domain:   q.Get("domain"),
		Type:     q.Get("type"),
		Tags:     q["tag"],
		Scenario: q.Get("scenario"),
		Filters:  filters,
		Sort:     q.Get("sort"),
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
	q := r.URL.Query()
	limit := parseIntDefault(q.Get("limit"), 50)
	offset := parseIntDefault(q.Get("offset"), 0)

	outgoing, backlinks, totalOut, totalBack, err := service.GetLinks(id, limit, offset)
	if err != nil {
		slog.Error("获取链接失败", "error", err)
		writeError(w, http.StatusInternalServerError, "获取链接失败")
		return
	}
	w.Header().Set("X-Total-Outgoing", itoa(totalOut))
	w.Header().Set("X-Total-Backlinks", itoa(totalBack))
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

// GET /api/v1/domains?q=
func listDomains(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	domains, err := service.ListDomains(q)
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
		if strings.Contains(err.Error(), "无 concept") {
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

// GET /api/v1/domains/{domain}
func getDomainMeta(w http.ResponseWriter, r *http.Request) {
	domain := chi.URLParam(r, "domain")
	meta, err := service.GetDomainMeta(domain)
	if err != nil {
		writeError(w, http.StatusNotFound, "domain 没有 README: "+domain)
		return
	}
	writeJSON(w, http.StatusOK, meta)
}

// PUT /api/v1/domains/{domain}
// Body: {"readme": "...markdown..."}  或直接传 markdown 文本
func putDomainMeta(w http.ResponseWriter, r *http.Request) {
	domain := chi.URLParam(r, "domain")
	var body struct {
		Readme string `json:"readme"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "JSON 解析失败: "+err.Error())
		return
	}
	if body.Readme == "" {
		writeError(w, http.StatusBadRequest, "readme 不能为空")
		return
	}
	meta, err := service.PutDomainMeta(domain, body.Readme)
	if err != nil {
		slog.Error("domain meta 写入失败", "domain", domain, "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, meta)
}

// GET /api/v1/concepts/sample
func sampleConcepts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := parseIntDefault(q.Get("limit"), 5)
	results, err := service.Sample(q.Get("domain"), q.Get("type"), limit)
	if err != nil {
		slog.Error("采样失败", "error", err)
		writeError(w, http.StatusInternalServerError, "采样失败")
		return
	}
	writeJSON(w, http.StatusOK, results)
}

// GET /api/v1/embed/stats
func embedStats(w http.ResponseWriter, r *http.Request) {
	stats := service.EmbedStats()
	writeJSON(w, http.StatusOK, stats)
}

// POST /api/v1/embed/batch?domain=X&limit=N
func embedBatch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	domain := q.Get("domain")
	limit := parseIntDefault(q.Get("limit"), 500)

	processed, errors := service.EmbedBatch(domain, limit)
	writeJSON(w, http.StatusOK, map[string]any{
		"processed": processed,
		"errors":    errors,
	})
}
