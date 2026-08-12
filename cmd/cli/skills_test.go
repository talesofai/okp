package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmdSkillsInstallWritesAgentsSkills(t *testing.T) {
	dir := t.TempDir()

	cmd := cmdSkillsInstall()
	cmd.SetArgs(nil)
	if err := cmd.Flags().Set("dir", dir); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Execute(); err != nil {
		t.Fatalf("install: %v", err)
	}

	importSkill := filepath.Join(dir, ".agents", "skills", "okp-import", "SKILL.md")
	b, err := os.ReadFile(importSkill)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "okp-import") {
		t.Fatalf("unexpected SKILL.md content: %q", b)
	}

	searchSkill := filepath.Join(dir, ".agents", "skills", "okp-search", "SKILL.md")
	if _, err := os.Stat(searchSkill); err != nil {
		t.Fatal(err)
	}

	// _ 开头的模板文件也必须被安装（embed 用 all: 前缀打包）
	template := filepath.Join(dir, ".agents", "skills", "okp-import", "references", "templates", "_template.md")
	if _, err := os.Stat(template); err != nil {
		t.Fatalf("underscore-prefixed template should be installed: %v", err)
	}
}

func TestCmdSkillsInstallIsIdempotentUpdate(t *testing.T) {
	dir := t.TempDir()

	run := func() error {
		cmd := cmdSkillsInstall()
		cmd.SetArgs(nil)
		if err := cmd.Flags().Set("dir", dir); err != nil {
			return err
		}
		return cmd.Execute()
	}
	if err := run(); err != nil {
		t.Fatal(err)
	}

	// 修改本地文件模拟过期版本，重新 install 应恢复内置内容
	path := filepath.Join(dir, ".agents", "skills", "okp-import", "SKILL.md")
	if err := os.WriteFile(path, []byte("stale"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := run(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "stale") || !strings.Contains(string(b), "okp-import") {
		t.Fatalf("install should overwrite stale file, got %q", b)
	}
}

func TestCmdSkillsListReportsEmbeddedSkills(t *testing.T) {
	cmd := cmdSkillsList()
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestReadSkillFrontmatter(t *testing.T) {
	meta := readSkillFrontmatter("okp-import")
	if meta["name"] != "okp-import" {
		t.Fatalf("name = %q", meta["name"])
	}
	if meta["version"] == "" {
		t.Fatalf("version missing: %+v", meta)
	}
	if meta["description"] == "" {
		t.Fatalf("description missing: %+v", meta)
	}
}
