package service

import (
	"testing"
	"time"

	"github.com/talesofai/okp/internal/model"
	"github.com/talesofai/okp/internal/testutil"
)

func TestSearchResultsExposeTimestamps(t *testing.T) {
	db := testutil.OpenDatabase(t)
	createdAt := time.Date(2026, 8, 1, 9, 30, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 8, 5, 16, 45, 0, 0, time.UTC)
	concept := testConcept("demo/Note/timestamps", "demo", "Timestamps")
	concept.CreatedAt = createdAt
	concept.UpdatedAt = updatedAt
	if err := db.Create(&concept).Error; err != nil {
		t.Fatal(err)
	}

	results, total, err := Search(SearchParams{UserID: "reader", Domain: "demo", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(results) != 1 {
		t.Fatalf("total=%d results=%+v", total, results)
	}
	if !results[0].CreatedAt.Equal(createdAt) || !results[0].UpdatedAt.Equal(updatedAt) {
		t.Fatalf("timestamps = %s / %s, want %s / %s", results[0].CreatedAt, results[0].UpdatedAt, createdAt, updatedAt)
	}

	merged := mergeResults(nil, []model.Concept{concept}, "timestamp", nil, 10)
	if len(merged) != 1 || !merged[0].CreatedAt.Equal(createdAt) || !merged[0].UpdatedAt.Equal(updatedAt) {
		t.Fatalf("merged timestamps missing: %+v", merged)
	}
}

func TestListDomainsAggregatesCreationAndUpdateTimes(t *testing.T) {
	db := testutil.OpenDatabase(t)
	metaCreatedAt := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	metaUpdatedAt := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	conceptCreatedAt := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	conceptUpdatedAt := time.Date(2026, 8, 8, 10, 0, 0, 0, time.UTC)
	if err := db.Create(&model.DomainMeta{
		Domain: "demo", Visibility: "public", CreatedAt: metaCreatedAt, UpdatedAt: metaUpdatedAt,
	}).Error; err != nil {
		t.Fatal(err)
	}
	concept := testConcept("demo/Note/one", "demo", "One")
	concept.CreatedAt = conceptCreatedAt
	concept.UpdatedAt = conceptUpdatedAt
	if err := db.Create(&concept).Error; err != nil {
		t.Fatal(err)
	}

	// Domains can exist from concepts alone, without a README row.
	orphan := testConcept("orphan/Note/one", "orphan", "One")
	orphan.CreatedAt = time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	orphan.UpdatedAt = time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)
	if err := db.Create(&orphan).Error; err != nil {
		t.Fatal(err)
	}

	domains, err := ListDomains("reader", "")
	if err != nil {
		t.Fatal(err)
	}
	byName := make(map[string]DomainInfo, len(domains))
	for _, domain := range domains {
		byName[domain.Domain] = domain
	}

	demo, ok := byName["demo"]
	if !ok {
		t.Fatalf("demo missing from domains: %+v", domains)
	}
	if !demo.CreatedAt.Equal(metaCreatedAt) || !demo.UpdatedAt.Equal(conceptUpdatedAt) {
		t.Fatalf("demo times = %s / %s", demo.CreatedAt, demo.UpdatedAt)
	}

	orphanInfo, ok := byName["orphan"]
	if !ok || !orphanInfo.CreatedAt.Equal(orphan.CreatedAt) || !orphanInfo.UpdatedAt.Equal(orphan.UpdatedAt) {
		t.Fatalf("orphan times = %+v", orphanInfo)
	}
}

func TestGetDomainActivityBucketsCreatesAndUpdates(t *testing.T) {
	db := testutil.OpenDatabase(t)
	now := time.Date(2026, 8, 17, 15, 0, 0, 0, time.UTC)
	createToday := testConcept("demo/Note/new", "demo", "New")
	createToday.CreatedAt = now.Add(-2 * time.Hour)
	createToday.UpdatedAt = createToday.CreatedAt
	updatedYesterday := testConcept("demo/Note/changed", "demo", "Changed")
	updatedYesterday.CreatedAt = now.AddDate(0, 0, -5)
	updatedYesterday.UpdatedAt = now.AddDate(0, 0, -1)
	outsideRange := testConcept("demo/Note/old", "demo", "Old")
	outsideRange.CreatedAt = now.AddDate(0, 0, -20)
	outsideRange.UpdatedAt = outsideRange.CreatedAt
	if err := db.Create(&[]model.Concept{createToday, updatedYesterday, outsideRange}).Error; err != nil {
		t.Fatal(err)
	}

	activity, err := getDomainActivityAt("demo", 7, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(activity.Points) != 7 || activity.From != "2026-08-11" || activity.To != "2026-08-17" {
		t.Fatalf("range = %s..%s points=%d", activity.From, activity.To, len(activity.Points))
	}
	if got := activity.Points[5]; got.Created != 0 || got.Updated != 1 || got.Activity != 1 {
		t.Fatalf("yesterday bucket = %+v", got)
	}
	if got := activity.Points[6]; got.Created != 1 || got.Updated != 0 || got.Activity != 1 {
		t.Fatalf("today bucket = %+v", got)
	}
}

func TestGetDomainActivityRejectsInvalidRange(t *testing.T) {
	for _, days := range []int{0, 366} {
		if _, err := GetDomainActivity("demo", days); err == nil {
			t.Fatalf("expected validation error for days=%d", days)
		}
	}
}
