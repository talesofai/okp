package middleware

import (
	"net/http"
	"strings"

	"github.com/talesofai/okp/internal/config"
)

// Auth 验证 Bearer Token。token 正确或为空配置时放行。
func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 健康检查不需要认证
		if r.URL.Path == "/api/v1/health" {
			next.ServeHTTP(w, r)
			return
		}

		expected := config.C.APIToken
		if expected == "" {
			// 测试/开发模式：无 token 配置时放行
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, `{"error":"missing Authorization header"}`, http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token != expected {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// DomainAuth 验证 per-domain token（可选细粒度控制）。
// 仅当配置了 OKP_DOMAIN_TOKENS 时才校验写入操作。
func DomainAuth(domain, token string) bool {
	if config.C.DomainTokens == nil || len(config.C.DomainTokens) == 0 {
		return true // 未配置 domain token 时全局放行
	}
	expected, ok := config.C.DomainTokens[domain]
	if !ok {
		return true // 该 domain 未配置独立 token，走全局认证
	}
	return token == expected
}
