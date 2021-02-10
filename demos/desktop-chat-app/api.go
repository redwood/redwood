package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	// "github.com/gorilla/websocket"
	"github.com/markbates/pkger"

	rw "redwood.dev"
)

func startAPI(host rw.Host, port uint) {
	http.HandleFunc("/ws", serveWs(host))
	http.HandleFunc("/", serveHome)

	pkger.Walk("/frontend/build", func(path string, info os.FileInfo, err error) error {
		host.Infof(0, "Serving %v", path)
		return nil
	})

	err := http.ListenAndServe(fmt.Sprintf(":%v", port), nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join("/frontend/build", r.URL.Path)

	switch filepath.Ext(path) {
	case ".html":
		w.Header().Add("Content-Type", "text/html")
	case ".js":
		w.Header().Add("Content-Type", "application/javascript")
	case ".css":
		w.Header().Add("Content-Type", "text/css")
	case ".svg":
		w.Header().Add("Content-Type", "image/svg+xml")
	case ".map":
		w.Header().Add("Content-Type", "text/plain")
	}

	indexHTML, err := pkger.Open(path)
	if err != nil {
		panic(err) // @@TODO
	}

	_, err = io.Copy(w, indexHTML)
	if err != nil {
		panic(err) // @@TODO
	}
}
