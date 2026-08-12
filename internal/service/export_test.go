package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/talesofai/okp/internal/model"
)

func TestWriteDomainBundleWritesFilesAndIndex(t *testing.T) {
	outDir := t.TempDir()
	concepts := []model.Concept{
		testConcept("demo/Note/alpha", "demo", "Alpha"),
		testConcept("demo/Guide/beta", "demo", "Beta"),
	}
	concepts[0].Body = "Alpha body"
	concepts[0].Description = "Alpha desc"
	concepts[1].Type = "Guide"
	concepts[1].Body = "Beta body"

	bundleDir, err := WriteDomainBundle("demo", concepts, outDir)
	if err != nil {
		t.Fatal(err)
	}
	if bundleDir != filepath.Join(outDir, "demo") {
		t.Fatalf("bundleDir = %q", bundleDir)
	}

	index, err := os.ReadFile(filepath.Join(bundleDir, "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	// 类型按字典序：Guide 在 Note 前
	if !strings.Contains(string(index), "[Beta](Guide/beta.md)") || !strings.Contains(string(index), "[Alpha](Note/alpha.md)") {
		t.Fatalf("index links: %s", index)
	}

	alpha, err := os.ReadFile(filepath.Join(bundleDir, "Note", "alpha.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(alpha), "type: Note") || !strings.Contains(string(alpha), "Alpha body") {
		t.Fatalf("alpha content: %s", alpha)
	}
}

func TestWriteDomainBundleDisambiguatesSlugCollisions(t *testing.T) {
	outDir := t.TempDir()
	// 同 type + 同 ID 末段 → 文件名冲突必须消歧，不能互相覆盖。
	c1 := testConcept("demo/Note/one/same", "demo", "One")
	c2 := testConcept("demo/Note/two/same", "demo", "Two")
	c1.Body = "content one"
	c2.Body = "content two"
	c1.ContentHash = "aaaa1111bbbb"
	c2.ContentHash = "cccc2222dddd"

	bundleDir, err := WriteDomainBundle("demo", []model.Concept{c1, c2}, outDir)
	if err != nil {
		t.Fatal(err)
	}

	noteDir := filepath.Join(bundleDir, "Note")
	entries, err := os.ReadDir(noteDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("files = %d, want 2", len(entries))
	}
	seen := map[string]string{}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(noteDir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		seen[e.Name()] = string(b)
	}
	if !strings.Contains(seen["same-aaaa1111.md"], "content one") {
		t.Fatalf("missing disambiguated file for c1: %v", seen)
	}
	if !strings.Contains(seen["same-cccc2222.md"], "content two") {
		t.Fatalf("missing disambiguated file for c2: %v", seen)
	}

	index, err := os.ReadFile(filepath.Join(bundleDir, "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), "same-aaaa1111.md") || !strings.Contains(string(index), "same-cccc2222.md") {
		t.Fatalf("index must reference both files: %s", index)
	}
}

func TestWriteDomainBundleRejectsEmptyAndRefreshesStaleFiles(t *testing.T) {
	outDir := t.TempDir()
	if _, err := WriteDomainBundle("demo", nil, outDir); err == nil {
		t.Fatal("expected error for empty concepts")
	}

	if _, err := WriteDomainBundle("demo", []model.Concept{testConcept("demo/Note/keep", "demo", "Keep")}, outDir); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(outDir, "demo", "Note", "stale.md")
	if err := os.WriteFile(stale, []byte("stale"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := WriteDomainBundle("demo", []model.Concept{testConcept("demo/Note/keep", "demo", "Keep")}, outDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale file should be removed on re-export")
	}
}
