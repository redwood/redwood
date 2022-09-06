package libp2p

import (
	"context"
	"fmt"
	"net"
	"time"

	datastore "github.com/ipfs/go-datastore"
	dohp2p "github.com/libp2p/go-doh-resolver"
	"github.com/libp2p/go-libp2p"
	cryptop2p "github.com/libp2p/go-libp2p-core/crypto"
	corehost "github.com/libp2p/go-libp2p-core/host"
	metrics "github.com/libp2p/go-libp2p-core/metrics"
	netp2p "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	corepeerstore "github.com/libp2p/go-libp2p-core/peerstore"
	discovery "github.com/libp2p/go-libp2p-discovery"
	p2phost "github.com/libp2p/go-libp2p-host"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	noisep2p "github.com/libp2p/go-libp2p-noise"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/libp2p/go-libp2p-peerstore/pstoremem"
	routing "github.com/libp2p/go-libp2p-routing"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	rhost "github.com/libp2p/go-libp2p/p2p/host/routed"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	ma "github.com/multiformats/go-multiaddr"
	madns "github.com/multiformats/go-multiaddr-dns"

	"redwood.dev/errors"
	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/state"
)

type Relay interface {
	process.Interface
	log.Logger
	Libp2pHost() p2phost.Host
	Libp2pPeerID() string
	DHT() *dht.IpfsDHT
	Peers() []peer.AddrInfo
	Peerstore() corepeerstore.Peerstore
}

type relay struct {
	process.Process
	log.Logger

	libp2pHost    p2phost.Host
	dht           *dht.IpfsDHT
	mdns          mdns.Service
	datastorePath string
	// datastore         *badgerds.Datastore
	peerstore         corepeerstore.Peerstore
	encryptionConfig  state.EncryptionConfig
	peerID            peer.ID
	port              uint
	p2pKey            cryptop2p.PrivKey
	bootstrapPeers    []string
	dohDNSResolverURL string
	*metrics.BandwidthCounter
}

func NewRelay(
	port uint,
	bootstrapPeers []string,
	p2pKey cryptop2p.PrivKey,
	dohDNSResolverURL string,
	datastorePath string,
	encryptionConfig state.EncryptionConfig,
) *relay {
	return &relay{
		Process:           *process.New("relay"),
		Logger:            log.NewLogger("relay"),
		port:              port,
		bootstrapPeers:    bootstrapPeers,
		p2pKey:            p2pKey,
		datastorePath:     datastorePath,
		encryptionConfig:  encryptionConfig,
		dohDNSResolverURL: dohDNSResolverURL,
	}
}

const dhtTTL = 5 * time.Minute

