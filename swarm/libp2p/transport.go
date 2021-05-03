package libp2p

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"

	dstore "github.com/ipfs/go-datastore"
	dsync "github.com/ipfs/go-datastore/sync"
	libp2p "github.com/libp2p/go-libp2p"
	cryptop2p "github.com/libp2p/go-libp2p-core/crypto"
	netp2p "github.com/libp2p/go-libp2p-core/network"
	corepeer "github.com/libp2p/go-libp2p-core/peer"
	p2phost "github.com/libp2p/go-libp2p-host"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	kbucket "github.com/libp2p/go-libp2p-kbucket"
	metrics "github.com/libp2p/go-libp2p-metrics"
	peer "github.com/libp2p/go-libp2p-peer"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	protocol "github.com/libp2p/go-libp2p-protocol"
	"github.com/libp2p/go-libp2p/p2p/discovery"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"

	"redwood.dev/blob"
	"redwood.dev/crypto"
	"redwood.dev/identity"
	"redwood.dev/log"
	"redwood.dev/swarm"
	"redwood.dev/tree"
	"redwood.dev/types"
	"redwood.dev/utils"
)

type transport struct {
	log.Logger
	chStop chan struct{}
	chDone chan struct{}

	libp2pHost     p2phost.Host
	dht            *dht.IpfsDHT
	disc           discovery.Service
	peerID         peer.ID
	port           uint
	p2pKey         cryptop2p.PrivKey
	reachableAt    string
	bootstrapPeers []string
	*metrics.BandwidthCounter

	address types.Address

	host          swarm.Host
	controllerHub tree.ControllerHub
	peerStore     swarm.PeerStore
	keyStore      identity.KeyStore
	blobStore     blob.Store

	writeSubsByPeerID   map[peer.ID]map[netp2p.Stream]swarm.WritableSubscription
	writeSubsByPeerIDMu sync.Mutex
}

const (
	PROTO_MAIN    protocol.ID = "/redwood/main/1.0.0"
	TransportName string      = "libp2p"
)

func NewTransport(
	port uint,
	reachableAt string,
	bootstrapPeers []string,
	controllerHub tree.ControllerHub,
	keyStore identity.KeyStore,
	blobStore blob.Store,
	peerStore swarm.PeerStore,
) swarm.Transport {
	t := &transport{
		Logger:            log.NewLogger("libp2p"),
		chStop:            make(chan struct{}),
		chDone:            make(chan struct{}),
		port:              port,
		reachableAt:       reachableAt,
		controllerHub:     controllerHub,
		keyStore:          keyStore,
		blobStore:         blobStore,
		peerStore:         peerStore,
		writeSubsByPeerID: make(map[peer.ID]map[netp2p.Stream]swarm.WritableSubscription),
	}
	keyStore.OnLoadUser(t.onLoadUser)
	keyStore.OnSaveUser(t.onSaveUser)
	return t
}

func (t *transport) onLoadUser(user identity.User) (err error) {
	defer utils.WithStack(&err)

	maybeKey, exists := user.ExtraData("libp2p:p2pkey")
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

	p2pKey, _, err := cryptop2p.GenerateKeyPair(cryptop2p.Secp256k1, 0)
	if err != nil {
		return err
	}
	t.p2pKey = p2pKey
	return nil
}

func (t *transport) onSaveUser(user identity.User) error {
	bs, err := cryptop2p.MarshalPrivateKey(t.p2pKey)
	if err != nil {
		return err
	}
	hexKey := hex.EncodeToString(bs)
	user.SaveExtraData("libp2p:p2pkey", hexKey)
	return nil
}

