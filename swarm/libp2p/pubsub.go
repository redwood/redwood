package libp2p

// import (
// 	"context"
// 	"fmt"
// 	"sync"

// 	ds "github.com/ipfs/go-datastore"
// 	corehost "github.com/libp2p/go-libp2p-core/host"
// 	corepeer "github.com/libp2p/go-libp2p-core/peer"
// 	dht "github.com/libp2p/go-libp2p-kad-dht"
// 	peer "github.com/libp2p/go-libp2p-peer"
// 	pubsub "github.com/libp2p/go-libp2p-pubsub"

// 	"redwood.dev/types"
// )

// type GossipSub struct {
// 	*pubsub.Pubsub

// 	topics   map[string]*pubsub.Subscription
// 	topicsMu sync.RWMutex
// }

// func NewGossipSub(host corehost.Host, subscriptionFilter pubsub.SubscriptionFilter) (*GossipSub, error) {
// 	return pubsub.NewGossipSub(context.TODO(), host,
// 		pubsub.WithSubscriptionFilter(subscriptionFilter),
// 	)
// }

// func (ps *GossipSub) Publish(ctx context.Context, topic string, data []byte) error {
// 	var t *pubsub.Topic
// 	var exists bool
// 	func() {
// 		ps.topicsMu.RLock()
// 		defer ps.topicsMu.RUnlock()
// 		t, exists = ps.topics[topic]
// 	}()
// 	if !exists {
// 		return errors.Err404
// 	}
// 	return t.Publish(ctx, data)
// }

// func (ps *GossipSub) Subscribe(ctx context.Context, topic string) error {
// 	t, err := ps.Join(topic)
// 	if err != nil {
// 		return err
// 	}
// 	sub, err := t.Subscribe()
// 	if err != nil {
// 		return err
// 	}
// 	ps.topicsMu.Lock()
// 	defer ps.topicsMu.Unlock()
// 	ps.topics[topic] = t
// 	return nil
// }

// type DHT struct {
// 	*dht.IpfsDHT
// }

// func NewDHT(host corehost.Host, datastore ds.Batching, bootstrapPeers []corepeer.AddrInfo) (*DHT, error) {
// 	dht, err := dht.New(t.Process.Ctx(), host,
// 		dht.BootstrapPeers(bootstrapPeers...),
// 		dht.Mode(dht.ModeServer),
// 		dht.Datastore(datastore),
// 		// dht.Validator(blankValidator{}), // Set a pass-through validator
// 	)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return &DHT{
// 		IpfsDHT: dht,
// 		topics:  make(map[string]*pubsub.Subscription),
// 	}, nil
// }

// func (ps *DHT) Publish(ctx context.Context, topic string, data []byte) error {
// 	cid, err := cidForString(topic)
// 	if err != nil {
// 		return err
// 	}
// 	return ps.dht.Provide(ctx, cid, true)
// }

// func (ps *DHT) Subscribe(ctx context.Context, topic string) error {
// 	return nil
// }

// type blankValidator struct{}

// func (blankValidator) Validate(key string, val []byte) error {
// 	fmt.Println("Validate ~>", key, string(val))
// 	return nil
// }
// func (blankValidator) Select(key string, vals [][]byte) (int, error) {
// 	fmt.Println("SELECT key", key)
// 	for _, val := range vals {
// 		fmt.Println("  -", string(val))
// 	}
// 	return 0, nil
// }

// type whitelistPeerMessageFilter struct {
// 	peerTopics   map[string]map[peer.ID]struct{}
// 	peerTopicsMu sync.RWMutex
// }

// func NewWhitelistPeerMessageFilter() *whitelistPeerMessageFilter {
// 	return &whitelistPeerMessageFilter{
// 		peerTopics: make(map[string]map[peer.ID]struct{}),
// 	}
// }

// func (f whitelistPeerMessageFilter) SetRule(pid peer.ID, topic string, allowed bool) {
// 	f.peerTopicsMu.Lock()
// 	defer f.peerTopicsMu.Unlock()

// 	_, exists := f.peerTopics[topic]

// 	if allowed {
// 		if !exists {
// 			f.peerTopics[topic] = make(map[peer.ID]struct{})
// 		}
// 		f.peerTopics[topic][pid] = struct{}{}
// 		return
// 	}

// 	if !exists {
// 		return
// 	}
// 	delete(f.peerTopics[topic], pid)
// }

// func (f whitelistPeerMessageFilter) AllowedToSend(pid peer.ID, msg *Message) bool {
// 	f.peerTopicsMu.RLock()
// 	defer f.peerTopicsMu.RUnlock()

// 	peerMap, exists := f.peerTopics[msg.GetTopic()]
// 	if !exists {
// 		return true
// 	}

// 	_, exists = peerMap[pid]
// 	return exists
// }
