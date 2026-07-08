---
name: okp-search
version: 1.0.0
description: "在 Open Knowledge Pool 中搜索和导航知识。面向消费 agent：filters 收窄 → light list → 精读少数 → 沿 links 导航。"
metadata:
  requires:
    bins: ["okp"]
  cliHelp: "okp search --help"
---

# okp-search — 知识搜索 Skill

**CRITICAL — 开始前 MUST 确认 okp CLI 已安装并可连接 API：**

```bash
# 安装
npm install -g okp-cli

okp domains   # 确认 API 可达，查看可用的知识领域
```

## 何时使用

- agent 需要查找知识（角色信息、画风参数、操作手册等）。
- 回答用户问题时需要检索知识池中的结构化知识。
- 遍历概念之间的关系（通过 links）。

## 检索循环（严格按此顺序）

### Step 1: 确定搜索范围

```bash
okp domains    # 列出所有领域，确定目标 domain
```

### Step 2: 结构过滤收窄

用领域、类型、标签、场景等过滤条件缩小范围：

```bash
# 查 fandom 领域所有角色
okp search --domain fandom --type Character

# 查画风领域的水墨类
okp search --domain art-style --type ArtStyle --tag chinese-ink

# 查特定场景
okp search --domain art-style --scenario character-illustration
```

### Step 3: 文本搜索（当知道具体名称时）

```bash
# 精确实体名
okp search "可莉" --domain fandom --type Character

# 模糊概念
okp search "水墨" --domain art-style
```

**`match_reason` 字段含义**：
- `id_exact` — 精确 ID 匹配（最高置信度）
- `text_match` — 文本/标题相似（需确认相关性）
- `tag_match` — 标签匹配
- `filter_match` — 仅结构过滤命中

**当 `match_reason` 为 `text_match` 时，agent 应检查结果的相关性再决定是否使用。**

### Step 4: Light list → 精读

搜索结果返回的是轻量列表（id + title + description + match_reason），**不要直接使用**。

选择最相关的 1-3 个 concept，用 `okp get` 获取完整内容：

```bash
okp get fandom/genshin-impact/characters/klee
```

**禁止行为**：批量拉取 body。只精读筛选后的少数几个。

### Step 5: 沿关系导航

查看 concept 的引用和被引用关系，发现关联知识：

```bash
okp links fandom/genshin-impact/characters/klee
```

返回：
- `outgoing` — 该 concept 引用了哪些概念
- `backlinks` — 哪些概念引用了该 concept

### Step 6: 不满意时改写查询

如果搜索结果不理想：
1. 尝试不同的 query 措辞（全名 vs 简称、中文 vs 英文）
2. 放宽过滤条件（去掉 type/tag 限制）
3. 换一个领域（`okp domains` 查看其他 domain）
4. 沿 links 图导航（从已知 concept 出发）

## 查询技巧

### 已知具体名称 → 路径式查询

```bash
okp search "fandom/genshin-impact"   # 按路径前缀
okp get fandom/genshin-impact/characters/klee  # 精确 ID
```

### 模糊概念 → 标签过滤 + 文本

```bash
okp search "火元素" --domain fandom --tag pyro
```

### 探索性浏览 → 结构过滤

```bash
okp search --domain art-style    # 查所有画风
okp search --domain fandom --type Character --limit 20  # 查所有角色
```

## 结果解读

| 字段 | 说明 |
|------|------|
| `id` | 唯一标识，可预测可构造 |
| `domain` | 知识领域 |
| `type` | 概念类型 |
| `title` | 人类可读标题 |
| `description` | 一句话摘要（卡片目录） |
| `match_reason` | 为什么匹配（agent 据此判断置信度） |
| `status` | 概念状态（draft / accepted） |

**完整内容通过 `okp get <id>` 获取，即使已经出现在搜索结果中也必须重新 get。**

## 不在本 skill 范围

- 知识导入和清洗 → okp-import
- 数据质量/去重检查 → `okp lint`（通过 okp-import 或手动执行）
- OKF bundle 导出供人类 review → `okp export <domain>`