func (t *transport) Start() error {
	t.Infof(0, "opening libp2p on port %v", t.port)

	peerID, err := peer.IDFromPublicKey(t.p2pKey.GetPublic())
	if err != nil {
		return err
	}
	t.peerID = peerID

	t.BandwidthCounter = metrics.NewBandwidthCounter()

	// ctx, cancel := utils.CombinedContext(t.chStop, 30*time.Second)
	// defer cancel()

	// Initialize the libp2p host
	libp2pHost, err := libp2p.New(context.Background(),
		libp2p.ListenAddrStrings(
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%v", t.port),
		),
		libp2p.Identity(t.p2pKey),
		libp2p.BandwidthReporter(t.BandwidthCounter),
		libp2p.NATPortMap(),
		libp2p.EnableNATService(),
		// libp2p.EnableAutoRelay(),
		// libp2p.DefaultStaticRelays(),
	)
	if err != nil {
		return errors.Wrap(err, "could not initialize libp2p host")
	}
	t.libp2pHost = libp2pHost
	t.libp2pHost.SetStreamHandler(PROTO_MAIN, t.handleIncomingStream)
	t.libp2pHost.Network().Notify(t) // Register for libp2p connect/disconnect notifications

	// Initialize the DHT
	// bootstrapPeers := dht.GetDefaultBootstrapPeerAddrInfos() // @@TODO: remove this
	var bootstrapPeers []corepeer.AddrInfo
	for _, bp := range t.bootstrapPeers {
		multiaddr, err := ma.NewMultiaddr(bp)
		if err != nil {
			t.Warnf("bad bootstrap peer multiaddress (%v): %v", bp, err)
			continue
		}
		addrInfo, err := corepeer.AddrInfoFromP2pAddr(multiaddr)
		if err != nil {
			t.Warnf("bad bootstrap peer multiaddress (%v): %v", multiaddr, err)
			continue
		}
		bootstrapPeers = append(bootstrapPeers, *addrInfo)
	}

	t.dht, err = dht.New(context.Background(), t.libp2pHost,
		dht.BootstrapPeers(bootstrapPeers...),
		dht.Mode(dht.ModeServer),
		dht.Datastore(
			dsync.MutexWrap(&notifyingDatastore{
				Batching: dstore.NewMapDatastore(),
			}),
		),
	)
	if err != nil {
		return errors.Wrap(err, "could not initialize libp2p dht")
	}

	t.dht.Validator = blankValidator{} // Set a pass-through validator

	err = t.dht.Bootstrap(context.Background())
	if err != nil {
		return errors.Wrap(err, "could not bootstrap DHT")
	}

	// Update our node's info in the peer store
	myDialAddrs := utils.NewStringSet(nil)
	for _, addr := range t.libp2pHost.Addrs() {
		addrStr := addr.String()
		myDialAddrs.Add(addrStr)
	}
	if t.reachableAt != "" {
		myDialAddrs.Add(fmt.Sprintf("%v/p2p/%v", t.reachableAt, t.Libp2pPeerID()))
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
					identity.Signing.SigningPublicKey.Address(),
					identity.Signing.SigningPublicKey,
					identity.Encrypting.EncryptingPublicKey,
				)
			}
		}
	}

	// Set up mDNS discovery
	t.disc, err = discovery.NewMdnsService(context.Background(), libp2pHost, 10*time.Second, "redwood")
	if err != nil {
		return err
	}
	t.disc.RegisterNotifee(t)

	// Update our node's info in the peer store
	myAddrs := utils.NewStringSet(nil)
	for _, addr := range t.libp2pHost.Addrs() {
		addrStr := addr.String()
		myAddrs.Add(addrStr)
	}
	if t.reachableAt != "" {
		myAddrs.Add(fmt.Sprintf("%v/p2p/%v", t.reachableAt, t.Libp2pPeerID()))
	}
	myAddrs = cleanLibp2pAddrs(myAddrs, t.peerID)

	go t.periodicallyAnnounceContent()

	t.Infof(0, "libp2p peer ID is %v", t.Libp2pPeerID())

	return nil
}

func (t *transport) Close() {
	close(t.chStop)
	// <-t.chDone

	err := t.disc.Close()
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

func (t *transport) Peers() []peerstore.PeerInfo {
	return peerstore.PeerInfos(t.libp2pHost.Peerstore(), t.libp2pHost.Peerstore().Peers())
}

func (t *transport) SetHost(h swarm.Host) {
	t.host = h
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
			t.host.HandleWritableSubscriptionClosed(sub)
		}
	}
}

