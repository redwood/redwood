package libp2p

import (
	"context"
	"sync"
	"time"

	cid "github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	kbucket "github.com/libp2p/go-libp2p-kbucket"

	"redwood.dev/blob"
	"redwood.dev/errors"
	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/swarm/protoblob"
	"redwood.dev/swarm/prototree"
	. "redwood.dev/utils/generics"
)

type DHTClient interface {
	process.Interface
	AnnounceStateURIs(ctx context.Context, stateURIs Set[string])
	AnnounceBlobs(ctx context.Context, blobIDs Set[blob.ID])
	ProvidersOfStateURI(ctx context.Context, stateURI string) (<-chan prototree.TreePeerConn, error)
	ProvidersOfBlob(ctx context.Context, blobID blob.ID) (<-chan protoblob.BlobPeerConn, error)
}

type DHT interface {
	Provide(context.Context, cid.Cid, bool) error
	FindProvidersAsync(ctx context.Context, key cid.Cid, count int) <-chan peer.AddrInfo
}

type dhtClient struct {
	process.Process
	log.Logger
	libp2pHost  host.Host
	dht         DHT
	peerManager PeerManager
	forceRerun  map[interface{}]func()
	mu          sync.Mutex
}

func NewDHTClient(libp2pHost host.Host, dht DHT, peerManager PeerManager) *dhtClient {
	return &dhtClient{
		Process:     *process.New("announce"),
		Logger:      log.NewLogger(TransportName),
		libp2pHost:  libp2pHost,
		dht:         dht,
		peerManager: peerManager,
		forceRerun:  make(map[interface{}]func()),
	}
}

func (client *dhtClient) Close() error {
	client.Infof("libp2p dht client shutting down")
	return client.Process.Close()
}

func (client *dhtClient) AnnounceStateURIs(ctx context.Context, stateURIs Set[string]) {
	client.mu.Lock()
	defer client.mu.Unlock()

	for stateURI := range stateURIs {
		cid, err := makeStateURICid(stateURI)
		if err != nil {
			client.Errorf("announce: error creating cid: %v", err)
			continue
		}

		forceRerun, exists := client.forceRerun[cid]
		if !exists {
			client.startAnnouncing(stateURI, cid)
		} else {
			forceRerun()
		}
	}
}

func (client *dhtClient) AnnounceBlobs(ctx context.Context, blobIDs Set[blob.ID]) {
	client.mu.Lock()
	defer client.mu.Unlock()

	for blobID := range blobIDs {
		cid, err := makeBlobCid(blobID)
		if err != nil {
			client.Errorf("announce: error creating cid: %v", err)
			continue
		}

		forceRerun, exists := client.forceRerun[cid]
		if !exists {
			client.startAnnouncing(blobID.String(), cid)
		} else {
			forceRerun()
		}
	}
}

func (client *dhtClient) startAnnouncing(thingName string, cid cid.Cid) {
	innerCancel := func() {}

	client.forceRerun[cid] = func() { innerCancel() }

	client.Process.Go(nil, thingName, func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			func() {
				var innerCtx context.Context
				innerCtx, innerCancel = context.WithTimeout(ctx, 30*time.Second)
				defer innerCancel()

				err := client.dht.Provide(innerCtx, cid, true)
				if err != nil && err != kbucket.ErrLookupFailure && errors.Cause(err) != context.DeadlineExceeded && errors.Cause(err) != context.Canceled {
					client.Errorf(`announce: could not dht.Provide "%v": %v`, thingName, err)
					return
				}
				time.Sleep(3 * time.Second)
			}()
		}
	})
}

func (client *dhtClient) ProvidersOfStateURI(ctx context.Context, stateURI string) (<-chan prototree.TreePeerConn, error) {
	cid, err := makeStateURICid(stateURI)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ch := make(chan prototree.TreePeerConn)
	client.Process.Go(ctx, "ProvidersOfStateURI "+stateURI, func(ctx context.Context) {
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			for pinfo := range client.dht.FindProvidersAsync(ctx, cid, 8) {
				if pinfo.ID == client.libp2pHost.ID() {
					select {
					case <-ctx.Done():
						return
					default:
						continue
					}
				}

				// @@TODO: validate peer as an authorized provider via web of trust, certificate authority,
				// whitelist, etc.

				peer, err := client.peerManager.MakeDisconnectedPeerConn(pinfo)
				if err != nil {
					client.Errorf("while making disconnected peer conn (%v): %v", peer.DialInfo().DialAddr, err)
					continue
				} else if peer.DialInfo().DialAddr == "" {
					continue
				}

				select {
				case ch <- peer:
				case <-ctx.Done():
					return
				}
			}
			time.Sleep(1 * time.Second) // @@TODO: make configurable?
		}
	})
	return ch, nil
}

func (client *dhtClient) ProvidersOfBlob(ctx context.Context, blobID blob.ID) (<-chan protoblob.BlobPeerConn, error) {
	blobCid, err := makeBlobCid(blobID)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ch := make(chan protoblob.BlobPeerConn)
	client.Process.Go(ctx, "ProvidersOfBlob "+blobID.String(), func(ctx context.Context) {
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			for pinfo := range client.dht.FindProvidersAsync(ctx, blobCid, 8) {
				if pinfo.ID == client.libp2pHost.ID() {
					continue
				}

				peer, err := client.peerManager.MakeDisconnectedPeerConn(pinfo)
				if err != nil {
					client.Errorf("while making disconnected peer conn (%v): %v", pinfo.ID.Pretty(), err)
					continue
				} else if peer.DialInfo().DialAddr == "" {
					continue
				}

				select {
				case ch <- peer:
				case <-ctx.Done():
				}
			}
		}
	})
	return ch, nil
}
