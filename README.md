<p align="center">
  <a href="https://neta.art">
    <img src="docs/assets/neta.png" alt="Neta.art" width="48" height="48" />
  </a>
  &nbsp;&nbsp;&nbsp;
  <a href="https://cohub.run">
    <img src="docs/assets/cohub.png" alt="Cohub" width="48" height="48" />
  </a>
</p>

<h1 align="center">OKP — Open Knowledge Pool</h1>

<p align="center">
  为 Cohub Agent 打造的结构化知识池。<br/>
  用 Domain · Concept · Link 三层模型管理知识，提供 API / CLI / Skill 三层访问面。
</p>

<p align="center">
  <a href="https://okp.neta.art">🌐 线上实例</a>
  &nbsp;·&nbsp;
  <a href="https://cohub.run/koujiaxin/real-canvas/w/okp">🖥 Cohub 门户</a>
  &nbsp;·&nbsp;
  <a href="https://npmjs.com/package/@markbangwu/okp"><img src="https://img.shields.io/npm/v/@markbangwu/okp" alt="npm" /></a>
</p>

---

## 是什么

OKP 是一个**面向 AI Agent 的开放知识池**。你可以把领域知识（Wiki 文档、社媒分享、操作手册、设计规范…）蒸馏成结构化的 Concept，再由 Agent 通过搜索和关系导航来精确取用。

它是 [Cohub](https://cohub.run) 生态的配套设施，主要服务于 Open Knowledge 场景，也可以在任何主机上独立部署。

## 核心概念

```
Domain → Concept → Link
领域      概念       关系
```

- **Domain**（领域）：一组知识的概念集合，有自己的 README 和 frontmatter schema。**所有 Domain 默认开放读取。**
- **Concept**（概念）：一条结构化知识，包含 title、tags、body（Markdown）、frontmatter（扩展字段）。
- **Link**（关系）：有向边，支持 outgoing（引用了谁）和 backlinks（谁引用了自己）。

## 权限模型

```
admin  >  host  >  writer  >  reader
全局    每 Domain   可通过       所有认证用户
管理员   唯一 host  邀请码获得    默认读取全部
```

- 创建 Domain 时自动获得该 Domain 的唯一 host。
- 邀请码只能授予 `writer`，不能授予 `host` 或 `admin`。
- 没有私有 Domain，所有已认证用户均可读取全部 Domain。

## 快速开始

### 安装 CLI

```bash
npm install -g @markbangwu/okp
```

### 导入知识

```bash
# 1. 创建 domain（写 README 即定义 domain + schema）
okp domain feishu-social --set readme.md

# 2. 导入 concept
okp put feishu-social/Link/example -f concept.json

# 3. 批量导入
okp batch concepts.ndjson
```

### 搜索知识

```bash
okp domains                  # 列出领域
okp domain artist-styles     # 读领域 README
okp search "赛博朋克" -d artist-styles -t Style
okp search -d feishu-social --filter platform=bilibili --sort date:desc
okp get <concept-id>         # 获取完整正文
okp links <concept-id>       # 导航关系
```

### 邀请成员加入 domain

```bash
okp invite create feishu-social --expires-hours 72 --max-uses 1
okp invite accept OKP-XXXX-XXXX
okp invite members feishu-social
```

## 为 Agent 安装 Skills

OKP 提供两个 Agent Skill，可直接安装到 Cohub：

```bash
npx skills add https://github.com/talesofai/okp \
  --skill "okp-search" \
  --agent codex \
  --yes \
  --copy

npx skills add https://github.com/talesofai/okp \
  --skill "okp-import" \
  --agent codex \
  --yes \
  --copy
```

- **okp-search** — 知识搜索 Skill，教 Agent 如何检索和导航知识
- **okp-import** — 知识导入 Skill，教 Agent 如何清洗和导入知识

## CI / 发布

仓库使用 **GitHub Actions**：

- `Deploy`：push 到 `main` 时构建镜像并部署到 K8s（secrets: `REGISTRY_TOKEN`, `KUBE_CONFIG`；镜像仍推送到 `git.talesofai.com` registry）
- 代码仓库只使用 GitHub：`https://github.com/talesofai/okp`（不再推送 Gitea）
- `Publish CLI`：打 `v*` tag 或手动触发，用 npm Trusted Publishing 发布 `@markbangwu/okp`

发布 CLI：

```bash
# 推荐：打 tag 触发
git tag v1.1.3
git push origin v1.1.3

# 或在 GitHub Actions 里 workflow_dispatch，填写 version
```

本地发布（需 npm 登录，一般不需要）：

```bash
./scripts/publish-cli.sh 1.1.3
```

npm 侧请为以下包配置 **Trusted Publisher → GitHub Actions**（repo `talesofai/okp`，workflow `publish-cli.yml`）：

- `@markbangwu/okp`
- `okp-cli-linux-x64`
- `okp-cli-linux-arm64`
- `okp-cli-darwin-x64`
- `okp-cli-darwin-arm64`

## 自托管部署

### Docker

```bash
docker run -d \
  -e OKP_DATABASE_URL=postgres://user:pass@host:5432/okp \
  -e OKP_API_TOKEN=your-static-token \
  -p 8080:8080 \
  ghcr.io/talesofai/okp:latest
```

### 环境变量

| 变量 | 必填 | 说明 |
|---|---|---|
| `OKP_DATABASE_URL` | 是 | PostgreSQL 连接串 |
| `OKP_API_TOKEN` | 是 | 静态 Bearer token（CLI / Agent 使用） |
| `OKP_API_PORT` | 否 | 默认 8080 |
| `OKP_LOG_LEVEL` | 否 | debug / info / warn（默认 warn） |
| `OKP_EMBED_API_URL` | 否 | Embedding API 地址 |
| `OKP_EMBED_API_KEY` | 否 | Embedding API Key |

## API 端点

```
PUT    /api/v1/concepts/*              — upsert concept
POST   /api/v1/concepts:batch          — 批量 upsert
GET    /api/v1/concepts                — 搜索 / 列出
GET    /api/v1/concepts/*              — 单取
PUT    /api/v1/links/*                 — 设置关系
GET    /api/v1/links/*                 — 关系查询
GET    /api/v1/domains                 — 领域清单
GET    /api/v1/domains/{domain}        — 领域 README
PUT    /api/v1/domains/{domain}        — 创建 / 更新领域
GET    /api/v1/domains/{domain}/export — OKF Bundle 导出
GET    /api/v1/me                      — 当前用户
POST   /api/v1/domains/{domain}/invites — 创建邀请码
POST   /api/v1/invites/accept          — 接受邀请
```

## 技术栈

- **后端**：Go 1.26+ · chi v5 · GORM v2 · PostgreSQL
- **前端**：React + TanStack Router + Kumo UI（作为 Cohub Work 运行）
- **CLI**：Cobra，与 API 共享 service 层

## License

MIT
