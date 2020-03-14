package main

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/notduncansmith/mcache"
)

/*
	Env vars:
		- HOST ("")
		- PORT (":1337")
		- ADMIN_SECRET ("")
		- XWT_PUBLIC_KEY ("")
		- MC_MAX_LRU_SIZE (DefaultConfig)
		- MC_MAX_INDEX_COUNT (DefaultConfig)
		- MC_MAX_INDEX_SIZE (DefaultConfig)
		- MC_DATA_DIR (DefaultConfig)
		- MONGODB_CONNECTION_STRING ("")
*/

type ck string
type genericMap map[string]interface{}

func main() {
	now := time.Now()
	manifestDoc, _ := mcache.EncodeManifest(mcache.Manifest{
		ID:          "hardcoded",
		UpdatedAt:   now.Add(-1 * time.Minute).Unix(),
		DocumentIDs: mcache.NewIDSet("a", "b"),
	})
	docs := mcache.DocSet{
		"a":         mcache.Document{ID: "a", UpdatedAt: now.Add(-3 * time.Minute).Unix(), Body: []byte("Document (a)")},
		"b":         mcache.Document{ID: "b", UpdatedAt: now.Add(-2 * time.Minute).Unix(), Body: []byte("Document (b)")},
		"c":         mcache.Document{ID: "c", UpdatedAt: now.Unix(), Body: []byte("Document (c)")},
		"hardcoded": *manifestDoc,
	}
	m, err := mcache.NewMCache(mcache.DefaultConfig)
	if err != nil {
		panic(err)
	}
	idx, _ := m.CreateIndex("dev")
	err = idx.Update(docs)
	if err != nil {
		panic(err)
	}
	router := httprouter.New()
	router.GET("/docs/:updatedAfter", requireToken(func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		indexID := getCtx(r, "indexID").(string)
		manifestID := getCtx(r, "manifestID").(string)
		updatedAfterStr := ps.ByName("updatedAfter")
		updatedAfter, err := strconv.ParseInt(updatedAfterStr, 10, 64)
		if err != nil {
			badRequest(&w, "Invalid updatedAfter ("+updatedAfterStr+")")
			return
		}
		w.WriteHeader(200)
		docs, err := m.Query(indexID, manifestID, updatedAfter)
		if err != nil {
			if strings.Contains(err.Error(), "No index") {
				badRequest(&w, "Invalid index ("+indexID+")")
				return
			}
			unknownError(&w, err)
			return
		}
		/*
			TODO:
				- Connect() and give channel to startSSE
		*/
	}))
}

func requireToken(next httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		token := getToken(r)
		if token == "" {
			notFound(&w)
		}
		pieces := strings.Split(token, ":")
		if len(pieces) != 2 {
			forbidden(&w, "Invalid token")
		}
		r = setCtx(r, genericMap{"indexID": pieces[0], "manifestID": pieces[1]})
		next(w, r, ps)
	}
}

func getToken(r *http.Request) string {
	authVals := r.Header["Authentication"]
	token := ""
	if len(authVals) > 0 {
		token = authVals[0]
	}
	return token
}

func setCtx(r *http.Request, kv genericMap) *http.Request {
	for k, v := range kv {
		r = r.WithContext(context.WithValue(r.Context(), ck(k), v))
	}
	return r
}

func getCtx(r *http.Request, k string) interface{} {
	return r.Context().Value(ck(k))
}

func badRequest(w *http.ResponseWriter, message string) {
	(*w).WriteHeader(400)
	(*w).Write([]byte("Bad request: " + message))
}

func forbidden(w *http.ResponseWriter, message string) {
	(*w).WriteHeader(401)
	(*w).Write([]byte("Not allowed: " + message))
}

func notFound(w *http.ResponseWriter) {
	(*w).WriteHeader(404)
	(*w).Write([]byte("Not found"))
}

func unknownError(w *http.ResponseWriter, err error) {
	(*w).WriteHeader(500)
	(*w).Write([]byte("Unknown error: " + err.Error()))
}

func startSSE(ww *http.ResponseWriter, msgs chan []byte) error {
	w := *ww
	f, ok := w.(http.Flusher)
	if !ok {
		return errors.New("Streaming unsupported")
	}

	// Set the headers related to event streaming.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")

	prefix := []byte("data: ")
	prefixLen := len(prefix)
	for msg := range msgs {
		bz := make([]byte, len(msg)+prefixLen)
		copy(bz[:prefixLen], prefix)
		copy(bz[prefixLen:], msg)
		_, err := w.Write(bz)
		if err != nil {
			return err
		}
		f.Flush()
	}

	return nil
}
