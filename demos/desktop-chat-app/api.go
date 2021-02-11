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
	http.HandleFunc("/", serveHome)
	// http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
	//  serveWs(host, w, r)
	// })

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

// // serveWs handles websocket requests from the peer.
// func serveWs(hub *Hub, host rw.Host, w http.ResponseWriter, r *http.Request) {
//  defer r.Body.Close()

//  stateURI := r.URL.Query().Get("state_uri")

//  sub, err := host.Subscribe(ctx, request.StateURI, rw.SubscriptionType_States, nil)
//  if err != nil {
//      panic(err) // @@TODO
//  }
//  defer sub.Close()

//  conn, err := upgrader.Upgrade(w, r, nil)
//  if err != nil {
//      panic(err) // @@TODO
//  }

//  client := Client{conn: conn}

//  client.start()
//  defer client.stop()
// }

// const (
//  writeWait      = 10 * time.Second    // Time allowed to write a message to the peer.
//  pongWait       = 60 * time.Second    // Time allowed to read the next pong message from the peer.
//  pingPeriod     = (pongWait * 9) / 10 // Send pings to peer with this period. Must be less than pongWait.
//  maxMessageSize = 512                 // Maximum message size allowed from peer.
// )

// var upgrader = websocket.Upgrader{
//  ReadBufferSize:  1024,
//  WriteBufferSize: 1024,
// }

// type Client struct {
//  conn   *websocket.Conn
//  chStop chan struct{}
// }

// func (c *Client) start() {
//  c.chStop = make(chan struct{})

//  c.conn.SetReadLimit(maxMessageSize)
//  c.conn.SetReadDeadline(time.Now().Add(pongWait))
//  c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

//  ch := make(chan rw.SubscriptionMsg)
//  go func() {
//      for {
//          msg, err := sub.Read()
//          if err != nil {
//              panic(err) // @@TODO
//          }
//          select {
//          case ch <- msg:
//          case <-c.chStop:
//              return
//          }
//      }
//  }()

//  ticker := time.NewTicker(pingPeriod)
//  defer ticker.Stop()
//  for {
//      select {
//      case msg := <-ch:
//          c.sendMsg(msg)
//      case <-ticker.C:
//          c.ping()
//      case <-c.chStop:
//          return
//      }
//  }
// }

// func (c *Client) stop() {
//  conn.WriteMessage(websocket.CloseMessage, []byte{})
//  conn.Close()
//  close(c.chStop)
// }

// func (c *Client) sendMsg(msg rw.SubscriptionMsg) {
//  err = conn.SetWriteDeadline(time.Now().Add(writeWait))
//  if err != nil {
//      panic(err) // @@TODO
//  }
//  err = conn.WriteJSON(msg)
//  if err != nil {
//      panic(err) // @@TODO
//  }
// }

// func (c *Client) ping() {
//  c.conn.SetWriteDeadline(time.Now().Add(writeWait))
//  if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
//      return
//  }
// }
