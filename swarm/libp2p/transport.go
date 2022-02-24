package libp2p

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"time"

	cid "github.com/ipfs/go-cid"
	badgerds "github.com/ipfs/go-ds-badger2"
	dohp2p "github.com/libp2p/go-doh-resolver"
	libp2p "github.com/libp2p/go-libp2p"
	circuitp2p "github.com/libp2p/go-libp2p-circuit"
	cryptop2p "github.com/libp2p/go-libp2p-core/crypto"
	corehost "github.com/libp2p/go-libp2p-core/host"
	metrics "github.com/libp2p/go-libp2p-core/metrics"
	netp2p "github.com/libp2p/go-libp2p-core/network"
	corepeer "github.com/libp2p/go-libp2p-core/peer"
	discovery "github.com/libp2p/go-libp2p-discovery"
	p2phost "github.com/libp2p/go-libp2p-host"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	kbucket "github.com/libp2p/go-libp2p-kbucket"
	mplex "github.com/libp2p/go-libp2p-mplex"
	noisep2p "github.com/libp2p/go-libp2p-noise"
	peer "github.com/libp2p/go-libp2p-peer"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/libp2p/go-libp2p-peerstore/pstoremem"
	protocol "github.com/libp2p/go-libp2p-protocol"
	routing "github.com/libp2p/go-libp2p-routing"
	mdns "github.com/libp2p/go-libp2p/p2p/discovery"
	relayp2p "github.com/libp2p/go-libp2p/p2p/host/relay"
	rhost "github.com/libp2p/go-libp2p/p2p/host/routed"
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
	"redwood.dev/utils"
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
	Peers() []corepeer.AddrInfo
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

	libp2pHost p2phost.Host
	dht        *dht.IpfsDHT
	// dht               *dual.DHT
	mdns              mdns.Service
	peerID            peer.ID
	port              uint
	p2pKey            cryptop2p.PrivKey
	reachableAt       string
	bootstrapPeers    PeerSet
	datastorePath     string
	dohDNSResolverURL string
	*metrics.BandwidthCounter

	store         Store
	controllerHub tree.ControllerHub
	peerStore     swarm.PeerStore
	keyStore      identity.KeyStore
	blobStore     blob.Store

	writeSubsByPeerID   map[peer.ID]map[netp2p.Stream]*writableSubscription
	writeSubsByPeerIDMu sync.Mutex

	connectToPeersTask *connectToPeersTask
}

