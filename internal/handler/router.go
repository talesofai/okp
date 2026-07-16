package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	chimw "github.com/go-chi/cors"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	auth "github.com/talesofai/okp/internal/middleware"
	"github.com/talesofai/okp/internal/model"
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
	r.Put("/api/v1/me/profile", updateMyProfile)

	// Domain members & invite codes
	r.Get("/api/v1/domains/{domain}/members", listDomainMembers)
	r.Post("/api/v1/domains/{domain}/invites", createInvite)
	r.Get("/api/v1/domains/{domain}/invites", listInvites)
	r.Delete("/api/v1/domains/{domain}/invites/{id}", revokeInvite)
	r.Post("/api/v1/invites/accept", acceptInvite)

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
		"uuid":         user.UUID,
		"auth_type":    authType,
		"role":         user.Role,
		"username":     user.Username,
		"display_name": user.DisplayName,
		"avatar_url":   user.AvatarURL,
		"last_seen":    user.LastSeen,
		"created_at":   user.CreatedAt,
		"domains":      auth.GetUserDomains(userID),
	})
}

// PUT /api/v1/me/profile
// Body: {"username": "...", "display_name": "...", "avatar_url": "..."}
func updateMyProfile(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r)

	var body struct {
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		AvatarURL   string `json:"avatar_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	updates := map[string]any{
		"username":     body.Username,
		"display_name":  body.DisplayName,
		"avatar_url":    body.AvatarURL,
	}
	if err := store.DB.Model(&model.User{}).Where("uuid = ?", userID).Updates(updates).Error; err != nil {
		slog.Error("update profile failed", "uuid", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "update failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
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

	if c.Domain == "" {
		writeError(w, http.StatusBadRequest, "domain 不能为空")
		return
	}
	if !auth.CanWriteDomain(auth.UserIDFromContext(r), c.Domain) {
		writeError(w, http.StatusForbidden, "write access denied: requires admin or domain host/writer")
		return
	}

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

	// Check write permission for all domains in the batch
	userID := auth.UserIDFromContext(r)
	seen := map[string]bool{}
	for _, c := range concepts {
		if c.Domain == "" {
			writeError(w, http.StatusBadRequest, "concept domain 不能为空: "+c.ID)
			return
		}
		if !seen[c.Domain] {
			seen[c.Domain] = true
			if !auth.CanWriteDomain(userID, c.Domain) {
				writeError(w, http.StatusForbidden, "write access denied for domain: "+c.Domain)
				return
			}
		}
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

// ── Domain members & invite codes ──────────────────────────

// GET /api/v1/domains/{domain}/members
func listDomainMembers(w http.ResponseWriter, r *http.Request) {
	domain := chi.URLParam(r, "domain")
	type memberResponse struct {
		Domain      string    `json:"domain"`
		UserID      string    `json:"user_id"`
		Role        string    `json:"role"`
		Username    string    `json:"username"`
		DisplayName string    `json:"display_name"`
		AvatarURL   string    `json:"avatar_url"`
		CreatedAt   time.Time `json:"created_at"`
		UpdatedAt   time.Time `json:"updated_at"`
	}

	var members []memberResponse
	if err := store.DB.Table("domain_members").
		Select("domain_members.domain, domain_members.user_id, domain_members.role, domain_members.created_at, domain_members.updated_at, COALESCE(users.username, '') AS username, COALESCE(users.display_name, '') AS display_name, COALESCE(users.avatar_url, '') AS avatar_url").
		Joins("LEFT JOIN users ON users.uuid = domain_members.user_id").
		Where("domain_members.domain = ?", domain).
		Order("domain_members.created_at asc").
		Scan(&members).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	writeJSON(w, http.StatusOK, members)
}

// POST /api/v1/domains/{domain}/invites
// Body: {"role":"writer","expires_in_hours":72,"max_uses":1}
// Returns invite metadata plus plaintext "code" once.
func createInvite(w http.ResponseWriter, r *http.Request) {
	domain := chi.URLParam(r, "domain")
	userID := auth.UserIDFromContext(r)
	userRole := auth.ResolveDomainRole(userID, domain)
	if userRole != "admin" && userRole != "host" {
		writeError(w, http.StatusForbidden, "only admin or host can create invites")
		return
	}

	var body struct {
		Role           string `json:"role"`
		ExpiresInHours int    `json:"expires_in_hours"`
		MaxUses        int    `json:"max_uses"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Role == "" {
		body.Role = "writer"
	}
	// Phase 1: public domains only invite writers. reader/host not allowed here.
	if body.Role != "writer" {
		writeError(w, http.StatusBadRequest, "role must be writer")
		return
	}
	if body.ExpiresInHours <= 0 {
		body.ExpiresInHours = 72
	}
	if body.ExpiresInHours > 24*30 {
		writeError(w, http.StatusBadRequest, "expires_in_hours must be <= 720")
		return
	}
	if body.MaxUses <= 0 {
		body.MaxUses = 1
	}
	if body.MaxUses > 100 {
		writeError(w, http.StatusBadRequest, "max_uses must be <= 100")
		return
	}

	code, err := generateInviteCode()
	if err != nil {
		slog.Error("generate invite code failed", "error", err)
		writeError(w, http.StatusInternalServerError, "create failed")
		return
	}
	now := time.Now().UTC()
	invite := model.DomainInvite{
		ID:        uuid.NewString(),
		CodeHash:  hashInviteCode(code),
		Domain:    domain,
		Role:      body.Role,
		CreatedBy: userID,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Duration(body.ExpiresInHours) * time.Hour),
		MaxUses:   body.MaxUses,
		UsedCount: 0,
	}
	if err := store.DB.Create(&invite).Error; err != nil {
		slog.Error("create invite failed", "error", err)
		writeError(w, http.StatusInternalServerError, "create failed")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":         invite.ID,
		"domain":     invite.Domain,
		"role":       invite.Role,
		"created_by": invite.CreatedBy,
		"created_at": invite.CreatedAt,
		"expires_at": invite.ExpiresAt,
		"max_uses":   invite.MaxUses,
		"used_count": invite.UsedCount,
		"code":       code,
		"work_url":   "https://cohub.run/koujiaxin/real-canvas/w/okp",
		"share_text": formatInviteShareText(invite.Domain, invite.Role, code),
	})
}

