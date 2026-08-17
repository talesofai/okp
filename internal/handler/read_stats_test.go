package handler

import (
	"net/http"
	"testing"
	"time"

	"github.com/talesofai/okp/internal/config"
	"github.com/talesofai/okp/internal/model"
	"github.com/talesofai/okp/internal/testutil"
)

func TestSuccessfulKnowledgeReadsFeedDomainActivity(t *testing.T) {
	db := testutil.OpenDatabase(t)
	previousConfig := config.C
	config.C = config.Config{ExecutionGrantKey: "test-key"}
	t.Cleanup(func() { config.C = previousConfig })

	for _, user := range []model.User{
		{UUID: "reader", AuthType: "test", Role: "reader"},
		{UUID: "outsider", AuthType: "test", Role: "reader"},
	} {
		if err := db.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&[]model.DomainMeta{
		{Domain: "public", Readme: "# Public", Visibility: "public"},
		{Domain: "private", Readme: "# Private", Visibility: "private"},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]model.Concept{
		{ID: "public/Note/one", Domain: "public", Type: "Note", Title: "One", Provenance: model.JSONMap{"source": "test", "agent": "test"}},
		{ID: "private/Note/hidden", Domain: "private", Type: "Note", Title: "Hidden", Provenance: model.JSONMap{"source": "test", "agent": "test"}},
	}).Error; err != nil {
		t.Fatal(err)
	}

	router := NewRouter()
	readerToken := testExecutionToken(t, "test-key", "reader")
	outsiderToken := testExecutionToken(t, "test-key", "outsider")

	assertStatus(t, testRequest(t, router, readerToken, http.MethodGet, "/api/v1/concepts?domain=public", nil), http.StatusOK)
	assertStatus(t, testRequest(t, router, readerToken, http.MethodGet, "/api/v1/concepts/public/Note/one", nil), http.StatusOK)
	assertStatus(t, testRequest(t, router, readerToken, http.MethodGet, "/api/v1/links/public/Note/one", nil), http.StatusOK)
	assertStatus(t, testRequest(t, router, readerToken, http.MethodGet, "/api/v1/domains/public", nil), http.StatusOK)
	assertStatus(t, testRequest(t, router, readerToken, http.MethodGet, "/api/v1/concepts?domain=public&q=absent", nil), http.StatusOK)

	// Hidden/private requests and the activity endpoint itself must not inflate reads.
	assertStatus(t, testRequest(t, router, outsiderToken, http.MethodGet, "/api/v1/concepts/private/Note/hidden", nil), http.StatusNotFound)
	assertStatus(t, testRequest(t, router, outsiderToken, http.MethodGet, "/api/v1/concepts?domain=private", nil), http.StatusOK)
	res := testRequest(t, router, readerToken, http.MethodGet, "/api/v1/domains/public/activity?days=1", nil)
	assertStatus(t, res, http.StatusOK)

	var activity struct {
		Points []struct {
			Reads int64 `json:"reads"`
		} `json:"points"`
	}
	decodeResponse(t, res, &activity)
	if len(activity.Points) != 1 || activity.Points[0].Reads != 5 {
		t.Fatalf("activity reads = %+v, want 5", activity.Points)
	}

	var privateReads int64
	if err := db.Model(&model.DomainReadStat{}).Where("domain = ?", "private").Count(&privateReads).Error; err != nil {
		t.Fatal(err)
	}
	if privateReads != 0 {
		t.Fatalf("private reads should remain zero, got %d", privateReads)
	}

	var publicStat model.DomainReadStat
	if err := db.Where("domain = ? AND date = ?", "public", time.Now().UTC().Format("2006-01-02")).First(&publicStat).Error; err != nil {
		t.Fatal(err)
	}
	if publicStat.Reads != 5 {
		t.Fatalf("stored reads = %d, want 5", publicStat.Reads)
	}
}
