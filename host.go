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
	subscriptionsOut map[string]*peerConn
	subscriptionsIn  map[string][]*peerConn

	onTxReceived  func(url string, tx Tx)
	onAckReceived func(url string, id ID)
}

func NewHost(id ID, transport Transport, onTxReceived func(url string, tx Tx), onAckReceived func(url string, id ID)) Host {
	h := &host{
		ID:               id,
		transport:        transport,
		subscriptionsOut: make(map[string]*peerConn),
		subscriptionsIn:  make(map[string][]*peerConn),
		onTxReceived:     onTxReceived,
		onAckReceived:    onAckReceived,
	}

	transport.OnIncomingStream(h.onIncomingStream)

	return h
}

func (h *host) onIncomingStream(stream io.ReadWriteCloser) {
	log.Infof("[host %v] incoming stream", h.ID)

	conn := newPeerConn(stream)

	// Handshake
	{
		msg, err := conn.Recv()
		if err != nil {
			panic(err)
		}

		switch msg.Type {
		case MsgType_Subscribe:
			url, ok := msg.Payload.(string)
			if !ok {
				log.Errorf("[host %v] Subscribe message: bad payload: (%T) %v", h.ID, msg.Payload, msg.Payload)
				return
			}
			h.subscriptionsIn[url] = append(h.subscriptionsIn[url], conn)
			go h.handleConn(url, conn)

		default:
			panic("protocol error")
		}
	}
}

func (h *host) Subscribe(ctx context.Context, url string) error {
	_, exists := h.subscriptionsOut[url]
	if exists {
		return errors.New("already subscribed to " + url)
	}

	stream, err := h.transport.GetPeerConn(ctx, url)
	if err != nil {
		return err
	}

	conn := newPeerConn(stream)
	h.subscriptionsOut[url] = conn

	conn.Send(Msg{Type: MsgType_Subscribe, Payload: url})

	go h.handleConn(url, conn)

	return nil
}

func (h *host) handleConn(url string, conn *peerConn) {
	defer conn.Close()

	for {
		msg, err := conn.Recv()
		if err != nil {
			panic(err)
		}

		switch msg.Type {
		case MsgType_Tx:
			tx, ok := msg.Payload.(Tx)
			if !ok {
				panic("protocol error")
			}
			h.onTxReceived(url, tx)

		case MsgType_Ack:
			id, ok := msg.Payload.(ID)
			if !ok {
				panic("protocol error")
			}
			h.onAckReceived(url, id)

		default:
			panic("protocol error")
		}
	}
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

		go func(conn *peerConn) {
			defer wg.Done()
			conn.Send(Msg{Type: MsgType_Tx, Payload: tx})
		}(conn)
	}

	wg.Wait()

	return nil
}

func (h *host) AddPeer(ctx context.Context, multiaddrString string) error {
	return h.transport.AddPeer(ctx, multiaddrString)
}

type peerConn struct {
	stream io.ReadWriteCloser
	chSend chan Msg
	chRecv chan Msg
	chDone chan struct{}
}

func newPeerConn(stream io.ReadWriteCloser) *peerConn {
	conn := &peerConn{
		stream: stream,
		chSend: make(chan Msg, 10),
		chRecv: make(chan Msg, 10),
		chDone: make(chan struct{}),
	}

	wg := &sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()

		for {
			select {
			case <-conn.chDone:
				return

			case msg := <-conn.chSend:
				err := WriteMsg(conn.stream, msg)
				if err != nil {
					panic(err)
				}
			}
		}
	}()

	go func() {
		defer wg.Done()

		for {
			select {
			case <-conn.chDone:
				return
			default:
			}

			var msg Msg
			err := ReadMsg(conn.stream, &msg)
			if err != nil {
				panic(err)
			}

			select {
			case <-conn.chDone:
				return
			case conn.chRecv <- msg:
			}
		}
	}()

	go func() {
		defer conn.stream.Close()
		wg.Wait()
	}()

	return conn
}

func (conn *peerConn) Send(msg Msg) {
	select {
	case conn.chSend <- msg:
	case <-conn.chDone:
	}
}

func (conn *peerConn) Recv() (Msg, error) {
	select {
	case msg := <-conn.chRecv:
		return msg, nil

	case <-conn.chDone:
		return Msg{}, errors.New("connection closed")
	}
}

func (conn *peerConn) Close() {
	close(conn.chDone)
}
