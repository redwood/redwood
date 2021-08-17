package libp2p

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	badgerds "github.com/ipfs/go-ds-badger2"
	dohp2p "github.com/libp2p/go-doh-resolver"
	libp2p "github.com/libp2p/go-libp2p"
	circuitp2p "github.com/libp2p/go-libp2p-circuit"
	cryptop2p "github.com/libp2p/go-libp2p-core/crypto"
	metrics "github.com/libp2p/go-libp2p-core/metrics"
	netp2p "github.com/libp2p/go-libp2p-core/network"
	p2phost "github.com/libp2p/go-libp2p-host"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	noisep2p "github.com/libp2p/go-libp2p-noise"
	peer "github.com/libp2p/go-libp2p-peer"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/libp2p/go-libp2p-peerstore/pstoreds"
	"github.com/libp2p/go-libp2p/p2p/discovery"
	ma "github.com/multiformats/go-multiaddr"
	madns "github.com/multiformats/go-multiaddr-dns"
	"github.com/pkg/errors"

	"redwood.dev/log"
	"redwood.dev/process"
)

type bootstrapNode struct {
	process.Process
	log.Logger

	libp2pHost        p2phost.Host
	dht               *dht.IpfsDHT
	datastorePath     string
	disc              discovery.Service
	peerID            peer.ID
	port              uint
	p2pKey            cryptop2p.PrivKey
	bootstrapPeers    []string
	dohDNSResolverURL string
	*metrics.BandwidthCounter
}

func NewBootstrapNode(port uint, bootstrapPeers []string, dohDNSResolverURL, datastorePath string) *bootstrapNode {
	return &bootstrapNode{
		Process:           *process.New("bootstrap"),
		Logger:            log.NewLogger("bootstrap"),
		port:              port,
		bootstrapPeers:    bootstrapPeers,
		datastorePath:     datastorePath,
		dohDNSResolverURL: dohDNSResolverURL,
	}
}

func (bn *bootstrapNode) Start() error {
	err := bn.Process.Start()
	if err != nil {
		return err
	}

	bn.Infof(0, "opening libp2p on port %v", bn.port)

	err = bn.findOrCreateP2PKey()
	if err != nil {
		return err
	}

	peerID, err := peer.IDFromPublicKey(bn.p2pKey.GetPublic())
	if err != nil {
		return err
	}
	bn.peerID = peerID

	bn.BandwidthCounter = metrics.NewBandwidthCounter()

	datastore, err := badgerds.NewDatastore(bn.datastorePath, nil)
	if err != nil {
		return err
	}

	peerstore, err := pstoreds.NewPeerstore(bn.Process.Ctx(), datastore, pstoreds.DefaultOpts())
	if err != nil {
		return err
	}

	dnsResolver, err := madns.NewResolver(madns.WithDefaultResolver(dohp2p.NewResolver(bn.dohDNSResolverURL)))
	if err != nil {
		return err
	}

	// Initialize the libp2p host
	libp2pHost, err := libp2p.New(bn.Process.Ctx(),
		libp2p.ListenAddrStrings(
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%v", bn.port),
		),
		libp2p.Identity(bn.p2pKey),
		libp2p.BandwidthReporter(bn.BandwidthCounter),
		libp2p.NATPortMap(),
		libp2p.EnableNATService(),
		libp2p.EnableAutoRelay(),
		// libp2p.DefaultStaticRelays(),
		libp2p.EnableRelay(circuitp2p.OptHop),
		libp2p.Peerstore(peerstore),
		libp2p.Security(noisep2p.ID, noisep2p.New),
		libp2p.MultiaddrResolver(dnsResolver),
	)
	if err != nil {
		return errors.Wrap(err, "could not initialize libp2p host")
	}
	bn.libp2pHost = libp2pHost
	bn.libp2pHost.Network().Notify(bn) // Register for libp2p connect/disconnect notifications

	// Initialize the DHT
	bootstrapPeers, err := addrInfosFromStrings(bn.bootstrapPeers)
	if err != nil {
		bn.Warnf("while decoding bootstrap peers: %v", err)
	}

	bn.dht, err = dht.New(bn.Process.Ctx(), bn.libp2pHost,
		dht.BootstrapPeers(bootstrapPeers...),
		dht.Mode(dht.ModeServer),
		dht.Datastore(datastore),
		// dht.Validator(blankValidator{}), // Set a pass-through validator
	)
	if err != nil {
		return errors.Wrap(err, "could not initialize libp2p dht")
	}

	err = bn.dht.Bootstrap(bn.Process.Ctx())
	if err != nil {
		return errors.Wrap(err, "could not bootstrap DHT")
	}

	// Set up mDNS discovery
	bn.disc, err = discovery.NewMdnsService(bn.Process.Ctx(), libp2pHost, 10*time.Second, "redwood")
	if err != nil {
		return err
	}
	bn.disc.RegisterNotifee(bn)

	bn.Infof(0, "libp2p peer ID is %v", bn.Libp2pPeerID())

	return nil
}

func (bn *bootstrapNode) Close() error {
	err := bn.disc.Close()
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

func (bn *bootstrapNode) findOrCreateP2PKey() error {
	filename := filepath.Join(".", "p2pkey")

	p2pkeyBytes, err := ioutil.ReadFile(filename)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	exists := !os.IsNotExist(err)
	if exists {
		p2pKey, err := cryptop2p.UnmarshalPrivateKey(p2pkeyBytes)
		if err != nil {
			return err
		}
		bn.p2pKey = p2pKey
		return nil
	}

	p2pKey, _, err := cryptop2p.GenerateKeyPair(cryptop2p.Secp256k1, 0)
	if err != nil {
		return err
	}
	bn.p2pKey = p2pKey

	bs, err := cryptop2p.MarshalPrivateKey(p2pKey)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, bs, 0600)
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
func (bn *bootstrapNode) HandlePeerFound(pinfo peerstore.PeerInfo) {}
