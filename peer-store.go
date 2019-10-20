package redwood

// import (
// 	"context"
// 	"sync"

// 	"github.com/plan-systems/plan-core/tools/ctx"
// )

// type PeerStore interface {
// 	PeersForURL(ctx context.Context, url string) ([]*Peer, error)
// }

// type peerStore struct {
// 	ctx.Context

// 	peersForURL map[string][]*Peer
// 	peersByAddr map[Address]*Peer
// 	muPeers     sync.RWMutex
// }

// type Peer struct {
// 	URL     string  `json:"url"`
// 	Address Address `json:"address"`
// }

// func NewPeerStore(addr Address) *peerStore {
// 	s := &peerStore{
// 		peersForURL: make(map[string][]*Peer),
// 		peersByAddr: make(map[Address]*Peer),
// 	}

// 	err = t.CtxStart(
// 		// on startup
// 		func() error {
// 			c.SetLogLabel(addr.Pretty() + " peer store")
// 			go s.periodicallyUpdatePeerLists()
// 			return nil
// 		},
// 		nil,
// 		nil,
// 		nil,
// 	)

// 	return s
// }

// func (s *peerStore) PeersForURL(ctx context.Context, url string) ([]Peer, error) {
// 	s.muPeers.RLock()
// 	peers, have := s.peersForURL[url]
// 	s.muPeers.RUnlock()

// 	if !have {
// 		s.muPeers.Lock()
// 		defer s.muPeers.Unlock()

// 		var err error
// 		peers, err = s.fetchPeerList(ctx, url)
// 		if err != nil {
// 			return nil, err
// 		}
// 	}
// 	return peers, nil
// }

// func (s *peerStore) periodicallyUpdatePeerLists() {
// 	for {
// 		select {
// 		case <-s.Ctx.Done():
// 			return
// 		default:
// 		}

// 		func() {
// 			s.muPeers.Lock()
// 			defer s.muPeers.Unlock()

// 			for url := range s.peersForURL {
// 				func() {
// 					s.Infof(0, `fetching peer list for "%v"`, url)
// 					ctx, cancel := context.WithTimeout(s.Ctx, 10*time.Second)
// 					defer cancel()

// 					_, err := s.fetchPeerList(ctx, url)
// 					if err != nil {
// 						s.Errorf(`error fetching peer list for url "%v": %v`, url)
// 					}
// 				}()
// 			}
// 		}()

// 		time.Sleep(60 * time.Second)
// 	}
// }

// // muPeers must be write-locked by the caller prior to calling this function.
// func (s *peerStore) fetchPeerList(ctx context.Context, url string) ([]peer, error) {
// 	url = braidURLToHTTP(url)

// 	req := http.NewRequest("GET", url+"/peers.json", nil).WithContext(ctx)
// 	resp, err := http.DefaultClient.Do(req)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer resp.Body.Close()

// 	var peerList []*Peer
// 	err = json.NewDecoder(resp.Body).Decode(&peerList)
// 	if err != nil {
// 		return nil, err
// 	}

// 	t.peersForURL[url] = peerList
// 	for _, peer := range peerList {
// 		s.peersByAddr[peer.Address] = peer
// 	}

// 	return peerList, nil
// }
