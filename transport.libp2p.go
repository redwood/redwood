package redwood

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

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
	pstore "github.com/libp2p/go-libp2p-peerstore"
	protocol "github.com/libp2p/go-libp2p-protocol"
	ma "github.com/multiformats/go-multiaddr"
	multihash "github.com/multiformats/go-multihash"
)

type libp2pTransport struct {
	ID         ID
	libp2pHost p2phost.Host
	dht        *dht.IpfsDHT
	*metrics.BandwidthCounter

	ackHandler AckHandler
	putHandler PutHandler

	subscriptionsIn  map[string][]libp2pSubscriptionIn
	subscriptionsOut map[string]subscriptionOut
}

type libp2pSubscriptionIn struct {
	domain  string
	keypath []string
	io.WriteCloser
}

const (
	PROTO_MAIN protocol.ID = "/redwood/main/1.0.0"
)

func NewLibp2pTransport(ctx context.Context, id ID, port uint) (Transport, error) {
	privkey, err := obtainP2PKey(id)
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
		ID:               id,
		libp2pHost:       libp2pHost,
		dht:              d,
		BandwidthCounter: bandwidthCounter,
		subscriptionsIn:  make(map[string][]libp2pSubscriptionIn),
		subscriptionsOut: make(map[string]subscriptionOut),
	}

	t.libp2pHost.SetStreamHandler(PROTO_MAIN, t.handleIncomingStream)

	go t.periodicallyAnnounceContent(ctx)

	// addrs := []string{}
	// for _, addr := range libp2pHost.Addrs() {
	// 	addrs = append(addrs, addr.String()+"/p2p/"+libp2pHost.ID().Pretty())
	// }
	// log.Infof("[transport %v] addrs:\n%v", id, strings.Join(addrs, "\n"))

	return t, nil
}

func (t *libp2pTransport) Libp2pPeerID() string {
	return t.libp2pHost.ID().Pretty()
}

func obtainP2PKey(id ID) (crypto.PrivKey, error) {
	keyfile := fmt.Sprintf("redwood.%v.key", id.String())

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

func (t *libp2pTransport) SetAckHandler(handler AckHandler) {
	t.ackHandler = handler
}

func (t *libp2pTransport) SetPutHandler(handler PutHandler) {
	t.putHandler = handler
}

func (t *libp2pTransport) onReceivedPut(tx Tx) {
	t.putHandler(tx)

	// Broadcast to peers
	err := t.Put(context.Background(), tx)
	if err != nil {
		panic(err)
	}
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
			log.Errorf("[transport %v] Subscribe message: bad payload: (%T) %v", t.ID, msg.Payload, msg.Payload)
			return
		}

		u, err := url.Parse(urlStr)
		if err != nil {
			log.Errorf("[transport %v] Subscribe message: bad url (%v): %v", t.ID, urlStr, err)
			return
		}

		domain := u.Hostname()
		keypath := strings.Split(u.Path, "/")

		t.subscriptionsIn[domain] = append(t.subscriptionsIn[domain], libp2pSubscriptionIn{domain, keypath, stream})

	case MsgType_Put:
		defer stream.Close()

		tx, ok := msg.Payload.(Tx)
		if !ok {
			log.Errorf("[transport %v] Put message: bad payload: (%T) %v", t.ID, msg.Payload, msg.Payload)
			return
		}

		t.onReceivedPut(tx)

		// ACK the PUT
		err := t.Ack(context.TODO(), tx.URL, tx.ID)
		if err != nil {
			log.Errorf("[transport %v] error ACKing a PUT: %v", err)
			return
		}

	case MsgType_Ack:
		defer stream.Close()

		// versionID, ok := msg.Payload.(ID)
		// if !ok {
		// 	log.Errorf("[transport %v] Ack message: bad payload: (%T) %v", t.ID, msg.Payload, msg.Payload)
		// 	return
		// }

		// t.Ack(ctx, )

	default:
		panic("protocol error")
	}
}

func (t *libp2pTransport) AddPeer(ctx context.Context, multiaddrString string) error {
	// The following code extracts the peer ID from the given multiaddress
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

	log.Infof("[transport %v] connected to peer", t.ID)

	return nil
}

