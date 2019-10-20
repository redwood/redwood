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
	crypto "github.com/libp2p/go-libp2p-crypto"
	p2phost "github.com/libp2p/go-libp2p-host"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	kbucket "github.com/libp2p/go-libp2p-kbucket"
	metrics "github.com/libp2p/go-libp2p-metrics"
	netp2p "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	protocol "github.com/libp2p/go-libp2p-protocol"
	ma "github.com/multiformats/go-multiaddr"
	multihash "github.com/multiformats/go-multihash"

	"github.com/plan-systems/plan-core/tools/ctx"
)

type libp2pTransport struct {
	ctx.Context

	libp2pHost p2phost.Host
	dht        *dht.IpfsDHT
	*metrics.BandwidthCounter

	address Address

	putHandler           PutHandler
	ackHandler           AckHandler
	verifyAddressHandler VerifyAddressHandler

	subscriptionsIn   map[string][]libp2pSubscriptionIn
	subscriptionsInMu sync.RWMutex
}

type libp2pSubscriptionIn struct {
	domain  string
	keypath []string
	stream  netp2p.Stream
}

const (
	PROTO_MAIN protocol.ID = "/redwood/main/1.0.0"
)

func NewLibp2pTransport(ctx context.Context, addr Address, port uint) (Transport, error) {
	privkey, err := obtainP2PKey(addr)
	if err != nil {
		return nil, err
	}

	bandwidthCounter := metrics.NewBandwidthCounter()

	// Initialize the libp2p host
	libp2pHost, err := libp2p.New(ctx,
		libp2p.ListenAddrStrings(
			// fmt.Sprintf("/ip4/%v/tcp/%v", cfg.Node.P2PListenAddr, cfg.Node.P2PListenPort),
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%v", port),
		),
		libp2p.Identity(privkey),
		libp2p.BandwidthReporter(bandwidthCounter),
		libp2p.NATPortMap(),
	)
	if err != nil {
		return nil, errors.Wrap(err, "could not initialize libp2p host")
	}

	// Initialize the DHT
	d := dht.NewDHT(ctx, libp2pHost, dsync.MutexWrap(dstore.NewMapDatastore()))
	d.Validator = blankValidator{} // Set a pass-through validator

	// Set up the transport
	t := &libp2pTransport{
		libp2pHost:       libp2pHost,
		dht:              d,
		BandwidthCounter: bandwidthCounter,
		address:          addr,
		subscriptionsIn:  make(map[string][]libp2pSubscriptionIn),
	}

	t.libp2pHost.SetStreamHandler(PROTO_MAIN, t.handleIncomingStream)

	err = t.CtxStart(
		// on startup
		func() error {
			t.SetLogLabel(addr.Pretty() + " transport")
			go t.periodicallyAnnounceContent(t.Ctx)
			return nil
		},
		nil,
		nil,
		nil,
	)

	return t, err
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

func (t *libp2pTransport) SetPutHandler(handler PutHandler) {
	t.putHandler = handler
}

func (t *libp2pTransport) SetAckHandler(handler AckHandler) {
	t.ackHandler = handler
}

func (t *libp2pTransport) SetVerifyAddressHandler(handler VerifyAddressHandler) {
	t.verifyAddressHandler = handler
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

		t.putHandler(tx)

	case MsgType_Ack:
		defer stream.Close()

		versionID, ok := msg.Payload.(ID)
		if !ok {
			t.Errorf("Ack message: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}

		t.ackHandler(versionID)

	case MsgType_VerifyAddress:
		defer stream.Close()

		challengeMsg, ok := msg.Payload.([]byte)
		if !ok {
			t.Errorf("VerifyAddress message: bad payload: (%T) %v", msg.Payload, msg.Payload)
			return
		}

		resp, err := t.verifyAddressHandler(challengeMsg)
		if err != nil {
			t.Errorf("VerifyAddress: error from verifyAddressHandler: %v", err)
			return
		}

		err = WriteMsg(stream, Msg{Type: MsgType_VerifyAddressResponse, Payload: resp})
		if err != nil {
			t.Errorf("VerifyAddress: error writing response: %v", err)
			return
		}

	default:
		panic("protocol error")
	}
}

func (t *libp2pTransport) AddPeer(ctx context.Context, multiaddrString string) error {
	addr, err := ma.NewMultiaddr(multiaddrString)
	if err != nil {
		return errors.Wrapf(err, "could not parse multiaddr '%v'", multiaddrString)
	}

	pinfo, err := pstore.InfoFromP2pAddr(addr)
	if err != nil {
		return errors.Wrapf(err, "could not parse PeerInfo from multiaddr '%v'", multiaddrString)
	}

	err = t.libp2pHost.Connect(ctx, *pinfo)
	if err != nil {
		return errors.Wrapf(err, "could not connect to peer '%v'", multiaddrString)
	}

	t.Infof(0, "connected to %v", pinfo.ID)

	return nil
}

func (t *libp2pTransport) ForEachProviderOfURL(ctx context.Context, theURL string, fn func(Peer) (bool, error)) error {
	u, err := url.Parse(theURL)
	if err != nil {
		return errors.WithStack(err)
	}

	urlCid, err := cidForString("serve:" + u.Host)
	if err != nil {
		return errors.WithStack(err)
	}

	for pinfo := range t.dht.FindProvidersAsync(ctx, urlCid, 8) {
		if pinfo.ID == t.libp2pHost.ID() {
			continue
		}

		// @@TODO: validate peer as an authorized provider via web of trust, certificate authority,
		// whitelist, etc.

		t.Infof(0, `found peer %v for url "%v"`, pinfo.ID, theURL)

		keepGoing, err := fn(&libp2pPeer{t, pinfo.ID, nil})
		if err != nil {
			return errors.WithStack(err)
		} else if !keepGoing {
			break
		}
	}
	return nil
}

func (t *libp2pTransport) ForEachSubscriberToURL(ctx context.Context, theURL string, fn func(Peer) (bool, error)) error {
	u, err := url.Parse(theURL)
	if err != nil {
		return errors.WithStack(err)
	}

	domain := u.Host

	t.subscriptionsInMu.RLock()
	defer t.subscriptionsInMu.RUnlock()

	for _, sub := range t.subscriptionsIn[domain] {
		keepGoing, err := fn(&libp2pPeer{t, sub.stream.Conn().RemotePeer(), sub.stream})
		if err != nil {
			return errors.WithStack(err)
		} else if !keepGoing {
			break
		}
	}
	return nil
}

func (t *libp2pTransport) PeersWithAddress(ctx context.Context, address Address) (<-chan Peer, error) {
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
			case <-t.Ctx.Done():
				return
			case ch <- &libp2pPeer{t, pinfo.ID, nil}:
			}
		}
	}()

	return ch, nil
}

