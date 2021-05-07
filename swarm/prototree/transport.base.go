package prototree

import (
	"sync"

	"redwood.dev/log"
	"redwood.dev/state"
	"redwood.dev/tree"
	"redwood.dev/types"
)

type BaseTreeTransport struct {
	muTxReceivedCallbacks                 sync.RWMutex
	muAckReceivedCallbacks                sync.RWMutex
	muWritableSubscriptionOpenedCallbacks sync.RWMutex
	muWritableSubscriptionClosedCallbacks sync.RWMutex

	txReceivedCallbacks                 []TxReceivedCallback
	ackReceivedCallbacks                []AckReceivedCallback
	writableSubscriptionOpenedCallbacks []WritableSubscriptionOpenedCallback
	writableSubscriptionClosedCallbacks []WritableSubscriptionClosedCallback
}

type TxReceivedCallback func(tx tree.Tx, peerConn TreePeerConn)
type AckReceivedCallback func(stateURI string, txID types.ID, peerConn TreePeerConn)
type WritableSubscriptionOpenedCallback func(stateURI string, keypath state.Keypath, subType SubscriptionType, writeSubImpl WritableSubscriptionImpl, fetchHistoryOpts *FetchHistoryOpts)
type WritableSubscriptionClosedCallback func(writeSub WritableSubscriptionImpl)

func (t *BaseTreeTransport) OnTxReceived(handler TxReceivedCallback) {
	t.muTxReceivedCallbacks.Lock()
	defer t.muTxReceivedCallbacks.Unlock()
	t.txReceivedCallbacks = append(t.txReceivedCallbacks, handler)
}

func (t *BaseTreeTransport) OnAckReceived(handler AckReceivedCallback) {
	t.muAckReceivedCallbacks.Lock()
	defer t.muAckReceivedCallbacks.Unlock()
	t.ackReceivedCallbacks = append(t.ackReceivedCallbacks, handler)
}

func (t *BaseTreeTransport) OnWritableSubscriptionOpened(handler WritableSubscriptionOpenedCallback) {
	t.muWritableSubscriptionOpenedCallbacks.Lock()
	defer t.muWritableSubscriptionOpenedCallbacks.Unlock()
	t.writableSubscriptionOpenedCallbacks = append(t.writableSubscriptionOpenedCallbacks, handler)
}

func (t *BaseTreeTransport) OnWritableSubscriptionClosed(handler WritableSubscriptionClosedCallback) {
	t.muWritableSubscriptionClosedCallbacks.Lock()
	defer t.muWritableSubscriptionClosedCallbacks.Unlock()
	t.writableSubscriptionClosedCallbacks = append(t.writableSubscriptionClosedCallbacks, handler)
}

func (t *BaseTreeTransport) HandleTxReceived(tx tree.Tx, peerConn TreePeerConn) {
	t.muTxReceivedCallbacks.RLock()
	defer t.muTxReceivedCallbacks.RUnlock()
	var wg sync.WaitGroup
	wg.Add(len(t.txReceivedCallbacks))
	for _, handler := range t.txReceivedCallbacks {
		go func() {
			defer wg.Done()
			handler(tx, peerConn)
		}()
	}
	wg.Wait()
}

func (t *BaseTreeTransport) HandleAckReceived(stateURI string, txID types.ID, peerConn TreePeerConn) {
	t.muAckReceivedCallbacks.RLock()
	defer t.muAckReceivedCallbacks.RUnlock()
	var wg sync.WaitGroup
	wg.Add(len(t.ackReceivedCallbacks))
	for _, handler := range t.ackReceivedCallbacks {
		go func() {
			defer wg.Done()
			handler(stateURI, txID, peerConn)
		}()
	}
	wg.Wait()
}

func (t *BaseTreeTransport) HandleWritableSubscriptionOpened(
	stateURI string,
	keypath state.Keypath,
	subType SubscriptionType,
	writeSubImpl WritableSubscriptionImpl,
	fetchHistoryOpts *FetchHistoryOpts,
) {
	logger := log.NewLogger("yeet")
	logger.Successf("HandleWritableSubscriptionOpened")
	t.muWritableSubscriptionOpenedCallbacks.RLock()
	defer t.muWritableSubscriptionOpenedCallbacks.RUnlock()
	var wg sync.WaitGroup
	wg.Add(len(t.writableSubscriptionOpenedCallbacks))
	for _, handler := range t.writableSubscriptionOpenedCallbacks {
		go func() {
			defer wg.Done()
			handler(stateURI, keypath, subType, writeSubImpl, fetchHistoryOpts)
		}()
	}
	wg.Wait()
}

func (t *BaseTreeTransport) HandleWritableSubscriptionClosed(writeSub WritableSubscriptionImpl) {
	t.muWritableSubscriptionClosedCallbacks.RLock()
	defer t.muWritableSubscriptionClosedCallbacks.RUnlock()
	var wg sync.WaitGroup
	wg.Add(len(t.writableSubscriptionClosedCallbacks))
	for _, handler := range t.writableSubscriptionClosedCallbacks {
		go func() {
			defer wg.Done()
			handler(writeSub)
		}()
	}
	wg.Wait()
}
