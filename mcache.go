package mcache

// Config describes the configuration of an MCache instance
type Config struct {
	LRUCacheSize  int
	MaxIndexCount int
	MaxIndexSize  int
	DataDir       string
}

// DefaultConfig describes a default configuration for MCache
var DefaultConfig = Config{
	LRUCacheSize:  10000,
	MaxIndexCount: 100000,
	MaxIndexSize:  100000,
	DataDir:       "./.tmp",
}

// MCache is an HTTP-accessible object cache
type MCache struct {
	im *IndexManager
	Config
}

// NewMCache returns an MCache with the given configuration
func NewMCache(config Config, loader Loader) (*MCache, error) {
	im := NewIndexManager(config, loader)
	err := im.Scan()
	if err != nil {
		return nil, err
	}
	return &MCache{im, config}, nil
}

// CreateIndex creates a new index with the given ID
func (m *MCache) CreateIndex(id string) (*Index, error) {
	return m.im.Open(id)
}

// Keys returns all keys in an index
func (m *MCache) Keys(indexID string) IDSet {
	index := m.im.GetIndex(indexID)
	return index.Keys()
}

// Get gets a document in an index
func (m *MCache) Get(indexID string, docID string) (*Document, error) {
	index := m.im.GetIndex(indexID)
	return index.Get(docID)
}

// GetAll gets all documents in an index
func (m *MCache) GetAll(indexID string) (DocSet, error) {
	index := m.im.GetIndex(indexID)
	return index.GetAll()
}

// Query gets all index documents matching a given manifest that were updated after a given timestamp
func (m *MCache) Query(indexID string, manifestID string, updatedAfter Timestamp) (DocSet, error) {
	index := m.im.GetIndex(indexID)
	return index.Query(manifestID, updatedAfter)
}

// Update applies any given documents as patches to documents in the given index
func (m *MCache) Update(indexID string, docs DocSet) error {
	index := m.im.GetIndex(indexID)
	return index.Update(docs)
}

// SoftDelete overwrites documents in the given index with the given IDs with tombstone values
func (m *MCache) SoftDelete(indexID string, ids IDSet) error {
	index := m.im.GetIndex(indexID)
	return index.SoftDelete(ids)
}