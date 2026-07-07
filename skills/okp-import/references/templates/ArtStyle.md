# ArtStyle 模板 — 画风

类型 `type`: `ArtStyle`。

注意：画风是图像密集型知识。当前 concept body 以 markdown 文本描述为主，**视觉参考通过 `resource` 字段引用 R2 URI**。正文中可嵌入 markdown 图片链接。

## Golden Example

```json
{
  "id": "art-style/chinese-ink/technique/wash-and-line",
  "domain": "art-style",
  "type": "ArtStyle",
  "title": "水墨勾线",
  "description": "中国传统水墨画的现代数字改编风格，强调线条的流动感和墨色的浓淡层次",
  "tags": ["chinese-ink", "traditional", "line-art", "monochrome"],
  "frontmatter": {
    "scenario": "character-illustration",
    "medium": "digital-painting",
    "color_palette": "monochrome",
    "line_weight": "varied",
    "ink_wash_level": "medium"
  },
  "body": "## 概述\n\n水墨勾线风格以中国传统水墨画为基础，通过数字工具还原毛笔的笔触感和墨色层次。核心特征是线条粗细变化丰富、墨色浓淡自然过渡。\n\n## 视觉特征\n\n- **线条**：毛笔笔触感，粗细变化明显，飞白效果\n- **墨色**：焦浓重淡清五色层次\n- **留白**：大面积留白，以无胜有\n- **构图**：散点透视，自上而下\n\n## 参考图像\n\n![水墨勾线示例](https://r2.neta.art/art-style/chinese-ink/wash-and-line-ref-01.png)\n\n## 适用场景\n\n- 古风角色立绘\n- 仙侠/武侠场景\n- 文学插图\n\n## 技术参数\n\n| 参数 | 建议值 |\n|------|--------|\n| 画布尺寸 | 2048×3072 |\n| 笔刷 | ink-brush-03 |\n| 墨色浓度 | 60-80% |",
  "resource": "https://r2.neta.art/art-style/chinese-ink/wash-and-line-ref-01.png",
  "provenance": {
    "source": "manual-curation",
    "agent": "okp-import/1.0",
    "raw_ref": "画风设计师 A 的手册 v3"
  }
}
```

## 特有字段

- `frontmatter.scenario` — 适用场景（如 `character-illustration`、`scene-design`）
- `frontmatter.medium` — 媒材（`digital-painting`、`oil`、`watercolor` 等）
- `frontmatter.color_palette` — 主色调
- `frontmatter.line_weight` — 线条风格

## 图像处理

- **不存 base64**：concept 不内嵌图像数据。
- 参考图上传到 R2，`resource` 存主图 URI，正文用 markdown `![alt](r2-uri)` 嵌入。
- 多张参考图：正文中分别嵌入，`resource` 存最具代表性的一张。

## 正文建议结构

```markdown
## 概述
## 视觉特征（列表）
## 参考图像（markdown 图片嵌入）
## 适用场景
## 技术参数（表格）
## 相关画风（链接到其他 ArtStyle concept）
```
