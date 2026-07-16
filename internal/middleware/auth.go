package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
	"gorm.io/gorm/clause"

	"github.com/talesofai/okp/internal/config"
	"github.com/talesofai/okp/internal/model"
	"github.com/talesofai/okp/internal/store"
)

// ── Context keys ───────────────────────────────────────────

type contextKey string

const (
	UserIDKey   contextKey = "okp_user_id"
	AuthTypeKey contextKey = "okp_auth_type"
)

// UserIDFromContext extracts the authenticated user ID from request context.
func UserIDFromContext(r *http.Request) string {
	if v, ok := r.Context().Value(UserIDKey).(string); ok {
		return v
	}
	return ""
}

// AuthTypeFromContext returns the auth method: "logto", "execution", "token", or "".
func AuthTypeFromContext(r *http.Request) string {
	if v, ok := r.Context().Value(AuthTypeKey).(string); ok {
		return v
	}
	return ""
}

// ── Execution grant validation ──────────────────────────────

type executionGrantHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

type executionGrantPayload struct {
	ActorUserID string `json:"actorUserId"`
	SpaceID     string `json:"spaceId"`
	SessionID   string `json:"sessionId"`
	TurnID      string `json:"turnId"`
	Source      string `json:"source"`
	Exp         int64  `json:"exp"`
	Iat         int64  `json:"iat"`
}

func b64Decode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

func validateExecutionGrant(token string, key string) (actorUserID string, ok bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", false
	}

	signingInput := parts[0] + "." + parts[1]
	providedSig, err := b64Decode(parts[2])
	if err != nil {
		return "", false
	}

	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(signingInput))
	expectedSig := mac.Sum(nil)

	if subtle.ConstantTimeCompare(providedSig, expectedSig) != 1 {
		return "", false
	}

	headerBytes, err := b64Decode(parts[0])
	if err != nil {
		return "", false
	}
	var header executionGrantHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return "", false
	}
	if header.Alg != "HS256" || header.Typ != "JWT" {
		return "", false
	}

	payloadBytes, err := b64Decode(parts[1])
	if err != nil {
		return "", false
	}
	var payload executionGrantPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return "", false
	}
	if payload.Exp <= time.Now().Unix() {
		return "", false
	}
	if payload.SpaceID == "" || payload.Source == "" {
		return "", false
	}

	if payload.ActorUserID != "" {
		return payload.ActorUserID, true
	}
	return "execution:" + payload.SpaceID, true
}

// ── Cohub work_session token validation ─────────────────────
// work_session tokens are HS256 JWTs minted by Cohub's API
// (POST /api/works/:id/session) and signed with the same
// APP_ENCRYPTION_KEY used for execution grants.
// Payload fields: typ="work_session", userUuid, workId, spaceId, exp.

type workSessionPayload struct {
	Typ      string `json:"typ"`
	UserUUID string `json:"userUuid"`
	WorkID   string `json:"workId"`
	SpaceID  string `json:"spaceId"`
	Exp      int64  `json:"exp"`
	Iat      int64  `json:"iat"`
}

func validateWorkSessionToken(token string, key string) (userUUID string, ok bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", false
	}

	signingInput := parts[0] + "." + parts[1]
	providedSig, err := b64Decode(parts[2])
	if err != nil {
		return "", false
	}

	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(signingInput))
	expectedSig := mac.Sum(nil)

	if subtle.ConstantTimeCompare(providedSig, expectedSig) != 1 {
		return "", false
	}

	payloadBytes, err := b64Decode(parts[1])
	if err != nil {
		return "", false
	}
	var payload workSessionPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return "", false
	}

	if payload.Typ != "work_session" {
		return "", false
	}
	if payload.Exp <= time.Now().Unix() {
		return "", false
	}
	if payload.UserUUID == "" {
		return "", false
	}

	return payload.UserUUID, true
}

// ── Logto JWT validation ────────────────────────────────────

