package handler

import (
	"bufio"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/talesofai/okp/internal/config"
	"github.com/talesofai/okp/internal/model"
	"github.com/talesofai/okp/internal/testutil"
)

func TestExportDomainStreamsConcepts(t *testing.T) {
	db := testutil.OpenDatabase(t)
	previousConfig := config.C
	config.C = config.Config{ExecutionGrantKey: "test-key"}
	t.Cleanup(func() { config.C = previousConfig })

	if err := db.Create(&model.User{UUID: "admin", AuthType: "test", Role: "admin"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.DomainMeta{Domain: "public", Visibility: "public"}).Error; err != nil {
		t.Fatal(err)
	}
	for _, c := range []model.Concept{
		{ID: "public/Note/zeta", Domain: "public", Type: "Note", Title: "Zeta", Provenance: model.JSONMap{"source": "t", "agent": "t"}},
		{ID: "public/Guide/alpha", Domain: "public", Type: "Guide", Title: "Alpha", Provenance: model.JSONMap{"source": "t", "agent": "t"}},
		{ID: "public/Note/beta", Domain: "public", Type: "Note", Title: "Beta", Provenance: model.JSONMap{"source": "t", "agent": "t"}},
	} {
		if err := db.Create(&c).Error; err != nil {
			t.Fatal(err)
		}
	}

	router := NewRouter()
	token := testExecutionToken(t, "test-key", "admin")
	res := testRequest(t, router, token, http.MethodGet, "/api/v1/domains/public/export", nil)
	assertStatus(t, res, http.StatusOK)
	if got := res.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/x-ndjson") {
		t.Fatalf("content type = %q", got)
	}
	if got := res.Header().Get("X-Total-Count"); got != "3" {
		t.Fatalf("X-Total-Count = %q", got)
	}

	var ids []string
	scanner := bufio.NewScanner(res.Body)
	for scanner.Scan() {
		var c model.Concept
		if err := json.Unmarshal(scanner.Bytes(), &c); err != nil {
			t.Fatalf("line %q: %v", scanner.Text(), err)
		}
		ids = append(ids, c.ID)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	want := []string{"public/Guide/alpha", "public/Note/beta", "public/Note/zeta"}
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
}

func TestExportDomainHidesPrivateAndRejectsEmpty(t *testing.T) {
	db := testutil.OpenDatabase(t)
	previousConfig := config.C
	config.C = config.Config{ExecutionGrantKey: "test-key"}
	t.Cleanup(func() { config.C = previousConfig })

	for _, u := range []model.User{
		{UUID: "creator", AuthType: "test", Role: "reader"},
		{UUID: "admin", AuthType: "test", Role: "admin"},
	} {
		if err := db.Create(&u).Error; err != nil {
			t.Fatal(err)
		}
	}

	router := NewRouter()
	creatorToken := testExecutionToken(t, "test-key", "creator")
	adminToken := testExecutionToken(t, "test-key", "admin")

	res := testRequest(t, router, creatorToken, http.MethodPut, "/api/v1/domains/private-team", map[string]any{
		"readme":     "# Private",
		"visibility": "private",
	})
	assertStatus(t, res, http.StatusCreated)
	res = testRequest(t, router, creatorToken, http.MethodPut, "/api/v1/concepts/private-team/Note/plan", map[string]any{
		"domain":     "private-team",
		"type":       "Note",
		"title":      "Plan",
		"provenance": map[string]any{"source": "t", "agent": "t"},
	})
	assertStatus(t, res, http.StatusOK)

	// 非成员（含全局 admin）不可导出私有域
	assertStatus(t, testRequest(t, router, adminToken, http.MethodGet, "/api/v1/domains/private-team/export", nil), http.StatusNotFound)
	// host 可导出
	res = testRequest(t, router, creatorToken, http.MethodGet, "/api/v1/domains/private-team/export", nil)
	assertStatus(t, res, http.StatusOK)
	if got := res.Header().Get("X-Total-Count"); got != "1" {
		t.Fatalf("X-Total-Count = %q, want 1", got)
	}

	// 可见但空的 domain → 404
	assertStatus(t, testRequest(t, router, adminToken, http.MethodPut, "/api/v1/domains/empty-public", map[string]any{"readme": "# Empty"}), http.StatusCreated)
	assertStatus(t, testRequest(t, router, adminToken, http.MethodGet, "/api/v1/domains/empty-public/export", nil), http.StatusNotFound)
}