var _ Transport = (*transport)(nil)

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
		writeSubsByPeerID: make(map[peer.ID]map[netp2p.Stream]*writableSubscription),
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

	opts := badgerds.DefaultOptions
	opts.Options.EncryptionKey = t.keyStore.LocalSymEncKey().Bytes()
	opts.Options.EncryptionKeyRotationDuration = 24 * time.Hour
	opts.Options.IndexCacheSize = 100 << 20 // @@TODO: make configurable
	opts.Options.KeepL0InMemory = true      // @@TODO: make configurable
	opts.GcDiscardRatio = 0.5
	opts.GcInterval = 5 * time.Minute
	datastore, err := badgerds.NewDatastore(t.datastorePath, &opts)
	if err != nil {
		return err
	}

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

	relayp2p.AdvertiseBootDelay = 10 * time.Second

	// Initialize the libp2p host
	libp2pHost, err := libp2p.New(t.Process.Ctx(),
		libp2p.ListenAddrStrings(
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%v", t.port),
			fmt.Sprintf("/ip6/::/tcp/%v", t.port),
		),
		libp2p.Identity(t.p2pKey),
		libp2p.BandwidthReporter(t.BandwidthCounter),
		libp2p.NATPortMap(),
		libp2p.EnableNATService(),
		// libp2p.ForceReachabilityPrivate(),
		libp2p.StaticRelays(t.store.StaticRelays().Slice()),
		libp2p.EnableRelay(circuitp2p.OptActive, circuitp2p.OptHop),
		libp2p.EnableAutoRelay(),
		libp2p.Peerstore(pstoremem.NewPeerstore()),
		libp2p.Security(noisep2p.ID, noisep2p.New),
		libp2p.MultiaddrResolver(dnsResolver),
		libp2p.Muxer("/mplex/6.7.0", mplex.DefaultTransport),
		libp2p.Routing(func(host corehost.Host) (routing.PeerRouting, error) {
			t.dht, err = dht.New(t.Process.Ctx(), host,
				dht.BootstrapPeers(append(t.store.StaticRelays().Slice(), t.bootstrapPeers.Slice()...)...),
				dht.Mode(dht.ModeServer),
				dht.Datastore(datastore),
				dht.MaxRecordAge(dhtTTL),
				// dht.RoutingTableFilter(dht.PublicRoutingTableFilter),
			)
			return t.dht, err
		}),
	)
	if err != nil {
		return errors.Wrap(err, "could not initialize libp2p host")
	}

	t.libp2pHost = rhost.Wrap(libp2pHost, t.dht)

	t.libp2pHost = libp2pHost
	t.libp2pHost.SetStreamHandler(PROTO_MAIN, t.handleIncomingStream)
	t.libp2pHost.SetStreamHandler(PROTO_BLOB, t.handleBlobStream)
	t.libp2pHost.SetStreamHandler(PROTO_HUSH, t.handleHushStream)
	t.libp2pHost.Network().Notify(t) // Register for libp2p connect/disconnect notifications

	err = t.dht.Bootstrap(t.Process.Ctx())
	if err != nil {
		return errors.Wrap(err, "could not bootstrap DHT")
	}

	for _, addrInfo := range t.store.StaticRelays().Slice() {
		addrInfo := addrInfo
		go func() {
			err = t.libp2pHost.Connect(t.Process.Ctx(), addrInfo)
			if err != nil {
				t.Errorf("couldn't connect to static relay: %v", err)
				return
			}
			t.Successf("connected to static relay %v", addrInfo)
		}()
	}

	// Set up DHT discovery
	routingDiscovery := discovery.NewRoutingDiscovery(t.dht)
	discovery.Advertise(t.Process.Ctx(), routingDiscovery, "redwood")

	t.Process.Go(nil, "find peers", func(ctx context.Context) {
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
					t.Errorf("error finding peers: %v", err)
					return
				}
				for addrInfo := range chPeers {
					t.onPeerFound("routing", addrInfo)
				}
				time.Sleep(3 * time.Second)
			}()
		}
	})

	// Set up mDNS discovery
	t.mdns, err = mdns.NewMdnsService(t.Process.Ctx(), libp2pHost, 10*time.Second, "redwood")
	if err != nil {
		return err
	}
	t.mdns.RegisterNotifee(t)

	// Update our node's info in the peer store
	myDialAddrs := types.NewStringSet(nil)
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

	if t.controllerHub != nil {
		announceStateURIsTask := NewAnnounceStateURIsTask(30*time.Second, t.controllerHub, t.dht)
		announceStateURIsTask.Enqueue()
		t.Process.SpawnChild(nil, announceStateURIsTask)
		t.controllerHub.OnNewStateURIWithData(func(stateURI string) {
			announceStateURIsTask.ForceRerun()
		})
	}

	if t.blobStore != nil {
		announceBlobsTask := NewAnnounceBlobsTask(30*time.Second, t, t.blobStore, t.dht)
		announceBlobsTask.Enqueue()
		t.Process.SpawnChild(nil, announceBlobsTask)
		t.blobStore.OnBlobsSaved(announceBlobsTask.Enqueue)
	}

	announcePeersTask := NewAnnouncePeersTask(10*time.Second, t)
	err = t.Process.SpawnChild(nil, announcePeersTask)
	if err != nil {
		return err
	}
	announcePeersTask.Enqueue()

	t.connectToPeersTask = NewConnectToPeersTask(8*time.Second, t)
	t.connectToPeersTask.Enqueue()
	t.Process.SpawnChild(nil, t.connectToPeersTask)

	t.Infof(0, "libp2p peer ID is %v", t.Libp2pPeerID())

	return nil
}

