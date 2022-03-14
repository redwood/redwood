package libp2p

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"time"

	datastore "github.com/ipfs/go-datastore"
	dohp2p "github.com/libp2p/go-doh-resolver"
	libp2p "github.com/libp2p/go-libp2p"
	cryptop2p "github.com/libp2p/go-libp2p-core/crypto"
	corehost "github.com/libp2p/go-libp2p-core/host"
	metrics "github.com/libp2p/go-libp2p-core/metrics"
	"github.com/libp2p/go-libp2p-core/network"
	peer "github.com/libp2p/go-libp2p-core/peer"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	mplex "github.com/libp2p/go-libp2p-mplex"
	noisep2p "github.com/libp2p/go-libp2p-noise"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/libp2p/go-libp2p-peerstore/pstoremem"
	protocol "github.com/libp2p/go-libp2p-protocol"
	routing "github.com/libp2p/go-libp2p-routing"
	"github.com/libp2p/go-libp2p/p2p/host/autorelay"
	ma "github.com/multiformats/go-multiaddr"
	madns "github.com/multiformats/go-multiaddr-dns"

	"redwood.dev/blob"
	"redwood.dev/errors"
	"redwood.dev/identity"
	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/swarm"
	"redwood.dev/swarm/libp2p/pb"
	"redwood.dev/swarm/protoauth"
	"redwood.dev/swarm/protoblob"
	"redwood.dev/swarm/protohush"
	"redwood.dev/swarm/prototree"
	"redwood.dev/tree"
	"redwood.dev/types"
)

type Transport interface {
	process.Interface
	swarm.Transport
	protoauth.AuthTransport
	protoblob.BlobTransport
	protohush.HushTransport
	prototree.TreeTransport
	Libp2pPeerID() string
	ListenAddrs() []string
	Peers() []peer.AddrInfo
	AddStaticRelay(relayAddr string) error
	RemoveStaticRelay(relayAddr string) error
	StaticRelays() PeerSet
}

type transport struct {
	process.Process
	log.Logger
	protoauth.BaseAuthTransport
	protoblob.BaseBlobTransport
	protohush.BaseHushTransport
	prototree.BaseTreeTransport

	libp2pHost corehost.Host
	dht        *dht.IpfsDHT
	// dht               *dual.DHT
	// mdns              mdns.Service
	peerID            peer.ID
	port              uint
	p2pKey            cryptop2p.PrivKey
	reachableAt       string
	bootstrapPeers    PeerSet
	datastorePath     string
	dohDNSResolverURL string
	*metrics.BandwidthCounter

	dhtClient   DHTClient
	peerManager PeerManager

	store         Store
	controllerHub tree.ControllerHub
	peerStore     swarm.PeerStore
	keyStore      identity.KeyStore
	blobStore     blob.Store

	writeSubsByPeerID   map[peer.ID]map[network.Stream]*writableSubscription
	writeSubsByPeerIDMu sync.Mutex
}

var _ Transport = (*transport)(nil)
var _ protohush.HushTransport = (*transport)(nil)

const (
	PROTO_MAIN    protocol.ID = "/redwood/main/1.0.0"
	PROTO_BLOB    protocol.ID = "/redwood/blob/1.0.0"
	PROTO_HUSH    protocol.ID = "/redwood/hush/1.0.0"
	TransportName string      = "libp2p"
)

func NewTransport(
	port uint,
	reachableAt string,
	bootstrapPeers []string,
	datastorePath string,
	dohDNSResolverURL string,
	store Store,
	controllerHub tree.ControllerHub,
	keyStore identity.KeyStore,
	blobStore blob.Store,
	peerStore swarm.PeerStore,
) (*transport, error) {
	bootstrapPeerSet, err := NewPeerSetFromStrings(bootstrapPeers)
	if err != nil {
		return nil, err
	}

	t := &transport{
		Process:           *process.New(TransportName),
		Logger:            log.NewLogger(TransportName),
		port:              port,
		reachableAt:       reachableAt,
		bootstrapPeers:    bootstrapPeerSet,
		datastorePath:     datastorePath,
		dohDNSResolverURL: dohDNSResolverURL,
		store:             store,
		controllerHub:     controllerHub,
		keyStore:          keyStore,
		blobStore:         blobStore,
		peerStore:         peerStore,
		writeSubsByPeerID: make(map[peer.ID]map[network.Stream]*writableSubscription),
	}
	return t, nil
}

