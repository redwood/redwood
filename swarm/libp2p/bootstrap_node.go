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
	mdns "github.com/libp2p/go-libp2p/p2p/discovery/mdns_legacy"
	"github.com/libp2p/go-libp2p/p2p/host/autorelay"

	rhost "github.com/libp2p/go-libp2p/p2p/host/routed"
	ma "github.com/multiformats/go-multiaddr"
	madns "github.com/multiformats/go-multiaddr-dns"

	"redwood.dev/errors"
	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/state"
)

type BootstrapNode interface {
	process.Interface
	log.Logger
	Libp2pHost() p2phost.Host
	Libp2pPeerID() string
	DHT() *dht.IpfsDHT
	Peers() []peer.AddrInfo
	Peerstore() corepeerstore.Peerstore
}

type bootstrapNode struct {
	process.Process
	log.Logger

	libp2pHost    p2phost.Host
	dht           *dht.IpfsDHT
	datastorePath string
	// datastore         *badgerds.Datastore
	peerstore         corepeerstore.Peerstore
	encryptionConfig  state.EncryptionConfig
	mdns              mdns.Service
	peerID            peer.ID
	port              uint
	p2pKey            cryptop2p.PrivKey
	bootstrapPeers    []string
	dohDNSResolverURL string
	*metrics.BandwidthCounter
}

func NewBootstrapNode(
	port uint,
	bootstrapPeers []string,
	p2pKey cryptop2p.PrivKey,
	dohDNSResolverURL string,
	datastorePath string,
	encryptionConfig state.EncryptionConfig,
) *bootstrapNode {
	return &bootstrapNode{
		Process:           *process.New("bootstrap"),
		Logger:            log.NewLogger("bootstrap"),
		port:              port,
		bootstrapPeers:    bootstrapPeers,
		p2pKey:            p2pKey,
		datastorePath:     datastorePath,
		encryptionConfig:  encryptionConfig,
		dohDNSResolverURL: dohDNSResolverURL,
	}
}

const dhtTTL = 5 * time.Minute

func (bn *bootstrapNode) Start() error {
	err := bn.Process.Start()
	if err != nil {
		return err
	}

	bn.Infof("opening libp2p on port %v", bn.port)

	peerID, err := peer.IDFromPublicKey(bn.p2pKey.GetPublic())
	if err != nil {
		return err
	}
	bn.peerID = peerID

	bn.BandwidthCounter = metrics.NewBandwidthCounter()

	// opts := badgerds.DefaultOptions
	// opts.Options.EncryptionKey = bn.encryptionConfig.Key
	// opts.Options.EncryptionKeyRotationDuration = bn.encryptionConfig.KeyRotationInterval
	// opts.Options.IndexCacheSize = 100 << 20 // @@TODO: make configurable
	// opts.Options.KeepL0InMemory = true      // @@TODO: make configurable
	// opts.GcDiscardRatio = 0.5
	// opts.GcInterval = 5 * time.Minute
	// datastore, err := badgerds.NewDatastore(bn.datastorePath, &opts)
	// if err != nil {
	// 	return err
	// }
	// bn.datastore = datastore

	// bn.peerstore, err = pstoreds.NewPeerstore(bn.Process.Ctx(), datastore.NewMapDatastore(), pstoreds.DefaultOpts())
	// if err != nil {
	// 	return err
	// }

	var innerResolver madns.BasicResolver
	if bn.dohDNSResolverURL != "" {
		innerResolver = dohp2p.NewResolver(bn.dohDNSResolverURL)
	} else {
		innerResolver = net.DefaultResolver
	}
	dnsResolver, err := madns.NewResolver(madns.WithDefaultResolver(innerResolver))
	if err != nil {
		return err
	}

	bootstrapPeers, err := addrInfosFromStrings(bn.bootstrapPeers)
	if err != nil {
		bn.Warnf("while decoding bootstrap peers: %v", err)
	}

	peerStore, err := pstoremem.NewPeerstore()
	if err != nil {
		return err
	}

	autorelay.AdvertiseBootDelay = 10 * time.Second

	// Initialize the libp2p host
	libp2pHost, err := libp2p.New(
		libp2p.ListenAddrStrings(
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%v", bn.port),
			fmt.Sprintf("/ip6/::/tcp/%v", bn.port),
		),
		libp2p.Identity(bn.p2pKey),
		libp2p.BandwidthReporter(bn.BandwidthCounter),
		libp2p.NATPortMap(),
		libp2p.EnableNATService(),
		libp2p.EnableRelay(),
		libp2p.EnableAutoRelay(
		// autorelay.WithStaticRelays(static),
		// autorelay.WithDiscoverer(discover discovery.Discoverer)
		),
		libp2p.EnableRelayService(
		// relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
		//     WithResources(rc Resources)
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
			bn.dht, err = dht.New(bn.Process.Ctx(), host,
				dht.BootstrapPeers(bootstrapPeers...),
				dht.Mode(dht.ModeServer),
				dht.Datastore(datastore.NewMapDatastore()),
				dht.MaxRecordAge(dhtTTL),
				// dht.RoutingTableFilter(dht.PublicRoutingTableFilter),
			)
			if err != nil {
				return nil, err
			}
			return bn.dht, nil
		}),
	)
	if err != nil {
		return errors.Wrap(err, "could not initialize libp2p host")
	}

	bn.libp2pHost = rhost.Wrap(libp2pHost, bn.dht)
	bn.libp2pHost.Network().Notify(bn) // Register for libp2p connect/disconnect notifications

	err = bn.dht.Bootstrap(bn.Process.Ctx())
	if err != nil {
		return errors.Wrap(err, "could not bootstrap DHT")
	}

	// Set up DHT discovery
	routingDiscovery := discovery.NewRoutingDiscovery(bn.dht)
	discovery.Advertise(bn.Process.Ctx(), routingDiscovery, "redwood")

	// Attempt to connect to peers we find through the DHT. The bootstrap node will only log that
	// it has found a peer once per `dhtTTL` (to avoid log spam). However, it will repeatedly attempt
	// to connect until `dhtTTL` elapses (silently).
	bn.Process.Go(nil, "find peers", func(ctx context.Context) {
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
					bn.Errorf("error finding peers: %v", err)
					return
				}
				for pinfo := range chPeers {
					if pinfo.ID == bn.peerID {
						continue
					} else if len(bn.libp2pHost.Network().ConnsToPeer(pinfo.ID)) > 0 {
						continue
					}

					// Try to avoid log spam after a peer disconnects. Only log once during the DHT entry TTL window.
					if time.Now().Sub(lastLogged[pinfo.ID]) > dhtTTL {
						bn.Debugf("DHT peer found: %v", pinfo.ID.Pretty())
						lastLogged[pinfo.ID] = time.Now()
					}

					pinfo := pinfo
					bn.Process.Go(nil, fmt.Sprintf("connect to %v", pinfo.ID.Pretty()), func(ctx context.Context) {
						err := bn.libp2pHost.Connect(ctx, pinfo)
						if err != nil {
							// bn.Errorf("could not connect to %v: %v", pinfo.ID, err)
						}
					})
				}
				time.Sleep(1 * time.Second)
			}()
		}
	})

	// Set up mDNS discovery
	bn.mdns, err = mdns.NewMdnsService(bn.Process.Ctx(), libp2pHost, 30*time.Second, "redwood")
	if err != nil {
		return err
	}
	bn.mdns.RegisterNotifee(bn)

	bn.Infof("libp2p peer ID is %v", bn.Libp2pPeerID())

	return nil
}

