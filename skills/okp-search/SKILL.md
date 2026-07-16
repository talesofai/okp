---
name: okp-search
version: 1.5.0
description: "在 Open Knowledge Pool 中搜索和导航知识。面向消费 agent：先读 domain README → 按 schema 精确过滤/搜索 → light list → 精读少数 → 沿 links 导航。"
metadata:
  requires:
    bins: ["okp"]
  cliHelp: "okp search --help"
---

# okp-search — 知识搜索 Skill

**CRITICAL — 开始前 MUST 确认 okp CLI 已安装并可连接 API：**

```bash
npm install -g @markbangwu/okp
okp domains   # 确认 API 可达
```

## 何时使用

- agent 需要查找知识（角色、画风、操作手册、社媒分享等）。
- 回答用户问题时需要检索知识池中的结构化知识。
- 遍历概念之间的关系（通过 links）。

## 读权限

当前默认：**已认证用户可读公开 domain**。  
搜索/浏览不要求 domain `writer`。  
写入权限见 okp-import；本 skill 只读。

## 检索循环（严格按此顺序）

### Step 1: 确定 domain

```bash
okp domains              # 列出所有领域
okp domains -q <keyword> # 按名称模糊找领域
```

返回字段：`domain`、`concept_count`、`has_readme`。  
`concept_count=0` 表示已定义 README 但还没有 concept。

### Step 2: 读 domain README（必做）

每个 domain 的 README 定义了：数据是什么、有哪些 type、frontmatter 有哪些可过滤字段、怎么搜。

```bash
okp domain <domain>
```

**禁止跳过 README 直接瞎搜。** 不同 domain 的字段和用法不同，参数要以 README 为准。

### Step 3: 不熟悉时先 sample

```bash
okp sample --domain <domain> --limit 5
```

看几条真实数据，确认 title / tags / frontmatter 长什么样，再决定怎么搜。

### Step 4: 按 README 做精确搜索

根据 README 里的 schema 和 How to use，选择合适的组合：

```bash
# 列出某个 domain 的 concept（无 query 即按 domain 浏览；默认 limit 50）
okp search --domain <domain>
okp search --domain <domain> --limit 100
okp search --domain <domain> --limit 50 --offset 50

# 结构过滤（domain / type / tag 等）
okp search --domain <domain> --type <type>
okp search --domain <domain> --tag <tag>

# frontmatter 过滤（字段名以 README 为准）
okp search --domain <domain> --filter <field>=<value>
okp search --domain <domain> --filter <field>=<value> --sort date:desc

# 文本 / 语义查询
okp search "<query>" --domain <domain>
okp search "<query>" --domain <domain> --type <type>
```

**原则：**
- 要浏览某 domain 下有哪些 concept → `okp search --domain <domain>`（可加 `--limit` / `--offset` 分页）
- 知道具体实体名 → 文本查询
- 知道结构化字段（谁发的、哪个群、哪个 wiki）→ `--filter`，比纯文本更准
- 多字段可叠加，均为 AND
- 时序类数据用 README 推荐的 `--sort`
- 精确字符/子串命中优先；语义结果作为补充（实现细节对用户透明，不必在回答里展开）

### Step 5: Light list → 精读

搜索和 sample 返回摘要列表（id、title、description、tags、frontmatter）。  
**不要直接把列表当正文用。** 选 1–3 个最相关的再 get：

```bash
okp get <id>
```

**禁止**批量拉取 body。

### Step 6: 沿关系导航

```bash
okp links <id>
```

返回 outgoing（引用了谁）和 backlinks（谁引用了它）。

### Step 7: 不满意时改写

1. 再读一遍 domain README，确认字段用对了
2. 换 query 措辞（全名 / 简称 / 中英文）
3. 放宽或收紧 type / tag / filter
4. 换 domain 或用 `sample` 重新摸底

## 常用 flag 参考

| flag | 作用 |
|---|---|
| `--domain` / `-d` | 限定领域 |
| `--type` / `-t` | 限定类型 |
| `--tag` | 限定标签（可重复） |
| `--filter key=val` | 按 frontmatter 字段过滤（字段以 README 为准） |
| `--sort` | 排序，如 `date:desc`、`updated_at:asc`、`title:asc` |
| `--limit` / `-n` | 返回数量 |

## 结果字段

| 字段 | 说明 |
|---|---|
| `id` | 路径式 ID：`domain/type/slug` |
| `domain` / `type` | 领域和类型 |
| `title` / `description` | 标题和摘要 |
| `tags` | 标签 |
| `frontmatter` | 扩展字段（因 domain 而异） |

**完整正文只通过 `okp get <id>` 获取。**

## 不在本 skill 范围

- 知识导入 / 写权限 / 邀请码 → okp-import
- domain README 维护 → `okp domain <domain> --set readme.md`
- OKF bundle 导出 → `okp export <domain>`
