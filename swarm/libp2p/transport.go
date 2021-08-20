package libp2p

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"

	cid "github.com/ipfs/go-cid"
	badgerds "github.com/ipfs/go-ds-badger2"
	dohp2p "github.com/libp2p/go-doh-resolver"
	libp2p "github.com/libp2p/go-libp2p"
	cryptop2p "github.com/libp2p/go-libp2p-core/crypto"
	corehost "github.com/libp2p/go-libp2p-core/host"
	metrics "github.com/libp2p/go-libp2p-core/metrics"
	netp2p "github.com/libp2p/go-libp2p-core/network"
	corepeer "github.com/libp2p/go-libp2p-core/peer"
	discovery "github.com/libp2p/go-libp2p-discovery"
	p2phost "github.com/libp2p/go-libp2p-host"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	kbucket "github.com/libp2p/go-libp2p-kbucket"
	noisep2p "github.com/libp2p/go-libp2p-noise"
	peer "github.com/libp2p/go-libp2p-peer"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/libp2p/go-libp2p-peerstore/pstoreds"
	protocol "github.com/libp2p/go-libp2p-protocol"
	routing "github.com/libp2p/go-libp2p-routing"
	mdns "github.com/libp2p/go-libp2p/p2p/discovery"
	ma "github.com/multiformats/go-multiaddr"
	madns "github.com/multiformats/go-multiaddr-dns"
	"github.com/pkg/errors"

	"redwood.dev/blob"
	"redwood.dev/crypto"
	"redwood.dev/identity"
	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/swarm"
	"redwood.dev/swarm/protoauth"
	"redwood.dev/swarm/protoblob"
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
	prototree.TreeTransport
	Libp2pPeerID() string
	ListenAddrs() []string
	Peers() []corepeer.AddrInfo
}

type transport struct {
	process.Process
	log.Logger
	protoauth.BaseAuthTransport
	protoblob.BaseBlobTransport
	prototree.BaseTreeTransport

	libp2pHost        p2phost.Host
	dht               *dht.IpfsDHT
	mdns              mdns.Service
	peerID            peer.ID
	port              uint
	p2pKey            cryptop2p.PrivKey
	reachableAt       string
	bootstrapPeers    []string
	staticRelays      []string
	datastorePath     string
	dohDNSResolverURL string
	*metrics.BandwidthCounter

	address types.Address

	controllerHub tree.ControllerHub
	peerStore     swarm.PeerStore
	keyStore      identity.KeyStore
	blobStore     blob.Store

	writeSubsByPeerID   map[peer.ID]map[netp2p.Stream]prototree.WritableSubscriptionImpl
	writeSubsByPeerIDMu sync.Mutex
}

var _ Transport = (*transport)(nil)

const (
	PROTO_MAIN    protocol.ID = "/redwood/main/1.0.0"
	TransportName string      = "libp2p"
)