func (bn *bootstrapNode) Close() error {
	err := bn.mdns.Close()
	if err != nil {
		bn.Errorf("error closing libp2p mDNS service: %v", err)
	}
	err = bn.dht.Close()
	if err != nil {
		bn.Errorf("error closing libp2p dht: %v", err)
	}
	err = bn.libp2pHost.Close()
	if err != nil {
		bn.Errorf("error closing libp2p host: %v", err)
	}
	return bn.Process.Close()
}

func (bn *bootstrapNode) Libp2pPeerID() string {
	return bn.libp2pHost.ID().Pretty()
}

func (bn *bootstrapNode) Libp2pHost() p2phost.Host {
	return bn.libp2pHost
}
func (bn *bootstrapNode) DHT() *dht.IpfsDHT {
	return bn.dht
}

func (bn *bootstrapNode) Peers() []peer.AddrInfo {
	return peerstore.PeerInfos(bn.libp2pHost.Peerstore(), bn.libp2pHost.Peerstore().Peers())
}

func (bn *bootstrapNode) Peerstore() corepeerstore.Peerstore {
	return bn.peerstore
}

func (bn *bootstrapNode) Listen(network netp2p.Network, multiaddr ma.Multiaddr)      {}
func (bn *bootstrapNode) ListenClose(network netp2p.Network, multiaddr ma.Multiaddr) {}

func (bn *bootstrapNode) Connected(network netp2p.Network, conn netp2p.Conn) {
	addr := conn.RemoteMultiaddr().String() + "/p2p/" + conn.RemotePeer().Pretty()
	bn.Debugf("libp2p connected: %v", addr)
}

func (bn *bootstrapNode) Disconnected(network netp2p.Network, conn netp2p.Conn) {
	addr := conn.RemoteMultiaddr().String() + "/p2p/" + conn.RemotePeer().Pretty()
	bn.Debugf("libp2p disconnected: %v", addr)
}

func (bn *bootstrapNode) OpenedStream(network netp2p.Network, stream netp2p.Stream) {}
func (bn *bootstrapNode) ClosedStream(network netp2p.Network, stream netp2p.Stream) {}

// HandlePeerFound is the libp2p mDNS peer discovery callback
func (bn *bootstrapNode) HandlePeerFound(pinfo peer.AddrInfo) {
	if pinfo.ID == bn.peerID {
		return
	} else if len(bn.libp2pHost.Network().ConnsToPeer(pinfo.ID)) > 0 {
		return
	}

	bn.Debugf("mDNS peer found: %v", pinfo.ID.Pretty())

	err := bn.libp2pHost.Connect(bn.Process.Ctx(), pinfo)
	if err != nil {
		// bn.Errorf("could not connect to %v: %v", pinfo.ID, err)
	}
}
