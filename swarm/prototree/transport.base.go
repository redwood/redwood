package prototree

import (
	"sync"

	"redwood.dev/state"
	"redwood.dev/tree"
)

type BaseTreeTransport struct {
	muTxReceivedCallbacks                sync.RWMutex
	muPrivateTxReceivedCallbacks         sync.RWMutex
	muAckReceivedCallbacks               sync.RWMutex
	muWritableSubscriptionOpenedCallback sync.RWMutex
	muP2PStateURIReceivedCallbacks       sync.RWMutex

	txReceivedCallbacks                []TxReceivedCallback
	privateTxReceivedCallbacks         []PrivateTxReceivedCallback
	ackReceivedCallbacks               []AckReceivedCallback
	writableSubscriptionOpenedCallback WritableSubscriptionOpenedCallback
	p2pStateURIReceivedCallbacks       []P2PStateURIReceivedCallback
}

type TxReceivedCallback func(tx tree.Tx, peerConn TreePeerConn)
type PrivateTxReceivedCallback func(encryptedTx EncryptedTx, peerConn TreePeerConn)
type AckReceivedCallback func(stateURI string, txID state.Version, peerConn TreePeerConn)
type WritableSubscriptionOpenedCallback func(req SubscriptionRequest, writeSubImplFactory WritableSubscriptionImplFactory) (<-chan struct{}, error)
type P2PStateURIReceivedCallback func(stateURI string, peerConn TreePeerConn)

type WritableSubscriptionImplFactory func() (WritableSubscriptionImpl, error)

func (t *BaseTreeTransport) OnTxReceived(handler TxReceivedCallback) {
	t.muTxReceivedCallbacks.Lock()
	defer t.muTxReceivedCallbacks.Unlock()
	t.txReceivedCallbacks = append(t.txReceivedCallbacks, handler)
}

func (t *BaseTreeTransport) OnPrivateTxReceived(handler PrivateTxReceivedCallback) {
	t.muPrivateTxReceivedCallbacks.Lock()
	defer t.muPrivateTxReceivedCallbacks.Unlock()
	t.privateTxReceivedCallbacks = append(t.privateTxReceivedCallbacks, handler)
}

func (t *BaseTreeTransport) OnAckReceived(handler AckReceivedCallback) {
	t.muAckReceivedCallbacks.Lock()
	defer t.muAckReceivedCallbacks.Unlock()
	t.ackReceivedCallbacks = append(t.ackReceivedCallbacks, handler)
}

func (t *BaseTreeTransport) OnWritableSubscriptionOpened(handler WritableSubscriptionOpenedCallback) {
	t.muWritableSubscriptionOpenedCallback.Lock()
	defer t.muWritableSubscriptionOpenedCallback.Unlock()
	if t.writableSubscriptionOpenedCallback != nil {
		panic("only one")
	}
	t.writableSubscriptionOpenedCallback = handler
}

func (t *BaseTreeTransport) OnP2PStateURIReceived(handler P2PStateURIReceivedCallback) {
	t.muP2PStateURIReceivedCallbacks.Lock()
	defer t.muP2PStateURIReceivedCallbacks.Unlock()
	t.p2pStateURIReceivedCallbacks = append(t.p2pStateURIReceivedCallbacks, handler)
}

func (t *BaseTreeTransport) HandleTxReceived(tx tree.Tx, peerConn TreePeerConn) {
	t.muTxReceivedCallbacks.RLock()
	defer t.muTxReceivedCallbacks.RUnlock()
	var wg sync.WaitGroup
	wg.Add(len(t.txReceivedCallbacks))
	for _, handler := range t.txReceivedCallbacks {
		handler := handler
		go func() {
			defer wg.Done()
			handler(tx, peerConn)
		}()
	}
	wg.Wait()
}

func (t *BaseTreeTransport) HandlePrivateTxReceived(encryptedTx EncryptedTx, peerConn TreePeerConn) {
	t.muPrivateTxReceivedCallbacks.RLock()
	defer t.muPrivateTxReceivedCallbacks.RUnlock()
	var wg sync.WaitGroup
	wg.Add(len(t.privateTxReceivedCallbacks))
	for _, handler := range t.privateTxReceivedCallbacks {
		handler := handler
		go func() {
			defer wg.Done()
			handler(encryptedTx, peerConn)
		}()
	}
	wg.Wait()
}

func (t *BaseTreeTransport) HandleAckReceived(stateURI string, txID state.Version, peerConn TreePeerConn) {
	t.muAckReceivedCallbacks.RLock()
	defer t.muAckReceivedCallbacks.RUnlock()
	var wg sync.WaitGroup
	wg.Add(len(t.ackReceivedCallbacks))
	for _, handler := range t.ackReceivedCallbacks {
		handler := handler
		go func() {
			defer wg.Done()
			handler(stateURI, txID, peerConn)
		}()
	}
	wg.Wait()
}

func (t *BaseTreeTransport) HandleWritableSubscriptionOpened(req SubscriptionRequest, writeSubImplFactory WritableSubscriptionImplFactory) (<-chan struct{}, error) {
	t.muWritableSubscriptionOpenedCallback.RLock()
	defer t.muWritableSubscriptionOpenedCallback.RUnlock()
	return t.writableSubscriptionOpenedCallback(req, writeSubImplFactory)
}

func (t *BaseTreeTransport) HandleP2PStateURIReceived(stateURI string, peerConn TreePeerConn) {
	t.muP2PStateURIReceivedCallbacks.RLock()
	defer t.muP2PStateURIReceivedCallbacks.RUnlock()
	var wg sync.WaitGroup
	wg.Add(len(t.p2pStateURIReceivedCallbacks))
	for _, handler := range t.p2pStateURIReceivedCallbacks {
		handler := handler
		go func() {
			defer wg.Done()
			handler(stateURI, peerConn)
		}()
	}
	wg.Wait()
}
