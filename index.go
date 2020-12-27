package mcache

import (
	"encoding/json"
	"fmt"
	"time"

	lru "github.com/hashicorp/golang-lru"
	du "github.com/notduncansmith/duramap"
)

// Index represents a collection of documents managed by the cache
type Index struct {
	ID    string
	docs  *du.Duramap
	cache *lru.TwoQueueCache
}

// NewIndex returns a new Index
func NewIndex(id string, docs *du.Duramap, cacheSize int) (*Index, error) {
	cache, err := lru.New2Q(cacheSize)
	if err != nil {
		return nil, err
	}

	return &Index{id, docs, cache}, nil
}

// Update updates the index documents with the latest versions
func (i *Index) Update(docs *DocSet) (*DocSet, error) {
	updated := NewDocSet()
	now := time.Now().Unix()
	err := i.docs.UpdateMap(func(tx *du.Tx) error {
		for _, d := range docs.Docs {
			d.UpdatedAt = now
			updated.Add(d)
			i.cache.Add(d.ID, d)
			tx.Set(d.ID, d)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return updated, nil
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
func (i *Index) GetAll() (docs *DocSet, err error) {
	docs = NewDocSet()
	i.docs.DoWithMap(func(m du.GenericMap) {
		for _, v := range m {
			stored, ok := v.(Document)
			if !ok {
				fmt.Printf("Unable to decode document %+v", v)
				continue
			}
			docs.Add(stored)
		}
	})
	return
}

// SoftDelete updates the index documents with a tombstone value
func (i *Index) SoftDelete(ids IDSet) (*DocSet, error) {
	now := time.Now().Unix()
	updates := NewDocSet()
	for id := range ids {
		updates.Add(Document{ID: id, Deleted: true, UpdatedAt: now})
	}

	return i.Update(updates)
}

// GetManifest returns a manifest document
func (i *Index) GetManifest(id string) (*Manifest, error) {
	docs, err := i.LoadDocuments(NewIDSet(id), 0)
	if err != nil {
		return nil, fmt.Errorf("Unable to load manifest %v: %v", id, err)
	}
	if docs == nil || len(docs.Docs) == 0 {
		return nil, fmt.Errorf("Unable to load manifest %v: not found", id)
	}

	manifestDocument := docs.Docs[id]
	docIds := IDSet{}

	if err = json.Unmarshal(manifestDocument.Body, &docIds); err != nil {
		return nil, fmt.Errorf("Unable to decode manifest body: %v", err)
	}

	m := Manifest{ID: id, UpdatedAt: manifestDocument.UpdatedAt, DocumentIDs: docIds}
	return &m, nil
}

// Query returns any documents matching the manifest with the given id that were updated after the given timestamp
func (i *Index) Query(manifestID string, updatedAfter Timestamp) (*DocSet, error) {
	m, err := i.GetManifest(manifestID)
	if err != nil {
		return nil, err
	}
	m.DocumentIDs[manifestID] = SetEntry{}
	return i.LoadDocuments(m.DocumentIDs, updatedAfter)
}

// LoadDocuments will, for a given set of document IDs, query the LRU cache for the latest matching versions and fetch the rest from the store
func (i *Index) LoadDocuments(docIDs IDSet, updatedAfter Timestamp) (*DocSet, error) {
	results := NewDocSet()
	uncachedIds := IDSet{}

	for k := range docIDs {
		if !i.cache.Contains(k) {
			uncachedIds[k] = SetEntry{}
			continue
		}
		cached, ok := i.cache.Get(k)
		if !ok {
			panic("Failed to get doc from cache")
		}
		doc, ok := cached.(Document)
		if !ok {
			panic("Non-document (id: " + k + ") found in store")
		}
		if doc.UpdatedAt > updatedAfter {
			results.Add(doc)
		}
	}

	i.docs.DoWithMap(func(m du.GenericMap) {
		var doc Document
		var ok bool
		for k := range uncachedIds {
			doc, ok = m[k].(Document)
			if !ok {
				panic("Non-document (id: " + k + ") found in store")
			}
			if doc.UpdatedAt > updatedAfter {
				results.Add(doc)
				i.cache.Add(k, doc)
			}
		}
	})

	return results, nil
}

// Keys returns all the keys in an index
func (i *Index) Keys() IDSet {
	keys := IDSet{}
	i.docs.DoWithMap(func(m du.GenericMap) {
		for k := range m {
			keys[k] = SetEntry{}
		}
	})
	return keys
}

// LRUKeys returns all the keys in an index's LRU cache
func (i *Index) LRUKeys() IDSet {
	keys := IDSet{}
	for _, k := range i.cache.Keys() {
		keys[k.(string)] = SetEntry{}
	}
	return keys
}