// HandlePeerFound is the mDNS peer discovery callback
func (t *transport) HandlePeerFound(pinfo peerstore.PeerInfo) {
	ctx, cancel := utils.CombinedContext(t.chStop, 10*time.Second)
	defer cancel()

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

	t.addPeerInfosToPeerStore([]peerstore.PeerInfo{pinfo})

	peer := t.makeDisconnectedPeerConn(pinfo)
	if peer == nil {
		t.Infof(0, "mDNS: peer %+v is nil", pinfo.ID.Pretty())
		return
	}
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
	case MsgType_Subscribe:
		stateURI, ok := msg.Payload.(string)
		if !ok {
			t.Errorf("Subscribe message: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}
		t.Infof(0, "incoming libp2p subscription (stateURI: %v)", stateURI)

		writeSub := swarm.NewWritableSubscription(t.host, stateURI, nil, swarm.SubscriptionType_Txs, &writableSubscription{peer})
		func() {
			t.writeSubsByPeerIDMu.Lock()
			defer t.writeSubsByPeerIDMu.Unlock()
			if _, exists := t.writeSubsByPeerID[peer.pinfo.ID]; !exists {
				t.writeSubsByPeerID[peer.pinfo.ID] = make(map[netp2p.Stream]swarm.WritableSubscription)
			}
			t.writeSubsByPeerID[peer.pinfo.ID][stream] = writeSub
		}()

		fetchHistoryOpts := &swarm.FetchHistoryOpts{} // Fetch all history (@@TODO)
		t.host.HandleWritableSubscriptionOpened(writeSub, fetchHistoryOpts)

	case MsgType_Put:
		defer peer.Close()

		tx, ok := msg.Payload.(tree.Tx)
		if !ok {
			t.Errorf("Put message: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}
		t.host.HandleTxReceived(tx, peer)

	case MsgType_Ack:
		defer peer.Close()

		ackMsg, ok := msg.Payload.(swarm.AckMsg)
		if !ok {
			t.Errorf("Ack message: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}
		t.host.HandleAckReceived(ackMsg.StateURI, ackMsg.TxID, peer)

	case MsgType_ChallengeIdentityRequest:
		defer peer.Close()

		challengeMsg, ok := msg.Payload.(types.ChallengeMsg)
		if !ok {
			t.Errorf("MsgType_ChallengeIdentityRequest message: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}

		err := t.host.HandleChallengeIdentity(challengeMsg, peer)
		if err != nil {
			t.Errorf("MsgType_ChallengeIdentityRequest: error from verifyAddressHandler: %v", err)
			return
		}

	case MsgType_FetchRef:
		defer peer.Close()

		refID, ok := msg.Payload.(types.RefID)
		if !ok {
			t.Errorf("FetchRef message: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}
		t.host.HandleFetchRefReceived(refID, peer)

	case MsgType_Private:
		defer peer.Close()

		encryptedTx, ok := msg.Payload.(swarm.EncryptedTx)
		if !ok {
			t.Errorf("Private message: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}
		bs, err := t.keyStore.OpenMessageFrom(
			encryptedTx.RecipientAddress,
			crypto.EncryptingPublicKeyFromBytes(encryptedTx.SenderPublicKey),
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
		t.host.HandleTxReceived(tx, peer)

	case MsgType_AnnouncePeers:
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

func (t *transport) NewPeerConn(ctx context.Context, dialAddr string) (swarm.Peer, error) {
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

func (t *transport) ProvidersOfStateURI(ctx context.Context, stateURI string) (<-chan swarm.Peer, error) {
	urlCid, err := cidForString("serve:" + stateURI)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ctx, cancel := utils.CombinedContext(ctx, t.chStop)

	ch := make(chan swarm.Peer)
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

func (t *transport) ProvidersOfRef(ctx context.Context, refID types.RefID) (<-chan swarm.Peer, error) {
	refCid, err := cidForString("ref:" + refID.String())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ch := make(chan swarm.Peer)
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

func (t *transport) PeersClaimingAddress(ctx context.Context, address types.Address) (<-chan swarm.Peer, error) {
	ctx, cancel := utils.CombinedContext(ctx, t.chStop)
	defer cancel()

	addrCid, err := cidForString("addr:" + address.String())
	if err != nil {
		t.Errorf("error creating cid: %v", err)
		return nil, err
	}

	ch := make(chan swarm.Peer)

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

// Periodically announces our repos and objects to the network.
func (t *transport) periodicallyAnnounceContent() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		func() {
			select {
			case <-t.chStop:
				return
			default:
			}

			select {
			case <-t.chStop:
				return
			case <-ticker.C:
			}

			ctx, cancel := utils.CombinedContext(t.chStop, 10*time.Second)
			defer cancel()

			wg := utils.NewWaitGroupChan(ctx)
			defer wg.Close()

			wg.Add(1)

			// Announce the URLs we're serving
			stateURIs, err := t.controllerHub.KnownStateURIs()
			if err != nil {
				t.Errorf("error fetching known state URIs from DB: %v", err)

			} else {
				for _, stateURI := range stateURIs {
					stateURI := stateURI

					wg.Add(1)
					go func() {
						defer wg.Done()

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
					}()
				}
			}

			// Announce the blobs we're serving
			refHashes, err := t.blobStore.AllHashes()
			if err != nil {
				t.Errorf("error fetching blobStore hashes: %v", err)
			}
			for _, refHash := range refHashes {
				refHash := refHash

				wg.Add(1)
				go func() {
					defer wg.Done()

					err := t.AnnounceRef(ctx, refHash)
					if err != nil {
						t.Errorf("announce: error: %v", err)
					}
				}()
			}

			// Announce our address (for exchanging private txs)
			identities, err := t.keyStore.Identities()
			if err != nil {
				t.Errorf("announce: error creating cid: %v", err)
				return
			}

			for _, identity := range identities {
				identity := identity

				wg.Add(1)
				go func() {
					defer wg.Done()

					c, err := cidForString("addr:" + identity.Address().String())
					if err != nil {
						t.Errorf("announce: error creating cid: %v", err)
						return
					}

					err = t.dht.Provide(ctx, c, true)
					if err != nil && err != kbucket.ErrLookupFailure {
						t.Errorf(`announce: could not dht.Provide pubkey: %v`, err)
					}
				}()
			}

			wg.Done()
			<-wg.Wait()
		}()
	}
}

func (t *transport) AnnounceRef(ctx context.Context, refID types.RefID) error {
	c, err := cidForString("ref:" + refID.String())
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

func (t *transport) addPeerInfosToPeerStore(pinfos []peerstore.PeerInfo) {
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

func (t *transport) makeDisconnectedPeerConn(pinfo peerstore.PeerInfo) *peerConn {
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