func (t *transport) Start() error {
	err := t.Process.Start()
	if err != nil {
		return err
	}

	t.Infof(0, "opening libp2p on port %v", t.port)

	err = t.findOrCreateP2PKey()
	if err != nil {
		return err
	}

	peerID, err := peer.IDFromPublicKey(t.p2pKey.GetPublic())
	if err != nil {
		return err
	}
	t.peerID = peerID

	t.BandwidthCounter = metrics.NewBandwidthCounter()

	var innerResolver madns.BasicResolver
	if t.dohDNSResolverURL != "" {
		innerResolver = dohp2p.NewResolver(t.dohDNSResolverURL)
	} else {
		innerResolver = net.DefaultResolver
	}
	dnsResolver, err := madns.NewResolver(madns.WithDefaultResolver(innerResolver))
	if err != nil {
		return err
	}

	peerStore, err := pstoremem.NewPeerstore()
	if err != nil {
		return err
	}

	autorelay.AdvertiseBootDelay = 10 * time.Second

	// Initialize the libp2p host
	t.libp2pHost, err = libp2p.New(
		libp2p.ListenAddrStrings(
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%v", t.port),
			fmt.Sprintf("/ip6/::/tcp/%v", t.port),
		),
		libp2p.Identity(t.p2pKey),
		libp2p.BandwidthReporter(t.BandwidthCounter),
		libp2p.NATPortMap(),
		libp2p.EnableNATService(),
		// libp2p.ForceReachabilityPrivate(),
		// libp2p.StaticRelays(t.store.StaticRelays().Slice()),
		libp2p.EnableRelay(),
		libp2p.EnableAutoRelay(
		// autorelay.WithStaticRelays(t.StaticRelays().Slice()),
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
		libp2p.Security(noisep2p.ID, noisep2p.New),
		libp2p.MultiaddrResolver(dnsResolver),
		libp2p.Muxer("/mplex/6.7.0", mplex.DefaultTransport),
		libp2p.Routing(func(host corehost.Host) (routing.PeerRouting, error) {
			t.dht, err = dht.New(t.Process.Ctx(), host,
				dht.BootstrapPeers(append(t.store.StaticRelays().Slice(), t.bootstrapPeers.Slice()...)...),
				dht.Mode(dht.ModeServer),
				dht.Datastore(datastore.NewMapDatastore()),
				dht.Resiliency(1),
				// dht.MaxRecordAge(dhtTTL),
				// dht.RoutingTableFilter(dht.PublicRoutingTableFilter),
			)
			return t.dht, err
		}),
	)
	if err != nil {
		return errors.Wrap(err, "could not initialize libp2p host")
	}

	// Update our node's info in the peer store
	myDialAddrs := types.NewSet[string](nil)
	for _, addr := range t.ListenAddrs() {
		myDialAddrs.Add(addr)
	}
	if t.reachableAt != "" {
		myDialAddrs.Add(t.reachableAt)
	}
	for dialAddr := range myDialAddrs {
		identities, err := t.keyStore.Identities()
		if err != nil {
			return err
		}
		for _, identity := range identities {
			t.peerStore.AddVerifiedCredentials(
				swarm.PeerDialInfo{TransportName: TransportName, DialAddr: dialAddr},
				t.libp2pHost.ID().Pretty(),
				identity.SigKeypair.SigningPublicKey.Address(),
				identity.SigKeypair.SigningPublicKey,
				identity.AsymEncKeypair.AsymEncPubkey,
			)
		}
	}

	// t.libp2pHost = rhost.Wrap(libp2pHost, t.dht)

	t.libp2pHost.SetStreamHandler(PROTO_MAIN, t.handleIncomingStream)
	t.libp2pHost.SetStreamHandler(PROTO_BLOB, t.handleBlobStream)
	t.libp2pHost.SetStreamHandler(PROTO_HUSH, t.handleHushStream)
	t.libp2pHost.Network().Notify(t) // Register for libp2p connect/disconnect notifications

	err = t.dht.Bootstrap(t.Process.Ctx())
	if err != nil {
		return errors.Wrap(err, "could not bootstrap DHT")
	}

	t.peerManager = NewPeerManager(t, "redwood", t.libp2pHost, t.dht, t.store, t.peerStore)
	err = t.Process.SpawnChild(nil, t.peerManager)
	if err != nil {
		return errors.Wrap(err, "could not start peer discovery")
	}

	t.dhtClient = NewDHTClient(t.libp2pHost, t.dht, t.peerManager)
	err = t.Process.SpawnChild(nil, t.dhtClient)
	if err != nil {
		return errors.Wrap(err, "could not start dht client")
	}

	t.Infof(0, "libp2p peer ID is %v", t.Libp2pPeerID())

	return nil
}

func (t *transport) Close() error {
	t.Infof(0, "libp2p transport shutting down")

	err := t.dht.Close()
	if err != nil {
		t.Errorf("error closing libp2p dht: %v", err)
	}
	err = t.libp2pHost.Close()
	if err != nil {
		t.Errorf("error closing libp2p host: %v", err)
	}
	return t.Process.Close()
}

func (t *transport) findOrCreateP2PKey() error {
	maybeKey, exists, err := t.keyStore.ExtraUserData("libp2p:p2pkey")
	if err != nil {
		return err
	}

	p2pkeyBytes, isString := maybeKey.(string)
	if exists && isString {
		bs, err := hex.DecodeString(p2pkeyBytes)
		if err != nil {
			return err
		}
		p2pKey, err := cryptop2p.UnmarshalPrivateKey(bs)
		if err != nil {
			return err
		}
		t.p2pKey = p2pKey
		return nil
	}

	p2pKey, _, err := cryptop2p.GenerateKeyPair(cryptop2p.Ed25519, 0)
	if err != nil {
		return err
	}
	t.p2pKey = p2pKey

	bs, err := cryptop2p.MarshalPrivateKey(p2pKey)
	if err != nil {
		return err
	}
	hexKey := hex.EncodeToString(bs)
	return t.keyStore.SaveExtraUserData("libp2p:p2pkey", hexKey)
}

func (t *transport) Name() string {
	return TransportName
}

func (t *transport) NewPeerConn(ctx context.Context, dialAddr string) (swarm.PeerConn, error) {
	return t.peerManager.NewPeerConn(ctx, dialAddr)
}

func (t *transport) ProvidersOfStateURI(ctx context.Context, stateURI string) (<-chan prototree.TreePeerConn, error) {
	return t.dhtClient.ProvidersOfStateURI(ctx, stateURI)
}

func (t *transport) ProvidersOfBlob(ctx context.Context, blobID blob.ID) (<-chan protoblob.BlobPeerConn, error) {
	return t.dhtClient.ProvidersOfBlob(ctx, blobID)
}

func (t *transport) AnnounceStateURIs(ctx context.Context, stateURIs types.Set[string]) {
	t.dhtClient.AnnounceStateURIs(ctx, stateURIs)
}

func (t *transport) AnnounceBlobs(ctx context.Context, blobIDs types.Set[blob.ID]) {
	t.dhtClient.AnnounceBlobs(ctx, blobIDs)
}

func (t *transport) Libp2pPeerID() string {
	return t.libp2pHost.ID().Pretty()
}

func (t *transport) ListenAddrs() []string {
	addrs := []string{}
	for _, addr := range t.libp2pHost.Addrs() {
		addrs = append(addrs, addr.String()+"/p2p/"+t.libp2pHost.ID().Pretty())
	}
	return addrs
}

func (t *transport) Peers() []peer.AddrInfo {
	return peerstore.PeerInfos(t.libp2pHost.Peerstore(), t.libp2pHost.Peerstore().Peers())
}

func (t *transport) StaticRelays() PeerSet {
	return t.store.StaticRelays()
}

func (t *transport) AddStaticRelay(relayAddr string) error {
	err := t.store.AddStaticRelay(relayAddr)
	if err != nil {
		return err
	}
	t.peerManager.EnsureConnectedToAllPeers()
	return nil
}

func (t *transport) RemoveStaticRelay(relayAddr string) error {
	return t.store.RemoveStaticRelay(relayAddr)
}

func (t *transport) Listen(network network.Network, multiaddr ma.Multiaddr)      {}
func (t *transport) ListenClose(network network.Network, multiaddr ma.Multiaddr) {}

func (t *transport) Connected(network network.Network, conn network.Conn) {
	t.peerManager.OnConnectedToPeer(network, conn)
}

func (t *transport) Disconnected(network network.Network, conn network.Conn) {
	addr := conn.RemoteMultiaddr().String() + "/p2p/" + conn.RemotePeer().Pretty()
	t.Debugf("libp2p disconnected: %v", addr)
}

func (t *transport) OpenedStream(network network.Network, stream network.Stream) {}

func (t *transport) ClosedStream(network network.Network, stream network.Stream) {
	peerID := stream.Conn().RemotePeer()
	t.writeSubsByPeerIDMu.Lock()
	defer t.writeSubsByPeerIDMu.Unlock()

	writeSubs, exists := t.writeSubsByPeerID[peerID]
	if exists {
		if sub, exists := writeSubs[stream]; exists && sub != nil {
			t.Debugf("closing libp2p writable subscription (peer: %v, stateURI: %v)", peerID.Pretty(), sub.stateURI)
			delete(t.writeSubsByPeerID[peerID], stream)
			sub.Close()
		}
	}
}

func (t *transport) handleBlobStream(stream network.Stream) {
	peerConn := t.peerManager.MakeConnectedPeerConn(stream)
	defer peerConn.Close()

	var proto pb.BlobMessage
	err := peerConn.readProtobuf(&proto)
	if err != nil {
		t.Errorf("while reading incoming blob stream: %v", err)
		return
	}

	if msg := proto.GetFetchManifest(); msg != nil {
		t.HandleBlobManifestRequest(msg.BlobID, peerConn)

	} else if msg := proto.GetFetchChunk(); msg != nil {
		sha3, err := types.HashFromBytes(msg.SHA3)
		if err != nil {
			t.Errorf("while parsing blob hash: %v", err)
			return
		}
		t.HandleBlobChunkRequest(sha3, peerConn)

	} else {
		t.Errorf("while reading incoming blob stream: got unknown message")
	}
}

func (t *transport) handleHushStream(stream network.Stream) {
	peerConn := t.peerManager.MakeConnectedPeerConn(stream)
	defer peerConn.Close()

	var proto pb.HushMessage
	err := peerConn.readProtobuf(&proto)
	if err != nil {
		t.Errorf("while reading incoming hush stream: %v", err)
		return
	}

	ctx := context.TODO()

	if msg := proto.GetDhPubkeyAttestations(); msg != nil {
		t.HandleIncomingDHPubkeyAttestations(ctx, msg.Attestations, peerConn)

	} else if msg := proto.GetProposeIndividualSession(); msg != nil {
		t.HandleIncomingIndividualSessionProposal(ctx, msg.EncryptedProposal, peerConn)

	} else if msg := proto.GetRespondToIndividualSessionProposal(); msg != nil {
		t.HandleIncomingIndividualSessionResponse(ctx, *msg.Response, peerConn)

	} else if msg := proto.GetSendIndividualMessage(); msg != nil {
		t.HandleIncomingIndividualMessage(ctx, *msg.Message, peerConn)

	} else if msg := proto.GetSendGroupMessage(); msg != nil {
		t.HandleIncomingGroupMessage(ctx, *msg.Message, peerConn)

	} else {
		t.Errorf("while reading incoming hush stream: got unknown message")
	}
}

func (t *transport) handleIncomingStream(stream network.Stream) {
	peer := t.peerManager.MakeConnectedPeerConn(stream)

	msg, err := peer.readMsg()
	if err != nil {
		t.Errorf("incoming stream error: %v", err)
		stream.Close()
		return
	}

	switch msg.Type {
	case msgType_Subscribe:
		stateURI, ok := msg.Payload.(string)
		if !ok {
			t.Errorf("subscribe message: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}
		t.Infof(0, "incoming libp2p subscription: %v %v", peer.DialInfo(), stateURI)

		fetchHistoryOpts := &prototree.FetchHistoryOpts{FromTxID: tree.GenesisTxID} // Fetch all history (@@TODO)

		var writeSub *writableSubscription
		req := prototree.SubscriptionRequest{
			StateURI:         stateURI,
			Keypath:          nil,
			Type:             prototree.SubscriptionType_Txs,
			FetchHistoryOpts: fetchHistoryOpts,
			Addresses:        peer.Addresses(),
		}
		_, err := t.HandleWritableSubscriptionOpened(req, func() (prototree.WritableSubscriptionImpl, error) {
			writeSub = newWritableSubscription(peer, stateURI)
			return writeSub, nil
		})
		if err != nil && errors.Cause(err) != errors.Err404 {
			peer.Close()
			t.Errorf("while starting incoming subscription: %v", err)
			return
		}

		func() {
			t.writeSubsByPeerIDMu.Lock()
			defer t.writeSubsByPeerIDMu.Unlock()
			if _, exists := t.writeSubsByPeerID[peer.pinfo.ID]; !exists {
				t.writeSubsByPeerID[peer.pinfo.ID] = make(map[network.Stream]*writableSubscription)
			}
			t.writeSubsByPeerID[peer.pinfo.ID][stream] = writeSub
		}()

	case msgType_Tx:
		defer peer.Close()

		tx, ok := msg.Payload.(tree.Tx)
		if !ok {
			t.Errorf("Put message: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}
		t.HandleTxReceived(tx, peer)

	case msgType_Ack:
		defer peer.Close()

		ackMsg, ok := msg.Payload.(ackMsg)
		if !ok {
			t.Errorf("Ack message: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}
		t.HandleAckReceived(ackMsg.StateURI, ackMsg.TxID, peer)

	case msgType_AnnounceP2PStateURI:
		defer peer.Close()

		stateURI, ok := msg.Payload.(string)
		if !ok {
			t.Errorf("P2PStateURI message: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}
		t.HandleP2PStateURIReceived(stateURI, peer)

	case msgType_ChallengeIdentityRequest:
		defer peer.Close()

		challengeMsg, ok := msg.Payload.(protoauth.ChallengeMsg)
		if !ok {
			t.Errorf("msgType_ChallengeIdentityRequest message: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}

		err := t.HandleChallengeIdentity(challengeMsg, peer)
		if err != nil {
			t.Errorf("msgType_ChallengeIdentityRequest: error from verifyAddressHandler: %v", err)
			return
		}

	case msgType_AnnouncePeers:
		defer peer.Close()

		dialInfos, ok := msg.Payload.([]swarm.PeerDialInfo)
		if !ok {
			t.Errorf("Announce peers: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}
		for _, dialInfo := range dialInfos {
			if dialInfo.TransportName == TransportName {
				addrInfo, err := addrInfoFromString(dialInfo.DialAddr)
				if err != nil {
					continue
				}
				t.peerManager.OnPeerFound("announce", addrInfo)
			} else {
				t.peerStore.AddDialInfo(dialInfo, "")
			}
		}

	default:
		panic("protocol error")
	}
}
