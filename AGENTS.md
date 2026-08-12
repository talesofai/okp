# AGENTS.md — Project Conventions for okp

## Overview

Open Knowledge Protocol (okp) — open structured knowledge for people and agents. PostgreSQL-backed storage with API/CLI/Skills access surfaces.

## Tech Stack

- **Backend**: Go 1.26+, chi v5 (router), GORM v2 (ORM), pgx (PG driver)
- **Database**: PostgreSQL — 唯一存储层，无 Redis/缓存依赖
- **CLI**: cobra (命令行框架)，与 API 共享 service 层
- **Skills**: 纯 Markdown 指令文件，同构于 lark-* skill 模式

## Architecture

```
cmd/api/          — HTTP API 入口
cmd/cli/          — CLI 入口（开发者工具 + 脚本集成）

internal/
  config/         — 环境变量配置
  model/          — GORM 数据模型（Concept, Link, Revision）
  store/          — 数据库初始化和连接管理
  handler/        — HTTP handlers（6 端点 + health + domains list）
  service/        — 业务逻辑（CRUD, search, lint, export）
  middleware/     — HTTP 中间件（auth token, logging, recovery）

skills/
  okp-import/      — 导入 skill（面向领域 owner 的 agent）
    references/   — 各 type 的 concept 模板和 golden examples
  okp-search/      — 搜索 skill（面向消费 agent）
```

## API 端点

```
PUT    /api/v1/concepts/*              — upsert（L1 硬门禁在此校验；concept ID 含 /，用 catch-all 通配）
DELETE /api/v1/concepts/*              — 删除 concept + links + revisions
POST   /api/v1/concepts:batch          — 批量 upsert
GET    /api/v1/concepts                — list/search（domain/type/tag/status/q）
GET    /api/v1/concepts/*              — 单取
GET    /api/v1/links/*                 — 出链 + 反向引用（links 拆为独立顶层资源）
PUT    /api/v1/links/*                 — 替换概念的出链
GET    /api/v1/domains/{domain}/export — OKF bundle 导出
GET    /api/v1/domains                 — 领域清单
PUT    /api/v1/domains/{domain}        — 创建/更新 README、schema、visibility
DELETE /api/v1/domains/{domain}        — 级联删除 domain 全部数据
GET    /api/v1/health                  — 健康检查
```

**路由设计**：concept ID 用 OKF 原生 `/` 分隔符。Go net/http 和 chi 的 `{id}` 无法跨 `/` 段捕获，故 concepts 用 catch-all `/*`；chi 的 `*` 必须在末尾（`/concepts/*/links` 会 panic），因此 links 拆为独立顶层资源 `/links/*`。好处：ID 保持 OKF 原生格式，export/import 与 OKF bundle 1:1 映射，body cross-reference 可直接解析。

## Data Model (PostgreSQL)

```
concepts  — 主干表，每个 concept 一行（= OKF concept）
links     — 有向关系（from_id → to_id，容忍断链）
revisions — 变更历史（替代 git）
```

详见 `internal/model/` 和 `migrations/001_init.sql`。

## CLI 命令

```
okp put    <id>       — upsert
okp get    <id>       — 单取
okp search <query>    — 搜索
okp batch  <file>     — 批量导入（NDJSON）
okp links  <id>       — 查看关系
okp export <domain>   — OKF 导出
okp lint   <file>     — 本地校验
okp domains           — 领域清单
okp delete <id>       — 删除 concept
```

## 核心设计原则

1. **L1 硬门禁在 API 层**：schema/去重/provenance 校验不可绕过，4xx 返回教学化报错（明确写清怎么改）。
2. **可逆性**：`concepts` 表随时可 dump 为 OKF markdown bundle（`export` 命令）。
3. **池外原始层不进池**：fandom 爬虫数据、wiki 原始页面保留在自己的库里，进池的只有蒸馏后的 concept。
4. **搜索引擎可替换**：初始 trgm+filters，接口隔离（`internal/service/search.go`），后续换 pgroonga/pgvector 不动 API 契约。
5. **provenance 必填**：`{source, agent, raw_ref, content_hash, imported_at}`，幂等重导入依赖 content_hash。
6. **私有领域无 admin 旁路**：private domain 只对显式 host/writer/reader 成员可见；全局 admin 必须受邀，且邀请角色不会升级为管理权限。

## 命名约定

- Package name = last path component, lowercase, no underscores
- Handler files: `{resource}.go` (concept.go, link.go, domain.go)
- Test files: `{resource}_test.go`
- Config: environment variables with `OKP_` prefix (OKP_DATABASE_URL, OKP_API_PORT, OKP_API_TOKEN)
