package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
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
	ID                    ID
	libp2pHost            p2phost.Host
	dht                   *dht.IpfsDHT
	incomingStreamHandler func(stream io.ReadWriteCloser)
	*metrics.BandwidthCounter
}

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
	}

	t.libp2pHost.SetStreamHandler(PROTO_MAIN, func(stream netp2p.Stream) {
		t.incomingStreamHandler(stream)
	})

	go t.periodicallyAnnounceContent(ctx)

	// addrs := []string{}
	// for _, addr := range libp2pHost.Addrs() {
	// 	addrs = append(addrs, addr.String()+"/p2p/"+libp2pHost.ID().Pretty())
	// }
	// log.Infof("[transport %v] addrs:\n%v", id, strings.Join(addrs, "\n"))

	return t, nil
}

const (
	PROTO_MAIN protocol.ID = "/redwood/main/1.0.0"
	// PROTO_READ  protocol.ID = "/redwood/read/1.0.0"
	// PROTO_WRITE protocol.ID = "/redwood/write/1.0.0"
)

func obtainP2PKey(id ID) (crypto.PrivKey, error) {
	keyfile := fmt.Sprintf("/tmp/redwood.%v.key", id.String())

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

func (t *libp2pTransport) OnIncomingStream(handler func(stream io.ReadWriteCloser)) {
	t.incomingStreamHandler = handler
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

func (t *libp2pTransport) GetPeerConn(ctx context.Context, url string) (io.ReadWriteCloser, error) {
	urlCid, err := cidForString("serve:" + url)
	if err != nil {
		log.Errorf("[transport %v] GetPeerConn: error creating cid: %v", t.ID, err)
		return nil, err
	}

	var peerInfo pstore.PeerInfo
	for pinfo := range t.dht.FindProvidersAsync(ctx, urlCid, 8) {
		if pinfo.ID == t.libp2pHost.ID() {
			continue
		}
		peerInfo = pinfo
		log.Infof(`[transport %v] found peer %v for url "%v"`, t.ID, pinfo.ID, url)
		break
	}

	// peerinfo, err := peerstore.InfoFromP2pAddr(multiaddr)
	// if err != nil {
	// 	return nil, errors.Wrapf(err, "could not parse PeerInfo from multiaddr '%v'", multiaddr.String())
	// }

	if len(t.libp2pHost.Network().ConnsToPeer(peerInfo.ID)) == 0 {
		err = t.libp2pHost.Connect(ctx, peerInfo)
		if err != nil {
			return nil, errors.Wrapf(err, "could not connect to peer %+v", peerInfo)
		}
	}

	return t.libp2pHost.NewStream(ctx, peerInfo.ID, PROTO_MAIN)
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
