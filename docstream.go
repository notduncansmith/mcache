package mcache

import (
	"fmt"
	"time"

	"github.com/notduncansmith/mutable"
)

// ConnectionKey is a nanosecond timestamp
type ConnectionKey int64

// ChangeFeed is a channel which receives updated DocSets
type ChangeFeed chan DocSet

// Connection contains , and a Key which can be used to Disconnect() later
type Connection struct {
	stream *DocStream
	ChangeFeed
	Key ConnectionKey
}

// Disconnect will terminate the connection
func (c *Connection) Disconnect() error {
	return c.stream.Disconnect(c.Key)
}

// ConnectionMap is a map of connection keys to channels
type ConnectionMap map[ConnectionKey]*Connection

// DocStream is a broker for broadcasting updates to documents referenced by a specific manifest
type DocStream struct {
	*mutable.RW
	Index       *Index
	ManifestID  string
	Connections ConnectionMap
}

// NewDocStream returns a DocStream with the given Index and ManifestID
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
		for _, c := range s.Connections {
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
				c.ChangeFeed <- connDocs
			}
		}
	})
}

// Connect adds a new connection
func (s *DocStream) Connect() *Connection {
	ch := make(ChangeFeed, 1024)
	k := ConnectionKey(time.Now().UnixNano())
	c := &Connection{Key: k, ChangeFeed: ch, stream: s}
	s.DoWithRWLock(func() {
		s.Connections[k] = c
	})
	return c
}

// Disconnect removes a connection
func (s *DocStream) Disconnect(k ConnectionKey) (err error) {
	s.DoWithRWLock(func() {
		if s.Connections[k] == nil {
			err = fmt.Errorf("No connection for key %v", k)
		}
		close(s.Connections[k].ChangeFeed)
		delete(s.Connections, k)
	})
	return
}
