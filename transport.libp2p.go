package redwood

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	cid "github.com/ipfs/go-cid"
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
	recpb "github.com/libp2p/go-libp2p-record/pb"
	"github.com/libp2p/go-libp2p/p2p/discovery"
	ma "github.com/multiformats/go-multiaddr"
	multihash "github.com/multiformats/go-multihash"
	"github.com/pkg/errors"

	"redwood.dev/crypto"
	"redwood.dev/ctx"
	"redwood.dev/identity"
	"redwood.dev/tree"
	"redwood.dev/types"
	"redwood.dev/utils"
)

type libp2pTransport struct {
	ctx.Logger
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
	enckeys *crypto.EncryptingKeypair

	host          Host
	controllerHub ControllerHub
	keyStore      identity.KeyStore
	refStore      RefStore
	peerStore     PeerStore

	writeSubsByPeerID   map[peer.ID]map[netp2p.Stream]WritableSubscription
	writeSubsByPeerIDMu sync.Mutex
}

const (
	PROTO_MAIN protocol.ID = "/redwood/main/1.0.0"
)

func NewLibp2pTransport(
	port uint,
	reachableAt string,
	keyfilePath string,
	bootstrapPeers []string,
	controllerHub ControllerHub,
	keyStore identity.KeyStore,
	refStore RefStore,
	peerStore PeerStore,
) Transport {
	t := &libp2pTransport{
		Logger:            ctx.NewLogger("libp2p"),
		chStop:            make(chan struct{}),
		chDone:            make(chan struct{}),
		port:              port,
		reachableAt:       reachableAt,
		controllerHub:     controllerHub,
		keyStore:          keyStore,
		refStore:          refStore,
		peerStore:         peerStore,
		writeSubsByPeerID: make(map[peer.ID]map[netp2p.Stream]WritableSubscription),
	}
	keyStore.OnLoadUser(t.onLoadUser)
	keyStore.OnSaveUser(t.onSaveUser)
	return t
}

