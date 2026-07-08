---
name: okp-search
version: 1.1.0
description: "在 Open Knowledge Pool 中搜索和导航知识。面向消费 agent：filters 收窄 → light list → 精读少数 → 沿 links 导航。"
metadata:
  requires:
    bins: ["okp"]
  cliHelp: "okp search --help"
---

# okp-search — 知识搜索 Skill

**CRITICAL — 开始前 MUST 确认 okp CLI 已安装并可连接 API：**

```bash
npm install -g @markbangwu/okp
okp domains   # 确认 API 可达，查看可用的知识领域
```

## 何时使用

- agent 需要查找知识（角色信息、画风参数、操作手册等）。
- 回答用户问题时需要检索知识池中的结构化知识。
- 遍历概念之间的关系（通过 links）。

## 检索循环（严格按此顺序）

### Step 1: 确定搜索范围

```bash
okp domains    # 列出所有领域及数量，确定目标 domain
```

如果不确定 domain 的数据内容和字段规范，先读 README：

```bash
okp domain <domain>   # 打印该 domain 的 README（含 frontmatter schema）
```

### Step 2: 结构过滤收窄

```bash
okp search --domain feishu-social --type Link
okp search --domain stardew-valley --tag 农业
okp search --domain artist-styles --type Artist
```

### Step 3: 文本搜索

```bash
# 单词或短语
okp search "可莉" --domain fandom
okp search "水墨 留白" --domain artist-styles   # 多词自动拆分，每词 ILIKE AND

# 注意：包含 / 的字符串按文本搜索，不当作路径
okp search "emilkowalski/skills"
```

**`match_reason` 字段含义**：
- `id_exact` — 精确 ID 匹配
- `text_match` — 标题/描述文本命中
- `tag_match` — 标签匹配
- `filter_match` — 仅结构过滤命中

### Step 4: 按 frontmatter 字段筛选

搜索结果现在包含 `frontmatter` 字段，可在结果中按字段筛选：

```bash
# 获取 feishu-social 结果后，按 group 过滤
okp search --domain feishu-social | jq '[.[] | select(.frontmatter.group == "feishu-100x")]'

# 按 sender 过滤
okp search --domain feishu-social | jq '[.[] | select(.frontmatter.sender == "kjx")]'
```

### Step 5: Light list → 精读

搜索结果是摘要列表，**不要直接使用正文**。选 1-3 个最相关的用 `okp get` 获取完整内容：

```bash
okp get feishu-social/Link/太离谱了居然可以在自己画的虚拟世界游玩
```

**禁止**批量拉取 body。

### Step 6: 沿关系导航

```bash
okp links feishu-social/Link/太离谱了居然可以在自己画的虚拟世界游玩
# 返回 outgoing（该 concept 引用了谁）和 backlinks（谁引用了它）
```

### Step 7: 不满意时改写查询

1. 换措辞（全名 vs 简称、中文 vs 英文）
2. 去掉 type/tag 限制放宽过滤
3. 换 domain
4. 从已知 concept 沿 links 导航

## 搜索结果字段

| 字段 | 说明 |
|---|---|
| `id` | 路径式唯一标识 `domain/type/slug` |
| `domain` | 知识领域 |
| `type` | 概念类型 |
| `title` | 标题 |
| `description` | 一句话摘要 |
| `tags` | 标签列表 |
| `frontmatter` | 扩展字段（sender、group、date 等，按 domain 不同） |
| `match_reason` | 匹配原因 |

**完整正文通过 `okp get <id>` 获取。**

## 不在本 skill 范围

- 知识导入 → okp-import
- OKF bundle 导出 → `okp export <domain>`
