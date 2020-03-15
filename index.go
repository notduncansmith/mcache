package mcache

import (
	"encoding/json"
	"fmt"

	lru "github.com/hashicorp/golang-lru"
	du "github.com/notduncansmith/duramap"
)

// Index represents a collection of documents managed by the cache
type Index struct {
	ID      string
	docs    *du.Duramap
	cache   *lru.TwoQueueCache
	streams map[string]DocStream
}

// NewIndex returns a new Index
func NewIndex(id string, docs *du.Duramap, cacheSize int) (*Index, error) {
	cache, err := lru.New2Q(cacheSize)
	if err != nil {
		return nil, err
	}

	return &Index{id, docs, cache, map[string]DocStream{}}, nil
}

// Update updates the index documents with the latest versions
func (i *Index) Update(docs DocSet) error {
	updated := DocSet{}

	i.docs.UpdateMap(func(tx *du.Tx) error {
		for _, d := range docs {
			stored := tx.Get(d.ID)
			if stored == nil {
				tx.Set(d.ID, d)
				i.cache.Add(d.ID, d)
				continue
			}
			storedDoc, ok := stored.(Document)
			if !ok {
				fmt.Printf("Unable to decode document: %+v", stored)
				continue
			}

			if storedDoc.UpdatedAt < d.UpdatedAt {
				tx.Set(d.ID, d)
				i.cache.Add(d.ID, d)
				updated[d.ID] = d
			}
		}

		return nil
	})

	for _, s := range i.streams {
		s.Update(updated)
	}

	return nil
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

// TODO: Vacuum will permanently delete any tombstone documents that haven't been updated since a certain date

// GetManifest returns a manifest document
func (i *Index) GetManifest(manifestDocumentID string) (*Manifest, error) {
	docs, err := i.LoadDocuments(NewIDSet(manifestDocumentID), 0)
	if err != nil {
		return nil, fmt.Errorf("Unable to load manifest %v: %v", manifestDocumentID, err)
	}
	if docs == nil || len(docs) == 0 {
		return nil, fmt.Errorf("Unable to load manifest %v: not found", manifestDocumentID)
	}

	manifestDocument := docs[manifestDocumentID]
	docIds := IDSet{}

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

// Connect returns a Connection to the DocStream for a given manifestID
func (i *Index) Connect(manifestID string) *Connection {
	stream := i.streams[manifestID]
	if stream.ManifestID == "" {
		stream = *NewDocStream(i, manifestID)
		i.streams[manifestID] = stream
	}
	return stream.Connect()
}

// LoadDocuments will, for a given set of document IDs, query the LRU cache for the latest matching versions and fetch the rest from the store
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
			uncachedIds[k] = true
		}
	}

	i.docs.DoWithMap(func(m du.GenericMap) {
		var doc Document
		var ok bool
		for k := range uncachedIds {
			doc, ok = m[k].(Document)
			if !ok {
				continue
			}
			if doc.UpdatedAt > updatedAfter {
				docs[k] = doc
				i.cache.Add(k, doc)
			}
		}
	})

	return docs, nil
}

// Keys returns all the keys in an index
func (i *Index) Keys() IDSet {
	keys := IDSet{}
	i.docs.DoWithMap(func(m du.GenericMap) {
		for k := range m {
			keys[k] = true
		}
	})
	return keys
}

// LRUKeys returns all the keys in an index's LRU cache
func (i *Index) LRUKeys() IDSet {
	keys := IDSet{}
	for _, k := range i.cache.Keys() {
		keys[k.(string)] = true
	}
	return keys
}
