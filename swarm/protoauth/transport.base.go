package protoauth

import (
	"sync"

	"go.uber.org/multierr"
)

type BaseAuthTransport struct {
	muChallengeIdentityCallbacks sync.RWMutex
	challengeIdentityCallbacks   []ChallengeIdentityCallback
}

type ChallengeIdentityCallback func(challengeMsg ChallengeMsg, peerConn AuthPeerConn) error

func (t *BaseAuthTransport) OnChallengeIdentity(handler ChallengeIdentityCallback) {
	t.muChallengeIdentityCallbacks.Lock()
	defer t.muChallengeIdentityCallbacks.Unlock()
	t.challengeIdentityCallbacks = append(t.challengeIdentityCallbacks, handler)
}

func (t *BaseAuthTransport) HandleChallengeIdentity(challengeMsg ChallengeMsg, peerConn AuthPeerConn) error {
	t.muChallengeIdentityCallbacks.RLock()
	defer t.muChallengeIdentityCallbacks.RUnlock()
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		merr error
	)
	wg.Add(len(t.challengeIdentityCallbacks))
	for _, handler := range t.challengeIdentityCallbacks {
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
