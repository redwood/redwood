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
	"redwood.dev/process"
	"redwood.dev/swarm"
	"redwood.dev/swarm/libp2p/pb"
	"redwood.dev/swarm/protoauth"
	"redwood.dev/swarm/protoblob"
	"redwood.dev/swarm/protohush"
	"redwood.dev/swarm/prototree"
	"redwood.dev/tree"
	"redwood.dev/types"
	. "redwood.dev/utils/generics"
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
	swarm.BaseTransport
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

const (
	PROTO_AUTH    protocol.ID = "/redwood/auth/1.0.0"
	PROTO_BLOB    protocol.ID = "/redwood/blob/1.0.0"
	PROTO_HUSH    protocol.ID = "/redwood/hush/1.0.0"
	PROTO_PEER    protocol.ID = "/redwood/peer/1.0.0"
	PROTO_TREE    protocol.ID = "/redwood/tree/1.0.0"
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
		BaseTransport:     swarm.NewBaseTransport(TransportName),
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

	t.Infof("opening libp2p on port %v", t.port)

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
	myDialAddrs := NewSet[string](nil)
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

	t.libp2pHost.SetStreamHandler(PROTO_AUTH, t.handleAuthStream)
	t.libp2pHost.SetStreamHandler(PROTO_BLOB, t.handleBlobStream)
	t.libp2pHost.SetStreamHandler(PROTO_HUSH, t.handleHushStream)
	t.libp2pHost.SetStreamHandler(PROTO_PEER, t.handlePeerStream)
	t.libp2pHost.SetStreamHandler(PROTO_TREE, t.handleTreeStream)
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

	t.Infof("libp2p peer ID is %v", t.Libp2pPeerID())

	t.MarkReady()
	return nil
}

func (t *transport) Close() error {
	t.Infof("libp2p transport shutting down")

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
	ok := <-t.AwaitReady(ctx)
	if !ok {
		return nil, errors.New("libp2p not started")
	}
	return t.peerManager.NewPeerConn(ctx, dialAddr)
}

func (t *transport) ProvidersOfStateURI(ctx context.Context, stateURI string) (<-chan prototree.TreePeerConn, error) {
	ok := <-t.AwaitReady(ctx)
	if !ok {
		return nil, errors.New("libp2p not started")
	}
	return t.dhtClient.ProvidersOfStateURI(ctx, stateURI)
}

func (t *transport) ProvidersOfBlob(ctx context.Context, blobID blob.ID) (<-chan protoblob.BlobPeerConn, error) {
	ok := <-t.AwaitReady(ctx)
	if !ok {
		return nil, errors.New("libp2p not started")
	}
	return t.dhtClient.ProvidersOfBlob(ctx, blobID)
}

func (t *transport) AnnounceStateURIs(ctx context.Context, stateURIs Set[string]) {
	ok := <-t.AwaitReady(ctx)
	if !ok {
		return
	}
	t.dhtClient.AnnounceStateURIs(ctx, stateURIs)
}

func (t *transport) AnnounceBlobs(ctx context.Context, blobIDs Set[blob.ID]) {
	ok := <-t.AwaitReady(ctx)
	if !ok {
		return
	}
	t.dhtClient.AnnounceBlobs(ctx, blobIDs)
}

func (t *transport) Libp2pPeerID() string {
	if !t.Ready() {
		return "<ERR: libp2p not started>"
	}
	return t.libp2pHost.ID().Pretty()
}

func (t *transport) ListenAddrs() []string {
	if !t.Ready() {
		return nil
	}
	addrs := []string{}
	for _, addr := range t.libp2pHost.Addrs() {
		addrs = append(addrs, addr.String()+"/p2p/"+t.libp2pHost.ID().Pretty())
	}
	return addrs
}