func (t *libp2pTransport) Subscribe(ctx context.Context, url string) error {
	_, exists := t.subscriptionsOut[url]
	if exists {
		return errors.New("already subscribed to " + url)
	}

	var peerInfo pstore.PeerInfo

	// @@TODO: subscribe to more than one peer?
	err := t.forEachProviderOfURL(ctx, url, func(pinfo pstore.PeerInfo) (bool, error) {
		if len(t.libp2pHost.Network().ConnsToPeer(pinfo.ID)) == 0 {
			err := t.libp2pHost.Connect(ctx, pinfo)
			if err != nil {
				return false, errors.Wrapf(err, "could not connect to peer %+v", pinfo)
			}
		}
		peerInfo = pinfo
		return false, nil
	})
	if err != nil {
		return errors.WithStack(err)
	}

	if peerInfo.ID == "" {
		return errors.New("cannot find provider for Subscribe url")
	}

	stream, err := t.libp2pHost.NewStream(ctx, peerInfo.ID, PROTO_MAIN)
	if err != nil {
		return errors.WithStack(err)
	}

	err = WriteMsg(stream, Msg{Type: MsgType_Subscribe, Payload: url})
	if err != nil {
		return errors.WithStack(err)
	}

	chDone := make(chan struct{})
	t.subscriptionsOut[url] = subscriptionOut{stream, chDone}

	go func() {
		defer stream.Close()
		for {
			select {
			case <-chDone:
				return
			default:
			}

			var msg Msg
			err := ReadMsg(stream, &msg)
			if err != nil {
				log.Errorln("xyzzy", err)
				return
			}

			if msg.Type != MsgType_Put {
				panic("protocol error")
			}

			tx := msg.Payload.(Tx)
			tx.URL = url
			t.onReceivedPut(tx)

			// @@TODO: ACK the PUT
		}
	}()

	return nil
}

func (t *libp2pTransport) Ack(ctx context.Context, url string, versionID ID) error {
	return t.forEachProviderOfURL(ctx, url, func(pinfo pstore.PeerInfo) (bool, error) {
		if len(t.libp2pHost.Network().ConnsToPeer(pinfo.ID)) == 0 {
			err := t.libp2pHost.Connect(ctx, pinfo)
			if err != nil {
				return false, errors.Wrapf(err, "could not connect to peer %+v", pinfo)
			}
		}

		stream, err := t.libp2pHost.NewStream(ctx, pinfo.ID, PROTO_MAIN)
		if err != nil {
			return false, errors.WithStack(err)
		}
		defer stream.Close()

		err = WriteMsg(stream, Msg{Type: MsgType_Ack, Payload: versionID})
		if err != nil {
			return false, errors.WithStack(err)
		}

		return true, nil
	})
}

func (t *libp2pTransport) Put(ctx context.Context, tx Tx) error {

	// @@TODO: should we also send all PUTs to some set of authoritative peers (like a central server)?

	u, err := url.Parse(tx.URL)
	if err != nil {
		return errors.WithStack(err)
	}

	// @@TODO: do we need to trim the tx's patches' keypaths so that they don't include
	// the keypath that the subscription is listening to?

	for _, subscriber := range t.subscriptionsIn[u.Hostname()] {
		err := WriteMsg(subscriber, Msg{Type: MsgType_Put, Payload: tx})
		if err != nil {
			fmt.Printf("ZZZZZZZ ~> %+v\n", tx)
			return errors.WithStack(err)
		}

	}
	return nil
}

func (t *libp2pTransport) forEachProviderOfURL(ctx context.Context, theURL string, fn func(pstore.PeerInfo) (bool, error)) error {
	u, err := url.Parse(theURL)
	if err != nil {
		return errors.WithStack(err)
	}

	urlCid, err := cidForString("serve:" + u.Hostname())
	if err != nil {
		return errors.WithStack(err)
	}

	for pinfo := range t.dht.FindProvidersAsync(ctx, urlCid, 8) {
		if pinfo.ID == t.libp2pHost.ID() {
			continue
		}
		log.Infof(`[transport %v] found peer %v for url "%v"`, t.ID, pinfo.ID, theURL)

		keepGoing, err := fn(pinfo)
		if err != nil {
			return errors.WithStack(err)
		} else if !keepGoing {
			break
		}
	}
	return nil
}

func (t *libp2pTransport) Peers() []pstore.PeerInfo {
	return pstore.PeerInfos(t.libp2pHost.Peerstore(), t.libp2pHost.Peerstore().Peers())
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

		log.Debugf("[transport] announce")

		// Announce the URLs we're serving
		for _, url := range URLS_TO_ADVERTISE {
			func() {
				ctxInner, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()

				c, err := cidForString("serve:" + url)
				if err != nil {
					log.Errorf("[transport] announce: error creating cid: %v", err)
					return
				}

				err = t.dht.Provide(ctxInner, c, true)
				if err != nil && err != kbucket.ErrLookupFailure {
					log.Errorf(`[transport] announce: could not dht.Provide url "%v": %v`, url, err)
					return
				}
			}()
		}

		time.Sleep(10 * time.Second)
	}
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
