package redwood

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type httpServer struct {
	*http.ServeMux
	*host
}

func NewHTTPServer(host *host) *httpServer {
	s := &httpServer{
		ServeMux: http.NewServeMux(),
		host:     host,
	}

	s.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// if r.Method == "GET" {
		// 	s.get(w, r)
		// } else
		if r.Method == "POST" {
			s.post(w, r)
		}
	})

	go func() {
		err := http.ListenAndServe(":9999", s)
		if err != nil {
			panic(err)
		}
	}()

	return s
}

func (h *httpServer) get(w http.ResponseWriter, r *http.Request) {
	keypath := strings.Split(r.URL.Path[1:], "/")
	stateMap, isMap := h.host.Store.State().(map[string]interface{})
	if !isMap {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	val, exists := M(stateMap).GetValue(keypath...)
	if !exists {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	if r.Header.Get("Accept") == "application/json" {
		var resp struct {
			MostRecentTxID ID          `json:"mostRecentTxID"`
			Data           interface{} `json:"data"`
		}
		resp.MostRecentTxID = h.host.Store.MostRecentTxID()

		switch v := val.(type) {
		case string:
			resp.Data = v

		case []byte:
			resp.Data = string(v) // @@TODO: probably don't want this

		case map[string]interface{}, []interface{}:
			resp.Data = v

		default:
			http.Error(w, "not found", http.StatusNotFound)
		}

		j, err := json.Marshal(resp)
		if err != nil {
			panic(err)
		}

		_, err = io.Copy(w, bytes.NewBuffer(j))
		if err != nil {
			panic(err)
		}

	} else {
		switch v := val.(type) {
		case string:
			_, err := io.Copy(w, bytes.NewBuffer([]byte(v)))
			if err != nil {
				panic(err)
			}

		case []byte:
			_, err := io.Copy(w, bytes.NewBuffer(v))
			if err != nil {
				panic(err)
			}

		case map[string]interface{}, []interface{}:
			j, err := json.Marshal(v)
			if err != nil {
				panic(err)
			}
			_, err = io.Copy(w, bytes.NewBuffer(j))
			if err != nil {
				panic(err)
			}

		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}
}

func (h *httpServer) post(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var tx Tx
	err := json.NewDecoder(r.Body).Decode(&tx)
	if err != nil {
		panic(err)
	}

	err = h.host.AddTx(context.TODO(), tx)
	if err != nil {
		panic(err)
	}
}
