# Character 模板 — 角色

类型 `type`: `Character`。

## Golden Example

```json
{
  "id": "fandom/genshin-impact/characters/klee",
  "domain": "fandom",
  "type": "Character",
  "title": "可莉 (Klee)",
  "description": "蒙德的火花骑士，西风骑士团的一员，擅长使用炸弹",
  "tags": ["genshin-impact", "pyro", "mondstadt", "playable"],
  "frontmatter": {
    "scenario": "character-design",
    "source_id": "https://genshin-impact.fandom.com/wiki/Klee",
    "game": "genshin-impact",
    "rarity": 5,
    "element": "pyro",
    "weapon": "catalyst",
    "region": "mondstadt"
  },
  "body": "## 概述\n\n可莉是米哈游开放世界动作 RPG《原神》中的五星火元素法器角色。她是蒙德的火花骑士，以爆破物专家著称。\n\n## 核心属性\n\n| 属性 | 值 |\n|------|----|\n| 稀有度 | ★★★★★ |\n| 元素 | 火 |\n| 武器 | 法器 |\n| 所属 | 蒙德 |\n| 命之座 | 四叶草座 |\n| 生日 | 7月27日 |\n\n## 角色故事\n\n可莉是西风骑士团的正式成员，称号\"火花骑士\"。她因对炸弹制作的天赋而闻名，虽然多次因爆炸事件被关禁闭，但从未停止她的爆破实验。\n\n## 相关\n\n- [/fandom/genshin-impact/characters/jean](/fandom/genshin-impact/characters/jean) — 代理团长琴\n- [/fandom/genshin-impact/characters/albedo](/fandom/genshin-impact/characters/albedo) — 阿贝多，可莉的监护人",
  "resource": "https://r2.neta.art/fandom/genshin-impact/characters/klee.png",
  "provenance": {
    "source": "fandom-crawl",
    "agent": "okp-import/1.0",
    "raw_ref": "https://genshin-impact.fandom.com/wiki/Klee"
  }
}
```

## 特有字段

- `frontmatter.game` — 所属游戏
- `frontmatter.rarity` — 稀有度（数字）
- `frontmatter.element` — 元素/属性
- `frontmatter.weapon` — 武器类型
- `frontmatter.region` — 所属区域/地区

## 正文建议结构

```markdown
## 概述
## 核心属性（表格）
## 角色故事/背景
## 能力/技能
## 相关（链接到其他 concept）
```
