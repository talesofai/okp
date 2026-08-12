package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/talesofai/okp/skills"
)

func cmdSkills() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "安装/管理 okp agent skills（写入 .agents/skills/）",
	}
	cmd.AddCommand(cmdSkillsInstall(), cmdSkillsList())
	return cmd
}

func cmdSkillsInstall() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "install",
		Short: "安装/更新 okp skills 到 {dir}/.agents/skills/",
		Long: `把随 CLI 内置的 okp skills（okp-import、okp-search）写入目标目录的 .agents/skills/ 下。

重复执行即更新：同名文件被覆盖，未变化的文件保持不动。
skills 随 CLI 版本发布；升级 CLI（okp update）后再执行 install 即可更新 skills。`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := filepath.Join(dir, ".agents", "skills")
			created, updated, unchanged := 0, 0, 0
			err := fs.WalkDir(skills.FS, ".", func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					return nil
				}
				content, err := skills.FS.ReadFile(path)
				if err != nil {
					return err
				}
				target := filepath.Join(root, filepath.FromSlash(path))
				prev, statErr := os.ReadFile(target)
				if statErr == nil && string(prev) == string(content) {
					unchanged++
					return nil
				}
				if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
					return err
				}
				if err := os.WriteFile(target, content, 0644); err != nil {
					return err
				}
				if statErr == nil {
					updated++
				} else {
					created++
				}
				return nil
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "skills 已写入 %s（新增 %d，更新 %d，未变 %d）\n", root, created, updated, unchanged)
			prettyPrint(map[string]any{
				"root":      root,
				"skills":    embeddedSkillNames(),
				"created":   created,
				"updated":   updated,
				"unchanged": unchanged,
			})
			return nil
		},
	}
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "目标目录（skills 写入 {dir}/.agents/skills/）")
	return cmd
}

func cmdSkillsList() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "列出 CLI 内置的 okp skills",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			results := make([]map[string]string, 0, len(embeddedSkillNames()))
			for _, name := range embeddedSkillNames() {
				results = append(results, readSkillFrontmatter(name))
			}
			prettyPrint(results)
			return nil
		},
	}
}

func embeddedSkillNames() []string {
	entries, err := fs.ReadDir(skills.FS, ".")
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names
}

// readSkillFrontmatter 解析 SKILL.md 的 YAML frontmatter，返回 name/version/description。
func readSkillFrontmatter(name string) map[string]string {
	meta := map[string]string{"name": name}
	b, err := skills.FS.ReadFile(name + "/SKILL.md")
	if err != nil {
		return meta
	}
	inFM := false
	for _, line := range strings.Split(string(b), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			if !inFM {
				inFM = true
				continue
			}
			break
		}
		if !inFM {
			continue
		}
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		switch key {
		case "version", "description":
			meta[key] = strings.Trim(strings.TrimSpace(value), `"'`)
		}
	}
	return meta
}