var (
	jwksMu        sync.Mutex
	jwksCachedSet jwk.Set
	jwksEndpoint  string
	jwksExpiry    time.Time
)

func getJWKS(ctx context.Context, logtoEndpoint string) (jwk.Set, error) {
	jwksMu.Lock()
	defer jwksMu.Unlock()

	jwksURL := logtoEndpoint + "/oidc/jwks"

	// 缓存命中（1 小时内有效）
	if jwksCachedSet != nil && jwksEndpoint == logtoEndpoint && time.Now().Before(jwksExpiry) {
		return jwksCachedSet, nil
	}

	// 直接 Fetch
	set, err := jwk.Fetch(ctx, jwksURL)
	if err != nil {
		slog.Error("JWKS 获取失败", "endpoint", jwksURL, "error", err)
		return nil, err
	}

	slog.Info("JWKS 获取成功", "endpoint", jwksURL, "keys", set.Len())
	// 确保 key 有正确的 algorithm（Logto JWKS 有时不设 alg）
	for i := 0; i < set.Len(); i++ {
		if key, ok := set.Key(i); ok {
			kid, _ := key.KeyID()
			if _, hasAlg := key.Algorithm(); !hasAlg {
				// key 未声明 alg，设为 RS256（Logto 默认）
				if err := key.Set("alg", "RS256"); err != nil {
					slog.Warn("无法设置 key alg", "kid", kid, "error", err)
				} else {
					slog.Info("已补全 key algorithm", "kid", kid, "alg", "RS256")
				}
			}
		}
	}
	jwksCachedSet = set
	jwksEndpoint = logtoEndpoint
	jwksExpiry = time.Now().Add(1 * time.Hour)
	return set, nil
}

func validateLogtoToken(ctx context.Context, tokenStr string, logtoEndpoint, resource string) (userUUID string, ok bool) {
	jwksSet, err := getJWKS(ctx, logtoEndpoint)
	if err != nil {
		slog.Warn("JWKS 获取失败", "error", err)
		return "", false
	}

	issuer := strings.TrimRight(logtoEndpoint, "/") + "/oidc"

	parsed, err := jwt.Parse([]byte(tokenStr),
		jwt.WithKeySet(jwksSet),
		jwt.WithIssuer(issuer),
		jwt.WithAudience(resource),
		jwt.WithValidate(true),
	)
	if err != nil {
		slog.Warn("JWT 解析失败", "error", err)
		return "", false
	}

	var uuid string
	if err := parsed.Get("talesofai_uuid", &uuid); err == nil && uuid != "" {
		return uuid, true
	}

	if sub, ok := parsed.Subject(); ok && sub != "" {
		return sub, true
	}

	return "", false
}

// ── User tracking ──────────────────────────────────────────

// upsertUser inserts or updates the user's last_seen and auth_type.
// Does NOT modify existing role.
func upsertUser(userID, authType string) {
	if userID == "" || store.DB == nil {
		return
	}
	now := time.Now()
	user := model.User{
		UUID:      userID,
		AuthType:  authType,
		Role:      "reader",
		LastSeen:  now,
		CreatedAt: now,
	}
	if err := store.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "uuid"}},
		DoUpdates: clause.AssignmentColumns([]string{"last_seen", "auth_type"}),
	}).Create(&user).Error; err != nil {
		slog.Warn("用户记录失败", "user_id", userID, "error", err)
	}
}

// isAdmin checks if the user has global admin role.
func isAdmin(userID string) bool {
	if userID == "" || store.DB == nil {
		return false
	}
	var user model.User
	if err := store.DB.Where("uuid = ?", userID).First(&user).Error; err != nil {
		return false
	}
	return user.Role == "admin"
}

