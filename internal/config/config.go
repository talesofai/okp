package config

import (
	"os"
	"strings"
)

// C 全局配置实例，在 main 中调用 Load() 初始化
var C Config

type Config struct {
	DatabaseURL       string
	DatabaseType      string // "postgres" | "sqlite"
	SQLitePath        string // SQLite 文件路径
	APIPort           string
	APIToken          string // 静态 token（向下兼容，可选）
	LogLevel          string
	DomainTokens      map[string]string

	// Cohub auth
	LogtoEndpoint     string
	LogtoResource     string
	ExecutionGrantKey string
	EmbedAPIKey       string // OpenAI-compatible embedding API key
	EmbedAPIBase      string // embedding API base URL
}

func Load() {
	C = Config{
		DatabaseURL:  os.Getenv("OKP_DATABASE_URL"),
		DatabaseType: envDefault("OKP_DATABASE_TYPE", "postgres"),
		SQLitePath:   envDefault("OKP_SQLITE_PATH", "./okp.db"),
		APIPort:      envDefault("OKP_API_PORT", "8720"),
		APIToken:     os.Getenv("OKP_API_TOKEN"),
		LogLevel:     envDefault("OKP_LOG_LEVEL", "info"),
		DomainTokens: parseDomainTokens(os.Getenv("OKP_DOMAIN_TOKENS")),

		LogtoEndpoint:     envDefault("LOGTO_ENDPOINT", envDefault("OKP_LOGTO_ENDPOINT", "https://auth.neta.art")),
		LogtoResource:     envDefault("OKP_LOGTO_RESOURCE", "https://api.talesofai"),
		ExecutionGrantKey: os.Getenv("OKP_EXECUTION_GRANT_KEY"),
		EmbedAPIKey:       os.Getenv("OKP_EMBED_API_KEY"),
		EmbedAPIBase:      envDefault("OKP_EMBED_API_BASE", "https://yunwu.ai/v1"),
	}

	// SQLite 模式不需要 DATABASE_URL
	if C.DatabaseType == "sqlite" && C.DatabaseURL == "" {
		C.DatabaseURL = C.SQLitePath // 兼容内部使用
	}
	if C.DatabaseType != "sqlite" && C.DatabaseURL == "" {
		panic("非 SQLite 模式必须设置 OKP_DATABASE_URL")
	}

	// API Token 不再强制要求（支持 Logto JWT / Execution Grant 认证）
}

func envDefault(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

// parseDomainTokens 解析 "domain1:token1,domain2:token2" 格式
func parseDomainTokens(raw string) map[string]string {
	m := make(map[string]string)
	if raw == "" {
		return m
	}
	for _, pair := range strings.Split(raw, ",") {
		parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
		if len(parts) == 2 {
			m[parts[0]] = parts[1]
		}
	}
	return m
}
