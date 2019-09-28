package main

import (
	"context"
	"io"
	"sync"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	// peer "github.com/libp2p/go-libp2p-peer"
	// pstore "github.com/libp2p/go-libp2p-peerstore"
)

type Host interface {
	AddPeer(ctx context.Context, multiaddrString string) error
	Subscribe(ctx context.Context, url string) error
	BroadcastTx(ctx context.Context, url string, tx Tx) error
}

type host struct {
	ID ID

	transport        Transport
	subscriptionsOut map[string]io.ReadWriteCloser
	subscriptionsIn  map[string][]io.ReadWriteCloser

	onTxReceived  func(url string, tx Tx)
	onAckReceived func(url string, id ID)
}

func NewHost(id ID, transport Transport, onTxReceived func(url string, tx Tx), onAckReceived func(url string, id ID)) Host {
	h := &host{
		ID:               id,
		transport:        transport,
		subscriptionsOut: make(map[string]io.ReadWriteCloser),
		subscriptionsIn:  make(map[string][]io.ReadWriteCloser),
		onTxReceived:     onTxReceived,
		onAckReceived:    onAckReceived,
	}

	transport.OnIncomingStream(h.onIncomingStream)

	return h
}

func (h *host) onIncomingStream(conn io.ReadWriteCloser) {
	log.Infof("[host %v] incoming stream", h.ID)

	// Handshake and then associate the subscriber with the requested URL
	var msg Msg
	err := ReadMsg(conn, &msg)
	if err != nil {
		panic(err)
	} else if msg.Type != MsgType_Subscribe {
		panic("protocol error")
	}

	url, ok := msg.Payload.(string)
	if !ok {
		log.Errorf("[host %v] Subscribe message: bad payload: (%T) %v", h.ID, msg.Payload, msg.Payload)
		return
	}

	h.subscriptionsIn[url] = append(h.subscriptionsIn[url], conn)
}

func (h *host) Subscribe(ctx context.Context, url string) error {
	_, exists := h.subscriptionsOut[url]
	if exists {
		return errors.New("already subscribed to " + url)
	}

	conn, err := h.transport.GetPeerConn(ctx, url)
	if err != nil {
		return err
	}

	h.subscriptionsOut[url] = conn

	go func() {
		WriteMsg(conn, Msg{Type: MsgType_Subscribe, Payload: url})
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			var msg Msg
			ReadMsg(conn, &msg)
			if msg.Type != MsgType_Tx {
				panic("protocol error")
			}
			tx := msg.Payload.(Tx)

			h.onTxReceived(url, tx)
		}
	}()

	return nil
}

func (h *host) Unsubscribe(url string) {
	conn := h.subscriptionsOut[url]
	if conn != nil {
		conn.Close()
		delete(h.subscriptionsOut, url)
	}
}

func (h *host) BroadcastTx(ctx context.Context, url string, tx Tx) error {
	log.Infof("[host %v] broadcasting tx %v (%v)", h.ID, tx.ID, len(h.subscriptionsIn[url]))

	wg := &sync.WaitGroup{}

	for _, conn := range h.subscriptionsIn[url] {
		wg.Add(1)

		go func(conn io.ReadWriteCloser) {
			defer wg.Done()

			err := WriteMsg(conn, Msg{Type: MsgType_Tx, Payload: tx})
			if err != nil {
				panic(err)
			}
		}(conn)
	}

	wg.Wait()

	return nil
}

func (h *host) AddPeer(ctx context.Context, multiaddrString string) error {
	return h.transport.AddPeer(ctx, multiaddrString)
}
