package redwood

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	"redwood.dev/tree"
	"redwood.dev/types"
	"redwood.dev/utils"
)

type (
	ReadableSubscription interface {
		Read() (*SubscriptionMsg, error)
		Close() error
	}

	WritableSubscription interface {
		StateURI() string
		Type() SubscriptionType
		Keypath() tree.Keypath
		EnqueueWrite(stateURI string, tx *Tx, state tree.Node, leaves []types.ID)
		Close() error
	}

	SubscriptionMsg struct {
		StateURI    string       `json:"stateURI"`
		Tx          *Tx          `json:"tx,omitempty"`
		EncryptedTx *EncryptedTx `json:"encryptedTx,omitempty"`
		State       tree.Node    `json:"state,omitempty"`
		Leaves      []types.ID   `json:"leaves,omitempty"`
		Error       error        `json:"error,omitempty"`
	}

	SubscriptionType uint8
)

const (
	SubscriptionType_Txs SubscriptionType = 1 << iota
	SubscriptionType_States
)

func (t *SubscriptionType) UnmarshalText(bs []byte) error {
	str := strings.Trim(string(bs), `"`)
	parts := strings.Split(str, ",")
	var st SubscriptionType
	for i := range parts {
		switch strings.TrimSpace(parts[i]) {
		case "transactions":
			st |= SubscriptionType_Txs
		case "states":
			st |= SubscriptionType_States
		default:
			return errors.Errorf("bad value for SubscriptionType: %v", str)
		}
	}
	if st == 0 {
		return errors.New("empty value for SubscriptionType")
	}
	*t = st
	return nil
}

func (t SubscriptionType) MarshalText() ([]byte, error) {
	return []byte(t.String()), nil
}

func (t SubscriptionType) String() string {
	var strs []string
	if t.Includes(SubscriptionType_Txs) {
		strs = append(strs, "transactions")
	}
	if t.Includes(SubscriptionType_States) {
		strs = append(strs, "states")
	}
	return strings.Join(strs, ",")
}

func (t SubscriptionType) Includes(x SubscriptionType) bool {
	return t&x == x
}

type writableSubscription struct {
	stateURI         string
	keypath          tree.Keypath
	subscriptionType SubscriptionType
	host             Host
	subImpl          WritableSubscriptionImpl
	messages         *utils.Mailbox
	chMsgNotif       chan struct{}
	chErrored        chan struct{}
	chStop           chan struct{}
	chDone           chan struct{}
	stopOnce         sync.Once
}

type WritableSubscriptionImpl interface {
	Transport() Transport
	Put(ctx context.Context, stateURI string, tx *Tx, state tree.Node, leaves []types.ID) error
	UpdateConnStats(ok bool)
	Close() error
}

func newWritableSubscription(
	host Host,
	stateURI string,
	keypath tree.Keypath,
	subscriptionType SubscriptionType,
	subImpl WritableSubscriptionImpl,
) *writableSubscription {
	writeSub := &writableSubscription{
		stateURI:         stateURI,
		keypath:          keypath,
		subscriptionType: subscriptionType,
		host:             host,
		subImpl:          subImpl,
		messages:         utils.NewMailbox(10000),
		chErrored:        make(chan struct{}),
		chStop:           make(chan struct{}),
		chDone:           make(chan struct{}),
	}
	writeSub.chMsgNotif = writeSub.messages.Notify()

	go func() {
		defer func() {
			if perr := recover(); perr != nil {
				writeSub.host.Errorf("caught panic: %+v", perr)
			}
		}()
		defer writeSub.destroy()

		for {
			select {
			case <-writeSub.chMsgNotif:
				writeSub.writeMessages()
			case <-writeSub.chStop:
				return
			case <-writeSub.chErrored:
				return
			}
		}
	}()

	return writeSub
}

func (sub *writableSubscription) writeMessages() {
	var err error
	defer func() {
		sub.subImpl.UpdateConnStats(err == nil)
		if err != nil {
			close(sub.chErrored)
		}
	}()
	for {
		x := sub.messages.Retrieve()
		if x == nil {
			return
		}
		msg := x.(*SubscriptionMsg)
		var tx *Tx
		var state tree.Node
		if sub.subscriptionType.Includes(SubscriptionType_Txs) {
			tx = msg.Tx
		}
		if sub.subscriptionType.Includes(SubscriptionType_States) {
			state = msg.State
		}
		err = sub.subImpl.Put(context.TODO(), msg.StateURI, tx, state, msg.Leaves)
		if err != nil {
			sub.host.Errorf("error writing to subscribed peer: %+v", err)
			return
		}
	}
}

func (sub *writableSubscription) destroy() {
	defer close(sub.chDone)

	sub.host.HandleWritableSubscriptionClosed(sub)
	sub.messages.Clear()
	err := sub.subImpl.Close()
	if err != nil {
		sub.host.Errorf("error closing writable subscription (%v): %v", sub.subImpl.Transport().Name(), err)
	}
}

func (sub *writableSubscription) StateURI() string       { return sub.stateURI }
func (sub *writableSubscription) Type() SubscriptionType { return sub.subscriptionType }
func (sub *writableSubscription) Keypath() tree.Keypath  { return sub.keypath }

func (sub *writableSubscription) EnqueueWrite(stateURI string, tx *Tx, state tree.Node, leaves []types.ID) {
	sub.messages.Deliver(&SubscriptionMsg{StateURI: stateURI, Tx: tx, State: state, Leaves: leaves})
}

