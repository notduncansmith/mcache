package mcache

import (
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

const testDataDir = "./.tmp"
const testIndexName = "test"
const testManifestName = "m:a&b"

func TestMCacheRoundtrip(t *testing.T) {
	os.RemoveAll(testDataDir + "/mcache-*")
	defer os.RemoveAll(testDataDir + "/mcache-*")
	now := time.Now()
	manifestDoc, _ := EncodeManifest(&Manifest{
		ID:          "m:a&b",
		UpdatedAt:   now.Add(-1 * time.Minute).Unix(),
		DocumentIDs: NewIDSet("a", "b"),
	})
	knownDocs := NewDocSet(
		Document{ID: "a"},
		Document{ID: "b"},
		Document{ID: "c"},
		*manifestDoc,
	)
	config := DefaultConfig
	config.DataDir = testDataDir
	m, err := NewMCache(config)

	if err != nil {
		t.Fatalf("Failed to open mcache: %v", err)
	}
	m.im.Scan()

	idx, err := m.CreateIndex(testIndexName)
	if err != nil {
		t.Fatalf("Failed to open index: %v", err)
	}
	stored, err := idx.Update(knownDocs)
	if err != nil {
		t.Fatalf("Failed to update index: %v", err)
	}

	keys, err := m.Keys(idx.ID)
	if diff := cmp.Diff(NewIDSet("a", "b", "c", testManifestName), keys); diff != "" {
		t.Fatalf("Keys mismatch (-expected +actual):\n%s" + diff)
	}

	if diff := cmp.Diff(NewIDSet("a", "b", "c", testManifestName), idx.LRUKeys()); diff != "" {
		t.Fatalf("LRU keys mismatch (-expected +actual):\n%s" + diff)
	}

	idx.cache.Remove("b")

	if diff := cmp.Diff(NewIDSet("a", "c", testManifestName), idx.LRUKeys()); diff != "" {
		t.Fatalf("LRU keys mismatch (-expected +actual):\n%s" + diff)
	}

	knownDocs.Merge(stored)

	results, err := m.Query(testIndexName, testManifestName, now.Add(-1*time.Minute).Unix())
	if err != nil {
		t.Fatalf("Failed to query index: %v", err)
	}

	expected := NewDocSet(knownDocs.Docs["a"], knownDocs.Docs["b"])
	expectDocs(t, expected, results)

	manifest, err := idx.GetManifest(testManifestName)
	if err != nil {
		t.Fatalf("Failed to get manifest: %v", err)
	}

	manifest.Add("c")
	newManifestDoc, err := EncodeManifest(manifest)
	if err != nil {
		t.Fatalf("Failed to encode manifest: %v", err)
	}
	newManifest, err := DecodeManifest(*newManifestDoc)
	if diff := cmp.Diff(NewIDSet("a", "b", "c"), newManifest.DocumentIDs); diff != "" {
		t.Fatalf("Manifest keys mismatch (-expected +actual):\n%s" + diff)
	}

	stored, err = m.Update(testIndexName, NewDocSet(*newManifestDoc))
	if err != nil {
		t.Fatalf("Failed to update index: %v", err)
	}

	knownDocs.Merge(stored)

	results, err = m.Query(testIndexName, testManifestName, now.Add(-1*time.Minute).Unix())
	if err != nil {
		t.Fatalf("Failed to query index: %v", err)
	}
	expected = NewDocSet(knownDocs.Docs["a"], knownDocs.Docs["b"], knownDocs.Docs["c"])
	expectDocs(t, expected, results)

	stored, err = m.SoftDelete(testIndexName, NewIDSet("c"))
	if err != nil {
		t.Fatalf("Failed to delete document from index: %v", err)
	}

	knownDocs.Merge(stored)

	results, err = m.Query(testIndexName, testManifestName, now.Add(-1*time.Minute).Unix())
	if err != nil {
		t.Fatalf("Failed to query index: %v", err)
	}
	expected = NewDocSet(knownDocs.Docs["a"], knownDocs.Docs["b"], knownDocs.Docs["c"])
	expectDocs(t, expected, results)

	result, err := m.Get(idx.ID, "c")
	expectDocs(t, NewDocSet(knownDocs.Docs["c"]), NewDocSet(*result))

	expected = NewDocSet(knownDocs.Docs["a"], knownDocs.Docs["b"], knownDocs.Docs["c"], knownDocs.Docs[testManifestName])
	results, err = m.GetAll(idx.ID)
	expectDocs(t, expected, results)
}

func expectDocs(t *testing.T, expected *DocSet, actual *DocSet) {
	if diff := cmp.Diff(expected, actual); diff != "" {
		panic("Documents mismatch (-expected +actual):\n%s" + diff)
	}
}
