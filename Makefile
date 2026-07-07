.PHONY: build run test lint dev clean

# 二进制输出目录
BIN_DIR=./bin

# 默认目标
all: build

# 编译 API 和 CLI
build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/okp-api ./cmd/api
	go build -o $(BIN_DIR)/okp ./cmd/cli

# 仅编译 API
build-api:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/okp-api ./cmd/api

# 仅编译 CLI
build-cli:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/okp ./cmd/cli

# 运行 API（开发模式，自动加载 .env）
run:
	go run ./cmd/api

# 测试
test:
	go test ./... -v -count=1

# 代码检查
lint:
	go vet ./...

# 开发模式（API 热重载 — 需要 air 工具）
dev:
	@which air > /dev/null || (echo "请先安装 air: go install github.com/air-verse/air@latest"; exit 1)
	air

# 清理
clean:
	rm -rf $(BIN_DIR)

# 数据库迁移（使用 CLI 自动迁移）
migrate:
	go run ./cmd/cli migrate

# 初始化数据库并创建扩展
db-init:
	@echo "请手动执行: CREATE EXTENSION IF NOT EXISTS pg_trgm;"
	@echo "请手动执行: CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\";"

# 导入 .env（如果存在）
-include .env
export
