---
name: okp-import
version: 1.0.0
description: "将领域知识清洗并导入 Open Knowledge Pool。面向各领域 owner 的 agent：读原始数据 → 查重 → 按模板蒸馏 → 本地校验 → 写入。领域 owner 不碰数据库——只通过本 skill 操作。"
metadata:
  requires:
    bins: ["okp"]
  cliHelp: "okp --help"
---

# okp-import — 知识导入 Skill

**CRITICAL — 开始前 MUST 确认 okp CLI 已安装并可连接 API：**

```bash
# 安装
npm install -g @markbangwu/okp-cli

okp domains    # 确认 API 可达
```

## 何时使用

- 领域 owner 要求将新知识导入统一知识池。
- fandom/wiki/文档/画风描述等原始数据需要清洗后入库。
- 存量数据需要迁移或批量导入。

## 工作流（严格按顺序）

### Step 1: 确认领域和类型

理解要导入的数据属于哪个 `domain` 和 `type`。

```bash
okp domains    # 查看已有领域，确定是新 domain 还是已有 domain
```

查看该 type 的模板和 golden examples：

模板位置：`skills/okp-import/references/templates/<type>.md`（本 skill 目录下的 references/）
如果模板不存在，参考 `references/templates/_template.md` 创建。

### Step 2: 查重（search-before-insert）

在写入前，用目标的 title 搜索已有概念，避免重复：

```bash
okp search "<title>" --domain <domain> --type <type>
```

如果查到高度相似的已有概念（`match_reason: text_match`），则：
- 如果是同一概念 → 使用已有 id 做更新
- 如果是不同概念 → 修改 title 以区分

### Step 3: 按模板蒸馏

参照 `references/templates/<type>.md` 将原始数据转换为 OKP concept JSON。

关键要求：
- `id`：路径式，必须唯一。格式 `{domain}/{type}/{slug}`
- `domain`、`type`：必填
- `provenance`：必填，至少包含 `source`、`agent`、`raw_ref`
- `description`：一句话摘要，不超过 500 字符
- `body`：markdown 正文，结构化优先（表格/列表/标题）
- `frontmatter`：扩展字段放这里（如 scenario、source_id 等）
- `tags`：至少 1 个标签

### Step 4: 本地校验

```bash
cat concept.json | okp put <id>   # API 端会自动执行 L1 硬门禁校验
```

校验失败时，API 返回 422 + 具体修复建议（`detail` 数组里的 `fix` 字段）。按提示修改后重试。

**常见失败：**
- `provenance.source` 为空 → 填写数据来源
- `provenance.agent` 为空 → 填写 `"okp-import/1.0"`
- `description` 过长 → 精简到 500 字符以内
- `title` 疑似重复 → 检查是否已存在同 type 概念

### Step 5: 写入

```bash
# 单个写入
echo '<concept-json>' | okp put <id>

# 从文件写入
okp put <id> -f concept.json

# 批量写入（NDJSON 格式，每行一个 concept JSON）
okp batch concepts.ndjson
```

写入成功返回完整的 concept 对象（含 `content_hash`、`updated_at`）。
写入内容与已有 concept 完全相同时（`content_hash` 匹配）→ skip，返回已有记录。

### Step 6: 追加链接（可选）

如果概念之间有引用关系：

```bash
# 查看当前链接
okp links <id>

# 链接通过 API 操作：
# PUT /api/v1/concepts/<id>/links
# body: {"links": [{"to_id": "...", "context": "references"}]}
```

### Step 7: 汇报

每次导入结束后，向 owner 汇报：

```
本次导入 domain=<domain>, type=<type>:
- 新建: N 个
- 跳过（未变更）: N 个
- 失败: N 个（附具体错误）
- 待人工确认疑似重复: N 个

可使用 okp search --domain <domain> --type <type> 验证导入结果。
可使用 okp export <domain> 导出 OKF bundle 供人类 review。
```

## id 命名规范

```
{domain}/{type}/{slug}

例：
  fandom/genshin-impact/characters/klee
  art-style/chinese-ink/technique/wash-and-line
  nieta-wiki/playbook/deployment-checklist
```

- slug 使用 kebab-case
- 避免中文、空格、特殊字符（使用拼音或英文翻译）
- 同一 domain+type 下 slug 唯一

## provenance 契约

```json
{
  "provenance": {
    "source": "fandom-crawl",
    "agent": "okp-import/1.0",
    "raw_ref": "https://genshin-impact.fandom.com/wiki/Klee",
    "content_hash": "<自动计算>",
    "imported_at": "<自动填充>"
  }
}
```

- `source`：数据来源标识（`fandom-crawl`/`manual`/`agent-import`/`docs-export`）
- `agent`：写入方标识，建议 `okp-import/{version}`
- `raw_ref`：原始数据的 URL 或路径，方便溯源

## 批量导入

对于大规模存量迁移（如 fandom 角色卡），使用 NDJSON 格式 + `okp batch`：

```bash
okp batch fandom-characters.ndjson
```

NDJSON 格式（每行一个完整 concept JSON）：

```jsonl
{"id":"fandom/genshin-impact/characters/klee","domain":"fandom","type":"Character","title":"Klee",...}
{"id":"fandom/genshin-impact/characters/venti","domain":"fandom","type":"Character","title":"Venti",...}
```

批次大小建议 ≤ 500 条/次。

## 注意事项

- **幂等安全**：重复导入同一 concept（内容未变）自动 skip，不会产生重复数据。
- **原始数据不进池**：只导入蒸馏后的 concept，不要把原始 wiki 页面全文当 concept body。
- **图像走 R2 URI**：concept 的 `resource` 字段存图像 R2 URI，不存 base64。

## 不在本 skill 范围

- 知识搜索和消费 → okp-search
- 数据库管理、API 部署 → 运维团队
- 原始数据爬取和存储 → 各 domain 自己的数据管道

## 模板文件

`references/templates/` 下的模板文件定义了各 type 的标准格式和 golden examples。导入前先读取对应模板。

- `references/templates/_template.md` — 通用模板和说明
- `references/templates/Character.md` — 角色类模板（示例）
- `references/templates/ArtStyle.md` — 画风类模板（示例）
