package redwood

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	cid "github.com/ipfs/go-cid"
	dstore "github.com/ipfs/go-datastore"
	dsync "github.com/ipfs/go-datastore/sync"
	libp2p "github.com/libp2p/go-libp2p"
	cryptop2p "github.com/libp2p/go-libp2p-crypto"
	p2phost "github.com/libp2p/go-libp2p-host"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	kbucket "github.com/libp2p/go-libp2p-kbucket"
	metrics "github.com/libp2p/go-libp2p-metrics"
	netp2p "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	protocol "github.com/libp2p/go-libp2p-protocol"
	ma "github.com/multiformats/go-multiaddr"
	multihash "github.com/multiformats/go-multihash"

	"github.com/brynbellomy/redwood/ctx"
	"github.com/brynbellomy/redwood/types"
)

type libp2pTransport struct {
	*ctx.Context

	libp2pHost p2phost.Host
	dht        *dht.IpfsDHT
	peerID     peer.ID
	port       uint
	p2pKey     cryptop2p.PrivKey
	*metrics.BandwidthCounter

	address types.Address

	subscriptionsIn   map[string]map[*libp2pTxSubscriptionServer]struct{}
	subscriptionsInMu sync.RWMutex

	host          Host
	controllerHub ControllerHub
	refStore      RefStore
	peerStore     PeerStore
}

type libp2pTxSubscriptionServer struct {
	stateURI string
	stream   netp2p.Stream
}

const (
	PROTO_MAIN protocol.ID = "/redwood/main/1.0.0"
)

func NewLibp2pTransport(addr types.Address, port uint, controllerHub ControllerHub, refStore RefStore, peerStore PeerStore) (Transport, error) {
	t := &libp2pTransport{
		Context:         &ctx.Context{},
		port:            port,
		address:         addr,
		subscriptionsIn: make(map[string]map[*libp2pTxSubscriptionServer]struct{}),
		controllerHub:   controllerHub,
		refStore:        refStore,
		peerStore:       peerStore,
	}
	return t, nil
}