func (t *transport) Peers() []peer.AddrInfo {
	if !t.Ready() {
		return nil
	}
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

func (t *transport) handleAuthStream(stream network.Stream) {
	peerConn := t.peerManager.MakeConnectedPeerConn(stream)
	defer peerConn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var proto pb.AuthMessage
	err := peerConn.readProtobuf(ctx, &proto)
	if err != nil {
		t.Errorf("while reading incoming auth stream: %v", err)
		return
	}

	if msg := proto.GetChallengeRequest(); msg != nil {
		err := t.HandleIncomingIdentityChallengeRequest(peerConn)
		if err != nil {
			t.Errorf("while handling incoming identity challenge request: %v", err)
		}
		return

	} else if msg := proto.GetChallenge(); msg != nil {
		err := t.HandleIncomingIdentityChallenge(msg.Challenge, peerConn)
		if err != nil {
			t.Errorf("while handling incoming identity challenge: %v", err)
			return
		}

		err = peerConn.readProtobuf(ctx, &proto)
		if err != nil {
			t.Errorf("while reading incoming auth stream: %v", err)
			return
		}

		msg := proto.GetUcan()
		if msg == nil {
			t.Errorf("while reading incoming auth stream: protocol error")
			return
		}
		t.HandleIncomingUcan(msg.Ucan, peerConn)

	} else {
		t.Errorf("while reading incoming auth stream: got unknown message")
	}
}

// if msg := proto.GetSignatures(); msg != nil {
//         responses := ZipFunc3(payload.Challenges, payload.Signatures, payload.AsymEncPubkeys,
//             func(c crypto.ChallengeMsg, s crypto.Signature, k crypto.AsymEncPubkey, _ int) protoauth.ChallengeIdentityResponse {
//                 return protoauth.ChallengeIdentityResponse{c, s, k}
//             },
//         )
//         t.HandleIncomingIdentityChallengeResponse(responses, peerConn)
// }

func (t *transport) handleBlobStream(stream network.Stream) {
	peerConn := t.peerManager.MakeConnectedPeerConn(stream)
	defer peerConn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var proto pb.BlobMessage
	err := peerConn.readProtobuf(ctx, &proto)
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var proto pb.HushMessage
	err := peerConn.readProtobuf(ctx, &proto)
	if err != nil {
		t.Errorf("while reading incoming hush stream: %v", err)
		return
	}

	if msg := proto.GetPubkeyBundles(); msg != nil {
		t.HandleIncomingPubkeyBundles(msg.Bundles)

	} else {
		t.Errorf("while reading incoming hush stream: got unknown message")
	}
}

func (t *transport) handlePeerStream(stream network.Stream) {
	peerConn := t.peerManager.MakeConnectedPeerConn(stream)
	defer peerConn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var proto pb.PeerMessage
	err := peerConn.readProtobuf(ctx, &proto)
	if err != nil {
		t.Errorf("while reading incoming peer stream: %v", err)
		return
	}

	if msg := proto.GetAnnouncePeers(); msg != nil {
		for _, dialInfo := range msg.DialInfos {
			if dialInfo.TransportName == TransportName {
				addrInfo, err := addrInfoFromString(dialInfo.DialAddr)
				if err != nil {
					continue
				}
				t.peerManager.OnPeerFound("announce", addrInfo)
			}
			t.peerStore.AddDialInfo(dialInfo, "")
		}

	} else {
		t.Errorf("while reading incoming peer stream: got unknown message")
	}
}

func (t *transport) handleTreeStream(stream network.Stream) {
	peerConn := t.peerManager.MakeConnectedPeerConn(stream)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var proto pb.TreeMessage
	err := peerConn.readProtobuf(ctx, &proto)
	if err != nil {
		t.Errorf("while reading incoming tree stream: %v", err)
		peerConn.Close()
		return
	}

	if msg := proto.GetSubscribe(); msg != nil {
		t.Infof("incoming libp2p subscription: %v %v", peerConn.DialInfo(), msg.StateURI)

		fetchHistoryOpts := &prototree.FetchHistoryOpts{FromTxID: tree.GenesisTxID} // Fetch all history (@@TODO)

		var writeSub *writableSubscription
		req := prototree.SubscriptionRequest{
			StateURI:         msg.StateURI,
			Keypath:          nil,
			Type:             prototree.SubscriptionType_Txs,
			FetchHistoryOpts: fetchHistoryOpts,
			Addresses:        peerConn.Addresses(),
		}
		_, err := t.HandleWritableSubscriptionOpened(req, func() (prototree.WritableSubscriptionImpl, error) {
			writeSub = newWritableSubscription(peerConn, msg.StateURI)
			return writeSub, nil
		})
		if err != nil && errors.Cause(err) != errors.Err404 {
			peerConn.Close()
			t.Errorf("while starting incoming subscription: %v", err)
			return
		}

		func() {
			t.writeSubsByPeerIDMu.Lock()
			defer t.writeSubsByPeerIDMu.Unlock()
			if _, exists := t.writeSubsByPeerID[peerConn.pinfo.ID]; !exists {
				t.writeSubsByPeerID[peerConn.pinfo.ID] = make(map[network.Stream]*writableSubscription)
			}
			t.writeSubsByPeerID[peerConn.pinfo.ID][stream] = writeSub
		}()

	} else if msg := proto.GetTx(); msg != nil {
		defer peerConn.Close()
		t.HandleTxReceived(*msg, peerConn)

	} else if msg := proto.GetEncryptedTx(); msg != nil {
		defer peerConn.Close()
		t.HandleEncryptedTxReceived(msg.EncryptedTx, peerConn)

	} else if msg := proto.GetAck(); msg != nil {
		defer peerConn.Close()
		t.HandleAckReceived(msg.StateURI, msg.TxID, peerConn)

	} else if msg := proto.GetAnnounceP2PStateURI(); msg != nil {
		defer peerConn.Close()
		t.HandleP2PStateURIReceived(msg.StateURI, peerConn)

	} else {
		t.Errorf("while reading incoming tree stream: got unknown message")
	}
}
