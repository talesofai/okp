# Concept 模板 — 通用

此文件是所有 type 模板的基类。各具体 type 继承此结构并补充该 type 特有的字段和要求。

## 最小 valid concept

```json
{
  "id": "domain/type/slug",
  "domain": "domain-name",
  "type": "ConceptType",
  "title": "人类可读标题",
  "description": "一句话摘要，不超过 500 字符",
  "provenance": {
    "source": "数据来源标识",
    "agent": "okp-import/1.0",
    "raw_ref": "原始数据 URL 或路径"
  }
}
```

## 完整 concept 结构

```json
{
  "id": "domain/type/slug",
  "domain": "domain-name",
  "type": "ConceptType",
  "title": "人类可读标题",
  "description": "一句话摘要，不超过 500 字符",
  "tags": ["tag1", "tag2"],
  "frontmatter": {
    "scenario": "使用场景",
    "source_id": "来源系统内的 ID"
  },
  "body": "## 概述\n\n...\n\n## 详情\n\n...",
  "resource": "https://r2.example.com/images/xxx.png",
  "provenance": {
    "source": "数据来源标识",
    "agent": "okp-import/1.0",
    "raw_ref": "原始数据 URL 或路径"
  }
}
```

## 字段说明

| 字段 | 必填 | 类型 | 说明 |
|------|------|------|------|
| id | ✅ | string | 路径式唯一 ID，格式 `domain/type/slug` |
| domain | ✅ | string | 知识领域（如 `fandom`、`art-style`） |
| type | ✅ | string | 概念类型（如 `Character`、`ArtStyle`） |
| title | 推荐 | string | 人类可读标题，搜索和列表展示用 |
| description | 推荐 | string | 一句话摘要（≤500字符），agent 卡片目录用 |
| tags | 推荐 | string[] | 至少 1 个，方便跨领域分类检索 |
| frontmatter | - | object | 扩展字段（scenario、source_id 等） |
| body | 推荐 | string | markdown 正文，结构化优先 |
| resource | - | string | 底层资产 URI（图像走 R2） |
| provenance | ✅ | object | 数据溯源，至少含 source/agent/raw_ref |
| provenance.source | ✅ | string | 数据来源标识 |
| provenance.agent | ✅ | string | 写入方标识 |
| provenance.raw_ref | 推荐 | string | 原始数据 URL/路径 |

## 正文（body）建议结构

```markdown
## 概述

一段话概述这是什么知识。

## 核心属性

| 属性 | 值 |
|------|----|
| ... | ... |

## 详情

...

## 相关

- 链接到相关 concept（用 concept id 做 markdown 链接）
```