func (t *libp2pTransport) onLoadUser(user identity.User) (err error) {
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

func (t *libp2pTransport) onSaveUser(user identity.User) error {
	bs, err := cryptop2p.MarshalPrivateKey(t.p2pKey)
	if err != nil {
		return err
	}
	hexKey := hex.EncodeToString(bs)
	user.SaveExtraData("libp2p:p2pkey", hexKey)
	return nil
}

func (t *libp2pTransport) Start() error {
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
		libp2p.EnableAutoRelay(),
		libp2p.DefaultStaticRelays(),
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
					PeerDialInfo{TransportName: t.Name(), DialAddr: dialAddr},
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

func (t *libp2pTransport) Close() {
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

type notifyingDatastore struct {
	dstore.Batching
}

func (ds *notifyingDatastore) Put(k dstore.Key, v []byte) error {
	err := ds.Batching.Put(k, v)
	if err != nil {
		return err
	}
	key, value, err := decodeDatastoreKeyValue(ds.Batching, k, v)
	if err != nil {
		return err
	}
	ctx.NewLogger("libp2p datastore").Debugf("key=%v value=%v", key, string(value))
	return nil
}

func decodeDatastoreKeyValue(ds dstore.Batching, key dstore.Key, value []byte) (string, []byte, error) {
	buf, err := ds.Get(key)
	if err == dstore.ErrNotFound {
		return "", nil, nil
	} else if err != nil {
		return "", nil, err
	}

	rec := new(recpb.Record)
	err = proto.Unmarshal(buf, rec)
	if err != nil {
		// Bad data in datastore, log it but don't return an error, we'll just overwrite it
		fmt.Printf("Bad record data stored in datastore with key %s: could not unmarshal record\n", key)
		return "", nil, nil
	}
	return string(rec.Key), rec.Value, nil
}

func (t *libp2pTransport) Name() string {
	return "libp2p"
}

func (t *libp2pTransport) Libp2pPeerID() string {
	return t.libp2pHost.ID().Pretty()
}

func (t *libp2pTransport) ListenAddrs() []string {
	addrs := []string{}
	for _, addr := range t.libp2pHost.Addrs() {
		addrs = append(addrs, addr.String()+"/p2p/"+t.libp2pHost.ID().Pretty())
	}
	return addrs
}

func (t *libp2pTransport) Peers() []peerstore.PeerInfo {
	return peerstore.PeerInfos(t.libp2pHost.Peerstore(), t.libp2pHost.Peerstore().Peers())
}

func (t *libp2pTransport) SetHost(h Host) {
	t.host = h
}

func (t *libp2pTransport) Listen(network netp2p.Network, multiaddr ma.Multiaddr)      {}
func (t *libp2pTransport) ListenClose(network netp2p.Network, multiaddr ma.Multiaddr) {}

func (t *libp2pTransport) Connected(network netp2p.Network, conn netp2p.Conn) {
	t.addPeerInfosToPeerStore(t.Peers())

	addr := conn.RemoteMultiaddr().String() + "/p2p/" + conn.RemotePeer().Pretty()
	t.Debugf("libp2p connected: %v", addr)
	t.peerStore.AddDialInfos([]PeerDialInfo{{TransportName: t.Name(), DialAddr: addr}})
}

func (t *libp2pTransport) Disconnected(network netp2p.Network, conn netp2p.Conn) {
	t.addPeerInfosToPeerStore(t.Peers())

	addr := conn.RemoteMultiaddr().String() + "/p2p/" + conn.RemotePeer().Pretty()
	t.Debugf("libp2p disconnected: %v", addr)
}

func (t *libp2pTransport) OpenedStream(network netp2p.Network, stream netp2p.Stream) {}

func (t *libp2pTransport) ClosedStream(network netp2p.Network, stream netp2p.Stream) {
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
func (t *libp2pTransport) HandlePeerFound(pinfo peerstore.PeerInfo) {
	ctx, cancel := utils.CombinedContext(t.chStop, 10*time.Second)
	defer cancel()

	// Ensure all peers discovered on the libp2p layer are in the peer store
	if pinfo.ID == peer.ID("") {
		return
	}
	// dialInfos := t.peerDialInfosFromPeerInfo(pinfo)
	// t.peerStore.AddDialInfos(dialInfos)

	var i uint
	for _, dialInfo := range t.peerDialInfosFromPeerInfo(pinfo) {
		if !t.peerStore.IsKnownPeer(dialInfo) {
			i++
		}
	}

	if i > 0 {
		t.Infof(0, "mDNS: peer %+v found", pinfo.ID.Pretty())
	}

	t.addPeerInfosToPeerStore([]peerstore.PeerInfo{pinfo})

	peer := t.makeDisconnectedPeer(pinfo)
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

func (t *libp2pTransport) handleIncomingStream(stream netp2p.Stream) {
	msg, err := libp2pReadMsg(stream)
	if err != nil {
		t.Errorf("incoming stream error: %v", err)
		stream.Close()
		return
	}

	peer := t.makeConnectedPeer(stream)

	switch msg.Type {
	case MsgType_Subscribe:
		stateURI, ok := msg.Payload.(string)
		if !ok {
			t.Errorf("Subscribe message: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}

		writeSub := newWritableSubscription(t.host, stateURI, nil, SubscriptionType_Txs, &libp2pWritableSubscription{peer})
		func() {
			t.writeSubsByPeerIDMu.Lock()
			defer t.writeSubsByPeerIDMu.Unlock()
			if _, exists := t.writeSubsByPeerID[peer.pinfo.ID]; !exists {
				t.writeSubsByPeerID[peer.pinfo.ID] = make(map[netp2p.Stream]WritableSubscription)
			}
			t.writeSubsByPeerID[peer.pinfo.ID][stream] = writeSub
		}()

		fetchHistoryOpts := &FetchHistoryOpts{} // Fetch all history (@@TODO)
		t.host.HandleWritableSubscriptionOpened(writeSub, fetchHistoryOpts)

	case MsgType_Put:
		defer peer.Close()

		tx, ok := msg.Payload.(Tx)
		if !ok {
			t.Errorf("Put message: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}
		t.host.HandleTxReceived(tx, peer)

	case MsgType_Ack:
		defer peer.Close()

		ackMsg, ok := msg.Payload.(libp2pAckMsg)
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

		encryptedTx, ok := msg.Payload.(EncryptedTx)
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

		var tx Tx
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

		tuples, ok := msg.Payload.([]PeerDialInfo)
		if !ok {
			t.Errorf("Announce peers: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}
		t.peerStore.AddDialInfos(tuples)

	default:
		panic("protocol error")
	}
}

func (t *libp2pTransport) NewPeerConn(ctx context.Context, dialAddr string) (Peer, error) {
	addr, err := ma.NewMultiaddr(dialAddr)
	if err != nil {
		return nil, errors.Wrapf(err, "could not parse multiaddr '%v'", dialAddr)
	}
	pinfo, err := peerstore.InfoFromP2pAddr(addr)
	if err != nil {
		return nil, errors.Wrapf(err, "could not parse PeerInfo from multiaddr '%v'", dialAddr)
	} else if pinfo.ID == t.peerID {
		return nil, errors.WithStack(ErrPeerIsSelf)
	}
	peer := t.makeDisconnectedPeer(*pinfo)
	return peer, nil
}

func (t *libp2pTransport) ProvidersOfStateURI(ctx context.Context, stateURI string) (<-chan Peer, error) {
	urlCid, err := cidForString("serve:" + stateURI)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ch := make(chan Peer)
	go func() {
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

				peer := t.makeDisconnectedPeer(pinfo)
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

func (t *libp2pTransport) ProvidersOfRef(ctx context.Context, refID types.RefID) (<-chan Peer, error) {
	refCid, err := cidForString("ref:" + refID.String())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ch := make(chan Peer)
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

				t.Infof(0, `found peer %v for ref "%v"`, pinfo.ID, refID.String())

				peer := t.makeDisconnectedPeer(pinfo)
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

func (t *libp2pTransport) PeersClaimingAddress(ctx context.Context, address types.Address) (<-chan Peer, error) {
	ctx, cancel := utils.CombinedContext(ctx, t.chStop)
	defer cancel()

	addrCid, err := cidForString("addr:" + address.String())
	if err != nil {
		t.Errorf("error creating cid: %v", err)
		return nil, err
	}

	ch := make(chan Peer)

	go func() {
		defer close(ch)

		for pinfo := range t.dht.FindProvidersAsync(ctx, addrCid, 8) {
			if pinfo.ID == t.libp2pHost.ID() {
				continue
			}

			peer := t.makeDisconnectedPeer(pinfo)
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
func (t *libp2pTransport) periodicallyAnnounceContent() {
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
			refHashes, err := t.refStore.AllHashes()
			if err != nil {
				t.Errorf("error fetching refStore hashes: %v", err)
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

func (t *libp2pTransport) AnnounceRef(ctx context.Context, refID types.RefID) error {
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

func (t *libp2pTransport) addPeerInfosToPeerStore(pinfos []peerstore.PeerInfo) {
	for _, pinfo := range pinfos {
		if pinfo.ID == peer.ID("") {
			continue
		}
		dialInfos := t.peerDialInfosFromPeerInfo(pinfo)
		t.peerStore.AddDialInfos(dialInfos)
	}
}

func (t *libp2pTransport) makeConnectedPeer(stream netp2p.Stream) *libp2pPeer {
	pinfo := t.libp2pHost.Peerstore().PeerInfo(stream.Conn().RemotePeer())
	peer := &libp2pPeer{t: t, pinfo: pinfo, stream: stream}
	dialAddrs := multiaddrsFromPeerInfo(pinfo)

	var dialInfos []PeerDialInfo
	dialAddrs.ForEach(func(dialAddr string) bool {
		dialInfo := PeerDialInfo{TransportName: t.Name(), DialAddr: dialAddr}
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

func (t *libp2pTransport) makeDisconnectedPeer(pinfo peerstore.PeerInfo) *libp2pPeer {
	peer := &libp2pPeer{t: t, pinfo: pinfo, stream: nil}
	dialAddrs := multiaddrsFromPeerInfo(pinfo)
	if dialAddrs.Len() == 0 {
		return nil
	}

	var peerDetails *peerDetails
	dialAddrs.ForEach(func(dialAddr string) bool {
		peerDetails = t.peerStore.PeerWithDialInfo(PeerDialInfo{TransportName: t.Name(), DialAddr: dialAddr})
		if peerDetails != nil {
			return false
		}
		return true
	})
	if peerDetails == nil {
		var dialInfos []PeerDialInfo
		dialAddrs.ForEach(func(dialAddr string) bool {
			dialInfos = append(dialInfos, PeerDialInfo{TransportName: t.Name(), DialAddr: dialAddr})
			return true
		})
		t.peerStore.AddDialInfos(dialInfos)
		var peerDetails PeerDetails
		for _, dialInfo := range dialInfos {
			peerDetails = t.peerStore.PeerWithDialInfo(dialInfo)
			if peerDetails != nil {
				break
			}
		}
		if peerDetails == nil {
			return nil
		}
		peer.PeerDetails = peerDetails
	} else {
		peer.PeerDetails = peerDetails
	}
	return peer
}

type libp2pPeer struct {
	PeerDetails
	t      *libp2pTransport
	pinfo  peerstore.PeerInfo
	stream netp2p.Stream
	mu     sync.Mutex
}

func (peer *libp2pPeer) Transport() Transport {
	return peer.t
}

func (peer *libp2pPeer) EnsureConnected(ctx context.Context) error {
	if peer.stream == nil {
		if len(peer.t.libp2pHost.Network().ConnsToPeer(peer.pinfo.ID)) == 0 {
			err := peer.t.libp2pHost.Connect(ctx, peer.pinfo)
			if err != nil {
				return errors.Wrapf(types.ErrConnection, "(peer %v): %v", peer.pinfo.ID, err)
			}
		}

		stream, err := peer.t.libp2pHost.NewStream(ctx, peer.pinfo.ID, PROTO_MAIN)
		if err != nil {
			peer.UpdateConnStats(false)
			return err
		}
		peer.UpdateConnStats(true)

		peer.stream = stream
	}
	return nil
}

func (peer *libp2pPeer) Subscribe(ctx context.Context, stateURI string) (_ ReadableSubscription, err error) {
	defer func() { peer.UpdateConnStats(err == nil) }()

	err = peer.EnsureConnected(ctx)
	if err != nil {
		peer.t.Errorf("error connecting to peer: %v", err)
		return nil, err
	}

	err = peer.writeMsg(Msg{Type: MsgType_Subscribe, Payload: stateURI})
	if err != nil {
		return nil, err
	}

	return &libp2pReadableSubscription{peer}, nil
}

func (peer *libp2pPeer) Put(ctx context.Context, tx *Tx, state tree.Node, leaves []types.ID) error {
	// Note: libp2p peers ignore `state` and `leaves`
	if tx.IsPrivate() {
		marshalledTx, err := json.Marshal(tx)
		if err != nil {
			return errors.WithStack(err)
		}

		peerAddrs := types.OverlappingAddresses(tx.Recipients, peer.Addresses())
		if len(peerAddrs) == 0 {
			return errors.New("tx not intended for this peer")
		}
		peerSigPubkey, peerEncPubkey := peer.PublicKeys(peerAddrs[0])

		encryptedTxBytes, err := peer.t.keyStore.SealMessageFor(tx.From, peerEncPubkey, marshalledTx)
		if err != nil {
			return errors.WithStack(err)
		}

		etx := EncryptedTx{
			TxID:             tx.ID,
			EncryptedPayload: encryptedTxBytes,
			SenderPublicKey:  peer.t.enckeys.EncryptingPublicKey.Bytes(),
			RecipientAddress: peerSigPubkey.Address(),
		}
		return peer.writeMsg(Msg{Type: MsgType_Private, Payload: etx})
	}
	return peer.writeMsg(Msg{Type: MsgType_Put, Payload: tx})
}

type libp2pAckMsg struct {
	StateURI string   `json:"stateURI"`
	TxID     types.ID `json:"txID"`
}

func (p *libp2pPeer) Ack(stateURI string, txID types.ID) error {
	return p.writeMsg(Msg{Type: MsgType_Ack, Payload: libp2pAckMsg{stateURI, txID}})
}

func (p *libp2pPeer) ChallengeIdentity(challengeMsg types.ChallengeMsg) error {
	return p.writeMsg(Msg{Type: MsgType_ChallengeIdentityRequest, Payload: challengeMsg})
}

func (p *libp2pPeer) ReceiveChallengeIdentityResponse() ([]ChallengeIdentityResponse, error) {
	msg, err := p.readMsg()
	if err != nil {
		return nil, err
	}
	resp, ok := msg.Payload.([]ChallengeIdentityResponse)
	if !ok {
		return nil, ErrProtocol
	}
	return resp, nil
}

func (p *libp2pPeer) RespondChallengeIdentity(challengeIdentityResponse []ChallengeIdentityResponse) error {
	return p.writeMsg(Msg{Type: MsgType_ChallengeIdentityResponse, Payload: challengeIdentityResponse})
}

func (p *libp2pPeer) FetchRef(refID types.RefID) error {
	return p.writeMsg(Msg{Type: MsgType_FetchRef, Payload: refID})
}

func (p *libp2pPeer) SendRefHeader() error {
	return p.writeMsg(Msg{Type: MsgType_FetchRefResponse, Payload: FetchRefResponse{Header: &FetchRefResponseHeader{}}})
}

func (p *libp2pPeer) SendRefPacket(data []byte, end bool) error {
	return p.writeMsg(Msg{Type: MsgType_FetchRefResponse, Payload: FetchRefResponse{Body: &FetchRefResponseBody{Data: data, End: end}}})
}

func (p *libp2pPeer) ReceiveRefPacket() (FetchRefResponseBody, error) {
	msg, err := p.readMsg()
	if err != nil {
		return FetchRefResponseBody{}, errors.Errorf("error reading from peer: %v", err)
	} else if msg.Type != MsgType_FetchRefResponse {
		return FetchRefResponseBody{}, ErrProtocol
	}

	resp, is := msg.Payload.(FetchRefResponse)
	if !is {
		return FetchRefResponseBody{}, ErrProtocol
	} else if resp.Body == nil {
		return FetchRefResponseBody{}, ErrProtocol
	}
	return *resp.Body, nil
}

func (p *libp2pPeer) ReceiveRefHeader() (FetchRefResponseHeader, error) {
	msg, err := p.readMsg()
	if err != nil {
		return FetchRefResponseHeader{}, errors.Errorf("error reading from peer: %v", err)
	} else if msg.Type != MsgType_FetchRefResponse {
		return FetchRefResponseHeader{}, ErrProtocol
	}

	resp, is := msg.Payload.(FetchRefResponse)
	if !is {
		return FetchRefResponseHeader{}, ErrProtocol
	} else if resp.Header == nil {
		return FetchRefResponseHeader{}, ErrProtocol
	}
	return *resp.Header, nil
}

func (p *libp2pPeer) AnnouncePeers(ctx context.Context, peerDialInfos []PeerDialInfo) error {
	return p.writeMsg(Msg{Type: MsgType_AnnouncePeers, Payload: peerDialInfos})
}

func (p *libp2pPeer) writeMsg(msg Msg) (err error) {
	defer func() { p.UpdateConnStats(err == nil) }()

	bs, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	buflen := uint64(len(bs))

	err = p.stream.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err != nil {
		return err
	}

	err = WriteUint64(p.stream, buflen)
	if err != nil {
		return err
	}

	n, err := io.Copy(p.stream, bytes.NewReader(bs))
	if err != nil {
		return err
	} else if n != int64(buflen) {
		return errors.New("WriteMsg: could not write entire packet")
	}
	return nil
}

func (p *libp2pPeer) readMsg() (msg Msg, err error) {
	defer func() { p.UpdateConnStats(err == nil) }()
	return libp2pReadMsg(p.stream)
}

func libp2pReadMsg(r io.Reader) (msg Msg, err error) {
	size, err := ReadUint64(r)
	if err != nil {
		return Msg{}, err
	}

	buf := &bytes.Buffer{}
	_, err = io.CopyN(buf, r, int64(size))
	if err != nil {
		return Msg{}, err
	}

	err = json.NewDecoder(buf).Decode(&msg)
	return msg, err
}

func (p *libp2pPeer) Close() error {
	if p.stream != nil {
		return p.stream.Close()
	}
	return nil
}

type libp2pReadableSubscription struct {
	*libp2pPeer
}

func (sub *libp2pReadableSubscription) Read() (_ *SubscriptionMsg, err error) {
	defer func() { sub.UpdateConnStats(err == nil) }()

	msg, err := sub.readMsg()
	if err != nil {
		return nil, errors.Errorf("error reading from subscription: %v", err)
	}

	switch msg.Type {
	case MsgType_Put:
		tx := msg.Payload.(Tx)
		return &SubscriptionMsg{Tx: &tx}, nil

	case MsgType_Private:
		encryptedTx, ok := msg.Payload.(EncryptedTx)
		if !ok {
			return nil, errors.Errorf("Private message: bad payload: (%T) %v", msg.Payload, msg.Payload)
		}
		bs, err := sub.t.keyStore.OpenMessageFrom(
			encryptedTx.RecipientAddress,
			crypto.EncryptingPublicKeyFromBytes(encryptedTx.SenderPublicKey),
			encryptedTx.EncryptedPayload,
		)
		if err != nil {
			return nil, errors.Errorf("error decrypting tx: %v", err)
		}

		var tx Tx
		err = json.Unmarshal(bs, &tx)
		if err != nil {
			return nil, errors.Errorf("error decoding tx: %v", err)
		} else if encryptedTx.TxID != tx.ID {
			return nil, errors.Errorf("private tx id does not match")
		}
		return &SubscriptionMsg{Tx: &tx, EncryptedTx: &encryptedTx}, nil

	default:
		return nil, errors.New("protocol error, expecting MsgType_Put or MsgType_Private")
	}
}

type libp2pWritableSubscription struct {
	*libp2pPeer
}

func (sub *libp2pWritableSubscription) Put(ctx context.Context, tx *Tx, state tree.Node, leaves []types.ID) (err error) {
	defer func() { sub.UpdateConnStats(err == nil) }()

	err = sub.libp2pPeer.EnsureConnected(ctx)
	if err != nil {
		return err
	}
	return sub.libp2pPeer.Put(ctx, tx, state, leaves)
}

func cidForString(s string) (cid.Cid, error) {
	pref := cid.NewPrefixV1(cid.Raw, multihash.SHA2_256)
	c, err := pref.Sum([]byte(s))
	if err != nil {
		return cid.Cid{}, errors.Wrap(err, "could not create cid")
	}
	return c, nil
}

var (
	protoDNS4 = ma.ProtocolWithName("dns4")
	protoIP4  = ma.ProtocolWithName("ip4")
)

func multiaddrsFromPeerInfo(pinfo peerstore.PeerInfo) *utils.SortedStringSet {
	multiaddrs, err := peerstore.InfoToP2pAddrs(&pinfo)
	if err != nil {
		panic(err)
	}

	// Deduplicate the addrs
	deduped := make(map[string]ma.Multiaddr, len(multiaddrs))
	for _, addr := range multiaddrs {
		deduped[addr.String()] = addr
	}
	multiaddrs = make([]ma.Multiaddr, 0, len(deduped))
	for _, addr := range deduped {
		multiaddrs = append(multiaddrs, addr)
	}

	// Sort them
	sort.Slice(multiaddrs, func(i, j int) bool {
		if val := protocolValue(multiaddrs[i], protoIP4); val != "" {
			if val == "127" {
				return true
			} else if val == "192" {
				return true
			}
		} else if protocolValue(multiaddrs[i], protoDNS4) != "" {
			return true
		}
		return false
	})

	// Filter and clean them
	multiaddrStrings := make([]string, 0, len(multiaddrs))
	for _, addr := range multiaddrs {
		if cleaned := cleanLibp2pAddr(addr.String(), pinfo.ID); cleaned != "" {
			multiaddrStrings = append(multiaddrStrings, cleaned)
		}
	}
	return utils.NewSortedStringSet(multiaddrStrings)
}

func cleanLibp2pAddr(addrStr string, peerID peer.ID) string {
	if addrStr[:len("/p2p-circuit")] == "/p2p-circuit" {
		return ""
	}

	addrStr = strings.Replace(addrStr, "/ipfs/", "/p2p/", 1)

	if !strings.Contains(addrStr, "/p2p/") {
		addrStr = addrStr + "/p2p/" + peerID.Pretty()
	}
	return addrStr
}

func cleanLibp2pAddrs(addrStrs utils.StringSet, peerID peer.ID) utils.StringSet {
	keep := utils.NewStringSet(nil)
	for addrStr := range addrStrs {
		if strings.Index(addrStr, "/ip4/172.") == 0 {
			// continue
			// } else if strings.Index(addrStr, "/ip4/0.0.0.0") == 0 {
			//  continue
			// } else if strings.Index(addrStr, "/ip4/127.0.0.1") == 0 {
			//  continue
		} else if addrStr[:len("/p2p-circuit")] == "/p2p-circuit" {
			continue
		}

		addrStr = strings.Replace(addrStr, "/ipfs/", "/p2p/", 1)

		if !strings.Contains(addrStr, "/p2p/") {
			addrStr = addrStr + "/p2p/" + peerID.Pretty()
		}

		keep.Add(addrStr)
	}
	return keep
}

func protocolValue(addr ma.Multiaddr, proto ma.Protocol) string {
	val, err := addr.ValueForProtocol(proto.Code)
	if err == ma.ErrProtocolNotFound {
		return ""
	}
	return val
}

// func sortLibp2pAddrs(addrs StringSet) SortedStringSet {
// 	s := addrs.Slice()
// 	sort.Slice(s, func(i, j int) bool {
//         net.ParseIP(s[i])
//         strconv.ParseUint(s[i][])
//         switch s[i][:3] {
//         case "127":
//             return true
//         case "192"
//         }
// 	})
// 	return utils.NewSortedStringSet(s)
// }

func (t *libp2pTransport) peerDialInfosFromPeerInfo(pinfo peerstore.PeerInfo) []PeerDialInfo {
	addrs := multiaddrsFromPeerInfo(pinfo)
	var dialInfos []PeerDialInfo
	addrs.ForEach(func(addr string) bool {
		dialInfos = append(dialInfos, PeerDialInfo{TransportName: t.Name(), DialAddr: addr})
		return true
	})
	return dialInfos
}

type blankValidator struct{}

func (blankValidator) Validate(key string, val []byte) error {
	fmt.Println("Validate ~>", key, string(val))
	return nil
}
func (blankValidator) Select(key string, vals [][]byte) (int, error) {
	fmt.Println("SELECT key", key)
	for _, val := range vals {
		fmt.Println("  -", string(val))
	}
	return 0, nil
}

type Msg struct {
	Type    MsgType     `json:"type"`
	Payload interface{} `json:"payload"`
}

type MsgType string

const (
	MsgType_Subscribe                 MsgType = "subscribe"
	MsgType_Unsubscribe               MsgType = "unsubscribe"
	MsgType_Put                       MsgType = "put"
	MsgType_Private                   MsgType = "private"
	MsgType_Ack                       MsgType = "ack"
	MsgType_Error                     MsgType = "error"
	MsgType_ChallengeIdentityRequest  MsgType = "challenge identity"
	MsgType_ChallengeIdentityResponse MsgType = "challenge identity response"
	MsgType_FetchRef                  MsgType = "fetch ref"
	MsgType_FetchRefResponse          MsgType = "fetch ref response"
	MsgType_AnnouncePeers             MsgType = "announce peers"
)

func ReadUint64(r io.Reader) (uint64, error) {
	buf := make([]byte, 8)
	_, err := io.ReadFull(r, buf)
	if err == io.EOF {
		return 0, err
	} else if err != nil {
		return 0, errors.Wrap(err, "ReadUint64")
	}
	return binary.LittleEndian.Uint64(buf), nil
}

func WriteUint64(w io.Writer, n uint64) error {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, n)
	written, err := w.Write(buf)
	if err != nil {
		return err
	} else if written < 8 {
		return errors.New("WriteUint64: wrote too few bytes")
	}
	return nil
}

func (msg *Msg) UnmarshalJSON(bs []byte) error {
	var m struct {
		Type         string          `json:"type"`
		PayloadBytes json.RawMessage `json:"payload"`
	}

	err := json.Unmarshal(bs, &m)
	if err != nil {
		return err
	}

	msg.Type = MsgType(m.Type)

	switch msg.Type {
	case MsgType_Subscribe:
		url := string(m.PayloadBytes)
		msg.Payload = url[1 : len(url)-1] // remove quotes

	case MsgType_Put:
		var tx Tx
		err := json.Unmarshal(m.PayloadBytes, &tx)
		if err != nil {
			return err
		}
		msg.Payload = tx

	case MsgType_Ack:
		var payload libp2pAckMsg
		err := json.Unmarshal(m.PayloadBytes, &payload)
		if err != nil {
			return err
		}
		msg.Payload = payload

	case MsgType_Private:
		var ep EncryptedTx
		err := json.Unmarshal(m.PayloadBytes, &ep)
		if err != nil {
			return err
		}
		msg.Payload = ep

	case MsgType_ChallengeIdentityRequest:
		var challenge types.ChallengeMsg
		err := json.Unmarshal(m.PayloadBytes, &challenge)
		if err != nil {
			return err
		}
		msg.Payload = challenge

	case MsgType_ChallengeIdentityResponse:
		var resp []ChallengeIdentityResponse
		err := json.Unmarshal([]byte(m.PayloadBytes), &resp)
		if err != nil {
			return err
		}

		msg.Payload = resp

	case MsgType_FetchRef:
		var refID types.RefID
		err := json.Unmarshal([]byte(m.PayloadBytes), &refID)
		if err != nil {
			return err
		}
		msg.Payload = refID

	case MsgType_FetchRefResponse:
		var resp FetchRefResponse
		err := json.Unmarshal([]byte(m.PayloadBytes), &resp)
		if err != nil {
			return err
		}
		msg.Payload = resp

	case MsgType_AnnouncePeers:
		var peerDialInfos []PeerDialInfo
		err := json.Unmarshal([]byte(m.PayloadBytes), &peerDialInfos)
		if err != nil {
			return err
		}
		msg.Payload = peerDialInfos

	default:
		return errors.Errorf("bad msg: %v", msg.Type)
	}

	return nil
}
