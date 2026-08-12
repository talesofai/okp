package service

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/talesofai/okp/internal/model"
)

// WriteDomainBundle 将 concepts 渲染为 OKF bundle 文件树：
//
//	{outDir}/{domain}/
//	  index.md
//	  {type}/
//	    {concept-slug}.md
//	    ...
//
// 不访问数据库：concept 数据由调用方提供（API 流式读取，CLI 流式接收后调用）。
// 同名 slug 冲突时用 content_hash 前 8 位消歧，保证导出无损可逆。
// 重复导出会先清空旧 bundle，避免残留已删除的 concept 文件。
func WriteDomainBundle(domain string, concepts []model.Concept, outDir string) (string, error) {
	if len(concepts) == 0 {
		return "", fmt.Errorf("domain '%s' 下无 concept", domain)
	}

	types := make(map[string][]model.Concept)
	for _, c := range concepts {
		types[c.Type] = append(types[c.Type], c)
	}
	names := assignFileNames(types)

	bundleDir := filepath.Join(outDir, domain)
	if err := os.RemoveAll(bundleDir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(bundleDir, 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "index.md"), []byte(buildIndex(domain, types, names)), 0644); err != nil {
		return "", err
	}
	for _, typeName := range sortedTypeNames(types) {
		typeDir := filepath.Join(bundleDir, sanitizePath(typeName))
		if err := os.MkdirAll(typeDir, 0755); err != nil {
			return "", err
		}
		for _, c := range types[typeName] {
			name := names[c.ID]
			if err := os.WriteFile(filepath.Join(typeDir, name), []byte(renderConceptMD(c)), 0644); err != nil {
				return "", err
			}
		}
	}
	return bundleDir, nil
}

// assignFileNames 为每个 concept 分配文件名：默认用 ID 最后一段；
// 同 type 内 slug 冲突时追加 content_hash 前 8 位，仍冲突则加序号兜底。
func assignFileNames(types map[string][]model.Concept) map[string]string {
	names := make(map[string]string, totalConcepts(types))
	for _, typeName := range sortedTypeNames(types) {
		group := types[typeName]
		slugCount := make(map[string]int, len(group))
		for _, c := range group {
			slugCount[sanitizePath(fileName(c.ID))]++
		}
		used := make(map[string]bool, len(group))
		for _, c := range group {
			slug := sanitizePath(fileName(c.ID))
			name := slug
			if slugCount[slug] > 1 {
				name = fmt.Sprintf("%s-%s", slug, shortHash(c.ContentHash))
			}
			if used[name] {
				for i := 2; ; i++ {
					alt := fmt.Sprintf("%s-%d", name, i)
					if !used[alt] {
						name = alt
						break
					}
				}
			}
			used[name] = true
			names[c.ID] = name + ".md"
		}
	}
	return names
}

func shortHash(h string) string {
	if len(h) >= 8 {
		return h[:8]
	}
	if h == "" {
		return "x"
	}
	return h
}

// buildIndex 生成 OKF index.md
func buildIndex(domain string, types map[string][]model.Concept, names map[string]string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# %s — Knowledge Bundle\n\n", domain))
	b.WriteString("> OKF bundle exported from okp.\n")
	b.WriteString(fmt.Sprintf("> %d concepts across %d types.\n\n", totalConcepts(types), len(types)))

	for _, typeName := range sortedTypeNames(types) {
		group := types[typeName]
		b.WriteString(fmt.Sprintf("## %s\n\n", typeName))
		for _, c := range group {
			desc := c.Description
			if desc == "" {
				desc = c.Title
			}
			b.WriteString(fmt.Sprintf("* [%s](%s/%s) — %s\n", c.Title, sanitizePath(typeName), names[c.ID], desc))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// renderConceptMD 渲染单个 concept 为 OKF markdown
func renderConceptMD(c model.Concept) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("type: %s\n", c.Type))
	if c.Title != "" {
		b.WriteString(fmt.Sprintf("title: %s\n", c.Title))
	}
	if c.Description != "" {
		b.WriteString(fmt.Sprintf("description: %s\n", c.Description))
	}
	if c.Resource != "" {
		b.WriteString(fmt.Sprintf("resource: %s\n", c.Resource))
	}
	if len(c.Tags) > 0 {
		b.WriteString(fmt.Sprintf("tags: [%s]\n", strings.Join(c.Tags, ", ")))
	}
	b.WriteString(fmt.Sprintf("timestamp: %s\n", c.UpdatedAt.Format("2006-01-02T15:04:05Z")))
	b.WriteString("---\n\n")
	b.WriteString(c.Body)
	b.WriteString("\n")

	// provenance 作为尾注
	b.WriteString("\n---\n*Source: ")
	b.WriteString(fmt.Sprintf("%v", c.Provenance["source"]))
	b.WriteString("*\n")
	return b.String()
}

func fileName(id string) string {
	// OKF concept ID 以 / 分隔，取最后一段作为文件名
	parts := strings.Split(id, "/")
	return parts[len(parts)-1]
}

func sanitizePath(s string) string {
	s = strings.NewReplacer("/", "-", "\\", "-").Replace(s)
	if s == "" || s == "." || s == ".." {
		return "_"
	}
	return s
}

func sortedTypeNames(types map[string][]model.Concept) []string {
	names := make([]string, 0, len(types))
	for name := range types {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func totalConcepts(types map[string][]model.Concept) int {
	n := 0
	for _, g := range types {
		n += len(g)
	}
	return n
}