// CanWriteDomain checks if the user can write to a specific domain.
// Returns true if user is admin, or is host/writer of the domain.
func CanWriteDomain(userID, domain string) bool {
	if userID == "" || store.DB == nil {
		return false
	}
	// admin can write anywhere
	if isAdmin(userID) {
		return true
	}
	// check domain membership
	var member model.DomainMember
	if err := store.DB.Where("domain = ? AND user_id = ?", domain, userID).First(&member).Error; err != nil {
		return false
	}
	return member.Role == "host" || member.Role == "writer"
}

// ResolveDomainRole returns the user's role for a domain.
// Returns "admin" for global admins, otherwise the domain_members role, defaulting to "reader".
func ResolveDomainRole(userID, domain string) string {
	if userID == "" || store.DB == nil {
		return "reader"
	}
	if isAdmin(userID) {
		return "admin"
	}
	var member model.DomainMember
	if err := store.DB.Where("domain = ? AND user_id = ?", domain, userID).First(&member).Error; err != nil {
		return "reader"
	}
	return member.Role
}

// GetUserDomains returns all domain memberships for a user.
func GetUserDomains(userID string) []model.DomainMember {
	if userID == "" || store.DB == nil {
		return nil
	}
	var members []model.DomainMember
	store.DB.Where("user_id = ?", userID).Find(&members)
	return members
}

// ── Middleware ──────────────────────────────────────────────

// Auth 验证请求身份，支持三种模式（按优先级）：
//  1. Execution Grant（HMAC-SHA256，沙箱 COHUB_EXECUTION_TOKEN）
//  2. Logto JWT（JWKS 验证，用户登录 token）
//  3. 静态 API Token（向下兼容）
//
// 健康检查路径 /api/v1/health 免认证。
// 所有认证用户自动记录到 users 表。
// GET/HEAD/OPTIONS 请求所有认证用户均可访问；写请求需要白名单权限。
func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/health" {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, `{"error":"missing Authorization header"}`, http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == authHeader {
			http.Error(w, `{"error":"Authorization header must use Bearer scheme"}`, http.StatusUnauthorized)
			return
		}

		var userID, authType string

		// 1. 静态 API Token（最快，先检查）
		if config.C.APIToken != "" && token == config.C.APIToken {
			userID = "token:api"
			authType = "token"
		}

		// 2. Execution Grant
		if userID == "" && config.C.ExecutionGrantKey != "" {
			if uid, ok := validateExecutionGrant(token, config.C.ExecutionGrantKey); ok {
				userID = uid
				authType = "execution"
			}
		}

		// 3. Cohub work_session token (for portal/Work frontend)
		if userID == "" && config.C.ExecutionGrantKey != "" && strings.Count(token, ".") == 2 {
			if uid, ok := validateWorkSessionToken(token, config.C.ExecutionGrantKey); ok {
				userID = uid
				authType = "cohub_work"
			}
		}

		// 4. Logto JWT（仅当 token 像 JWT 时才尝试，避免无效网络请求）
		if userID == "" && config.C.LogtoEndpoint != "" && strings.Count(token, ".") == 2 {
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()
			if uid, ok := validateLogtoToken(ctx, token, config.C.LogtoEndpoint, config.C.LogtoResource); ok {
				userID = uid
				authType = "logto"
			}
		}

		if userID == "" {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}

		// 记录用户
		upsertUser(userID, authType)

		// admin 路由仅限静态 API token
		if isAdminRoute(r.URL.Path) && authType != "token" {
			http.Error(w, `{"error":"admin endpoints require OKP_API_TOKEN"}`, http.StatusForbidden)
			return
		}

		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		ctx = context.WithValue(ctx, AuthTypeKey, authType)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func isWriteMethod(method string) bool {
	switch method {
	case "POST", "PUT", "PATCH", "DELETE":
		return true
	}
	return false
}

// isAdminRoute 允许 admin 路由绕过 writer 角色检查（用于 bootstrap）
func isAdminRoute(path string) bool {
	return strings.HasPrefix(path, "/api/v1/admin/")
}