// GET /api/v1/domains/{domain}/invites
func listInvites(w http.ResponseWriter, r *http.Request) {
	domain := chi.URLParam(r, "domain")
	userRole := auth.ResolveDomainRole(auth.UserIDFromContext(r), domain)
	if userRole != "admin" && userRole != "host" {
		writeError(w, http.StatusForbidden, "only admin or host can view invites")
		return
	}
	var invites []model.DomainInvite
	if err := store.DB.Where("domain = ?", domain).Order("created_at desc").Find(&invites).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	out := make([]map[string]any, 0, len(invites))
	now := time.Now().UTC()
	for _, inv := range invites {
		status := "active"
		if inv.RevokedAt != nil {
			status = "revoked"
		} else if inv.UsedCount >= inv.MaxUses {
			status = "exhausted"
		} else if !inv.ExpiresAt.IsZero() && inv.ExpiresAt.Before(now) {
			status = "expired"
		}
		out = append(out, map[string]any{
			"id":           inv.ID,
			"domain":       inv.Domain,
			"role":         inv.Role,
			"created_by":   inv.CreatedBy,
			"created_at":   inv.CreatedAt,
			"expires_at":   inv.ExpiresAt,
			"max_uses":     inv.MaxUses,
			"used_count":   inv.UsedCount,
			"revoked_at":   inv.RevokedAt,
			"last_used_at": inv.LastUsedAt,
			"last_used_by": inv.LastUsedBy,
			"status":       status,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// DELETE /api/v1/domains/{domain}/invites/{id}
func revokeInvite(w http.ResponseWriter, r *http.Request) {
	domain := chi.URLParam(r, "domain")
	id := chi.URLParam(r, "id")
	userRole := auth.ResolveDomainRole(auth.UserIDFromContext(r), domain)
	if userRole != "admin" && userRole != "host" {
		writeError(w, http.StatusForbidden, "only admin or host can revoke invites")
		return
	}
	now := time.Now().UTC()
	res := store.DB.Model(&model.DomainInvite{}).
		Where("id = ? AND domain = ? AND revoked_at IS NULL", id, domain).
		Update("revoked_at", now)
	if res.Error != nil {
		writeError(w, http.StatusInternalServerError, "revoke failed")
		return
	}
	if res.RowsAffected == 0 {
		writeError(w, http.StatusNotFound, "invite not found or already revoked")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "revoked", "id": id})
}

// POST /api/v1/invites/accept
// Body: {"code":"OKP-XXXX-XXXX"}
func acceptInvite(w http.ResponseWriter, r *http.Request) {
	userUUID := auth.UserIDFromContext(r)
	if userUUID == "" {
		writeError(w, http.StatusUnauthorized, "missing user")
		return
	}

	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	code := normalizeInviteCode(body.Code)
	if code == "" {
		writeError(w, http.StatusBadRequest, "code is required")
		return
	}
	codeHash := hashInviteCode(code)

	var accepted model.DomainMember
	var inviteDomain, inviteRole string

	err := store.DB.Transaction(func(tx *gorm.DB) error {
		var invite model.DomainInvite
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("code_hash = ?", codeHash).
			First(&invite).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return errInviteNotFound
			}
			return err
		}
		now := time.Now().UTC()
		if invite.RevokedAt != nil {
			return errInviteRevoked
		}
		if !invite.ExpiresAt.IsZero() && invite.ExpiresAt.Before(now) {
			return errInviteExpired
		}
		if invite.UsedCount >= invite.MaxUses {
			return errInviteExhausted
		}
		if invite.Role != "reader" && invite.Role != "writer" {
			return errInviteInvalidRole
		}

		// Do not downgrade existing higher role.
		var existing model.DomainMember
		err := tx.Where("domain = ? AND user_id = ?", invite.Domain, userUUID).First(&existing).Error
		if err == nil {
			if roleRank(existing.Role) >= roleRank(invite.Role) {
				if err := tx.Model(&model.DomainInvite{}).Where("id = ?", invite.ID).Updates(map[string]any{
					"used_count":   invite.UsedCount + 1,
					"last_used_at": now,
					"last_used_by": userUUID,
				}).Error; err != nil {
					return err
				}
				accepted = existing
				inviteDomain = invite.Domain
				inviteRole = existing.Role
				return nil
			}
		} else if err != gorm.ErrRecordNotFound {
			return err
		}

		member := model.DomainMember{
			Domain:    invite.Domain,
			UserID:    userUUID,
			Role:      invite.Role,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "domain"}, {Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"role", "updated_at"}),
		}).Create(&member).Error; err != nil {
			return err
		}

		if err := tx.Model(&model.DomainInvite{}).Where("id = ?", invite.ID).Updates(map[string]any{
			"used_count":   invite.UsedCount + 1,
			"last_used_at": now,
			"last_used_by": userUUID,
		}).Error; err != nil {
			return err
		}

		accepted = member
		inviteDomain = invite.Domain
		inviteRole = invite.Role
		return nil
	})
	if err != nil {
		switch err {
		case errInviteNotFound:
			writeError(w, http.StatusNotFound, "invite not found")
		case errInviteRevoked:
			writeError(w, http.StatusGone, "invite revoked")
		case errInviteExpired:
			writeError(w, http.StatusGone, "invite expired")
		case errInviteExhausted:
			writeError(w, http.StatusGone, "invite already used up")
		case errInviteInvalidRole:
			writeError(w, http.StatusBadRequest, "invalid invite role")
		default:
			slog.Error("accept invite failed", "error", err)
			writeError(w, http.StatusInternalServerError, "accept failed")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "accepted",
		"domain": inviteDomain,
		"role":   inviteRole,
		"member": accepted,
	})
}

