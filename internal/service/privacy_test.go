package service

import (
	"testing"

	"github.com/talesofai/okp/internal/model"
	"github.com/talesofai/okp/internal/testutil"
	"gorm.io/gorm"
)

func TestSearchDomainListAndLinksHidePrivateDomains(t *testing.T) {
	db := testutil.OpenDatabase(t)
	for _, meta := range []model.DomainMeta{
		{Domain: "public", Visibility: "public"},
		{Domain: "secret", Visibility: "private"},
	} {
		if err := db.Create(&meta).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, concept := range []model.Concept{
		testConcept("public/Note/open", "public", "Open"),
		testConcept("secret/Note/hidden", "secret", "Hidden"),
	} {
		if err := db.Create(&concept).Error; err != nil {
			t.Fatal(err)
		}
	}
	links := []model.Link{
		{FromID: "public/Note/open", ToID: "secret/Note/hidden"},
		{FromID: "secret/Note/hidden", ToID: "public/Note/open"},
		{FromID: "public/Note/open", ToID: "secret/Note/missing"},
	}
	if err := db.Create(&links).Error; err != nil {
		t.Fatal(err)
	}

	results, total, err := Search(SearchParams{UserID: "admin", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(results) != 1 || results[0].Domain != "public" {
		t.Fatalf("unauthorized search leaked private results: total=%d results=%+v", total, results)
	}
	results, total, err = Search(SearchParams{UserID: "admin", Domain: "secret", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || len(results) != 0 {
		t.Fatalf("private domain filter leaked results: total=%d results=%+v", total, results)
	}
	results, total, err = Search(SearchParams{UserID: "admin", Query: "Hidden", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || len(results) != 0 {
		t.Fatalf("private text search leaked results: total=%d results=%+v", total, results)
	}
	domains, err := ListDomains("admin", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(domains) != 1 || domains[0].Domain != "public" {
		t.Fatalf("unauthorized domain list leaked private domain: %+v", domains)
	}
	samples, err := Sample("admin", "", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 1 || samples[0].Domain != "public" {
		t.Fatalf("unauthorized sample leaked private concept: %+v", samples)
	}
	samples, err = Sample("admin", "secret", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 0 {
		t.Fatalf("private domain sample leaked concepts: %+v", samples)
	}
	outgoing, backlinks, totalOut, totalBack, err := GetLinks("admin", "public/Note/open", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(outgoing) != 0 || len(backlinks) != 0 || totalOut != 0 || totalBack != 0 {
		t.Fatalf("links leaked private IDs: outgoing=%+v backlinks=%+v", outgoing, backlinks)
	}

	if err := db.Create(&model.DomainMember{Domain: "secret", UserID: "admin", Role: "reader"}).Error; err != nil {
		t.Fatal(err)
	}
	results, total, err = Search(SearchParams{UserID: "admin", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(results) != 2 {
		t.Fatalf("invited reader should search private concepts: total=%d results=%+v", total, results)
	}
	domains, err = ListDomains("admin", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(domains) != 2 {
		t.Fatalf("invited reader should list private domain: %+v", domains)
	}
	outgoing, backlinks, totalOut, totalBack, err = GetLinks("admin", "public/Note/open", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(outgoing) != 2 || len(backlinks) != 1 || totalOut != 2 || totalBack != 1 {
		t.Fatalf("invited reader should see private links: outgoing=%+v backlinks=%+v", outgoing, backlinks)
	}
}

func TestDeleteConceptCleansLinksAndRevisions(t *testing.T) {
	db := testutil.OpenDatabase(t)
	concept := testConcept("public/Note/delete-me", "public", "Delete me")
	other := testConcept("public/Note/other", "public", "Other")
	if err := db.Create(&[]model.Concept{concept, other}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]model.Link{
		{FromID: concept.ID, ToID: other.ID},
		{FromID: other.ID, ToID: concept.ID},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Revision{ConceptID: concept.ID, Rev: 1, Content: model.JSONMap{}}).Error; err != nil {
		t.Fatal(err)
	}

	if err := DeleteConcept(concept.ID); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, &model.Concept{}, 0, "id = ?", concept.ID)
	assertCount(t, db, &model.Link{}, 0, "from_id = ? OR to_id = ?", concept.ID, concept.ID)
	assertCount(t, db, &model.Revision{}, 0, "concept_id = ?", concept.ID)
	assertCount(t, db, &model.Concept{}, 1, "id = ?", other.ID)
}

func TestDeleteDomainCleansOwnedData(t *testing.T) {
	db := testutil.OpenDatabase(t)
	if err := db.Create(&model.DomainMeta{Domain: "secret", Visibility: "private"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.DomainMember{Domain: "secret", UserID: "host", Role: "host"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.DomainInvite{ID: "invite", CodeHash: "hash", Domain: "secret", Role: "reader", CreatedBy: "host", MaxUses: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.DomainReadStat{Domain: "secret", Date: "2026-08-17", Reads: 3}).Error; err != nil {
		t.Fatal(err)
	}
	secret := testConcept("secret/Note/delete-me", "secret", "Delete me")
	public := testConcept("public/Note/keep-me", "public", "Keep me")
	if err := db.Create(&[]model.Concept{secret, public}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]model.Link{
		{FromID: secret.ID, ToID: public.ID},
		{FromID: public.ID, ToID: secret.ID},
		{FromID: public.ID, ToID: "secret/Note/missing"},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Revision{ConceptID: secret.ID, Rev: 1, Content: model.JSONMap{}}).Error; err != nil {
		t.Fatal(err)
	}

	if err := DeleteDomain("secret"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, &model.DomainMeta{}, 0, "domain = ?", "secret")
	assertCount(t, db, &model.DomainMember{}, 0, "domain = ?", "secret")
	assertCount(t, db, &model.DomainInvite{}, 0, "domain = ?", "secret")
	assertCount(t, db, &model.DomainReadStat{}, 0, "domain = ?", "secret")
	assertCount(t, db, &model.Concept{}, 0, "domain = ?", "secret")
	assertCount(t, db, &model.Link{}, 0, "from_id = ? OR to_id = ?", secret.ID, secret.ID)
	assertCount(t, db, &model.Revision{}, 0, "concept_id = ?", secret.ID)
	assertCount(t, db, &model.Concept{}, 1, "id = ?", public.ID)
}

func testConcept(id, domain, title string) model.Concept {
	return model.Concept{
		ID: id, Domain: domain, Type: "Note", Title: title,
		Provenance: model.JSONMap{"source": "test", "agent": "test"},
	}
}

func assertCount(t *testing.T, db *gorm.DB, modelValue any, want int64, query string, args ...any) {
	t.Helper()
	var count int64
	if err := db.Model(modelValue).Where(query, args...).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("count for %T = %d, want %d", modelValue, count, want)
	}
}
