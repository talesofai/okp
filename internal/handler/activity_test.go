package handler

import (
	"net/http"
	"testing"
	"time"

	"github.com/talesofai/okp/internal/config"
	"github.com/talesofai/okp/internal/model"
	"github.com/talesofai/okp/internal/testutil"
)

func TestDomainActivityAndSearchExposeTimes(t *testing.T) {
	db := testutil.OpenDatabase(t)
	previousConfig := config.C
	config.C = config.Config{ExecutionGrantKey: "test-key"}
	t.Cleanup(func() { config.C = previousConfig })

	for _, user := range []model.User{
		{UUID: "admin", AuthType: "test", Role: "admin"},
		{UUID: "outsider", AuthType: "test", Role: "reader"},
	} {
		if err := db.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&model.DomainMeta{Domain: "public", Visibility: "public"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.DomainMeta{Domain: "private", Visibility: "private"}).Error; err != nil {
		t.Fatal(err)
	}

	createdAt := time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 8, 17, 9, 0, 0, 0, time.UTC)
	publicConcept := model.Concept{
		ID: "public/Note/timed", Domain: "public", Type: "Note", Title: "Timed concept",
		Provenance: model.JSONMap{"source": "test", "agent": "test"},
		CreatedAt:  createdAt, UpdatedAt: updatedAt,
	}
	privateConcept := model.Concept{
		ID: "private/Note/hidden", Domain: "private", Type: "Note", Title: "Hidden concept",
		Provenance: model.JSONMap{"source": "test", "agent": "test"},
	}
	if err := db.Create(&[]model.Concept{publicConcept, privateConcept}).Error; err != nil {
		t.Fatal(err)
	}

	router := NewRouter()
	adminToken := testExecutionToken(t, "test-key", "admin")
	outsiderToken := testExecutionToken(t, "test-key", "outsider")

	res := testRequest(t, router, adminToken, http.MethodGet, "/api/v1/domains/public/activity?days=7", nil)
	assertStatus(t, res, http.StatusOK)
	var activity struct {
		Domain string `json:"domain"`
		Days   int    `json:"days"`
		Points []struct {
			Date     string `json:"date"`
			Created  int64  `json:"created"`
			Updated  int64  `json:"updated"`
			Activity int64  `json:"activity"`
		} `json:"points"`
	}
	decodeResponse(t, res, &activity)
	if activity.Domain != "public" || activity.Days != 7 || len(activity.Points) != 7 {
		t.Fatalf("activity response = %+v", activity)
	}

	res = testRequest(t, router, adminToken, http.MethodGet, "/api/v1/domains/public/activity?days=0", nil)
	assertStatus(t, res, http.StatusBadRequest)
	res = testRequest(t, router, outsiderToken, http.MethodGet, "/api/v1/domains/private/activity", nil)
	assertStatus(t, res, http.StatusNotFound)
	if err := db.Create(&model.DomainMember{Domain: "private", UserID: "outsider", Role: "reader"}).Error; err != nil {
		t.Fatal(err)
	}
	res = testRequest(t, router, outsiderToken, http.MethodGet, "/api/v1/domains/private/activity", nil)
	assertStatus(t, res, http.StatusOK)

	res = testRequest(t, router, adminToken, http.MethodGet, "/api/v1/concepts?q=Timed", nil)
	assertStatus(t, res, http.StatusOK)
	var results []map[string]any
	decodeResponse(t, res, &results)
	if len(results) != 1 {
		t.Fatalf("search results = %+v", results)
	}
	if _, ok := results[0]["created_at"]; !ok {
		t.Fatalf("search result missing created_at: %+v", results[0])
	}
	if _, ok := results[0]["updated_at"]; !ok {
		t.Fatalf("search result missing updated_at: %+v", results[0])
	}
}
