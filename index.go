package mcache

import (
	"encoding/json"
	"fmt"

	lru "github.com/hashicorp/golang-lru"
	du "github.com/notduncansmith/duramap"
)

// Loader is a function that can load documents
type Loader = func(docIDs IDSet, updatedAfter Timestamp) (DocSet, error)

// Index represents a collection of documents managed by the cache
type Index struct {
	ID string
	Loader
	docs  *du.Duramap
	cache *lru.TwoQueueCache
}

// NewIndex returns a new Index
func NewIndex(id string, docs *du.Duramap, loader Loader, cacheSize int) (*Index, error) {
	cache, err := lru.New2Q(cacheSize)
	if err != nil {
		return nil, err
	}

	return &Index{id, loader, docs, cache}, nil
}

// Update updates the index documents with the latest versions
func (i *Index) Update(docs DocSet) error {
	return i.docs.UpdateMap(func(tx *du.Tx) error {
		for _, d := range docs {
			stored := tx.Get(d.ID)
			if stored == nil {
				tx.Set(d.ID, d)
				i.cache.Add(d.ID, d)
				continue
			}
			storedDoc, ok := stored.(Document)
			if !ok {
				fmt.Printf("Unable to decode document: %v", tx.Get(d.ID))
				continue
			}
			if storedDoc.UpdatedAt < d.UpdatedAt {
				tx.Set(d.ID, d)
				i.cache.Add(d.ID, d)
			}
		}

		return nil
	})
}

// Get gets the index document with the given ID
func (i *Index) Get(id string) (doc *Document, err error) {
	i.docs.DoWithMap(func(m du.GenericMap) {
		d := m[id]
		if d == nil {
			err = fmt.Errorf("Document not found for id %v", id)
			return
		}
		stored, ok := d.(Document)
		if !ok {
			err = fmt.Errorf("Unable to decode document %+v", d)
			return
		}
		doc = &stored
	})
	return
}

// GetAll gets all the index documents
func (i *Index) GetAll() (docs DocSet, err error) {
	i.docs.DoWithMap(func(m du.GenericMap) {
		for k, v := range m {
			stored, ok := v.(Document)
			if !ok {
				fmt.Printf("Unable to decode document %+v", v)
				continue
			}
			docs[k] = stored
		}
	})
	return
}

// SoftDelete updates the index documents with a tombstone value
func (i *Index) SoftDelete(ids IDSet) error {
	return i.docs.UpdateMap(func(tx *du.Tx) error {
		for id := range ids {
			doc := NewTombstone(id)
			tx.Set(id, doc)
			i.cache.Add(id, doc)
		}
		return nil
	})
}

// GetManifest returns a manifest document
func (i *Index) GetManifest(manifestDocumentID string) (*Manifest, error) {
	docs, err := i.LoadDocuments(NewIDSet(manifestDocumentID), 0)
	if err != nil {
		return nil, fmt.Errorf("Unable to load manifest: %v", err)
	}
	if len(docs) == 0 {
		return nil, fmt.Errorf("Unable to load manifest: not found")
	}

	manifestDocument := docs[manifestDocumentID]
	docIds := map[string]struct{}{}

	if err = json.Unmarshal(manifestDocument.Body, &docIds); err != nil {
		return nil, fmt.Errorf("Unable to decode manifest body: %v", err)
	}

	m := Manifest{ID: manifestDocumentID, UpdatedAt: manifestDocument.UpdatedAt, DocumentIDs: docIds}
	return &m, nil
}

// Query returns any documents matching the manifest with the given id that were updated after the given timestamp
func (i *Index) Query(manifestID string, updatedAfter Timestamp) (DocSet, error) {
	m, err := i.GetManifest(manifestID)
	if err != nil {
		return nil, err
	}

	return i.LoadDocuments(m.DocumentIDs, updatedAfter)
}

// LoadDocuments will, for a given set of document IDs, query the LRU cache for the latest matching versions and call Loader for any remaining
func (i *Index) LoadDocuments(docIDs IDSet, updatedAfter Timestamp) (DocSet, error) {
	docs := DocSet{}
	uncachedIds := IDSet{}

	for k := range docIDs {
		if i.cache.Contains(k) {
			cached, ok := i.cache.Get(k)
			if !ok {
				panic("Failed to get doc from cache")
			}
			doc, ok := cached.(Document)
			if !ok {
				panic("Non-document found in cache")
			}
			if doc.UpdatedAt > updatedAfter {
				docs[k] = doc
			}
		} else {
			uncachedIds[k] = struct{}{}
		}
	}

	remainingDocs, err := i.Loader(uncachedIds, updatedAfter)
	if err != nil {
		return nil, err
	}

	for k, v := range remainingDocs {
		// should be unnecessary but loaders may not perfectly filter
		if v.UpdatedAt > updatedAfter {
			docs[k] = v
		}
	}

	return docs, nil
}

// Keys returns all the keys in an index
func (i *Index) Keys() IDSet {
	keys := IDSet{}
	i.docs.DoWithMap(func(m du.GenericMap) {
		for k := range m {
			keys[k] = struct{}{}
		}
	})
	return keys
}

// LRUKeys returns all the keys in an index's LRU cache
func (i *Index) LRUKeys() IDSet {
	keys := IDSet{}
	for _, k := range i.cache.Keys() {
		keys[k.(string)] = struct{}{}
	}
	return keys
}