func NewTransport(
	port uint,
	reachableAt string,
	bootstrapPeers []string,
	staticRelays []string,
	datastorePath string,
	dohDNSResolverURL string,
	controllerHub tree.ControllerHub,
	keyStore identity.KeyStore,
	blobStore blob.Store,
	peerStore swarm.PeerStore,
) (*transport, error) {
	t := &transport{
		Process:           *process.New(TransportName),
		Logger:            log.NewLogger(TransportName),
		port:              port,
		reachableAt:       reachableAt,
		bootstrapPeers:    bootstrapPeers,
		staticRelays:      staticRelays,
		datastorePath:     datastorePath,
		dohDNSResolverURL: dohDNSResolverURL,
		controllerHub:     controllerHub,
		keyStore:          keyStore,
		blobStore:         blobStore,
		peerStore:         peerStore,
		writeSubsByPeerID: make(map[peer.ID]map[netp2p.Stream]prototree.WritableSubscriptionImpl),
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

	staticRelays, err := addrInfosFromStrings(t.staticRelays)
	if err != nil {
		t.Warnf("while decoding static relays: %v", err)
	}

	datastore, err := badgerds.NewDatastore(t.datastorePath, nil)
	if err != nil {
		return err
	}

	peerStore, err := pstoreds.NewPeerstore(t.Process.Ctx(), datastore, pstoreds.DefaultOpts())
	if err != nil {
		return err
	}

	dnsResolver, err := madns.NewResolver(madns.WithDefaultResolver(dohp2p.NewResolver(t.dohDNSResolverURL)))
	if err != nil {
		return err
	}

	// bootstrapPeers := dht.GetDefaultBootstrapPeerAddrInfos() // @@TODO: remove this
	bootstrapPeers, err := addrInfosFromStrings(t.bootstrapPeers)
	if err != nil {
		t.Warnf("while decoding bootstrap peers: %v", err)
	}

	// Initialize the libp2p host
	libp2pHost, err := libp2p.New(t.Process.Ctx(),
		libp2p.ListenAddrStrings(
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%v", t.port),
		),
		libp2p.Identity(t.p2pKey),
		libp2p.BandwidthReporter(t.BandwidthCounter),
		libp2p.NATPortMap(),
		libp2p.EnableNATService(),
		libp2p.EnableAutoRelay(),
		libp2p.StaticRelays(staticRelays),
		// libp2p.DefaultStaticRelays(),
		libp2p.Peerstore(peerStore),
		libp2p.Security(noisep2p.ID, noisep2p.New),
		libp2p.MultiaddrResolver(dnsResolver),
		libp2p.Routing(func(host corehost.Host) (routing.PeerRouting, error) {
			t.dht, err = dht.New(t.Process.Ctx(), host,
				dht.BootstrapPeers(bootstrapPeers...),
				dht.Mode(dht.ModeServer),
				dht.Datastore(datastore),
				// dht.Validator(blankValidator{}), // Set a pass-through validator
			)
			if err != nil {
				return nil, err
			}
			return t.dht, err
		}),
	)
	if err != nil {
		return errors.Wrap(err, "could not initialize libp2p host")
	}
	t.libp2pHost = libp2pHost
	t.libp2pHost.SetStreamHandler(PROTO_MAIN, t.handleIncomingStream)
	t.libp2pHost.Network().Notify(t) // Register for libp2p connect/disconnect notifications

	err = t.dht.Bootstrap(t.Process.Ctx())
	if err != nil {
		return errors.Wrap(err, "could not bootstrap DHT")
	}

	for _, relay := range staticRelays {
		relay := relay
		go func() {
			peerConn := t.makeDisconnectedPeerConn(relay)
			peerConn.EnsureConnected(t.Process.Ctx())

			err := t.libp2pHost.Connect(t.Process.Ctx(), relay)
			if err != nil {
				t.Errorf("couldn't connect to static relay: %v", err)
			}
		}()
	}

	// Set up DHT discovery
	routingDiscovery := discovery.NewRoutingDiscovery(t.dht)
	discovery.Advertise(t.Process.Ctx(), routingDiscovery, "redwood")

	t.Process.Go("find peers", func(ctx context.Context) {
		chPeers, err := routingDiscovery.FindPeers(ctx, "redwood")
		if err != nil {
			t.Errorf("error finding peers: %v", err)
		}
		for pinfo := range chPeers {
			t.onPeerFound("routing", pinfo)
		}
	})

	// Set up mDNS discovery
	t.mdns, err = mdns.NewMdnsService(t.Process.Ctx(), libp2pHost, 10*time.Second, "redwood")
	if err != nil {
		return err
	}
	t.mdns.RegisterNotifee(t)

	// Update our node's info in the peer store
	myDialAddrs := utils.NewStringSet(nil)
	for _, addr := range t.libp2pHost.Addrs() {
		addrStr := addr.String()
		myDialAddrs.Add(addrStr)
	}
	if t.reachableAt != "" {
		myDialAddrs.Add(t.reachableAt)
	}
	myDialAddrs = cleanLibp2pAddrs(myDialAddrs, t.peerID)

	if len(myDialAddrs) > 0 {
		for dialAddr := range myDialAddrs {
			identities, err := t.keyStore.Identities()
			if err != nil {
				return err
			}
			for _, identity := range identities {
				t.peerStore.AddVerifiedCredentials(
					swarm.PeerDialInfo{TransportName: TransportName, DialAddr: dialAddr},
					identity.SigKeypair.SigningPublicKey.Address(),
					identity.SigKeypair.SigningPublicKey,
					identity.AsymEncKeypair.AsymEncPubkey,
				)
			}
		}
	}

	announceContentTask := NewAnnounceContentTask(10*time.Second, t, t.controllerHub, t.keyStore, t.peerStore, t.blobStore, t.dht)
	t.Process.SpawnChild(nil, announceContentTask)
	if t.blobStore != nil {
		t.blobStore.OnBlobsSaved(announceContentTask.Enqueue)
	}

	t.Infof(0, "libp2p peer ID is %v", t.Libp2pPeerID())

	return nil
}

func (t *transport) Close() error {
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

func (t *transport) Listen(network netp2p.Network, multiaddr ma.Multiaddr)      {}
func (t *transport) ListenClose(network netp2p.Network, multiaddr ma.Multiaddr) {}

func (t *transport) Connected(network netp2p.Network, conn netp2p.Conn) {
	t.addPeerInfosToPeerStore(t.Peers())

	addr := conn.RemoteMultiaddr().String() + "/p2p/" + conn.RemotePeer().Pretty()
	t.Debugf("libp2p connected: %v", addr)
	t.peerStore.AddDialInfos([]swarm.PeerDialInfo{{TransportName: TransportName, DialAddr: addr}})
}

func (t *transport) Disconnected(network netp2p.Network, conn netp2p.Conn) {
	t.addPeerInfosToPeerStore(t.Peers())

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
			t.Debugf("closing libp2p writable subscription (peer: %v, stateURI: %v)", peerID.Pretty(), sub.StateURI())
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
	// Ensure all peers discovered on the libp2p layer are in the peer store
	if pinfo.ID == peer.ID("") {
		return
	}
	// dialInfos := peerDialInfosFromPeerInfo(pinfo)
	// t.peerStore.AddDialInfos(dialInfos)

	var i uint
	for _, dialInfo := range peerDialInfosFromPeerInfo(pinfo) {
		if !t.peerStore.IsKnownPeer(dialInfo) {
			i++
		}
	}

	if i > 0 {
		t.Infof(0, "mDNS: peer %+v found", pinfo.ID.Pretty())
	}

	t.addPeerInfosToPeerStore([]corepeer.AddrInfo{pinfo})

	peer := t.makeDisconnectedPeerConn(pinfo)
	if peer == nil {
		t.Infof(0, "mDNS: peer %+v is nil", pinfo.ID.Pretty())
		return
	}

	ctx, cancel := utils.CombinedContext(t.Process.Ctx(), 10*time.Second)
	defer cancel()

	err := peer.EnsureConnected(ctx)
	if err != nil {
		t.Errorf("error connecting to mDNS peer %v: %v", pinfo, err)
	}
	// if len(dialInfos) > 0 {
	//  err := t.libp2pHost.Connect(ctx, pinfo)
	//  if err != nil {
	//      t.Errorf("error connecting to peer %v: %v", pinfo.ID.Pretty(), err)
	//  }
	// }
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

		writeSub := newWritableSubscription(peer, stateURI)
		func() {
			t.writeSubsByPeerIDMu.Lock()
			defer t.writeSubsByPeerIDMu.Unlock()
			if _, exists := t.writeSubsByPeerID[peer.pinfo.ID]; !exists {
				t.writeSubsByPeerID[peer.pinfo.ID] = make(map[netp2p.Stream]prototree.WritableSubscriptionImpl)
			}
			t.writeSubsByPeerID[peer.pinfo.ID][stream] = writeSub
		}()

		fetchHistoryOpts := &prototree.FetchHistoryOpts{} // Fetch all history (@@TODO)
		t.HandleWritableSubscriptionOpened(stateURI, nil, prototree.SubscriptionType_Txs, writeSub, fetchHistoryOpts)

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

	case msgType_FetchBlob:
		defer peer.Close()

		refID, ok := msg.Payload.(blob.ID)
		if !ok {
			t.Errorf("FetchBlob message: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}
		t.HandleBlobRequest(refID, peer)

	case msgType_EncryptedTx:
		defer peer.Close()

		encryptedTx, ok := msg.Payload.(prototree.EncryptedTx)
		if !ok {
			t.Errorf("Private message: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}
		bs, err := t.keyStore.OpenMessageFrom(
			encryptedTx.RecipientAddress,
			crypto.AsymEncPubkeyFromBytes(encryptedTx.SenderPublicKey),
			encryptedTx.EncryptedPayload,
		)
		if err != nil {
			t.Errorf("error decrypting tx: %v", err)
			return
		}

		var tx tree.Tx
		err = json.Unmarshal(bs, &tx)
		if err != nil {
			t.Errorf("error decoding tx: %v", err)
			return
		} else if encryptedTx.TxID != tx.ID {
			t.Errorf("private tx id does not match")
			return
		}
		t.HandleTxReceived(tx, peer)

	case msgType_AnnouncePeers:
		defer peer.Close()

		tuples, ok := msg.Payload.([]swarm.PeerDialInfo)
		if !ok {
			t.Errorf("Announce peers: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}
		t.peerStore.AddDialInfos(tuples)

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
	peer := t.makeDisconnectedPeerConn(*pinfo)
	return peer, nil
}

func (t *transport) ProvidersOfStateURI(ctx context.Context, stateURI string) (<-chan prototree.TreePeerConn, error) {
	urlCid, err := cidForString("serve:" + stateURI)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ctx, cancel := utils.CombinedContext(ctx, t.Process.Ctx())

	ch := make(chan prototree.TreePeerConn)
	go func() {
		defer close(ch)
		defer cancel()
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

				peer := t.makeDisconnectedPeerConn(pinfo)
				if peer == nil {
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
	}()
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
				if pinfo.ID == t.libp2pHost.ID() {
					continue
				}

				// t.Infof(0, `found peer %v for ref "%v"`, pinfo.ID, refID.String())

				peer := t.makeDisconnectedPeerConn(pinfo)
				if peer == nil || peer.DialInfo().DialAddr == "" {
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

func (t *transport) PeersClaimingAddress(ctx context.Context, address types.Address) (<-chan protoauth.AuthPeerConn, error) {
	ctx, cancel := utils.CombinedContext(ctx, t.Process.Ctx())
	defer cancel()

	addrCid, err := cidForString("addr:" + address.String())
	if err != nil {
		t.Errorf("error creating cid: %v", err)
		return nil, err
	}

	ch := make(chan protoauth.AuthPeerConn)

	go func() {
		defer close(ch)

		for pinfo := range t.dht.FindProvidersAsync(ctx, addrCid, 8) {
			if pinfo.ID == t.libp2pHost.ID() {
				continue
			}

			peer := t.makeDisconnectedPeerConn(pinfo)
			if peer == nil || peer.DialInfo().DialAddr == "" {
				continue
			}

			select {
			case <-ctx.Done():
				return
			case ch <- peer:
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
	if err != nil && err != kbucket.ErrLookupFailure {
		t.Errorf(`announce: could not dht.Provide ref "%v": %v`, refID.String(), err)
		return err
	}

	return nil
}

func (t *transport) addPeerInfosToPeerStore(pinfos []corepeer.AddrInfo) {
	for _, pinfo := range pinfos {
		if pinfo.ID == peer.ID("") {
			continue
		}
		dialInfos := peerDialInfosFromPeerInfo(pinfo)
		t.peerStore.AddDialInfos(dialInfos)
	}
}

func (t *transport) makeConnectedPeerConn(stream netp2p.Stream) *peerConn {
	pinfo := t.libp2pHost.Peerstore().PeerInfo(stream.Conn().RemotePeer())
	peer := &peerConn{t: t, pinfo: pinfo, stream: stream}
	dialAddrs := multiaddrsFromPeerInfo(pinfo)

	var dialInfos []swarm.PeerDialInfo
	dialAddrs.ForEach(func(dialAddr string) bool {
		dialInfo := swarm.PeerDialInfo{TransportName: TransportName, DialAddr: dialAddr}
		dialInfos = append(dialInfos, dialInfo)
		return true
	})
	t.peerStore.AddDialInfos(dialInfos)

	for _, dialInfo := range dialInfos {
		peer.PeerDetails = t.peerStore.PeerWithDialInfo(dialInfo)
		if peer.PeerDetails != nil {
			break
		}
	}
	if peer.PeerDetails == nil {
		return nil
	}
	return peer
}

func (t *transport) makeDisconnectedPeerConn(pinfo corepeer.AddrInfo) *peerConn {
	peer := &peerConn{t: t, pinfo: pinfo, stream: nil}
	dialAddrs := multiaddrsFromPeerInfo(pinfo)
	if dialAddrs.Len() == 0 {
		return nil
	}

	var peerDetails swarm.PeerDetails
	dialAddrs.ForEach(func(dialAddr string) bool {
		peerDetails = t.peerStore.PeerWithDialInfo(swarm.PeerDialInfo{TransportName: TransportName, DialAddr: dialAddr})
		if peerDetails != nil {
			return false
		}
		return true
	})
	if reflect.ValueOf(peerDetails).IsNil() {
		var dialInfos []swarm.PeerDialInfo
		dialAddrs.ForEach(func(dialAddr string) bool {
			dialInfos = append(dialInfos, swarm.PeerDialInfo{TransportName: TransportName, DialAddr: dialAddr})
			return true
		})
		t.peerStore.AddDialInfos(dialInfos)
		var peerDetails swarm.PeerDetails
		for _, dialInfo := range dialInfos {
			peerDetails = t.peerStore.PeerWithDialInfo(dialInfo)
			if peerDetails != nil {
				break
			}
		}
		if reflect.ValueOf(peerDetails).IsNil() {
			return nil
		}
		peer.PeerDetails = peerDetails
	} else {
		peer.PeerDetails = peerDetails
	}
	return peer
}

type announceContentTask struct {
	process.PeriodicTask
	log.Logger
	transport     protoblob.BlobTransport
	controllerHub tree.ControllerHub
	keyStore      identity.KeyStore
	peerStore     swarm.PeerStore
	blobStore     blob.Store
	dht           DHT
}

type DHT interface {
	Provide(context.Context, cid.Cid, bool) error
}

func NewAnnounceContentTask(
	interval time.Duration,
	transport protoblob.BlobTransport,
	controllerHub tree.ControllerHub,
	keyStore identity.KeyStore,
	peerStore swarm.PeerStore,
	blobStore blob.Store,
	dht DHT,
) *announceContentTask {
	t := &announceContentTask{
		Logger:        log.NewLogger("libp2p"),
		transport:     transport,
		controllerHub: controllerHub,
		keyStore:      keyStore,
		peerStore:     peerStore,
		blobStore:     blobStore,
		dht:           dht,
	}
	t.PeriodicTask = *process.NewPeriodicTask("AnnounceContentTask", interval, t.announceContent)
	return t
}

// Periodically announces our objects to the network.
func (t *announceContentTask) announceContent(ctx context.Context) {
	// Announce the state URIs we're serving
	stateURIs, err := t.controllerHub.KnownStateURIs()
	if err != nil {
		t.Errorf("error fetching known state URIs from DB: %v", err)

	} else {
		for _, stateURI := range stateURIs {
			stateURI := stateURI

			t.Process.Go("state URIs", func(ctx context.Context) {
				c, err := cidForString("serve:" + stateURI)
				if err != nil {
					t.Errorf("announce: error creating cid: %v", err)
					return
				}

				err = t.dht.Provide(ctx, c, true)
				if err != nil && err != kbucket.ErrLookupFailure {
					t.Errorf(`announce: could not dht.Provide stateURI "%v": %v`, stateURI, err)
					return
				}
			})
		}
	}

	// Announce the blobs we're serving
	refHashes, err := t.blobStore.AllHashes()
	if err != nil {
		t.Errorf("error fetching blobStore hashes: %v", err)
	}
	for _, refHash := range refHashes {
		refHash := refHash

		t.Process.Go("blobs", func(ctx context.Context) {
			err := t.transport.AnnounceBlob(ctx, refHash)
			if err != nil {
				t.Errorf("announce: error: %v", err)
			}
		})
	}

	// Announce our address (for exchanging private txs)
	identities, err := t.keyStore.Identities()
	if err != nil {
		t.Errorf("announce: error creating cid: %v", err)
		return
	}

	for _, identity := range identities {
		identity := identity

		t.Process.Go("identity", func(ctx context.Context) {
			c, err := cidForString("addr:" + identity.Address().String())
			if err != nil {
				t.Errorf("announce: error creating cid: %v", err)
				return
			}

			err = t.dht.Provide(ctx, c, true)
			if err != nil && err != kbucket.ErrLookupFailure {
				t.Errorf(`announce: could not dht.Provide pubkey: %v`, err)
			}
		})
	}
}
