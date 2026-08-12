// Package skills 内嵌 okp 的 agent skills（okp-import、okp-search），
// 供 CLI 通过 `okp skills install` 安装到任意工作区的 .agents/skills/ 下。
// skills 内容随 CLI 版本发布：升级 CLI（okp update）后再 install 即完成更新。
package skills

import "embed"

// FS 包含 okp 的全部 agent skills 文件树（skills/<name>/...）。
// all: 前缀确保 _ 开头的模板文件（如 references/templates/_template.md）也被打包。
//
//go:embed all:okp-import all:okp-search
var FS embed.FS
