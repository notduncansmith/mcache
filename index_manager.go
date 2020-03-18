package mcache

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	du "github.com/notduncansmith/duramap"
	"github.com/notduncansmith/mutable"
)

const indexFilenamePrefix = "mcache-index-"
const indexFilenameSuffix = ".db"

// IndexManager manages a collection of Indexes
type IndexManager struct {
	*mutable.RW
	path          string
	maxIndexCount int
	maxIndexSize  int
	lruCacheSize  int
	Indexes       map[string]*Index
}

// NewIndexManager initializes an IndexManager at the given path
func NewIndexManager(config Config) *IndexManager {
	return &IndexManager{
		RW:            mutable.NewRW("IndexManager:" + config.DataDir),
		path:          config.DataDir,
		maxIndexCount: config.MaxIndexCount,
		maxIndexSize:  config.MaxIndexSize,
		lruCacheSize:  config.LRUCacheSize,
		Indexes:       map[string]*Index{},
	}
}

// Open creates or returns an index with the given id
func (m *IndexManager) Open(id string) (*Index, error) {
	i := m.GetIndex(id)
	if i != nil {
		fmt.Printf("Index %v already open\n", id)
		return i, nil
	}

	docs, err := du.NewDuramap(filepath.Join(m.path, indexFilenamePrefix+id+indexFilenameSuffix), id, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to open Duramap: %v", err)
	}

	i, err = NewIndex(id, docs, m.lruCacheSize)
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize index: %v", err)
	}

	m.DoWithRWLock(func() {
		m.Indexes[id] = i
	})

	return i, nil
}

// GetIndex returns the index with the given id, or nil if one is not found
func (m *IndexManager) GetIndex(id string) *Index {
	i := m.WithRLock(func() interface{} {
		return m.Indexes[id]
	})

	if i != nil {
		return i.(*Index)
	}

	return nil
}

// Scan will open any indexes whose data files are in the configured directory
func (m *IndexManager) Scan() error {
	_, err := os.Stat(m.path)
	if err != nil {
		fmt.Printf("Creating %v\n", m.path)
		if err = os.MkdirAll(m.path, 0700); err != nil {
			return errUnreachable(m.path, err.Error())
		}
	}

	files, _ := ioutil.ReadDir(m.path)
	fmt.Printf("Scanned path %v, found %v files\n", m.path, len(files))
	if len(files) == 0 {
		return nil
	}

	for i, file := range files {
		if !strings.HasPrefix(file.Name(), indexFilenamePrefix) {
			fmt.Printf("Skipping non-index file %v\n", file.Name())
			continue
		}
		fmt.Printf("Loading index #%v from %v\n", i, file.Name())
		_, err := m.Open(indexIDFromFilename(file.Name()))
		if err != nil {
			return fmt.Errorf("Error loading index #%v from %v: %v", i, file.Name(), err)
		}
	}

	return err
}

func errUnreachable(path string, reason string) error {
	return fmt.Errorf("File or directory %v cannot be opened (%v)", path, reason)
}

func indexIDFromFilename(name string) string {
	return strings.Replace(strings.Replace(name, indexFilenamePrefix, "", 1), indexFilenameSuffix, "", 1)
}
