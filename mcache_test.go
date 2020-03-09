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
	docs := DocSet{
		"a":   Document{ID: "a", UpdatedAt: now.Add(-3 * time.Minute).Unix()},
		"b":   Document{ID: "b", UpdatedAt: now.Add(-2 * time.Minute).Unix()},
		"c":   Document{ID: "c", UpdatedAt: now.Unix()},
		"m:a": *manifestDoc,
	}
	m, err := NewMCache(DefaultConfig, getLoader(docs))

	if err != nil {
		t.Errorf("Failed to open mcache: %v", err)
		return
	}

	idx, err := m.CreateIndex("test")

	if err != nil {
		t.Errorf("Failed to open index: %v", err)
		return
	}

	results, err := idx.Query("m:a", now.Add(-4*time.Minute).Unix())
	if err != nil {
		t.Errorf("Failed to query index: %v", err)
		return
	}
	expected := NewDocSet(docs["a"], docs["b"])
	expectDocs(t, expected, results)

	manifest, err := idx.GetManifest("m:a")
	if err != nil {
		t.Errorf("Failed to get manifest: %v", err)
		return
	}
	manifest.DocumentIDs["c"] = struct{}{}
	manifest.UpdatedAt = time.Now().Unix()
	newManifestDoc, err := EncodeManifest(*manifest)
	if err != nil {
		t.Errorf("Failed to encode manifest: %v", err)
		return
	}
	if err := idx.Update(NewDocSet(*newManifestDoc)); err != nil {
		t.Errorf("Failed to update index: %v", err)
		return
	}

	results, err = idx.Query("m:a", now.Add(-4*time.Minute).Unix())
	if err != nil {
		t.Errorf("Failed to query index: %v", err)
		return
	}
	expected = NewDocSet(docs["a"], docs["b"], docs["c"])
	expectDocs(t, expected, results)
}

func expectDocs(t *testing.T, expected DocSet, actual DocSet) {
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("Documents mismatch (-expected +actual):\n%s", diff)
		t.FailNow()
	}
}

func getLoader(baseDocs DocSet) Loader {
	return func(docIDs IDSet, updatedAfter Timestamp) (DocSet, error) {
		docs := DocSet{}

		for id := range docIDs {
			docs[id] = baseDocs[id]
		}

		return docs, nil
	}
}
