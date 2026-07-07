package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/talesofai/okp/internal/config"
	"github.com/talesofai/okp/internal/handler"
	"github.com/talesofai/okp/internal/store"
)

func main() {
	// 日志
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: parseLogLevel(config.C.LogLevel),
	})))

	slog.Info("okp-api 启动中...")

	// 配置
	config.Load()

	// 数据库
	store.Init()

	// 路由
	r := handler.NewRouter()

	// 启动
	addr := ":" + config.C.APIPort
	slog.Info("API 服务已启动", "addr", addr)
	slog.Info("数据库", "url", maskURL(config.C.DatabaseURL))

	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("服务启动失败", "error", err)
		os.Exit(1)
	}
}

func parseLogLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// maskURL 遮蔽数据库 URL 中的密码
func maskURL(url string) string {
	// 简单遮蔽：替换 :password@ 为 :***@
	atIdx := -1
	colonIdx := -1
	for i := 0; i < len(url); i++ {
		if url[i] == '@' {
			atIdx = i
		}
		if atIdx == -1 && url[i] == ':' && i > 3 {
			colonIdx = i
		}
	}
	if colonIdx > 0 && atIdx > colonIdx {
		return url[:colonIdx+1] + "***" + url[atIdx:]
	}
	return url
}
