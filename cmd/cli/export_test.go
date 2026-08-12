package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/talesofai/okp/internal/model"
)

func TestCmdExportWritesBundleLocally(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/domains/demo/export" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
		enc := json.NewEncoder(w)
		for _, c := range []model.Concept{
			{ID: "demo/Note/alpha", Domain: "demo", Type: "Note", Title: "Alpha", Body: "Alpha body", Provenance: model.JSONMap{"source": "t", "agent": "t"}},
			{ID: "demo/Note/beta", Domain: "demo", Type: "Note", Title: "Beta", Body: "Beta body", Provenance: model.JSONMap{"source": "t", "agent": "t"}},
		} {
			if err := enc.Encode(c); err != nil {
				t.Errorf("encode: %v", err)
			}
		}
	}))
	defer srv.Close()

	apiBase = srv.URL
	apiToken = ""
	defer func() { apiBase = "" }()

	outDir := filepath.Join(t.TempDir(), "out")
	cmd := cmdExport()
	cmd.SetArgs([]string{"demo"})
	if err := cmd.Flags().Set("out", outDir); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Execute(); err != nil {
		t.Fatalf("export: %v", err)
	}

	index, err := os.ReadFile(filepath.Join(outDir, "demo", "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), "[Alpha](Note/alpha.md)") || !strings.Contains(string(index), "[Beta](Note/beta.md)") {
		t.Fatalf("index missing links: %s", index)
	}

	alpha, err := os.ReadFile(filepath.Join(outDir, "demo", "Note", "alpha.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(alpha), "Alpha body") {
		t.Fatalf("alpha body missing: %s", alpha)
	}
}

func TestCmdExportPropagatesHTTPErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"domain 'missing' 下无 concept"}`))
	}))
	defer srv.Close()

	apiBase = srv.URL
	apiToken = ""
	defer func() { apiBase = "" }()

	cmd := cmdExport()
	cmd.SetArgs([]string{"missing"})
	if err := cmd.Flags().Set("out", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for HTTP 404")
	}
}
