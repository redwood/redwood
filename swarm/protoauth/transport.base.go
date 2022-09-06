package protoauth

import (
	"sync"

	"go.uber.org/multierr"

	"redwood.dev/crypto"
)

type BaseAuthTransport struct {
	muIncomingIdentityChallengeCallbacks         sync.RWMutex
	incomingIdentityChallengeCallbacks           []IncomingIdentityChallengeCallback
	muIncomingIdentityChallengeRequestCallbacks  sync.RWMutex
	incomingIdentityChallengeRequestCallbacks    []IncomingIdentityChallengeRequestCallback
	muIncomingIdentityChallengeResponseCallbacks sync.RWMutex
	incomingIdentityChallengeResponseCallbacks   []IncomingIdentityChallengeResponseCallback
	muIncomingUcanCallbacks                      sync.RWMutex
	incomingUcanCallbacks                        []IncomingUcanCallback
}

type IncomingIdentityChallengeCallback func(challengeMsg crypto.ChallengeMsg, peerConn AuthPeerConn) error
type IncomingIdentityChallengeRequestCallback func(peerConn AuthPeerConn) error
type IncomingIdentityChallengeResponseCallback func(signatures []ChallengeIdentityResponse, peerConn AuthPeerConn) error
type IncomingUcanCallback func(ucan string, peerConn AuthPeerConn)

func (t *BaseAuthTransport) OnIncomingIdentityChallenge(handler IncomingIdentityChallengeCallback) {
	t.muIncomingIdentityChallengeCallbacks.Lock()
	defer t.muIncomingIdentityChallengeCallbacks.Unlock()
	t.incomingIdentityChallengeCallbacks = append(t.incomingIdentityChallengeCallbacks, handler)
}

func (t *BaseAuthTransport) HandleIncomingIdentityChallenge(challengeMsg crypto.ChallengeMsg, peerConn AuthPeerConn) error {
	t.muIncomingIdentityChallengeCallbacks.RLock()
	defer t.muIncomingIdentityChallengeCallbacks.RUnlock()
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		merr error
	)
	wg.Add(len(t.incomingIdentityChallengeCallbacks))
	for _, handler := range t.incomingIdentityChallengeCallbacks {
		handler := handler
		go func() {
			defer wg.Done()
			err := handler(challengeMsg, peerConn)
			mu.Lock()
			merr = multierr.Append(merr, err)
			mu.Unlock()
		}()
	}
	wg.Wait()
	return merr
}

func (t *BaseAuthTransport) OnIncomingIdentityChallengeRequest(handler IncomingIdentityChallengeRequestCallback) {
	t.muIncomingIdentityChallengeRequestCallbacks.Lock()
	defer t.muIncomingIdentityChallengeRequestCallbacks.Unlock()
	t.incomingIdentityChallengeRequestCallbacks = append(t.incomingIdentityChallengeRequestCallbacks, handler)
}

func (t *BaseAuthTransport) HandleIncomingIdentityChallengeRequest(peerConn AuthPeerConn) error {
	t.muIncomingIdentityChallengeRequestCallbacks.RLock()
	defer t.muIncomingIdentityChallengeRequestCallbacks.RUnlock()
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		merr error
	)
	wg.Add(len(t.incomingIdentityChallengeRequestCallbacks))
	for _, handler := range t.incomingIdentityChallengeRequestCallbacks {
		handler := handler
		go func() {
			defer wg.Done()
			err := handler(peerConn)
			mu.Lock()
			merr = multierr.Append(merr, err)
			mu.Unlock()
		}()
	}
	wg.Wait()
	return merr
}

func (t *BaseAuthTransport) OnIncomingIdentityChallengeResponse(handler IncomingIdentityChallengeResponseCallback) {
	t.muIncomingIdentityChallengeResponseCallbacks.Lock()
	defer t.muIncomingIdentityChallengeResponseCallbacks.Unlock()
	t.incomingIdentityChallengeResponseCallbacks = append(t.incomingIdentityChallengeResponseCallbacks, handler)
}

func (t *BaseAuthTransport) HandleIncomingIdentityChallengeResponse(signatures []ChallengeIdentityResponse, peerConn AuthPeerConn) error {
	t.muIncomingIdentityChallengeResponseCallbacks.RLock()
	defer t.muIncomingIdentityChallengeResponseCallbacks.RUnlock()
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		merr error
	)
	wg.Add(len(t.incomingIdentityChallengeResponseCallbacks))
	for _, handler := range t.incomingIdentityChallengeResponseCallbacks {
		handler := handler
		go func() {
			defer wg.Done()
			err := handler(signatures, peerConn)
			mu.Lock()
			merr = multierr.Append(merr, err)
			mu.Unlock()
		}()
	}
	wg.Wait()
	return merr
}

func (t *BaseAuthTransport) OnIncomingUcan(handler IncomingUcanCallback) {
	t.muIncomingUcanCallbacks.Lock()
	defer t.muIncomingUcanCallbacks.Unlock()
	t.incomingUcanCallbacks = append(t.incomingUcanCallbacks, handler)
}

func (t *BaseAuthTransport) HandleIncomingUcan(ucan string, peerConn AuthPeerConn) {
	t.muIncomingUcanCallbacks.RLock()
	defer t.muIncomingUcanCallbacks.RUnlock()
	var (
		wg sync.WaitGroup
	)
	wg.Add(len(t.incomingUcanCallbacks))
	for _, handler := range t.incomingUcanCallbacks {
		handler := handler
		go func() {
			defer wg.Done()
			handler(ucan, peerConn)
		}()
	}
	wg.Wait()
}
