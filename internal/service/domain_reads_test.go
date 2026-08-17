package service

import (
	"testing"
	"time"

	"github.com/talesofai/okp/internal/testutil"
)

func TestDomainReadsAggregateIntoDailyActivity(t *testing.T) {
	testutil.OpenDatabase(t)
	now := time.Date(2026, 8, 17, 15, 0, 0, 0, time.UTC)

	for range 3 {
		if err := recordDomainReadAt("demo", now.Add(-time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	if err := recordDomainReadAt("demo", now.AddDate(0, 0, -1)); err != nil {
		t.Fatal(err)
	}
	if err := recordDomainReadAt("other", now); err != nil {
		t.Fatal(err)
	}

	activity, err := getDomainActivityAt("demo", 3, now)
	if err != nil {
		t.Fatal(err)
	}
	if got := activity.Points[1].Reads; got != 1 {
		t.Fatalf("yesterday reads = %d, want 1", got)
	}
	if got := activity.Points[2].Reads; got != 3 {
		t.Fatalf("today reads = %d, want 3", got)
	}
	if got := activity.Points[2].Activity; got != 0 {
		t.Fatalf("content activity = %d, want 0; reads must remain separate", got)
	}
}

func TestRecordDomainReadRequiresDomain(t *testing.T) {
	testutil.OpenDatabase(t)
	if err := recordDomainReadAt("", time.Now()); err == nil {
		t.Fatal("expected empty domain validation error")
	}
}
