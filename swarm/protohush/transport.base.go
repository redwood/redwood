package protohush

import (
	"sync"
)

type (
	BaseHushTransport struct {
		muIncomingPubkeyBundlesCallbacks sync.RWMutex
		// muIncomingIndividualMessageCallbacks sync.RWMutex
		// muIncomingGroupMessageCallbacks      sync.RWMutex

		incomingDHPubkeyExchangeCallbacks []IncomingPubkeyBundlesCallback
		// incomingIndividualMessageCallbacks []IncomingIndividualMessageCallback
		// incomingGroupMessageCallbacks      []IncomingGroupMessageCallback
	}

	IncomingPubkeyBundlesCallback func(bundles []PubkeyBundle)
	// IncomingIndividualMessageCallback func(ctx context.Context, msg IndividualMessage, peer HushPeerConn)
	// IncomingGroupMessageCallback      func(ctx context.Context, msg GroupMessage, peer HushPeerConn)
)

func (t *BaseHushTransport) OnIncomingPubkeyBundles(handler IncomingPubkeyBundlesCallback) {
	t.muIncomingPubkeyBundlesCallbacks.Lock()
	defer t.muIncomingPubkeyBundlesCallbacks.Unlock()
	t.incomingDHPubkeyExchangeCallbacks = append(t.incomingDHPubkeyExchangeCallbacks, handler)
}

func (t *BaseHushTransport) HandleIncomingPubkeyBundles(bundles []PubkeyBundle) {
	t.muIncomingPubkeyBundlesCallbacks.RLock()
	defer t.muIncomingPubkeyBundlesCallbacks.RUnlock()

	var wg sync.WaitGroup
	wg.Add(len(t.incomingDHPubkeyExchangeCallbacks))
	for _, handler := range t.incomingDHPubkeyExchangeCallbacks {
		handler := handler
		go func() {
			defer wg.Done()
			handler(bundles)
		}()
	}
	wg.Wait()
}

// func (t *BaseHushTransport) OnIncomingIndividualMessage(handler IncomingIndividualMessageCallback) {
// 	t.muIncomingIndividualMessageCallbacks.Lock()
// 	defer t.muIncomingIndividualMessageCallbacks.Unlock()
// 	t.incomingIndividualMessageCallbacks = append(t.incomingIndividualMessageCallbacks, handler)
// }

// func (t *BaseHushTransport) HandleIncomingIndividualMessage(ctx context.Context, msg IndividualMessage, peerConn HushPeerConn) {
// 	t.muIncomingIndividualMessageCallbacks.RLock()
// 	defer t.muIncomingIndividualMessageCallbacks.RUnlock()

// 	var wg sync.WaitGroup
// 	wg.Add(len(t.incomingIndividualMessageCallbacks))
// 	for _, handler := range t.incomingIndividualMessageCallbacks {
// 		handler := handler
// 		go func() {
// 			defer wg.Done()
// 			handler(ctx, msg, peerConn)
// 		}()
// 	}
// 	wg.Wait()
// }

// func (t *BaseHushTransport) OnIncomingGroupMessage(handler IncomingGroupMessageCallback) {
// 	t.muIncomingGroupMessageCallbacks.Lock()
// 	defer t.muIncomingGroupMessageCallbacks.Unlock()
// 	t.incomingGroupMessageCallbacks = append(t.incomingGroupMessageCallbacks, handler)
// }

// func (t *BaseHushTransport) HandleIncomingGroupMessage(ctx context.Context, msg GroupMessage, peerConn HushPeerConn) {
// 	t.muIncomingGroupMessageCallbacks.RLock()
// 	defer t.muIncomingGroupMessageCallbacks.RUnlock()

// 	var wg sync.WaitGroup
// 	wg.Add(len(t.incomingGroupMessageCallbacks))
// 	for _, handler := range t.incomingGroupMessageCallbacks {
// 		handler := handler
// 		go func() {
// 			defer wg.Done()
// 			handler(ctx, msg, peerConn)
// 		}()
// 	}
// 	wg.Wait()
// }
