---
name: okp-search
version: 1.2.0
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
okp domains    # 列出所有领域及数量
okp domain <domain>   # 读该 domain 的 README（含 frontmatter schema 定义）
```

### Step 2: 不熟悉 domain 时先 sample

```bash
okp sample --domain feishu-social --limit 5   # 随机采样，了解数据结构
okp sample --domain artist-styles             # 探索未知领域
```

### Step 3: 结构过滤收窄

```bash
okp search --domain feishu-social --type Link
okp search --domain stardew-valley --tag 农业
okp search --domain artist-styles --type BGMReference
```

### Step 4: frontmatter 字段过滤（sequential / grouped 数据专用）

```bash
# 按发送者过滤
okp search --domain feishu-social --filter sender=寇佳新

# 按来源群过滤 + 按日期排序
okp search --domain feishu-social --filter "group=the World Builders" --sort date:desc

# 多字段同时过滤（AND 逻辑）
okp search --domain feishu-social --filter sender=阿头 --filter platform=github

# 时序数据按时间排序
okp search --domain tech-monitor --sort date:desc --limit 10
```

**`--filter` 的字段名来自 domain README 的 schema 定义，先用 `okp domain <domain>` 查阅。**

### Step 5: 文本搜索

```bash
okp search "可莉" --domain fandom
okp search "水墨 留白" --domain artist-styles   # 多词自动拆分，每词 ILIKE AND
okp search "emilkowalski/skills"               # 含 / 的字符串按文本处理
```

**`match_reason` 字段含义**：
- `id_exact` — 精确 ID 匹配
- `text_match` — 标题/描述命中
- `tag_match` — 标签匹配
- `filter_match` — 仅结构过滤命中
- `sample` — 随机采样结果

### Step 6: Light list → 精读

搜索和 sample 返回的是摘要列表，含 `frontmatter` 字段。选 1-3 个最相关的用 `okp get` 获取完整内容（含 body）：

```bash
okp get feishu-social/Link/太离谱了居然可以在自己画的虚拟世界游玩
```

**禁止**批量拉取 body。

### Step 7: 沿关系导航

```bash
okp links feishu-social/Link/太离谱了居然可以在自己画的虚拟世界游玩
```

### Step 8: 不满意时改写查询

1. 换措辞（全名 vs 简称、中文 vs 英文）
2. 去掉 type/tag/filter 限制放宽
3. 换 domain
4. 用 `sample` 重新了解数据分布

## 排序参数（`--sort`）

| 参数 | 说明 | 适用场景 |
|---|---|---|
| `updated_at:desc` | 最近更新优先（默认） | 通用 |
| `updated_at:asc` | 最早更新优先 | 历史回溯 |
| `date:desc` | frontmatter.date 降序 | feishu-social、geopolitics 等时序数据 |
| `date:asc` | frontmatter.date 升序 | 按时间顺序阅读 |
| `title:asc` | 标题字母序 | wiki/entity 类目 |

## 搜索结果字段

| 字段 | 说明 |
|---|---|
| `id` | 路径式唯一标识 `domain/type/slug` |
| `domain` / `type` | 领域和类型 |
| `title` / `description` | 标题和摘要 |
| `tags` | 标签列表 |
| `frontmatter` | 扩展字段（sender、group、date 等，按 domain 不同）|
| `match_reason` | 匹配原因 |

**完整正文通过 `okp get <id>` 获取。**

## 不在本 skill 范围

- 知识导入 → okp-import
- OKF bundle 导出 → `okp export <domain>`
