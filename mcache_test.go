package mcache

import (
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestMCacheRoundtrip(t *testing.T) {
	os.RemoveAll("./.tmp")
	defer os.RemoveAll("./.tmp")
	now := time.Now()
	manifestDoc, _ := EncodeManifest(Manifest{
		ID:          "m:a",
		UpdatedAt:   now.Add(-1 * time.Minute).Unix(),
		DocumentIDs: NewIDSet("a", "b"),
	})
	docs := NewDocSet(
		Document{ID: "a", UpdatedAt: now.Add(-3 * time.Minute).Unix()},
		Document{ID: "b", UpdatedAt: now.Add(-2 * time.Minute).Unix()},
		Document{ID: "c", UpdatedAt: now.Unix()},
		*manifestDoc,
	)
	config := DefaultConfig
	config.DataDir = "./.tmp"
	m, err := NewMCache(config)

	if err != nil {
		t.Fatalf("Failed to open mcache: %v", err)
	}

	idx, err := m.CreateIndex("test")
	if err != nil {
		t.Fatalf("Failed to open index: %v", err)
	}
	if err = idx.Update(docs); err != nil {
		t.Fatalf("Failed to open index: %v", err)
	}

	results, err := idx.Query("m:a", now.Add(-4*time.Minute).Unix())
	if err != nil {
		t.Fatalf("Failed to query index: %v", err)
	}
	expected := NewDocSet(docs.Docs["a"], docs.Docs["b"])
	expectDocs(t, expected, results)

	manifest, err := idx.GetManifest("m:a")
	if err != nil {
		t.Fatalf("Failed to get manifest: %v", err)
	}
	manifest.DocumentIDs["c"] = SetEntry{}
	manifest.UpdatedAt = time.Now().Unix()
	newManifestDoc, err := EncodeManifest(*manifest)
	if err != nil {
		t.Fatalf("Failed to encode manifest: %v", err)
	}
	if err := idx.Update(NewDocSet(*newManifestDoc)); err != nil {
		t.Fatalf("Failed to update index: %v", err)
	}

	results, err = idx.Query("m:a", now.Add(-4*time.Minute).Unix())
	if err != nil {
		t.Fatalf("Failed to query index: %v", err)
	}
	expected = NewDocSet(docs.Docs["a"], docs.Docs["b"], docs.Docs["c"])
	expectDocs(t, expected, results)
}

func expectDocs(t *testing.T, expected *DocSet, actual *DocSet) {
	if diff := cmp.Diff(*expected, *actual); diff != "" {
		t.Fatalf("Documents mismatch (-expected +actual):\n%s", diff)
	}
}
