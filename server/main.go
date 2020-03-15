package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/notduncansmith/bbq"
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
type messageFeed chan []byte

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
		conn, err := m.Connect(indexID, manifestID)
		if err != nil {
			badRequest(&w, "Unabled to connect to index/manifest ("+indexID+", "+manifestID+"): "+err.Error())
			return
		}
		notify := w.(http.CloseNotifier).CloseNotify()
		go func() {
			<-notify
			conn.Disconnect()
		}()

		docs, err := m.Query(indexID, manifestID, updatedAfter)
		if err != nil {
			if strings.Contains(err.Error(), "No index") {
				badRequest(&w, "Invalid index ("+indexID+")")
				return
			}
			unknownError(&w, err)
			return
		}

		outgoing := make(messageFeed, 16384)
		q := bbq.NewBatchQueue(func(items []interface{}) error {
			for _, item := range items {
				bz, err := json.Marshal(item)
				if err != nil {
					fmt.Printf("Error writing update: %+v\n", err)
					continue
				}
				outgoing <- bz
			}
			return nil
		}, bbq.BatchQueueOptions{FlushTime: 3 * time.Second, FlushCount: 10})
		done := make(chan bool)
		q.Enqueue(docs)
		q.FlushNow()
		startSSE(&w, outgoing, done)

		defer close(outgoing)
		defer q.FlushNow()

		for {
			select {
			case docs, open := <-conn.ChangeFeed:
				if !open {
					return
				}
				q.Enqueue(docs)
				fmt.Printf("Enqueued changes: %+v", docs)
			case <-done:
				err := conn.Disconnect()
				if err != nil {
					fmt.Printf("Error disconnecting: %+v", err)
				}
				return
			default:
				q.FlushNow()
			}
		}
	}))

	router.POST("/docs", requireToken(func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		indexID := getCtx(r, "indexID").(string)
		bodyBz, err := ioutil.ReadAll(r.Body)
		if err != nil {
			badRequest(&w, "Error reading request body: "+err.Error())
			return
		}
		docs := mcache.DocSet{}
		if err = json.Unmarshal(bodyBz, &docs); err != nil {
			badRequest(&w, "Error decoding request body: "+err.Error())
			return
		}
		if err = m.Update(indexID, docs); err != nil {
			unknownError(&w, err)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte("Updated"))
	}))

	http.ListenAndServe(":1337", router)
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

func startSSE(ww *http.ResponseWriter, msgs messageFeed, done chan bool) {
	w := *ww
	f, ok := w.(http.Flusher)
	if !ok {
		badRequest(ww, "Streaming unsupported")
		done <- true
		return
	}
	// Set the headers related to event streaming.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")
	f.Flush()

	prefix := []byte("data: ")
	prefixLen := len(prefix)
	go func() {
		for {
			msg, open := <-msgs
			if !open {
				fmt.Println("Outgoing message channel closed")
				f.Flush()
				break
			}
			bz := make([]byte, len(msg)+prefixLen+2)
			copy(bz[:prefixLen], prefix)
			copy(bz[prefixLen:], msg)
			copy(bz[len(msg)+prefixLen:], "\n\n")
			fmt.Printf("Writing bz: %v\n", string(bz))
			count, err := w.Write(bz)
			if err != nil {
				fmt.Printf("Error writing: %v\n", err)
				break
			}
			fmt.Printf("Flushing %v bytes\n", count)
			f.Flush()
		}
		done <- true
	}()
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
