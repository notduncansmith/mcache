package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"

	"git.sr.ht/~dms/mcache"
	"github.com/joho/godotenv"
	"github.com/julienschmidt/httprouter"
)

func main() {
	config := loadConfig()
	m, err := mcache.NewMCache(config)
	if err != nil {
		panic("Error loading MCache: " + err.Error())
	}

	buildHardcodedSampleIndex(m)

	router := httprouter.New()
	router.POST("/i/:indexID", createHandler(m))
	router.PUT("/i/:indexID", updateHandler(m))
	router.GET("/i/:indexID/m/:manifestID/@/:updatedAfter", queryHandler(m))

	http.ListenAndServe(config.Host+":"+config.Port, router)
}

func queryHandler(m *mcache.MCache) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		indexID := ps.ByName("indexID")
		manifestID := ps.ByName("manifestID")
		updatedAfterStr := ps.ByName("updatedAfter")
		updatedAfter, err := strconv.ParseInt(updatedAfterStr, 10, 64)
		if err != nil {
			badRequest(&w, "Invalid updatedAfter ("+updatedAfterStr+")")
			return
		}

		docs, err := m.Query(indexID, manifestID, updatedAfter)
		if err != nil {
			if strings.Contains(err.Error(), "No index") {
				badRequest(&w, "Invalid index ("+indexID+")")
				return
			}
			unknownError(&w, err)
			return
		}

		bz, err := json.Marshal(docs)
		if err != nil {
			panic("Error encoding docs: " + err.Error())
		}
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(200)
		_, err = w.Write(bz)
		if err != nil {
			panic("Error writing HTTP response: " + err.Error())
		}
	}
}

func updateHandler(m *mcache.MCache) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		indexID := ps.ByName("indexID")
		bodyBz, err := ioutil.ReadAll(r.Body)
		if err != nil {
			badRequest(&w, "Error reading request body: "+err.Error())
			return
		}
		docsArray := []mcache.Document{}
		if err = json.Unmarshal(bodyBz, &docsArray); err != nil {
			badRequest(&w, "Error decoding request body: "+err.Error())
			return
		}

		docs := mcache.NewDocSet(docsArray...)
		updated, err := m.Update(indexID, docs)
		if err != nil {
			unknownError(&w, err)
			return
		}
		w.WriteHeader(200)
		body, err := json.Marshal(updated)
		if err != nil {
			badRequest(&w, "Error decoding request body: "+err.Error())
			return
		}
		w.Write([]byte(body))
	}
}

func createHandler(m *mcache.MCache) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		indexID := ps.ByName("indexID")
		if indexID == "" || len(indexID) > 512 {
			badRequest(&w, "Invalid index id ("+indexID+")")
			return
		}

		idx := m.GetIndex(indexID)
		if idx != nil {
			badRequest(&w, "Index exists")
			return
		}

		idx, err := m.CreateIndex(indexID)
		if err != nil {
			unknownError(&w, err)
		}

		bz, err := json.Marshal(idx)
		if err != nil {
			unknownError(&w, err)
		}

		jsonSuccess(&w, bz)
	}
}

func badRequest(w *http.ResponseWriter, message string) {
	(*w).WriteHeader(400)
	(*w).Write([]byte("Bad request: " + message))
}

func notFound(w *http.ResponseWriter) {
	(*w).WriteHeader(404)
	(*w).Write([]byte("Not found"))
}

func unknownError(w *http.ResponseWriter, err error) {
	(*w).WriteHeader(500)
	(*w).Write([]byte("Unknown error: " + err.Error()))
}

func jsonSuccess(w *http.ResponseWriter, bz []byte) {
	(*w).WriteHeader(200)
	(*w).Header().Add("Content-Type", "application/json")
}

func buildHardcodedSampleIndex(m *mcache.MCache) {
	manifestDoc, _ := mcache.EncodeManifest(&mcache.Manifest{
		ID:          "m",
		DocumentIDs: mcache.NewIDSet("a", "b"),
	})
	docs := mcache.DocSet{
		Docs: map[string]mcache.Document{
			"a": {ID: "a", Body: []byte("Document A")},
			"b": {ID: "b", Body: []byte("Document B")},
			"c": {ID: "c", Body: []byte("Document C")},
			"m": *manifestDoc,
		},
	}
	idx, _ := m.CreateIndex("sample")
	_, err := idx.Update(&docs)
	if err != nil {
		panic(err)
	}
}

func loadConfig() mcache.Config {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file: " + err.Error())
	}

	host := os.Getenv("HOST")
	if host == "" {
		host = "localhost"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "1337"
	}

	dataDir := os.Getenv("MC_DATA_DIR")
	if dataDir == "" {
		dataDir = mcache.DefaultConfig.DataDir
	}

	maxIndexCount := mustParseEnvInt("MC_MAX_INDEX_COUNT", mcache.DefaultConfig.MaxIndexCount)
	maxIndexSize := mustParseEnvInt("MC_MAX_INDEX_SIZE", mcache.DefaultConfig.MaxIndexSize)
	lruCacheSize := mustParseEnvInt("MC_LRU_CACHE_SIZE", mcache.DefaultConfig.LRUCacheSize)

	return mcache.Config{
		Host:          host,
		Port:          port,
		DataDir:       dataDir,
		MaxIndexCount: maxIndexCount,
		MaxIndexSize:  maxIndexSize,
		LRUCacheSize:  lruCacheSize,
	}
}

func mustParseEnvInt(key string, defaultVal int) int {
	valStr := os.Getenv(key)
	if len(valStr) == 0 {
		return defaultVal
	}
	valInt, err := strconv.Atoi(valStr)
	if err != nil {
		panic("Error parsing " + key)
	}
	return valInt
}
