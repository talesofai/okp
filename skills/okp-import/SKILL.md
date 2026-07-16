---
name: okp-import
version: 1.5.0
description: "将领域知识清洗并导入 Open Knowledge Pool。面向各领域 owner 的 agent：读 domain README → 查重 → 蒸馏 → 校验 → 写入。"
metadata:
  requires:
    bins: ["okp"]
  cliHelp: "okp --help"
---

# okp-import — 知识导入 Skill

**CRITICAL — 开始前 MUST 确认 okp CLI 已安装并可连接 API：**

```bash
npm install -g @markbangwu/okp@1.1.1
okp domains    # 确认 API 可达
```

当前推荐 CLI：`@markbangwu/okp@1.1.1`（含 `okp invite`）。

## 写权限（先确认再导入）

写入 concept / 更新 domain README 需要其一：

- 全局 `admin`
- 该 domain 的 `host` 或 `writer`

公开 domain 默认所有人可读，**不代表可写**。

若 `put` / `batch` / `domain --set` 返回 403（write access denied）：

1. 让该 domain 的 host/admin 生成邀请码：
   ```bash
   okp invite create <domain> --expires-hours 72 --max-uses 1
   ```
2. 当前用户接受：
   ```bash
   okp invite accept OKP-XXXX-XXXX
   ```
3. 确认成员身份：
   ```bash
   okp invite members <domain>
   ```

邀请码是短码，不是链接路由。门户右上角「邀请」也可输入同一邀请码。

不要通过 invite 授予 `host`；host 转移是独立流程。

## 工作流（严格按顺序）

### Step 1: 读 domain README

每个 domain 有自己的 README，定义了 frontmatter 字段规范。**必须先读。**

```bash
okp domain <domain>          # 打印 README（含 schema 定义）
okp domains                  # 查看所有 domain
okp domains -q <keyword>     # 模糊搜索领域名
```

如果是新 domain，先写 README 再导入（写 README 即定义 domain，之后 `okp domains` 立即可见）：

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

- 命中语义相近的结果 → 判断是否同一概念 → 是则用已有 id 更新，否则改 title 区分

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
    "agent": "okp-import/1.5",
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

`put` 与 `batch` 行为一致：
- 新写入 / 内容有变更 → 落库后自动进入可检索索引（异步，通常数秒内）
- 内容未变的幂等重导 → 跳过写入；若该条此前未建好索引，服务端会补上
- **不要**再找单独的「建索引 / embed」命令——没有，也不需要

常见失败：

| 状态 | 含义 | 处理 |
|---|---|---|
| 403 | 无 domain 写权限 | `okp invite accept <code>` 或联系 host |
| 422 | frontmatter/校验失败 | 按 `fix` 补字段或改写 |
| 401 | token 无效/过期 | 检查 `OKP_API_TOKEN` / sandbox execution token |

422 的 `fix` 常见项：
- `frontmatter.<field> 是必填字段` → 补填该字段
- `疑似重复` → 用 `okp search` 确认是否已有
- `provenance.source 为空` → 填数据来源

### Step 5: 验证

写入返回成功后稍等片刻再搜（大批量导入可多等几秒）：

```bash
okp sample --domain <domain> --limit 3     # 随机抽样确认数据结构
okp search "<title 关键词>" --domain <domain> --limit 5
okp search --domain <domain> --sort date:desc --limit 5   # 时序类看最新
okp get <id>                               # 确认单条完整
```

若刚写入的 concept 暂时搜不到：再等几秒重试 `okp search` / `okp get`；不要重写一遍，除非内容本身要改。

## id 命名规范

```
{domain}/{type}/{slug}   # slug 用 kebab-case，避免中文和空格
```

## provenance 必填字段

| 字段 | 说明 |
|---|---|
| `source` | 数据来源，如 `feishu-sync`、`manual`、`fandom-crawl` |
| `agent` | 写入方，如 `okp-import/1.5` |
| `raw_ref` | 原始数据 URL 或路径 |

## 邀请相关 CLI（host/admin）

```bash
okp invite create <domain> [--expires-hours 72] [--max-uses 1]
okp invite list <domain>
okp invite revoke <domain> <invite-id>
okp invite accept <code>
okp invite members <domain>
```

- create 时明文 code 只显示一次
- list 不返回明文 code
- 当前阶段公开 domain 邀请角色为 `writer`

## 不在本 skill 范围

- 知识搜索 → okp-search
- domain README 维护之外的门户 UI 操作
- 数据爬取 → 各 domain 自己的数据管道
- 私有 domain / reader 邀请（尚未开放）
