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

	"github.com/lestrrat-go/httprc/v3"
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

// ── Logto JWT validation ────────────────────────────────────

var (
	jwksMu       sync.Mutex
	jwksCache    *jwk.Cache
	jwksEndpoint string
)

func getJWKS(ctx context.Context, logtoEndpoint string) (jwk.Set, error) {
	jwksMu.Lock()
	defer jwksMu.Unlock()

	if jwksCache != nil && jwksEndpoint == logtoEndpoint {
		return jwksCache.Lookup(ctx, logtoEndpoint+"/oidc/jwks")
	}

	// 用带超时的 HTTP client
	httpClient := httprc.NewClient(
		httprc.WithHTTPClient(&http.Client{Timeout: 15 * time.Second}),
	)

	cache, err := jwk.NewCache(ctx, httpClient)
	if err != nil {
		return nil, err
	}
	if err := cache.Register(ctx, logtoEndpoint+"/oidc/jwks"); err != nil {
		return nil, err
	}
	// 首次刷新，加超时
	fetchCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if _, err := cache.Refresh(fetchCtx, logtoEndpoint+"/oidc/jwks"); err != nil {
		slog.Error("JWKS 首次获取失败", "endpoint", logtoEndpoint+"/oidc/jwks", "error", err)
		return nil, err
	}

	jwksCache = cache
	jwksEndpoint = logtoEndpoint
	slog.Info("JWKS 缓存初始化成功", "endpoint", logtoEndpoint)
	return cache.Lookup(ctx, logtoEndpoint+"/oidc/jwks")
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
		slog.Warn("JWT 解析失败", "error", err, "issuer", issuer, "audience", resource)
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

// canWrite checks the user's role from the database.
// Only users with role="writer" can perform write operations.
func canWrite(userID string) bool {
	if userID == "" || store.DB == nil {
		return false
	}
	var user model.User
	if err := store.DB.Where("uuid = ?", userID).First(&user).Error; err != nil {
		return false
	}
	return user.Role == "writer"
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

		// 3. Logto JWT（仅当 token 像 JWT 时才尝试，避免无效网络请求）
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

		// 写操作需要 writer 角色（admin 路由除外）
		if isWriteMethod(r.Method) && !isAdminRoute(r.URL.Path) && !canWrite(userID) {
			http.Error(w, `{"error":"write access denied: role 'writer' required"}`, http.StatusForbidden)
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
