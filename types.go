package mcache

import (
	"encoding/json"
)

// Timestamp is a Unix milliseconds offset
type Timestamp = int64

// Document is a resource that can be accessed by users
type Document struct {
	ID        string    `json:"id"`
	UpdatedAt Timestamp `json:"updatedAt"`
	Body      []byte    `json:"body"`
	Deleted   bool      `json:"deleted"`
}

// IDSet is an emulated Set (map of strings to empty structs) of document ID
type IDSet map[string]SetEntry

// A SetEntry is an empty struct that represents membership in a set
type SetEntry struct{}

// DocSet is a map of document IDs to documents
type DocSet struct {
	Docs  map[string]Document
	Start Timestamp
	End   Timestamp
}

// NewDocSet returns a DocSet for a set of docs
func NewDocSet(docs ...Document) *DocSet {
	docset := &DocSet{Docs: map[string]Document{}, Start: 0, End: 0}
	for _, v := range docs {
		docset.Add(v)
	}
	return docset
}

// Add adds a Document to the DocSet
func (d *DocSet) Add(docs ...Document) *DocSet {
	for _, doc := range docs {
		if d.Start == 0 || doc.UpdatedAt < d.Start {
			d.Start = doc.UpdatedAt
		}
		if doc.UpdatedAt > d.End {
			d.End = doc.UpdatedAt
		}
		d.Docs[doc.ID] = doc
	}
	return d
}

// Merge adds all Documents in a given DocSet to the DocSet
func (d *DocSet) Merge(docs *DocSet) *DocSet {
	for _, doc := range docs.Docs {
		d.Add(doc)
	}
	return d
}

// NewIDSet returns an IDSet for a set of IDs
func NewIDSet(ids ...string) IDSet {
	idset := IDSet{}
	for _, id := range ids {
		idset[id] = SetEntry{}
	}
	return idset
}

// Manifest is a user's set of accessible document IDs
type Manifest struct {
	ID          string `json:"id"`
	UpdatedAt   int64  `json:"updatedAt"`
	DocumentIDs IDSet  `json:"documentIDs"`
}

// Add a document to the manifest
func (m *Manifest) Add(documentID string) {
	m.DocumentIDs[documentID] = SetEntry{}
}

// EncodeManifest returns a Document that stores a Manifest
func EncodeManifest(m *Manifest) (*Document, error) {
	body, err := json.Marshal(m.DocumentIDs)
	if err != nil {
		return nil, err
	}
	return &Document{ID: m.ID, UpdatedAt: m.UpdatedAt, Body: body}, nil
}

// DecodeManifest returns a Manifest that is stored in a Document
func DecodeManifest(d Document) (*Manifest, error) {
	documentIDs := IDSet{}
	err := json.Unmarshal(d.Body, &documentIDs)
	if err != nil {
		return nil, err
	}
	return &Manifest{ID: d.ID, UpdatedAt: d.UpdatedAt, DocumentIDs: documentIDs}, nil
}