func (sub *writableSubscription) Close() error {
	sub.chMsgNotif = nil
	close(sub.chStop)
	<-sub.chDone
	return nil
}

type inProcessSubscription struct {
	stateURI         string
	keypath          tree.Keypath
	subscriptionType SubscriptionType
	host             Host
	messages         *utils.Mailbox
	chMessages       chan SubscriptionMsg
	chStop           chan struct{}
	chDone           chan struct{}
}

var _ ReadableSubscription = (*inProcessSubscription)(nil)
var _ WritableSubscription = (*inProcessSubscription)(nil)

func newInProcessSubscription(stateURI string, keypath tree.Keypath, subscriptionType SubscriptionType, host Host) *inProcessSubscription {
	sub := &inProcessSubscription{
		stateURI:         stateURI,
		keypath:          keypath,
		subscriptionType: subscriptionType,
		host:             host,
		messages:         utils.NewMailbox(10000),
		chMessages:       make(chan SubscriptionMsg),
		chStop:           make(chan struct{}),
		chDone:           make(chan struct{}),
	}

	go func() {
		defer close(sub.chDone)

		for {
			select {
			case <-sub.chStop:
				return

			case <-sub.messages.Notify():
				for {
					x := sub.messages.Retrieve()
					if x == nil {
						break
					}
					msg := x.(*SubscriptionMsg)
					select {
					case sub.chMessages <- *msg:
					case <-sub.chStop:
						return
					}
				}
			}
		}
	}()

	return sub
}

func (sub *inProcessSubscription) StateURI() string {
	return sub.stateURI
}

func (sub *inProcessSubscription) Type() SubscriptionType {
	return sub.subscriptionType
}

func (sub *inProcessSubscription) Keypath() tree.Keypath {
	return sub.keypath
}

func (sub *inProcessSubscription) EnqueueWrite(stateURI string, tx *Tx, state tree.Node, leaves []types.ID) {
	sub.messages.Deliver(&SubscriptionMsg{StateURI: stateURI, Tx: tx, State: state, Leaves: leaves})
}

func (sub *inProcessSubscription) Read() (*SubscriptionMsg, error) {
	select {
	case <-sub.chStop:
		return nil, errors.New("shutting down")
	case msg := <-sub.chMessages:
		return &msg, nil
	}
}

func (sub *inProcessSubscription) Close() error {
	sub.host.HandleWritableSubscriptionClosed(sub)
	sub.messages.Clear()
	close(sub.chStop)
	<-sub.chDone
	return nil
}

type multiReaderSubscription struct {
	stateURI string
	maxConns uint64
	host     Host
	conns    sync.Map
	chStop   chan struct{}
	chDone   chan struct{}
	peerPool *peerPool
}

func newMultiReaderSubscription(stateURI string, maxConns uint64, host Host) *multiReaderSubscription {
	return &multiReaderSubscription{
		stateURI: stateURI,
		maxConns: maxConns,
		host:     host,
		chStop:   make(chan struct{}),
		chDone:   make(chan struct{}),
	}
}

func (s *multiReaderSubscription) getPeer() (Peer, error) {
	ctx, cancel := utils.CombinedContext(s.chStop, 3*time.Second)
	defer cancel()
	return s.peerPool.GetPeer(ctx)
}

func (s *multiReaderSubscription) Start() {
	s.peerPool = newPeerPool(
		s.maxConns,
		func(ctx context.Context) (<-chan Peer, error) {
			return s.host.ProvidersOfStateURI(ctx, s.stateURI), nil
		},
	)

	go func() {
		var wgClose sync.WaitGroup
		defer func() {
			wgClose.Wait()
			close(s.chDone)
		}()

		for {
			select {
			case <-s.chStop:
				return
			default:
			}

			peer, err := s.getPeer()
			if err != nil {
				log.Errorf("error getting peer from pool: %v", err)
				// @@TODO: exponential backoff
				continue
			}
			if reflect.ValueOf(peer).IsNil() {
				panic("peer is nil")
			}
			s.conns.Store(peer, struct{}{})

			wgClose.Add(1)
			go func() {
				defer wgClose.Done()
				defer func() {
					s.peerPool.ReturnPeer(peer, false)
					s.conns.Delete(peer)
				}()

				err = peer.EnsureConnected(context.TODO())
				if err != nil {
					s.host.Errorf("error connecting to %v peer (stateURI: %v): %v", peer.Transport().Name(), s.stateURI, err)
					return
				}

				peerSub, err := peer.Subscribe(context.TODO(), s.stateURI)
				if err != nil {
					s.host.Errorf("error subscribing to %v peer (stateURI: %v): %v", peer.Transport().Name(), s.stateURI, err)
					return
				}
				defer peerSub.Close()

				for {
					select {
					case <-s.chStop:
						return
					default:
					}

					msg, err := peerSub.Read()
					if err != nil {
						s.host.Errorf("error reading: %v", err)
						return
					} else if msg.Tx == nil {
						s.host.Error("error: peer sent empty subscription message")
						return
					}

					s.host.HandleTxReceived(*msg.Tx, peer)
				}
			}()
		}
	}()
}

func (s *multiReaderSubscription) Close() error {
	// 1. Stop reading from the peer
	s.host.HandleReadableSubscriptionClosed(s.stateURI)

	// 2. Signal shutdown
	close(s.chStop)

	// 3. Close all active peer conns
	s.conns.Range(func(peer, val interface{}) bool {
		peer.(Peer).Close()
		return true
	})

	s.peerPool.Close()
	<-s.chDone
	return nil
}
