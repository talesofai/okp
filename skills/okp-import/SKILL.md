---
name: okp-import
version: 1.2.0
description: "将领域知识清洗并导入 Open Knowledge Pool。面向各领域 owner 的 agent：读 domain README → 查重 → 蒸馏 → 校验 → 写入。"
metadata:
  requires:
    bins: ["okp"]
  cliHelp: "okp --help"
---

# okp-import — 知识导入 Skill

**CRITICAL — 开始前 MUST 确认 okp CLI 已安装并可连接 API：**

```bash
npm install -g @markbangwu/okp
okp domains    # 确认 API 可达
```

## 工作流（严格按顺序）

### Step 1: 读 domain README

每个 domain 有自己的 README，定义了 frontmatter 字段规范。**必须先读。**

```bash
okp domain <domain>          # 打印 README（含 schema 定义）
okp domains                  # 查看所有 domain
```

如果是新 domain，先写 README 再导入：

```bash
okp domain <domain> --set readme.md
```

README 格式（YAML frontmatter 定义 schema）：

```markdown
---
fields:
  sender:
    type: string
    required: true
    description: 飞书发送者用户名
  group:
    type: string
    required: true
    description: 来源飞书群
  platform:
    type: enum
    required: false
    enum: [bilibili, douyin, xiaohongshu, github, youtube]
  date:
    type: string
    required: false
    description: 发布日期 YYYY-MM-DD
---

# feishu-social

飞书社媒分享数据...

## How to contribute
每条 concept 的 frontmatter 必须包含 sender 和 group。
```

### Step 2: 查重（search-before-insert）

```bash
okp search "<title>" --domain <domain> --type <type>
```

- 命中 `text_match` → 判断是否同一概念 → 是则用已有 id 更新，否则改 title 区分

### Step 3: 蒸馏 concept JSON

关键字段：

```json
{
  "id": "feishu-social/Link/太离谱了居然可以在自己画的虚拟世界游玩",
  "domain": "feishu-social",
  "type": "Link",
  "title": "太离谱了！居然可以在自己画的虚拟世界游玩！",
  "description": "一句话摘要，不超过 500 字符",
  "tags": ["AI视频", "虚拟世界"],
  "body": "markdown 正文",
  "frontmatter": {
    "sender": "寇佳新",
    "group": "the World Builders",
    "platform": "bilibili",
    "date": "2026-07-06",
    "url": "https://b23.tv/zyhaEno",
    "likes": "225",
    "views": "6357"
  },
  "provenance": {
    "source": "feishu-sync",
    "agent": "okp-import/1.2",
    "raw_ref": "https://..."
  }
}
```

**frontmatter 按 domain README 的 schema 填写。required 字段不能缺省，否则写入返回 422。**

### Step 4: 写入

```bash
# 单个
okp put <id> -f concept.json

# 批量（NDJSON，每行一个 concept）
okp batch concepts.ndjson
```

422 失败时 API 返回 `fix` 字段说明如何修复：
- `frontmatter.<field> 是必填字段` → 补填该字段
- `疑似重复` → 用 `okp search` 确认是否已有
- `provenance.source 为空` → 填数据来源

### Step 5: 验证

```bash
okp sample --domain <domain> --limit 3     # 随机抽样确认数据结构
okp search --domain <domain> --sort date:desc --limit 5   # 按时间看最新导入
okp get <id>                               # 确认单条完整
```

## id 命名规范

```
{domain}/{type}/{slug}   # slug 用 kebab-case，避免中文和空格
```

## provenance 必填字段

| 字段 | 说明 |
|---|---|
| `source` | 数据来源，如 `feishu-sync`、`manual`、`fandom-crawl` |
| `agent` | 写入方，如 `okp-import/1.2` |
| `raw_ref` | 原始数据 URL 或路径 |

## 不在本 skill 范围

- 知识搜索 → okp-search
- domain README 维护 → `okp domain <domain> --set readme.md`
- 数据爬取 → 各 domain 自己的数据管道