func (t *transport) Close() error {
	t.Infof(0, "libp2p transport shutting down")

	err := t.mdns.Close()
	if err != nil {
		t.Errorf("error closing libp2p mDNS service: %v", err)
	}
	err = t.dht.Close()
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

func (t *transport) Peers() []corepeer.AddrInfo {
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
	t.connectToPeersTask.ForceRerun()
	return nil
}

func (t *transport) RemoveStaticRelay(relayAddr string) error {
	return t.store.RemoveStaticRelay(relayAddr)
}

func (t *transport) Listen(network netp2p.Network, multiaddr ma.Multiaddr)      {}
func (t *transport) ListenClose(network netp2p.Network, multiaddr ma.Multiaddr) {}

func (t *transport) Connected(network netp2p.Network, conn netp2p.Conn) {
	p2pComponent, err := ma.NewMultiaddr("/p2p/" + conn.RemotePeer().Pretty())
	if err != nil {
		return
	}
	addrInfo, err := corepeer.AddrInfoFromP2pAddr(conn.RemoteMultiaddr().Encapsulate(p2pComponent))
	if err != nil {
		return
	}

	t.Debugf("libp2p connected: %v", addrInfo.String())
	t.onPeerFound("incoming", *addrInfo)
}

func (t *transport) Disconnected(network netp2p.Network, conn netp2p.Conn) {
	addr := conn.RemoteMultiaddr().String() + "/p2p/" + conn.RemotePeer().Pretty()
	t.Debugf("libp2p disconnected: %v", addr)
}

func (t *transport) OpenedStream(network netp2p.Network, stream netp2p.Stream) {}

func (t *transport) ClosedStream(network netp2p.Network, stream netp2p.Stream) {
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

// HandlePeerFound is the libp2p mDNS peer discovery callback
func (t *transport) HandlePeerFound(pinfo corepeer.AddrInfo) {
	t.onPeerFound("mDNS", pinfo)
}

func (t *transport) onPeerFound(via string, pinfo corepeer.AddrInfo) {
	if pinfo.ID == peer.ID("") { // Bad data, ignore
		return
	} else if pinfo.ID == t.libp2pHost.ID() { // Self, ignore
		return
	} else if t.store.StaticRelays().ContainsPeerID(pinfo.ID) {
		return
	}

	// Add to peer store (unless it's a static relay or local/private IP)
	// Once libp2p peers are added to the peer store, we automatically attempt
	// to connect to them (see `connectToPeersTask`)
	for _, multiaddr := range multiaddrsFromPeerInfo(pinfo) {
		// if manet.IsPrivateAddr(multiaddr) {
		// 	continue
		// }

		dialInfo := swarm.PeerDialInfo{TransportName: TransportName, DialAddr: multiaddr.String()}
		if !t.peerStore.IsKnownPeer(dialInfo) {
			t.Infof(0, "%v: peer (%v) %+v found", via, pinfo.ID, multiaddr)
		}
		t.peerStore.AddDialInfo(dialInfo, deviceUniqueID(pinfo.ID))

		if !multiaddrIsRelayed(multiaddr) {
			for _, relayAddrInfo := range t.store.StaticRelays().Slice() {
				multiaddr, err := ma.NewMultiaddr("/p2p/" + relayAddrInfo.ID.String() + "/p2p-circuit/p2p/" + pinfo.ID.String())
				if err != nil {
					continue
				}
				dialInfo := swarm.PeerDialInfo{TransportName: TransportName, DialAddr: multiaddr.String()}
				t.peerStore.AddDialInfo(dialInfo, deviceUniqueID(pinfo.ID))
			}
		}
	}
}

func (t *transport) handleBlobStream(stream netp2p.Stream) {
	peerConn := t.makeConnectedPeerConn(stream)
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

func (t *transport) handleHushStream(stream netp2p.Stream) {
	peerConn := t.makeConnectedPeerConn(stream)
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

func (t *transport) handleIncomingStream(stream netp2p.Stream) {
	msg, err := readMsg(stream)
	if err != nil {
		t.Errorf("incoming stream error: %v", err)
		stream.Close()
		return
	}

	peer := t.makeConnectedPeerConn(stream)

	switch msg.Type {
	case msgType_Subscribe:
		stateURI, ok := msg.Payload.(string)
		if !ok {
			t.Errorf("Subscribe message: bad payload: (%T) %v", msg.Payload, msg.Payload)
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
			Addresses:        types.NewAddressSet(peer.Addresses()),
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
				t.writeSubsByPeerID[peer.pinfo.ID] = make(map[netp2p.Stream]*writableSubscription)
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
				t.onPeerFound("announce", addrInfo)
			} else {
				t.peerStore.AddDialInfo(dialInfo, "")
			}
		}

	default:
		panic("protocol error")
	}
}

func (t *transport) NewPeerConn(ctx context.Context, dialAddr string) (swarm.PeerConn, error) {
	addr, err := ma.NewMultiaddr(dialAddr)
	if err != nil {
		return nil, errors.Wrapf(err, "could not parse multiaddr '%v'", dialAddr)
	}
	pinfo, err := peerstore.InfoFromP2pAddr(addr)
	if err != nil {
		return nil, errors.Wrapf(err, "could not parse PeerInfo from multiaddr '%v'", dialAddr)
	} else if pinfo.ID == t.peerID {
		return nil, errors.WithStack(swarm.ErrPeerIsSelf)
	}
	return t.makeDisconnectedPeerConn(*pinfo)
}

func (t *transport) ProvidersOfStateURI(ctx context.Context, stateURI string) (<-chan prototree.TreePeerConn, error) {
	urlCid, err := cidForString("serve:" + stateURI)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ch := make(chan prototree.TreePeerConn)
	t.Process.Go(ctx, "ProvidersOfStateURI "+stateURI, func(ctx context.Context) {
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			for pinfo := range t.dht.FindProvidersAsync(ctx, urlCid, 8) {
				if pinfo.ID == t.libp2pHost.ID() {
					select {
					case <-ctx.Done():
						return
					default:
						continue
					}
				}

				// @@TODO: validate peer as an authorized provider via web of trust, certificate authority,
				// whitelist, etc.

				peer, err := t.makeDisconnectedPeerConn(pinfo)
				if err != nil {
					t.Errorf("while making disconnected peer conn (%v): %v", peer.DialInfo().DialAddr, err)
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

func (t *transport) ProvidersOfBlob(ctx context.Context, refID blob.ID) (<-chan protoblob.BlobPeerConn, error) {
	refCid, err := cidForString("blob:" + refID.String())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ch := make(chan protoblob.BlobPeerConn)
	go func() {
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			for pinfo := range t.dht.FindProvidersAsync(ctx, refCid, 8) {
				t.Debugf("PROVIDER %v", pinfo.ID.Pretty())
				if pinfo.ID == t.libp2pHost.ID() {
					continue
				}

				peer, err := t.makeDisconnectedPeerConn(pinfo)
				if err != nil {
					t.Errorf("while making disconnected peer conn (%v): %v", pinfo.ID.Pretty(), err)
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
	}()
	return ch, nil
}

func (t *transport) AnnounceBlob(ctx context.Context, refID blob.ID) error {
	c, err := cidForString("blob:" + refID.String())
	if err != nil {
		return err
	}

	err = t.dht.Provide(ctx, c, true)
	if err != nil && err != kbucket.ErrLookupFailure && errors.Cause(err) != context.DeadlineExceeded && errors.Cause(err) != context.Canceled {
		t.Errorf(`announce: could not dht.Provide ref "%v": %v`, refID.String(), err)
		return err
	}
	return nil
}

func (t *transport) makeConnectedPeerConn(stream netp2p.Stream) *peerConn {
	pinfo := t.libp2pHost.Peerstore().PeerInfo(stream.Conn().RemotePeer())
	peer := &peerConn{t: t, pinfo: pinfo, stream: stream}

	duID := deviceUniqueID(pinfo.ID)
	maddrs := multiaddrsFromPeerInfo(pinfo)

	peer.PeerEndpoint = t.peerStore.AddDialInfo(swarm.PeerDialInfo{TransportName, maddrs[0].String()}, duID)
	for _, addr := range maddrs[1:] {
		t.peerStore.AddDialInfo(swarm.PeerDialInfo{TransportName, addr.String()}, duID)
	}
	return peer
}

func (t *transport) makeDisconnectedPeerConn(pinfo corepeer.AddrInfo) (*peerConn, error) {
	peer := &peerConn{t: t, pinfo: pinfo, stream: nil}

	multiaddrs := multiaddrsFromPeerInfo(pinfo)

	var filtered []ma.Multiaddr
	for _, multiaddr := range multiaddrs {
		// if manet.IsPrivateAddr(multiaddr) {
		// 	continue
		// }
		filtered = append(filtered, multiaddr)
	}
	if len(multiaddrs) == 0 {
		return nil, swarm.ErrUnreachable
	}

	duID := deviceUniqueID(pinfo.ID)

	peer.PeerEndpoint = t.peerStore.AddDialInfo(swarm.PeerDialInfo{TransportName: TransportName, DialAddr: multiaddrs[0].String()}, duID)
	for _, addr := range multiaddrs[1:] {
		t.peerStore.AddDialInfo(swarm.PeerDialInfo{TransportName: TransportName, DialAddr: addr.String()}, duID)
	}
	return peer, nil
}

type DHT interface {
	Provide(context.Context, cid.Cid, bool) error
}

type announceBlobsTask struct {
	process.PeriodicTask
	log.Logger
	transport protoblob.BlobTransport
	blobStore blob.Store
	dht       DHT
	interval  time.Duration
}

func NewAnnounceBlobsTask(
	interval time.Duration,
	transport protoblob.BlobTransport,
	blobStore blob.Store,
	dht DHT,
) *announceBlobsTask {
	t := &announceBlobsTask{
		Logger:    log.NewLogger("libp2p"),
		transport: transport,
		blobStore: blobStore,
		dht:       dht,
		interval:  interval,
	}
	t.PeriodicTask = *process.NewPeriodicTask("AnnounceBlobsTask", utils.NewStaticTicker(interval), t.announceBlobs)
	return t
}

// Periodically announces our objects to the network.
func (t *announceBlobsTask) announceBlobs(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, t.interval)

	child := t.Process.NewChild(ctx, "announce")
	defer child.AutocloseWithCleanup(cancel)

	sha1s, sha3s, err := t.blobStore.BlobIDs()
	if err != nil {
		t.Errorf("while fetching blob IDs: %v", err)
		// Even if we receive an error, there might still be blob
		// IDs, so we don't want to return here.
	}

	for _, sha1 := range sha1s {
		t.Process.Go(ctx, sha1.String(), func(ctx context.Context) {
			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			err := t.transport.AnnounceBlob(ctx, sha1)
			if err != nil && errors.Cause(err) != context.DeadlineExceeded && errors.Cause(err) != context.Canceled {
				t.Errorf("announce: error: %v", err)
			}
		})
	}

	for _, sha3 := range sha3s {
		t.Debugf("ANNOUNCE %v", sha3)
		t.Process.Go(ctx, sha3.String(), func(ctx context.Context) {
			err := t.transport.AnnounceBlob(ctx, sha3)
			if err != nil {
				t.Debugf("ANNOUNCE ERR %v", err)
			}
			if err != nil && errors.Cause(err) != context.DeadlineExceeded && errors.Cause(err) != context.Canceled {
				t.Errorf("announce: error: %v", err)
			}
		})
	}
}

type announceStateURIsTask struct {
	process.PeriodicTask
	log.Logger
	controllerHub tree.ControllerHub
	dht           DHT
	interval      time.Duration
}

func NewAnnounceStateURIsTask(
	interval time.Duration,
	controllerHub tree.ControllerHub,
	dht DHT,
) *announceStateURIsTask {
	t := &announceStateURIsTask{
		Logger:        log.NewLogger("libp2p"),
		controllerHub: controllerHub,
		dht:           dht,
		interval:      interval,
	}
	t.PeriodicTask = *process.NewPeriodicTask("AnnounceStateURIsTask", utils.NewStaticTicker(interval), t.announceStateURIs)
	return t
}

func (t *announceStateURIsTask) announceStateURIs(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, t.interval)

	child := t.Process.NewChild(ctx, "announce")
	defer child.AutocloseWithCleanup(cancel)

	stateURIs, err := t.controllerHub.StateURIsWithData()
	if err != nil {
		t.Errorf("error fetching known state URIs from DB: %v", err)
		return
	}

	for stateURI := range stateURIs {
		stateURI := stateURI

		child.Go(ctx, stateURI, func(ctx context.Context) {
			c, err := cidForString("serve:" + stateURI)
			if err != nil {
				t.Errorf("announce: error creating cid: %v", err)
				return
			}

			err = t.dht.Provide(ctx, c, true)
			if err != nil && err != kbucket.ErrLookupFailure && errors.Cause(err) != context.DeadlineExceeded && errors.Cause(err) != context.Canceled {
				t.Errorf(`announce: could not dht.Provide stateURI "%v": %v`, stateURI, err)
				return
			}
		})
	}
}

type connectToPeersTask struct {
	process.PeriodicTask
	log.Logger
	tpt      *transport
	interval time.Duration
}

func NewConnectToPeersTask(
	interval time.Duration,
	tpt *transport,
) *connectToPeersTask {
	t := &connectToPeersTask{
		Logger:   log.NewLogger("libp2p"),
		tpt:      tpt,
		interval: interval,
	}
	t.PeriodicTask = *process.NewPeriodicTask("ConnectToPeers", utils.NewStaticTicker(interval), t.connectToPeers)
	return t
}

func (t *connectToPeersTask) connectToPeers(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, t.interval)

	child := t.Process.NewChild(ctx, "connect to peers")
	defer child.AutocloseWithCleanup(cancel)

	for _, addrInfo := range t.tpt.StaticRelays().Slice() {
		addrInfo := addrInfo
		child.Go(ctx, "connect to static relay "+addrInfo.String(), func(ctx context.Context) {
			if len(t.tpt.libp2pHost.Network().ConnsToPeer(addrInfo.ID)) > 0 {
				return
			}
			err := t.tpt.libp2pHost.Connect(ctx, addrInfo)
			if err != nil {
				return
			}
			t.tpt.Successf("connected to static relay %v", addrInfo)
		})
	}

	for _, endpoint := range t.tpt.peerStore.PeersFromTransport(TransportName) {
		peerID, err := corepeer.Decode(endpoint.DeviceUniqueID())
		if err != nil {
			continue
		} else if peerID == t.tpt.libp2pHost.ID() {
			continue
		}

		if len(t.tpt.libp2pHost.Network().ConnsToPeer(peerID)) > 0 {
			continue
		}

		addrInfo := t.tpt.libp2pHost.Peerstore().PeerInfo(peerID)
		if len(addrInfo.Addrs) == 0 {
			continue
		}

		peerConn, err := t.tpt.makeDisconnectedPeerConn(addrInfo)
		if err != nil {
			t.Errorf("while making disconnected peer conn (%v): %v", addrInfo.ID.Pretty(), err)
			continue
		}

		if !peerConn.Ready() || !peerConn.Dialable() {
			continue
		}

		child.Go(ctx, "ensure connected to "+string(peerID), func(ctx context.Context) {
			err := peerConn.EnsureConnected(ctx)
			if err != nil {
				return
			}
			t.tpt.Successf("connected to libp2p peer %v", addrInfo)
		})

	}
}

type announcePeersTask struct {
	process.PeriodicTask
	log.Logger
	tpt      *transport
	interval time.Duration
}

func NewAnnouncePeersTask(
	interval time.Duration,
	tpt *transport,
) *announcePeersTask {
	t := &announcePeersTask{
		Logger:   log.NewLogger(TransportName),
		tpt:      tpt,
		interval: interval,
	}
	t.PeriodicTask = *process.NewPeriodicTask("AnnouncePeersTask", utils.NewStaticTicker(interval), t.announcePeers)
	return t
}

func (t *announcePeersTask) announcePeers(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, t.interval)

	child := t.Process.NewChild(ctx, "announce peers")
	defer child.AutocloseWithCleanup(cancel)

	var allDialInfos []swarm.PeerDialInfo
	for dialInfo := range t.tpt.peerStore.AllDialInfos() {
		if dialInfo.TransportName == TransportName {
			addrInfo, err := addrInfoFromString(dialInfo.DialAddr)
			if err != nil {
				continue
			}
			// Cull static relays from the peer store
			if t.tpt.StaticRelays().ContainsPeerID(addrInfo.ID) {
				_ = t.tpt.peerStore.RemovePeers([]string{string(addrInfo.ID)})
				continue
			}
		} else {
			allDialInfos = append(allDialInfos, dialInfo)
		}
	}

	for _, peerDevice := range t.tpt.peerStore.PeersFromTransport(t.tpt.Name()) {
		if !peerDevice.Ready() || !peerDevice.Dialable() {
			continue
		} else if peerDevice.DialInfo().TransportName != t.tpt.Name() {
			continue
		}

		peerDevice := peerDevice

		child.Go(ctx, "announce peers", func(ctx context.Context) {
			swarmPeerConn, err := t.tpt.NewPeerConn(ctx, peerDevice.DialInfo().DialAddr)
			if errors.Cause(err) == swarm.ErrPeerIsSelf {
				return
			} else if err != nil {
				t.Warnf("error creating new %v peerConn: %v", t.tpt.Name(), err)
				return
			}
			defer swarmPeerConn.Close()

			libp2pPeerConn := swarmPeerConn.(*peerConn)

			err = libp2pPeerConn.EnsureConnected(ctx)
			if errors.Cause(err) == errors.ErrConnection {
				return
			} else if err != nil {
				t.Warnf("error connecting to %v peer (%v): %v", t.tpt.Name(), peerDevice.DialInfo().DialAddr, err)
				return
			}

			err = libp2pPeerConn.AnnouncePeers(ctx, allDialInfos)
			if err != nil {
				// t.Errorf("error writing to peerConn: %+v", err)
			}
		})
	}
}