func (r *relay) Start() error {
	err := r.Process.Start()
	if err != nil {
		return err
	}

	r.Infof("opening libp2p on port %v", r.port)

	peerID, err := peer.IDFromPublicKey(r.p2pKey.GetPublic())
	if err != nil {
		return err
	}
	r.peerID = peerID

	r.BandwidthCounter = metrics.NewBandwidthCounter()

	// opts := badgerds.DefaultOptions
	// opts.Options.EncryptionKey = r.encryptionConfig.Key
	// opts.Options.EncryptionKeyRotationDuration = r.encryptionConfig.KeyRotationInterval
	// opts.Options.IndexCacheSize = 100 << 20 // @@TODO: make configurable
	// opts.Options.KeepL0InMemory = true      // @@TODO: make configurable
	// opts.GcDiscardRatio = 0.5
	// opts.GcInterval = 5 * time.Minute
	// datastore, err := badgerds.NewDatastore(r.datastorePath, &opts)
	// if err != nil {
	// 	return err
	// }
	// r.datastore = datastore

	// r.peerstore, err = pstoreds.NewPeerstore(r.Process.Ctx(), datastore.NewMapDatastore(), pstoreds.DefaultOpts())
	// if err != nil {
	// 	return err
	// }

	var innerResolver madns.BasicResolver
	if r.dohDNSResolverURL != "" {
		innerResolver = dohp2p.NewResolver(r.dohDNSResolverURL)
	} else {
		innerResolver = net.DefaultResolver
	}
	dnsResolver, err := madns.NewResolver(madns.WithDefaultResolver(innerResolver))
	if err != nil {
		return err
	}

	bootstrapPeers, err := addrInfosFromStrings(r.bootstrapPeers)
	if err != nil {
		r.Warnf("while decoding bootstrap peers: %v", err)
	}

	peerStore, err := pstoremem.NewPeerstore()
	if err != nil {
		return err
	}

	// autorelay.AdvertiseBootDelay = 10 * time.Second

	relayResources := relayv2.DefaultResources()
	relayResources.ReservationTTL = 30 * time.Minute

	// Initialize the libp2p host
	libp2pHost, err := libp2p.New(
		libp2p.ListenAddrStrings(
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%v", r.port),
			fmt.Sprintf("/ip6/::/tcp/%v", r.port),
		),
		libp2p.Identity(r.p2pKey),
		libp2p.BandwidthReporter(r.BandwidthCounter),
		libp2p.NATPortMap(),
		libp2p.EnableNATService(),
		// libp2p.EnableRelay(),
		libp2p.DisableRelay(),
		// libp2p.EnableAutoRelay(
		// autorelay.WithStaticRelays(static),
		// autorelay.WithDiscoverer(discover discovery.Discoverer)
		// ),
		libp2p.EnableRelayService(
			// relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
			relayv2.WithResources(relayResources),
		//     WithLimit(limit *RelayLimit)
		//     WithACL(acl ACLFilter)
		),
		libp2p.EnableHolePunching(
		//    WithTracer(tr EventTracer)
		),
		libp2p.Peerstore(peerStore),
		libp2p.ForceReachabilityPublic(),
		libp2p.Security(noisep2p.ID, noisep2p.New),
		libp2p.MultiaddrResolver(dnsResolver),
		libp2p.Routing(func(host corehost.Host) (routing.PeerRouting, error) {
			r.dht, err = dht.New(r.Process.Ctx(), host,
				dht.BootstrapPeers(bootstrapPeers...),
				dht.Mode(dht.ModeServer),
				dht.Datastore(datastore.NewMapDatastore()),
				dht.MaxRecordAge(dhtTTL),
				// dht.RoutingTableFilter(dht.PublicRoutingTableFilter),
			)
			if err != nil {
				return nil, err
			}
			return r.dht, nil
		}),
	)
	if err != nil {
		return errors.Wrap(err, "could not initialize libp2p host")
	}

	r.libp2pHost = rhost.Wrap(libp2pHost, r.dht)
	r.libp2pHost.Network().Notify(r) // Register for libp2p connect/disconnect notifications

	err = r.dht.Bootstrap(r.Process.Ctx())
	if err != nil {
		return errors.Wrap(err, "could not bootstrap DHT")
	}

	// Set up DHT discovery
	routingDiscovery := discovery.NewRoutingDiscovery(r.dht)
	discovery.Advertise(r.Process.Ctx(), routingDiscovery, "redwood")

	// Attempt to connect to peers we find through the DHT. The relay node will only log that
	// it has found a peer once per `dhtTTL` (to avoid log spam). However, it will repeatedly attempt
	// to connect until `dhtTTL` elapses (silently).
	r.Process.Go(nil, "find peers", func(ctx context.Context) {
		lastLogged := make(map[peer.ID]time.Time)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			func() {
				ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()

				chPeers, err := routingDiscovery.FindPeers(ctx, "redwood")
				if err != nil {
					r.Errorf("error finding peers: %v", err)
					return
				}
				for pinfo := range chPeers {
					if pinfo.ID == r.peerID {
						continue
					} else if len(r.libp2pHost.Network().ConnsToPeer(pinfo.ID)) > 0 {
						continue
					}

					// Try to avoid log spam after a peer disconnects. Only log once during the DHT entry TTL window.
					if time.Now().Sub(lastLogged[pinfo.ID]) > dhtTTL {
						r.Debugf("DHT peer found: %v", pinfo.ID.Pretty())
						lastLogged[pinfo.ID] = time.Now()
					}

					pinfo := pinfo
					r.Process.Go(nil, fmt.Sprintf("connect to %v", pinfo.ID.Pretty()), func(ctx context.Context) {
						err := r.libp2pHost.Connect(ctx, pinfo)
						if err != nil {
							// r.Errorf("could not connect to %v: %v", pinfo.ID, err)
						}
					})
				}
				time.Sleep(1 * time.Second)
			}()
		}
	})

	// Set up mDNS discovery
	r.mdns = mdns.NewMdnsService(r.libp2pHost, "redwood", r)
	err = r.mdns.Start()
	if err != nil {
		return err
	}

	r.Infof("libp2p peer ID is %v", r.Libp2pPeerID())

	return nil
}

func (r *relay) Close() error {
	err := r.mdns.Close()
	if err != nil {
		r.Errorf("error closing libp2p mDNS service: %v", err)
	}
	err = r.dht.Close()
	if err != nil {
		r.Errorf("error closing libp2p dht: %v", err)
	}
	err = r.libp2pHost.Close()
	if err != nil {
		r.Errorf("error closing libp2p host: %v", err)
	}
	return r.Process.Close()
}

func (r *relay) Libp2pPeerID() string {
	return r.libp2pHost.ID().Pretty()
}

func (r *relay) Libp2pHost() p2phost.Host {
	return r.libp2pHost
}
func (r *relay) DHT() *dht.IpfsDHT {
	return r.dht
}

func (r *relay) Peers() []peer.AddrInfo {
	return peerstore.PeerInfos(r.libp2pHost.Peerstore(), r.libp2pHost.Peerstore().Peers())
}

func (r *relay) Peerstore() corepeerstore.Peerstore {
	return r.peerstore
}

func (r *relay) Listen(network netp2p.Network, multiaddr ma.Multiaddr)      {}
func (r *relay) ListenClose(network netp2p.Network, multiaddr ma.Multiaddr) {}

func (r *relay) Connected(network netp2p.Network, conn netp2p.Conn) {
	addr := conn.RemoteMultiaddr().String() + "/p2p/" + conn.RemotePeer().Pretty()
	r.Debugf("libp2p connected: %v", addr)
}

func (r *relay) Disconnected(network netp2p.Network, conn netp2p.Conn) {
	addr := conn.RemoteMultiaddr().String() + "/p2p/" + conn.RemotePeer().Pretty()
	r.Debugf("libp2p disconnected: %v", addr)
}

func (r *relay) OpenedStream(network netp2p.Network, stream netp2p.Stream) {}
func (r *relay) ClosedStream(network netp2p.Network, stream netp2p.Stream) {}

// HandlePeerFound is the libp2p mDNS peer discovery callback
func (r *relay) HandlePeerFound(pinfo peer.AddrInfo) {
	if pinfo.ID == r.peerID {
		return
	} else if len(r.libp2pHost.Network().ConnsToPeer(pinfo.ID)) > 0 {
		return
	}

	r.Debugf("mDNS peer found: %v", pinfo.ID.Pretty())

	err := r.libp2pHost.Connect(r.Process.Ctx(), pinfo)
	if err != nil {
		// r.Errorf("could not connect to %v: %v", pinfo.ID, err)
	}
}
