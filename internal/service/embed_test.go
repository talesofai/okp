package service

import (
	"testing"

	"github.com/talesofai/okp/internal/model"
	"github.com/talesofai/okp/internal/testutil"
)

// 只改 tags：content_hash 不变但 embedding 输入变化 → 必须落库并触发重嵌（status=pending）
func TestPutConceptReembedsOnTagOnlyChange(t *testing.T) {
	db := testutil.OpenDatabase(t)
	concept := testConcept("demo/Note/tagged", "demo", "Tagged")
	concept.Tags = model.StringSlice{"old"}
	concept.EmbedStatus = "done"
	if err := db.Create(&concept).Error; err != nil {
		t.Fatal(err)
	}

	update := testConcept("demo/Note/tagged", "demo", "Tagged")
	update.Tags = model.StringSlice{"new"}
	if _, _, err := PutConcept(&update); err != nil {
		t.Fatal(err)
	}

	var saved model.Concept
	if err := db.First(&saved, "id = ?", concept.ID).Error; err != nil {
		t.Fatal(err)
	}
	if len(saved.Tags) != 1 || saved.Tags[0] != "new" {
		t.Fatalf("tags not persisted: %+v", saved.Tags)
	}
	if saved.EmbedStatus != "pending" {
		t.Fatalf("embed_status = %q, want pending", saved.EmbedStatus)
	}
}

// 只改 body：向量输入未变 → 落库但保留旧向量（status 保持 done，不浪费重嵌）
func TestPutConceptKeepsVectorOnBodyOnlyChange(t *testing.T) {
	db := testutil.OpenDatabase(t)
	concept := testConcept("demo/Note/body", "demo", "Body")
	concept.Body = "old body"
	concept.EmbedStatus = "done"
	if err := db.Create(&concept).Error; err != nil {
		t.Fatal(err)
	}

	update := testConcept("demo/Note/body", "demo", "Body")
	update.Body = "new body"
	if _, _, err := PutConcept(&update); err != nil {
		t.Fatal(err)
	}

	var saved model.Concept
	if err := db.First(&saved, "id = ?", concept.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.Body != "new body" {
		t.Fatalf("body not persisted: %q", saved.Body)
	}
	if saved.EmbedStatus != "done" {
		t.Fatalf("embed_status = %q, want done (vector input unchanged)", saved.EmbedStatus)
	}
}

// 完全相同的 re-put：幂等跳过，不产生新 revision、不改状态
func TestPutConceptSkipsIdenticalReput(t *testing.T) {
	db := testutil.OpenDatabase(t)
	concept := testConcept("demo/Note/same", "demo", "Same")
	concept.EmbedStatus = "done"
	if err := db.Create(&concept).Error; err != nil {
		t.Fatal(err)
	}

	result, _, err := PutConcept(&concept)
	if err != nil {
		t.Fatal(err)
	}
	if result.EmbedStatus != "done" {
		t.Fatalf("embed_status = %q, want done", result.EmbedStatus)
	}
}

// 批量路径：只改 tags 同样要落库并触发重嵌
func TestBatchPutConceptsReembedsOnTagOnlyChange(t *testing.T) {
	db := testutil.OpenDatabase(t)
	concept := testConcept("demo/Note/btag", "demo", "BTag")
	concept.Tags = model.StringSlice{"old"}
	concept.EmbedStatus = "done"
	if err := db.Create(&concept).Error; err != nil {
		t.Fatal(err)
	}

	update := testConcept("demo/Note/btag", "demo", "BTag")
	update.Tags = model.StringSlice{"new"}
	results := BatchPutConcepts([]model.Concept{update})
	if len(results) != 1 || results[0].Status != "updated" {
		t.Fatalf("results = %+v", results)
	}

	var saved model.Concept
	if err := db.First(&saved, "id = ?", concept.ID).Error; err != nil {
		t.Fatal(err)
	}
	if len(saved.Tags) != 1 || saved.Tags[0] != "new" {
		t.Fatalf("tags not persisted: %+v", saved.Tags)
	}
	if saved.EmbedStatus != "pending" {
		t.Fatalf("embed_status = %q, want pending", saved.EmbedStatus)
	}
}
