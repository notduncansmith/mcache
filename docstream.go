package mcache

import (
	"fmt"
	"time"

	"github.com/notduncansmith/mutable"
)

// ConnectionKey is a nanosecond timestamp
type ConnectionKey int64

// ConnectionMap is a map of connection keys to channels
type ConnectionMap map[ConnectionKey]chan DocSet

// DocStream is a broker for broadcasting updates to documents referenced by a specific manifest
type DocStream struct {
	*mutable.RW
	Index       *Index
	ManifestID  string
	Connections ConnectionMap
}

func NewDocStream(index *Index, manifestID string) *DocStream {
	return &DocStream{
		RW:          mutable.NewRW("docstream:" + manifestID),
		Index:       index,
		ManifestID:  manifestID,
		Connections: ConnectionMap{},
	}
}

// Update broadcasts a changed set of docs to any connections
func (s *DocStream) Update(docs DocSet) {
	s.DoWithRLock(func() {
		for _, ch := range s.Connections {
			manifest, err := s.Index.GetManifest(s.ManifestID)
			if err != nil {
				fmt.Printf("Error fetching manifest: %v", err)
				continue
			}
			connDocs := DocSet{}
			for id, doc := range docs {
				if manifest.DocumentIDs[id] {
					connDocs[id] = doc
				}
			}
			if len(connDocs) > 0 {
				ch <- connDocs
			}
		}
	})
}

// Connect adds a new connection
func (s *DocStream) Connect() (ConnectionKey, chan DocSet) {
	ch := make(chan DocSet, 100)
	k := ConnectionKey(time.Now().UnixNano())
	s.DoWithRWLock(func() {
		s.Connections[k] = ch
	})
	return k, ch
}

// Disconnect removes a connection
func (s *DocStream) Disconnect(k ConnectionKey) (err error) {
	s.DoWithRWLock(func() {
		if s.Connections[k] == nil {
			err = fmt.Errorf("No connection for key %v", k)
		}
		close(s.Connections[k])
		delete(s.Connections, k)
	})
	return
}