func (t *libp2pTransport) ensureConnected(ctx context.Context, peerID peer.ID) error {
	if len(t.libp2pHost.Network().ConnsToPeer(peerID)) == 0 {
		err := t.libp2pHost.Connect(ctx, t.libp2pHost.Peerstore().PeerInfo(peerID))
		if err != nil {
			return errors.Wrapf(err, "could not connect to peer %v", peerID)
		}
	}
	return nil
}

var URLS_TO_ADVERTISE = []string{
	"axon.science",
	"plan-systems.org",
	"braid.news",
}

// Periodically announces our repos and objects to the network.
func (t *libp2pTransport) periodicallyAnnounceContent(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		t.Info(0, "announce")

		// Announce the URLs we're serving
		for _, url := range URLS_TO_ADVERTISE {
			func() {
				ctxInner, cancel := context.WithTimeout(ctx, 10*time.Second)
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

		// Advertise our address (for exchanging private txs)
		func() {
			ctxInner, cancel := context.WithTimeout(ctx, 10*time.Second)
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

type libp2pPeer struct {
	t      *libp2pTransport
	peerID peer.ID
	stream netp2p.Stream
}

func (p *libp2pPeer) EnsureConnected(ctx context.Context) error {
	if p.stream == nil {
		err := p.t.ensureConnected(ctx, p.peerID)
		if err != nil {
			return err
		}

		stream, err := p.t.libp2pHost.NewStream(ctx, p.peerID, PROTO_MAIN)
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

func obtainP2PKey(addr Address) (crypto.PrivKey, error) {
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
		return crypto.UnmarshalPrivateKey(data)
	}

	privkey, _, err := crypto.GenerateKeyPair(crypto.Secp256k1, 0)
	if err != nil {
		return nil, err
	}

	bs, err := privkey.Bytes()
	if err != nil {
		return nil, err
	}

	err = ioutil.WriteFile(keyfile, bs, 0400)
	if err != nil {
		return nil, err
	}

	return privkey, nil
}

func cidForString(s string) (cid.Cid, error) {
	pref := cid.NewPrefixV1(cid.Raw, multihash.SHA2_256)
	c, err := pref.Sum([]byte(s))
	if err != nil {
		return cid.Cid{}, errors.Wrap(err, "could not create cid")
	}
	return c, nil
}

type blankValidator struct{}

func (blankValidator) Validate(_ string, _ []byte) error        { return nil }
func (blankValidator) Select(_ string, _ [][]byte) (int, error) { return 0, nil }
