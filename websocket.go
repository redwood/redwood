package redwood

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	wsWriteWait  = 10 * time.Second      // Time allowed to write a message to the peer.
	wsPongWait   = 10 * time.Second      // Time allowed to read the next pong message from the peer.
	wsPingPeriod = (wsPongWait * 9) / 10 // Send pings to peer with this period. Must be less than wsPongWait.
)

var (
	newline    = []byte{'\n'}
	space      = []byte{' '}
	wsUpgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(*http.Request) bool { return true },
	}
)
