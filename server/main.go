package main

import (
	"time"

	"github.com/notduncansmith/mcache"
)

func main() {
	now := time.Now()
	manifestDoc, _ := mcache.EncodeManifest(mcache.Manifest{
		ID:          "m:a",
		UpdatedAt:   now.Add(-1 * time.Minute).Unix(),
		DocumentIDs: mcache.NewIDSet("a", "b"),
	})
	docs := mcache.DocSet{
		"a":   mcache.Document{ID: "a", UpdatedAt: now.Add(-3 * time.Minute).Unix()},
		"b":   mcache.Document{ID: "b", UpdatedAt: now.Add(-2 * time.Minute).Unix()},
		"c":   mcache.Document{ID: "c", UpdatedAt: now.Unix()},
		"m:a": *manifestDoc,
	}
	m, err := mcache.NewMCache(mcache.DefaultConfig, getLoader(docs))
	if err != nil {
		panic(err)
	}
	m.StartGraphQL(":1337", true)
}

func getLoader(baseDocs mcache.DocSet) mcache.Loader {
	return func(docIDs mcache.IDSet, updatedAfter mcache.Timestamp) (mcache.DocSet, error) {
		docs := mcache.DocSet{}

		for id := range docIDs {
			docs[id] = baseDocs[id]
		}

		return docs, nil
	}
}
