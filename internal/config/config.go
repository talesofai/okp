package config

import (
	"fmt"
	"os"
	"strings"
)

// C 全局配置实例，在 main 中调用 Load() 初始化
var C Config

type Config struct {
	DatabaseURL  string
	DatabaseType string // "postgres" | "sqlite"
	SQLitePath   string // SQLite 文件路径
	APIPort      string
	APIToken     string
	LogLevel     string
	DomainTokens map[string]string
}

func Load() {
	C = Config{
		DatabaseURL:  os.Getenv("OKP_DATABASE_URL"),
		DatabaseType: envDefault("OKP_DATABASE_TYPE", "postgres"),
		SQLitePath:   envDefault("OKP_SQLITE_PATH", "./okp.db"),
		APIPort:      envDefault("OKP_API_PORT", "8720"),
		APIToken:     requireEnv("OKP_API_TOKEN"),
		LogLevel:     envDefault("OKP_LOG_LEVEL", "info"),
		DomainTokens: parseDomainTokens(os.Getenv("OKP_DOMAIN_TOKENS")),
	}

	// SQLite 模式不需要 DATABASE_URL
	if C.DatabaseType == "sqlite" && C.DatabaseURL == "" {
		C.DatabaseURL = C.SQLitePath // 兼容内部使用
	}
	if C.DatabaseType != "sqlite" && C.DatabaseURL == "" {
		panic("非 SQLite 模式必须设置 OKP_DATABASE_URL")
	}
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("环境变量 %s 未设置", key))
	}
	return v
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
