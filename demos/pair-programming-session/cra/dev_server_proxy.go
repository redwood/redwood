package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/yhat/wsutil"
)

func isDevServerRoute(method, url, subscribe string) bool {
	if strings.HasPrefix(url, "/static") {
		return true
	} else if strings.HasPrefix(url, "/manifest.json") {
		return true
	} else if strings.Contains(url, "hot-update") {
		return true
	} else if strings.Contains(url, "logo") {
		return true
	} else if method == "GET" && url == "/" && subscribe == "" {
		return true
	}
	return false
}

var backendURL = &url.URL{Scheme: "ws://", Host: ":3000"}
var wsproxy = wsutil.NewSingleHostReverseProxy(backendURL)

func handleHTTP(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	fmt.Println(req.Proto, req.Host, req.URL)

	var theURL string
	if isDevServerRoute(req.Method, req.URL.String(), req.Header.Get("Subscribe")) {
		fmt.Println("(dev server)")
		theURL = "http://localhost:3000" + req.URL.String()
	} else if strings.Contains(req.URL.String(), "/sockjs-node") {
		wsproxy.ServeHTTP(w, req)
		return
	} else {
		fmt.Println("(redwood)")
		theURL = "http://localhost:8080" + req.URL.String()
	}

	req2, err := http.NewRequest(req.Method, theURL, req.Body)
	if err != nil {
		panic(err)
	}
	copyHeader(req2.Header, req.Header)

	if req.Header.Get("Subscribe") != "" {
		u, err := url.Parse(theURL)
		if err != nil {
			panic(err)
		}

		proxy := httputil.NewSingleHostReverseProxy(u)
		proxy.ServeHTTP(w, req2)
		return
	}

	resp, err := http.DefaultClient.Do(req2)
	if err != nil {
		fmt.Println("err ~>", err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()
	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func main() {
	server := &http.Server{
		Addr: ":3001",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handleHTTP(w, r)
		}),
		// // Disable HTTP/2.
		// TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}
	log.Fatal(server.ListenAndServe())
}
