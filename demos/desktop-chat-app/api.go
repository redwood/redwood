// +build !headless

package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"

	"github.com/markbates/pkger"
)

var chLoggedOut = make(chan struct{}, 1)

func startAPI(port uint) {
	http.HandleFunc("/api/login", loginUser)
	http.HandleFunc("/api/logout", logoutUser)
	http.HandleFunc("/ws", serveWs)
	http.HandleFunc("/", serveHome)

	err := http.ListenAndServe(fmt.Sprintf(":%v", port), nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func loginUser(w http.ResponseWriter, r *http.Request) {
	app.configPath = "./node2.redwoodrc"
	app.devMode = true
	err := app.Start()
	if err != nil {
		panic(err)
	}
	fmt.Fprintf(w, "WORKED!")
}

func logoutUser(w http.ResponseWriter, r *http.Request) {
	select {
	case chLoggedOut <- struct{}{}:
	default:
	}
	app.Close()
	fmt.Fprintf(w, "WORKED!")
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join("/frontend/build", r.URL.Path)
	fmt.Println("PATH: ", path)
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
