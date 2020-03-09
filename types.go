package mcache

import (
	"encoding/json"
	"time"
)

// Timestamp is a Unix milliseconds offset
type Timestamp = int64

// A Stub is a representation of a document in an index that does not contain the full body
type Stub struct {
	ID        string    `json:"id"`
	UpdatedAt Timestamp `json:"updatedAt"`
}

// Document is a resource that can be accessed by users
type Document struct {
	ID        string    `json:"id"`
	UpdatedAt Timestamp `json:"updatedAt"`
	Body      []byte    `json:"body"`
}

// Tombstone represents a document that was deleted
type Tombstone struct {
	ID        string    `json:"id"`
	UpdatedAt Timestamp `json:"updatedAt"`
	Body      []byte    `json:"body"`
	Deleted   bool      `json:"deleted"`
}

// NewTombstone returns a Tombstone with the given id
func NewTombstone(id string) Tombstone {
	return Tombstone{id, time.Now().Unix(), []byte{}, true}
}

// IDSet is an emulated Set (map of strings to empty structs) of document ID
type IDSet = map[string]struct{}

// DocSet is a map of document IDs to documents
type DocSet = map[string]Document

// NewDocSet returns a DocSet for a set of docs
func NewDocSet(docs ...Document) DocSet {
	docset := DocSet{}
	for _, d := range docs {
		docset[d.ID] = d
	}
	return docset
}

// NewIDSet returns an IDSet for a set of IDs
func NewIDSet(ids ...string) IDSet {
	idset := IDSet{}
	for _, id := range ids {
		idset[id] = struct{}{}
	}
	return idset
}

// Manifest is a user's set of accessible document IDs
type Manifest struct {
	ID          string `json:"id"`
	UpdatedAt   int64  `json:"updatedAt"`
	DocumentIDs IDSet  `json:"documentIDs"`
}

// EncodeManifest returns a Document that stores a Manifest
func EncodeManifest(m Manifest) (*Document, error) {
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
