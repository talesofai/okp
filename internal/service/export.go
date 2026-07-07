package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/talesofai/okp/internal/model"
	"github.com/talesofai/okp/internal/store"
)

// ExportDomain 将指定 domain 的所有 accepted concept 导出为 OKF bundle 文件树。
// 输出结构：
//
//	{outDir}/{domain}/
//	  index.md
//	  {type}/
//	    {concept-slug}.md
//	    ...
func ExportDomain(domain, outDir string) (string, error) {
	var concepts []model.Concept
	if err := store.DB.Where("domain = ? AND status = ?", domain, "accepted").
		Order("type, id").
		Find(&concepts).Error; err != nil {
		return "", err
	}
	if len(concepts) == 0 {
		return "", fmt.Errorf("domain '%s' 下无 accepted concept", domain)
	}

	bundleDir := filepath.Join(outDir, domain)
	if err := os.MkdirAll(bundleDir, 0755); err != nil {
		return "", err
	}

	// 按 type 分组
	types := make(map[string][]model.Concept)
	for _, c := range concepts {
		types[c.Type] = append(types[c.Type], c)
	}

	// 写 index.md
	indexPath := filepath.Join(bundleDir, "index.md")
	indexContent := buildIndex(domain, types)
	if err := os.WriteFile(indexPath, []byte(indexContent), 0644); err != nil {
		return "", err
	}

	// 写各 concept
	for typeName, group := range types {
		typeDir := filepath.Join(bundleDir, sanitizePath(typeName))
		if err := os.MkdirAll(typeDir, 0755); err != nil {
			return "", err
		}
		for _, c := range group {
			// slug：取 id 最后一段
			slug := fileName(c.ID)
			filePath := filepath.Join(typeDir, slug+".md")
			content := renderConceptMD(c)
			if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
				return "", err
			}
		}
	}

	return bundleDir, nil
}

// buildIndex 生成 OKF index.md
func buildIndex(domain string, types map[string][]model.Concept) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# %s — Knowledge Bundle\n\n", domain))
	b.WriteString(fmt.Sprintf("> OKF bundle exported from okp.\n"))
	b.WriteString(fmt.Sprintf("> %d concepts across %d types.\n\n", totalConcepts(types), len(types)))

	for typeName, group := range types {
		b.WriteString(fmt.Sprintf("## %s\n\n", typeName))
		for _, c := range group {
			slug := fileName(c.ID)
			desc := c.Description
			if desc == "" {
				desc = c.Title
			}
			b.WriteString(fmt.Sprintf("* [%s](%s/%s.md) — %s\n", c.Title, sanitizePath(typeName), slug, desc))
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
	b.WriteString(fmt.Sprintf("---\n\n"))
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
	return strings.ReplaceAll(s, "/", "-")
}

func totalConcepts(types map[string][]model.Concept) int {
	n := 0
	for _, g := range types {
		n += len(g)
	}
	return n
}
