package redwood

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/url"
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
	pstore "github.com/libp2p/go-libp2p-peerstore"
	protocol "github.com/libp2p/go-libp2p-protocol"
	ma "github.com/multiformats/go-multiaddr"
	multihash "github.com/multiformats/go-multihash"

	"github.com/brynbellomy/redwood/ctx"
)

type libp2pTransport struct {
	*ctx.Context

	libp2pHost p2phost.Host
	dht        *dht.IpfsDHT
	peerID     peer.ID
	*metrics.BandwidthCounter
	port   uint
	p2pKey cryptop2p.PrivKey

	address Address

	fetchHistoryHandler  FetchHistoryHandler
	txHandler            TxHandler
	privateTxHandler     PrivateTxHandler
	ackHandler           AckHandler
	verifyAddressHandler VerifyAddressHandler
	fetchRefHandler      FetchRefHandler

	subscriptionsIn   map[string][]libp2pSubscriptionIn
	subscriptionsInMu sync.RWMutex

	refStore  RefStore
	peerStore PeerStore
}

type libp2pSubscriptionIn struct {
	domain  string
	keypath []string
	stream  netp2p.Stream
}

const (
	PROTO_MAIN protocol.ID = "/redwood/main/1.0.0"
)

func NewLibp2pTransport(addr Address, port uint, refStore RefStore, peerStore PeerStore) (Transport, error) {
	t := &libp2pTransport{
		Context:         &ctx.Context{},
		port:            port,
		address:         addr,
		subscriptionsIn: make(map[string][]libp2pSubscriptionIn),
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

func (t *libp2pTransport) Peers() []pstore.PeerInfo {
	return pstore.PeerInfos(t.libp2pHost.Peerstore(), t.libp2pHost.Peerstore().Peers())
}

func (t *libp2pTransport) SetFetchHistoryHandler(handler FetchHistoryHandler) {
	t.fetchHistoryHandler = handler
}

func (t *libp2pTransport) SetTxHandler(handler TxHandler) {
	t.txHandler = handler
}

func (t *libp2pTransport) SetPrivateTxHandler(handler PrivateTxHandler) {
	t.privateTxHandler = handler
}

func (t *libp2pTransport) SetAckHandler(handler AckHandler) {
	t.ackHandler = handler
}

func (t *libp2pTransport) SetVerifyAddressHandler(handler VerifyAddressHandler) {
	t.verifyAddressHandler = handler
}

func (t *libp2pTransport) SetFetchRefHandler(handler FetchRefHandler) {
	t.fetchRefHandler = handler
}

func (t *libp2pTransport) handleIncomingStream(stream netp2p.Stream) {
	var msg Msg
	err := ReadMsg(stream, &msg)
	if err != nil {
		panic(err)
	}

	switch msg.Type {
	case MsgType_Subscribe:
		urlStr, ok := msg.Payload.(string)
		if !ok {
			t.Errorf("Subscribe message: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}

		u, err := url.Parse(urlStr)
		if err != nil {
			t.Errorf("Subscribe message: bad url (%v): %v", urlStr, err)
			return
		}

		domain := u.Host
		keypath := strings.Split(u.Path, "/")

		t.subscriptionsInMu.Lock()
		defer t.subscriptionsInMu.Unlock()
		t.subscriptionsIn[domain] = append(t.subscriptionsIn[domain], libp2pSubscriptionIn{domain, keypath, stream})

	case MsgType_Put:
		defer stream.Close()

		tx, ok := msg.Payload.(Tx)
		if !ok {
			t.Errorf("Put message: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}

		pinfo := t.libp2pHost.Peerstore().PeerInfo(stream.Conn().RemotePeer())
		peer := &libp2pPeer{t: t, pinfo: pinfo, stream: nil} // nil so that we open a new stream
		err := peer.EnsureConnected(context.TODO())
		if err != nil {
			t.Errorf("can't connect to peer %v", pinfo.ID)
			return
		}

		t.txHandler(tx, peer)

	case MsgType_Ack:
		defer stream.Close()

		txID, ok := msg.Payload.(ID)
		if !ok {
			t.Errorf("Ack message: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}

		pinfo := t.libp2pHost.Peerstore().PeerInfo(stream.Conn().RemotePeer())
		peer := &libp2pPeer{t: t, pinfo: pinfo, stream: nil}
		err := peer.EnsureConnected(context.TODO())
		if err != nil {
			t.Errorf("can't connect to peer %v", pinfo.ID)
			return
		}

		t.ackHandler(txID, peer)

	case MsgType_VerifyAddress:
		defer stream.Close()

		challengeMsg, ok := msg.Payload.(ChallengeMsg)
		if !ok {
			t.Errorf("VerifyAddress message: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}

		pinfo := t.libp2pHost.Peerstore().PeerInfo(stream.Conn().RemotePeer())
		peer := &libp2pPeer{t: t, pinfo: pinfo, stream: stream}
		err := t.verifyAddressHandler(challengeMsg, peer)
		if err != nil {
			t.Errorf("VerifyAddress: error from verifyAddressHandler: %v", err)
			return
		}

	case MsgType_FetchRef:
		defer stream.Close()

		refHash, ok := msg.Payload.(Hash)
		if !ok {
			t.Errorf("FetchRef message: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}

		pinfo := t.libp2pHost.Peerstore().PeerInfo(stream.Conn().RemotePeer())
		peer := &libp2pPeer{t: t, pinfo: pinfo, stream: stream}
		t.fetchRefHandler(refHash, peer)

	case MsgType_Private:
		encryptedTx, ok := msg.Payload.(EncryptedTx)
		if !ok {
			t.Errorf("Private message: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}

		pinfo := t.libp2pHost.Peerstore().PeerInfo(stream.Conn().RemotePeer())
		peer := &libp2pPeer{t: t, pinfo: pinfo, stream: stream}
		t.privateTxHandler(encryptedTx, peer)

	case MsgType_AdvertisePeers:
		tuples, ok := msg.Payload.([]peerTuple)
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
	var pinfos []*pstore.PeerInfo
	for ra := range reachableAt {
		addr, err := ma.NewMultiaddr(ra)
		if err != nil {
			return nil, errors.Wrapf(err, "could not parse multiaddr '%v'", ra)
		}
		pinfo, err := pstore.InfoFromP2pAddr(addr)
		if err != nil {
			return nil, errors.Wrapf(err, "could not parse PeerInfo from multiaddr '%v'", ra)
		}
		pinfos = append(pinfos, pinfo)
	}
	for i := 1; i < len(pinfos); i++ {
		pinfos[0].Addrs = append(pinfos[0].Addrs, pinfos[i].Addrs[0])
	}
	pinfo := pinfos[0]
	return &libp2pPeer{t: t, pinfo: *pinfo}, nil
}

func (t *libp2pTransport) ForEachProviderOfURL(ctx context.Context, theURL string) (<-chan Peer, error) {
	// u, err := url.Parse(theURL)
	// if err != nil {
	// 	return errors.WithStack(err)
	// }

	urlCid, err := cidForString("serve:" + theURL)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ch := make(chan Peer)
	go func() {
		defer close(ch)
		for {
			for pinfo := range t.dht.FindProvidersAsync(ctx, urlCid, 8) {
				if pinfo.ID == t.libp2pHost.ID() {
					continue
				}

				// @@TODO: validate peer as an authorized provider via web of trust, certificate authority,
				// whitelist, etc.

				t.Infof(0, `found peer %v for url "%v"`, pinfo.ID, theURL)

				select {
				case ch <- &libp2pPeer{t: t, pinfo: pinfo, stream: nil}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return ch, nil
}

func (t *libp2pTransport) ForEachProviderOfRef(ctx context.Context, refHash Hash) (<-chan Peer, error) {
	refCid, err := cidForString("ref:" + refHash.String())
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

				t.Infof(0, `found peer %v for ref "%v"`, pinfo.ID, refHash.String())

				select {
				case ch <- &libp2pPeer{t: t, pinfo: pinfo, stream: nil}:
				case <-ctx.Done():
				}
			}
		}
	}()
	return ch, nil
}

func (t *libp2pTransport) ForEachSubscriberToURL(ctx context.Context, theURL string) (<-chan Peer, error) {
	u, err := url.Parse(theURL)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	domain := u.Host

	ch := make(chan Peer)
	go func() {
		t.subscriptionsInMu.RLock()
		defer t.subscriptionsInMu.RUnlock()
		defer close(ch)
		for _, sub := range t.subscriptionsIn[domain] {
			peerID := sub.stream.Conn().RemotePeer()
			pinfo := t.libp2pHost.Peerstore().PeerInfo(peerID)

			select {
			case ch <- &libp2pPeer{t: t, pinfo: pinfo, stream: sub.stream}:
			case <-ctx.Done():
			}
		}
	}()
	return ch, nil
}

func (t *libp2pTransport) PeersClaimingAddress(ctx context.Context, address Address) (<-chan Peer, error) {
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
			case ch <- &libp2pPeer{t: t, pinfo: pinfo, stream: nil}:
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

var URLS_TO_ADVERTISE = []string{
	"localhost:21231",
	"localhost:21241",
	"axon.science",
	"plan-systems.org",
	"braid.news",
}

// Periodically announces our repos and objects to the network.
func (t *libp2pTransport) periodicallyAnnounceContent() {
	for {
		select {
		case <-t.Ctx().Done():
			return
		default:
		}

		t.Info(0, "announce")

		// Announce the URLs we're serving
		for _, url := range URLS_TO_ADVERTISE {
			go func(url string) {
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
			}(url)
		}

		// Announce the blobs we're serving
		refHashes, err := t.refStore.AllHashes()
		if err != nil {
			t.Errorf("error fetching refStore hashes: %v", err)
		}
		for _, refHash := range refHashes {
			go func(refHash Hash) {
				err := t.AnnounceRef(refHash)
				if err != nil {
					t.Errorf("announce: error: %v", err)
				}
			}(refHash)
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
		t.peerStore.AddReachableAddresses(t.Name(), myAddrs)

		for _, pinfo := range t.Peers() {
			if pinfo.ID == peer.ID("") {
				continue
			}
			peer := &libp2pPeer{t: t, pinfo: pinfo}
			t.peerStore.AddReachableAddresses(t.Name(), peer.ReachableAt())
		}

		peerTuples := t.peerStore.PeerTuples()
		for _, pinfo := range t.Peers() {
			if pinfo.ID == t.libp2pHost.ID() {
				continue
			}
			func() {
				peer := &libp2pPeer{t: t, pinfo: pinfo}
				err := peer.EnsureConnected(context.TODO())
				if err != nil {
					t.Errorf("error connecting to peer: %+v", err)
				}
				defer peer.CloseConn()

				err = peer.WriteMsg(Msg{Type: MsgType_AdvertisePeers, Payload: peerTuples})
				if err != nil {
					t.Errorf("error writing to peer: %+v", err)
				}
			}()
		}

		time.Sleep(10 * time.Second) // @@TODO: make configurable
	}
}

func (t *libp2pTransport) AnnounceRef(refHash Hash) error {
	ctxInner, cancel := context.WithTimeout(t.Ctx(), 10*time.Second)
	defer cancel()

	c, err := cidForString("ref:" + refHash.String())
	if err != nil {
		return err
	}

	err = t.dht.Provide(ctxInner, c, true)
	if err != nil && err != kbucket.ErrLookupFailure {
		t.Errorf(`announce: could not dht.Provide refHash "%v": %v`, refHash.String(), err)
		return err
	}

	return nil
}

type libp2pPeer struct {
	t       *libp2pTransport
	pinfo   peerstore.PeerInfo
	stream  netp2p.Stream
	address Address
}

func (p *libp2pPeer) Transport() Transport {
	return p.t
}

func (p *libp2pPeer) ReachableAt() StringSet {
	reachableAt := NewStringSet(nil)
	multiaddrs, err := peerstore.InfoToP2pAddrs(&p.pinfo)
	if err != nil {
		return reachableAt
	}
	for _, addr := range multiaddrs {
		reachableAt.Add(addr.String())
	}
	return filterUselessLibp2pAddrs(reachableAt)
}

func (p *libp2pPeer) Address() Address {
	return p.address
}

func (p *libp2pPeer) SetAddress(addr Address) {
	p.address = addr
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

func (p *libp2pPeer) WriteMsg(msg Msg) error {
	return WriteMsg(p.stream, msg)
}

func (p *libp2pPeer) ReadMsg() (Msg, error) {
	var msg Msg
	err := ReadMsg(p.stream, &msg)
	return msg, err
}

func (p *libp2pPeer) CloseConn() error {
	return p.stream.Close()
}

func obtainP2PKey(addr Address) (cryptop2p.PrivKey, error) {
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

func filterUselessLibp2pAddrs(addrStrs StringSet) StringSet {
	keep := NewStringSet(nil)
	for addrStr := range addrStrs {
		if strings.Index(addrStr, "/ip4/172.") == 0 {
			continue
		} else if strings.Index(addrStr, "/ip4/0.0.0.0") == 0 {
			continue
		} else if strings.Index(addrStr, "/ip4/127.0.0.1") == 0 {
			continue
		} else if addrStr == "/p2p-circuit" {
			continue
		}
		keep.Add(addrStr)
	}
	return keep
}

type blankValidator struct{}

func (blankValidator) Validate(_ string, _ []byte) error        { return nil }
func (blankValidator) Select(_ string, _ [][]byte) (int, error) { return 0, nil }