var (
	errInviteNotFound    = errString("invite not found")
	errInviteRevoked     = errString("invite revoked")
	errInviteExpired     = errString("invite expired")
	errInviteExhausted   = errString("invite exhausted")
	errInviteInvalidRole = errString("invite invalid role")
)

type errString string

func (e errString) Error() string { return string(e) }

func roleRank(role string) int {
	switch role {
	case "host":
		return 3
	case "writer":
		return 2
	case "reader":
		return 1
	default:
		return 0
	}
}

func normalizeInviteCode(code string) string {
	code = strings.TrimSpace(strings.ToUpper(code))
	code = strings.ReplaceAll(code, " ", "")
	return code
}

func hashInviteCode(code string) string {
	sum := sha256.Sum256([]byte(normalizeInviteCode(code)))
	return hex.EncodeToString(sum[:])
}

// generateInviteCode returns OKP-XXXX-XXXX using Crockford base32 alphabet
// (no I/L/O/U) for easier human entry.
func generateInviteCode() (string, error) {
	const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	chars := make([]byte, 8)
	for i := 0; i < 8; i++ {
		chars[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return "OKP-" + string(chars[:4]) + "-" + string(chars[4:]), nil
}

func formatInviteShareText(domain, role, code string) string {
	return "邀请你加入 OKP Domain " + domain + "，权限为 " + role + "。\n" +
		"打开 https://cohub.run/koujiaxin/real-canvas/w/okp\n" +
		"输入邀请码 " + code
}