func (t *libp2pTransport) Start() error {
	return t.CtxStart(
		// on startup
		func() error {
			t.SetLogLabel(t.address.Pretty() + " transport")
			t.Infof(0, "opening libp2p on port %v", t.port)

			p2pKey, err := obtainP2PKey(t.address)
			if err != nil {
				return err
			}
			t.p2pKey = p2pKey
			peerID, err := peer.IDFromPublicKey(p2pKey.GetPublic())
			if err != nil {
				return err
			}
			t.peerID = peerID

			t.BandwidthCounter = metrics.NewBandwidthCounter()

			// Initialize the libp2p host
			libp2pHost, err := libp2p.New(t.Ctx(),
				libp2p.ListenAddrStrings(
					fmt.Sprintf("/ip4/0.0.0.0/tcp/%v", t.port),
				),
				libp2p.Identity(t.p2pKey),
				libp2p.BandwidthReporter(t.BandwidthCounter),
				libp2p.NATPortMap(),
			)
			if err != nil {
				return errors.Wrap(err, "could not initialize libp2p host")
			}
			t.libp2pHost = libp2pHost

			// Initialize the DHT
			t.dht = dht.NewDHT(t.Ctx(), t.libp2pHost, dsync.MutexWrap(dstore.NewMapDatastore()))
			t.dht.Validator = blankValidator{} // Set a pass-through validator

			t.libp2pHost.SetStreamHandler(PROTO_MAIN, t.handleIncomingStream)

			go t.periodicallyAnnounceContent()
			go t.periodicallyUpdatePeerStore()

			t.Infof(0, "libp2p peer ID is %v", t.Libp2pPeerID())

			return nil
		},
		nil,
		nil,
		// on shutdown
		nil,
	)
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

func (t *libp2pTransport) handleIncomingStream(stream netp2p.Stream) {
	var msg Msg
	err := ReadMsg(stream, &msg)
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

		t.subscriptionsInMu.Lock()
		defer t.subscriptionsInMu.Unlock()
		if _, exists := t.subscriptionsIn[stateURI]; !exists {
			t.subscriptionsIn[stateURI] = make(map[*libp2pTxSubscriptionServer]struct{})
		}
		sub := &libp2pTxSubscriptionServer{stateURI, stream}
		t.subscriptionsIn[stateURI][sub] = struct{}{}

		parents := []types.ID{}
		toVersion := types.ID{}

		err := t.host.HandleFetchHistoryRequest(stateURI, parents, toVersion, peer)
		if err != nil {
			t.Errorf("error fetching history: %v", err)
			// @@TODO: close subscription?
		}

	case MsgType_Put:
		defer peer.CloseConn()

		tx, ok := msg.Payload.(Tx)
		if !ok {
			t.Errorf("Put message: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}
		t.host.HandleTxReceived(tx, peer)

	case MsgType_Ack:
		defer peer.CloseConn()

		txID, ok := msg.Payload.(types.ID)
		if !ok {
			t.Errorf("Ack message: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}
		t.host.HandleAckReceived(txID, peer)

	case MsgType_ChallengeIdentityRequest:
		defer peer.CloseConn()

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
		defer peer.CloseConn()

		refID, ok := msg.Payload.(types.RefID)
		if !ok {
			t.Errorf("FetchRef message: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}
		t.host.HandleFetchRefReceived(refID, peer)

	case MsgType_Private:
		encryptedTx, ok := msg.Payload.(EncryptedTx)
		if !ok {
			t.Errorf("Private message: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}
		t.host.HandlePrivateTxReceived(encryptedTx, peer)

	case MsgType_AdvertisePeers:
		tuples, ok := msg.Payload.([]PeerDialInfo)
		if !ok {
			t.Errorf("Advertise peers: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}
		for _, tuple := range tuples {
			t.peerStore.AddReachableAddresses(tuple.TransportName, NewStringSet([]string{tuple.ReachableAt}))
		}

	default:
		panic("protocol error")
	}
}

func (t *libp2pTransport) GetPeerByConnStrings(ctx context.Context, reachableAt StringSet) (Peer, error) {
	var pinfos []*peerstore.PeerInfo
	for ra := range reachableAt {
		addr, err := ma.NewMultiaddr(ra)
		if err != nil {
			return nil, errors.Wrapf(err, "could not parse multiaddr '%v'", ra)
		}
		pinfo, err := peerstore.InfoFromP2pAddr(addr)
		if err != nil {
			return nil, errors.Wrapf(err, "could not parse PeerInfo from multiaddr '%v'", ra)
		} else if pinfo.ID == t.peerID {
			return nil, errors.WithStack(ErrPeerIsSelf)
		}

		pinfos = append(pinfos, pinfo)
	}
	for i := 1; i < len(pinfos); i++ {
		pinfos[0].Addrs = append(pinfos[0].Addrs, pinfos[i].Addrs[0])
	}
	pinfo := pinfos[0]
	peer := t.makeDisconnectedPeer(*pinfo)
	return peer, nil
}

func (t *libp2pTransport) ForEachProviderOfStateURI(ctx context.Context, stateURI string) (<-chan Peer, error) {
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

				t.Infof(0, `found peer %v for stateURI "%v"`, pinfo.ID, stateURI)

				select {
				case ch <- t.makeDisconnectedPeer(pinfo):
				case <-ctx.Done():
					return
				}
			}
			time.Sleep(1 * time.Second) // @@TODO: make configurable?
		}
	}()
	return ch, nil
}

func (t *libp2pTransport) ForEachProviderOfRef(ctx context.Context, refID types.RefID) (<-chan Peer, error) {
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

				select {
				case ch <- t.makeDisconnectedPeer(pinfo):
				case <-ctx.Done():
				}
			}
		}
	}()
	return ch, nil
}

func (t *libp2pTransport) ForEachSubscriberToStateURI(ctx context.Context, stateURI string) (<-chan Peer, error) {
	ch := make(chan Peer)
	go func() {
		t.subscriptionsInMu.RLock()
		defer t.subscriptionsInMu.RUnlock()
		defer close(ch)
		for sub := range t.subscriptionsIn[stateURI] {
			select {
			case ch <- t.makeConnectedPeer(sub.stream):
			case <-ctx.Done():
			}
		}
	}()
	return ch, nil
}

func (t *libp2pTransport) PeersClaimingAddress(ctx context.Context, address types.Address) (<-chan Peer, error) {
	addrCid, err := cidForString("addr:" + address.String())
	if err != nil {
		t.Errorf("announce: error creating cid: %v", err)
		return nil, err
	}

	ch := make(chan Peer)

	go func() {
		defer close(ch)

		for pinfo := range t.dht.FindProvidersAsync(ctx, addrCid, 8) {
			if pinfo.ID == t.libp2pHost.ID() {
				continue
			}

			select {
			case <-ctx.Done():
				return
			case <-t.Ctx().Done():
				return
			case ch <- t.makeDisconnectedPeer(pinfo):
			}
		}
	}()

	return ch, nil
}

func (t *libp2pTransport) ensureConnected(ctx context.Context, pinfo peerstore.PeerInfo) error {
	if len(t.libp2pHost.Network().ConnsToPeer(pinfo.ID)) == 0 {
		err := t.libp2pHost.Connect(ctx, pinfo)
		if err != nil {
			return errors.Wrapf(err, "could not connect to peer %v", pinfo.ID)
		}
	}
	return nil
}

// Periodically announces our repos and objects to the network.
func (t *libp2pTransport) periodicallyAnnounceContent() {
	for {
		select {
		case <-t.Ctx().Done():
			return
		default:
		}

		// t.Info(0, "announce")

		// Announce the URLs we're serving
		stateURIs, err := t.controllerHub.KnownStateURIs()
		if err != nil {
			t.Errorf("error fetching known state URIs from DB: %v", err)

		} else {
			for _, url := range stateURIs {
				url := url
				go func() {
					ctxInner, cancel := context.WithTimeout(t.Ctx(), 10*time.Second)
					defer cancel()

					c, err := cidForString("serve:" + url)
					if err != nil {
						t.Errorf("announce: error creating cid: %v", err)
						return
					}

					err = t.dht.Provide(ctxInner, c, true)
					if err != nil && err != kbucket.ErrLookupFailure {
						t.Errorf(`announce: could not dht.Provide url "%v": %v`, url, err)
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
			go func() {
				ctx, cancel := context.WithTimeout(t.Ctx(), 10*time.Second)
				defer cancel()

				err := t.AnnounceRef(ctx, refHash)
				if err != nil {
					t.Errorf("announce: error: %v", err)
				}
			}()
		}

		// Advertise our address (for exchanging private txs)
		go func() {
			ctxInner, cancel := context.WithTimeout(t.Ctx(), 10*time.Second)
			defer cancel()

			c, err := cidForString("addr:" + t.address.String())
			if err != nil {
				t.Errorf("announce: error creating cid: %v", err)
				return
			}

			err = t.dht.Provide(ctxInner, c, true)
			if err != nil && err != kbucket.ErrLookupFailure {
				t.Errorf(`announce: could not dht.Provide pubkey: %v`, err)
			}
		}()

		time.Sleep(10 * time.Second)
	}
}

func (t *libp2pTransport) periodicallyUpdatePeerStore() {
	for {
		// Update our node's info in the peer store
		myAddrs := NewStringSet(nil)
		for _, addr := range t.libp2pHost.Addrs() {
			addrStr := addr.String()
			if addrStr == "/p2p-circuit" {
				myAddrs.Add(fmt.Sprintf("/p2p-circuit/%v", t.peerID.Pretty()))
			} else {
				myAddrs.Add(fmt.Sprintf("%v/p2p/%v", addrStr, t.peerID.Pretty()))
			}
		}
		myAddrs = filterUselessLibp2pAddrs(myAddrs)
		host := t.host.(*host)
		t.peerStore.AddVerifiedCredentials(t.Name(), myAddrs, host.signingKeypair.SigningPublicKey.Address(), host.signingKeypair.SigningPublicKey, host.encryptingKeypair.EncryptingPublicKey)

		// Ensure all peers discovered on the libp2p layer are in the peer store
		for _, pinfo := range t.Peers() {
			if pinfo.ID == peer.ID("") {
				continue
			}
			peer := t.makeDisconnectedPeer(pinfo)
			t.peerStore.AddReachableAddresses(t.Name(), filterUselessLibp2pAddrs(peer.ReachableAt()))
		}

		// Share our peer store with all of our peers
		peerDialInfos := t.peerStore.PeerDialInfos()
		for _, pinfo := range t.Peers() {
			if pinfo.ID == t.libp2pHost.ID() {
				continue
			}
			func() {
				peer := t.makeDisconnectedPeer(pinfo)
				err := peer.EnsureConnected(context.TODO())
				if err != nil {
					t.Errorf("error connecting to peer: %+v", err)
					return
				}
				defer peer.CloseConn()

				err = peer.writeMsg(Msg{Type: MsgType_AdvertisePeers, Payload: peerDialInfos})
				if err != nil {
					t.Errorf("error writing to peer: %+v", err)
				}
			}()
		}

		time.Sleep(10 * time.Second) // @@TODO: make configurable
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

func (t *libp2pTransport) makeConnectedPeer(stream netp2p.Stream) *libp2pPeer {
	pinfo := t.libp2pHost.Peerstore().PeerInfo(stream.Conn().RemotePeer())

	storedPeer := t.peerStore.PeerReachableAt("libp2p", multiaddrsFromPeerInfo(pinfo))
	peer := &libp2pPeer{t: t, pinfo: pinfo, stream: stream}
	if storedPeer != nil {
		peer.storedPeer = *storedPeer
	}
	return peer
}

func (t *libp2pTransport) makeDisconnectedPeer(pinfo peerstore.PeerInfo) *libp2pPeer {
	peer := &libp2pPeer{t: t, pinfo: pinfo, stream: nil}

	storedPeer := t.peerStore.PeerReachableAt("libp2p", multiaddrsFromPeerInfo(pinfo))
	if storedPeer != nil {
		peer.storedPeer = *storedPeer
	}
	return peer
}

type libp2pPeer struct {
	storedPeer
	t      *libp2pTransport
	pinfo  peerstore.PeerInfo
	stream netp2p.Stream
}

func (p *libp2pPeer) Transport() Transport {
	return p.t
}

func (p *libp2pPeer) ReachableAt() StringSet {
	return multiaddrsFromPeerInfo(p.pinfo)
}

func (p *libp2pPeer) EnsureConnected(ctx context.Context) error {
	if p.stream == nil {
		err := p.t.ensureConnected(ctx, p.pinfo)
		if err != nil {
			return err
		}

		stream, err := p.t.libp2pHost.NewStream(ctx, p.pinfo.ID, PROTO_MAIN)
		if err != nil {
			return err
		}

		p.stream = stream
	}

	return nil
}

func (peer *libp2pPeer) Subscribe(ctx context.Context, stateURI string) (TxSubscription, error) {
	err := peer.EnsureConnected(ctx)
	if err != nil {
		peer.t.Errorf("error connecting to peer: %v", err)
		return nil, err
	}

	err = peer.writeMsg(Msg{Type: MsgType_Subscribe, Payload: stateURI})
	if err != nil {
		return nil, err
	}

	return &libp2pTxSubscriptionClient{
		stateURI: stateURI,
		peer:     peer,
	}, nil
}

type libp2pTxSubscriptionClient struct {
	peer     *libp2pPeer
	stateURI string
}

func (sub *libp2pTxSubscriptionClient) Read() (*Tx, error) {
	msg, err := sub.peer.readMsg()
	if err != nil {
		return nil, errors.Errorf("error reading from susbcription: %v", err)
	}

	if msg.Type != MsgType_Put {
		return nil, errors.New("protocol error, expecting MsgType_Put")
	}

	tx := msg.Payload.(Tx)
	return &tx, nil
}

func (sub *libp2pTxSubscriptionClient) Close() error {
	return sub.peer.CloseConn()
}

func (p *libp2pPeer) Put(tx Tx) error {
	return p.writeMsg(Msg{Type: MsgType_Put, Payload: tx})
}

func (p *libp2pPeer) PutPrivate(tx EncryptedTx) error {
	return p.writeMsg(Msg{Type: MsgType_Private, Payload: tx})
}

func (p *libp2pPeer) Ack(txID types.ID) error {
	return p.writeMsg(Msg{Type: MsgType_Ack, Payload: txID})
}

func (p *libp2pPeer) ChallengeIdentity(challengeMsg types.ChallengeMsg) error {
	return p.writeMsg(Msg{Type: MsgType_ChallengeIdentityRequest, Payload: challengeMsg})
}

func (p *libp2pPeer) ReceiveChallengeIdentityResponse() (ChallengeIdentityResponse, error) {
	msg, err := p.readMsg()
	if err != nil {
		return ChallengeIdentityResponse{}, err
	}

	resp, ok := msg.Payload.(ChallengeIdentityResponse)
	if !ok {
		return ChallengeIdentityResponse{}, errors.New("protocol error")
	}

	return resp, nil
}

func (p *libp2pPeer) RespondChallengeIdentity(challengeIdentityResponse ChallengeIdentityResponse) error {
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
		return FetchRefResponseBody{}, errors.New("protocol probs")
	}

	resp, is := msg.Payload.(FetchRefResponse)
	if !is {
		return FetchRefResponseBody{}, errors.New("protocol probs")
	} else if resp.Body == nil {
		return FetchRefResponseBody{}, errors.New("protocol probs")
	}
	return *resp.Body, nil
}

func (p *libp2pPeer) ReceiveRefHeader() (FetchRefResponseHeader, error) {
	msg, err := p.readMsg()
	if err != nil {
		return FetchRefResponseHeader{}, errors.Errorf("error reading from peer: %v", err)
	} else if msg.Type != MsgType_FetchRefResponse {
		return FetchRefResponseHeader{}, errors.New("protocol probs")
	}

	resp, is := msg.Payload.(FetchRefResponse)
	if !is {
		return FetchRefResponseHeader{}, errors.New("protocol probs")
	} else if resp.Header == nil {
		return FetchRefResponseHeader{}, errors.New("protocol probs")
	}
	return *resp.Header, nil
}

func (p *libp2pPeer) writeMsg(msg Msg) error {
	return WriteMsg(p.stream, msg)
}

func (p *libp2pPeer) readMsg() (Msg, error) {
	var msg Msg
	err := ReadMsg(p.stream, &msg)
	return msg, err
}

func (p *libp2pPeer) CloseConn() error {
	return p.stream.Close()
}

func obtainP2PKey(addr types.Address) (cryptop2p.PrivKey, error) {
	configPath, err := RedwoodConfigDirPath()
	if err != nil {
		return nil, err
	}

	keyfile := fmt.Sprintf("redwood.%v.key", addr.Pretty())
	keyfile = filepath.Join(configPath, keyfile)

	f, err := os.Open(keyfile)
	if err != nil && !os.IsNotExist(err) {
		return nil, err

	} else if err == nil {
		defer f.Close()

		data, err := ioutil.ReadFile(keyfile)
		if err != nil {
			return nil, err
		}
		return cryptop2p.UnmarshalPrivateKey(data)
	}

	p2pKey, _, err := cryptop2p.GenerateKeyPair(cryptop2p.Secp256k1, 0)
	if err != nil {
		return nil, err
	}

	bs, err := p2pKey.Bytes()
	if err != nil {
		return nil, err
	}

	err = ioutil.WriteFile(keyfile, bs, 0400)
	if err != nil {
		return nil, err
	}

	return p2pKey, nil
}

func cidForString(s string) (cid.Cid, error) {
	pref := cid.NewPrefixV1(cid.Raw, multihash.SHA2_256)
	c, err := pref.Sum([]byte(s))
	if err != nil {
		return cid.Cid{}, errors.Wrap(err, "could not create cid")
	}
	return c, nil
}

func multiaddrsFromPeerInfo(pinfo peerstore.PeerInfo) StringSet {
	reachableAt := NewStringSet(nil)
	multiaddrs, err := peerstore.InfoToP2pAddrs(&pinfo)
	if err != nil {
		panic(err)
	}
	for _, addr := range multiaddrs {
		reachableAt.Add(addr.String())
	}
	return filterUselessLibp2pAddrs(reachableAt)
}

func filterUselessLibp2pAddrs(addrStrs StringSet) StringSet {
	keep := NewStringSet(nil)
	for addrStr := range addrStrs {
		if strings.Index(addrStr, "/ip4/172.") == 0 {
			continue
			// } else if strings.Index(addrStr, "/ip4/0.0.0.0") == 0 {
			// 	continue
		} else if strings.Index(addrStr, "/ip4/127.0.0.1") == 0 {
			continue
		} else if addrStr[:len("/p2p-circuit")] == "/p2p-circuit" {
			continue
		}

		addrStr = strings.Replace(addrStr, "/ipfs/", "/p2p/", 1)

		keep.Add(addrStr)
	}
	return keep
}

type blankValidator struct{}

func (blankValidator) Validate(_ string, _ []byte) error        { return nil }
func (blankValidator) Select(_ string, _ [][]byte) (int, error) { return 0, nil }
