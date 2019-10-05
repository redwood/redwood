package redwood

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type httpTransport struct {
	ackHandler AckHandler
	putHandler PutHandler

	subscriptionsIn  map[string][]*httpSubscriptionIn
	subscriptionsOut map[string]subscriptionOut
}

func NewHTTPTransport(ctx context.Context, port uint) (Transport, error) {
	t := &httpTransport{}

	http.Handle("/", t)
	// http.ListenAndServe

	return t, nil
}

type httpSubscriptionIn struct {
	io.Writer
	http.Flusher
	chDone chan struct{}
}

func (t *httpTransport) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	switch r.Method {
	case "GET":
		// Make sure that the writer supports flushing.
		f, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		// Listen to the closing of the http connection via the CloseNotifier
		notify := w.(http.CloseNotifier).CloseNotify()
		go func() {
			<-notify
			log.Println("http connection closed")
		}()

		// Set the headers related to event streaming.
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Transfer-Encoding", "chunked")

		url := r.URL.String()
		sub := &httpSubscriptionIn{w, f, make(chan struct{})}
		t.subscriptionsIn[url] = append(t.subscriptionsIn[url], sub)

		// Block until the subscription is canceled
		<-sub.chDone

	case "ACK":
		var versionID ID // @@TODO
		t.ackHandler(r.URL.String(), versionID)

	case "PUT":
		var tx Tx
		err := json.NewDecoder(r.Body).Decode(&tx)
		if err != nil {
			panic(err)
		}
		tx.URL = r.URL.String()
		t.onReceivedPut(tx)

	default:
		panic("bad")
	}
}

func (s *httpSubscriptionIn) Close() error {
	close(s.chDone)
	return nil
}

func (t *httpTransport) Subscribe(ctx context.Context, url string) error {
	if url[:6] == "braid:" {
		url = "http:" + url[6:]
	}

	_, exists := t.subscriptionsOut[url]
	if exists {
		return errors.New("already subscribed to " + url)
	}

	resp, err := http.Get(url)
	if err != nil {
		return err
	}

	stream := resp.Body

	chDone := make(chan struct{})
	t.subscriptionsOut[url] = subscriptionOut{stream, chDone}

	go func() {
		defer stream.Close()
		for {
			select {
			case <-chDone:
				return
			default:
			}

			var msg Msg
			err := ReadMsg(stream, &msg)
			if err != nil {
				log.Errorln(err)
				return
			}

			if msg.Type != MsgType_Put {
				panic("protocol error")
			}

			tx := msg.Payload.(Tx)
			tx.URL = url
			t.onReceivedPut(tx)
		}
	}()

	return nil
}

func (t *httpTransport) Ack(ctx context.Context, url string, versionID ID) error {
	if url[:6] == "braid:" {
		url = "http:" + url[6:]
	}

	body := bytes.NewReader(versionID[:])

	client := http.Client{}
	req, err := http.NewRequest("ACK", url, body)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode > 299 {
		return errors.Errorf("http error: %v", resp.StatusCode)
	}
	return nil
}

func (t *httpTransport) Put(ctx context.Context, tx Tx) error {
	url := tx.URL

	if url[:6] == "braid:" {
		url = "http:" + url[6:]
	}

	txBytes, err := json.Marshal(tx)
	if err != nil {
		return errors.WithStack(err)
	}
	body := bytes.NewReader(txBytes)

	client := http.Client{}
	req, err := http.NewRequest("PUT", url, body)
	if err != nil {
		return errors.WithStack(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return errors.WithStack(err)
	}

	if resp.StatusCode > 299 {
		return errors.Errorf("http error: %v", resp.StatusCode)
	}
	return nil
}

func (t *httpTransport) onReceivedPut(tx Tx) {
	t.putHandler(tx)

	// Broadcast to peers
	err := t.Put(context.Background(), tx)
	if err != nil {
		panic(err)
	}
}

func (t *httpTransport) SetAckHandler(handler AckHandler) {
	t.ackHandler = handler
}

func (t *httpTransport) SetPutHandler(handler PutHandler) {
	t.putHandler = handler
}

func (t *httpTransport) AddPeer(ctx context.Context, multiaddrString string) error {
	return nil
}
