package main

// import (
// 	"context"
// 	"fmt"
// 	"io/ioutil"
// 	"os"
// 	"sync"
// 	"time"

// 	"github.com/pkg/errors"

// 	dstore "github.com/ipfs/go-datastore"
// 	dsync "github.com/ipfs/go-datastore/sync"
// 	libp2p "github.com/libp2p/go-libp2p"
// 	crypto "github.com/libp2p/go-libp2p-crypto"
// 	host "github.com/libp2p/go-libp2p-host"
// 	dht "github.com/libp2p/go-libp2p-kad-dht"
// 	kbucket "github.com/libp2p/go-libp2p-kbucket"
// 	metrics "github.com/libp2p/go-libp2p-metrics"
// 	netp2p "github.com/libp2p/go-libp2p-net"
// 	peer "github.com/libp2p/go-libp2p-peer"
// 	pstore "github.com/libp2p/go-libp2p-peerstore"
// 	protocol "github.com/libp2p/go-libp2p-protocol"
// 	ma "github.com/multiformats/go-multiaddr"
// )

// type host struct {
// 	libp2pHost host.Host
// 	dht        *dht.IpfsDHT
// 	store      Store
// 	*metrics.BandwidthCounter
// }

// func NewHost(ctx context.Context, store Store, port uint) (Host, error) {
// 	privkey, err := obtainP2PKey(cfg)
// 	if err != nil {
// 		return nil, err
// 	}

// 	bandwidthCounter := metrics.NewBandwidthCounter()

// 	// Initialize the libp2p host
// 	libp2pHost, err := libp2p.New(ctx,
// 		libp2p.ListenAddrStrings(
// 			// fmt.Sprintf("/ip4/%v/tcp/%v", cfg.Node.P2PListenAddr, cfg.Node.P2PListenPort),
// 			fmt.Sprintf("/ip4/0.0.0.0/tcp/%v", port),
// 		),
// 		libp2p.Identity(privkey),
// 		libp2p.BandwidthReporter(bandwidthCounter),
// 		libp2p.NATPortMap(),
// 	)
// 	if err != nil {
// 		return nil, errors.Wrap(err, "could not initialize libp2p host")
// 	}

// 	// Initialize the DHT
// 	d := dht.NewDHT(ctx, libp2pHost, dsync.MutexWrap(dstore.NewMapDatastore()))
// 	d.Validator = blankValidator{} // Set a pass-through validator

// 	h := &host{
// 		libp2pHost:       libp2pHost,
// 		dht:              d,
// 		BandwidthCounter: bandwidthCounter,
// 		store:            store,
// 	}

// 	h.libp2pHost.SetStreamHandler(MAIN_PROTO, h.handlePeerStream)

// 	// Connect to our list of bootstrap peers
// 	go func() {
// 		for _, peeraddr := range cfg.Node.BootstrapPeers {
// 			err = h.AddPeer(ctx, peeraddr)
// 			if err != nil {
// 				log.Errorf("[node] could not reach boostrap peer %v: %v", peeraddr, err)
// 			}
// 		}
// 	}()

// 	return h, nil
// }

// const (
// 	MAIN_PROTO protocol.ID = "/redwood/main/1.0.0"
// )

// func obtainP2PKey() (crypto.PrivKey, error) {
// 	const keyfile = "/tmp/redwood.key"

// 	f, err := os.Open(keyfile)
// 	if err != nil && !os.IsNotExist(err) {
// 		return nil, err

// 	} else if err == nil {
// 		defer f.Close()

// 		data, err := ioutil.ReadFile(keyfile)
// 		if err != nil {
// 			return nil, err
// 		}
// 		return crypto.UnmarshalPrivateKey(data)
// 	}

// 	privkey, _, err := crypto.GenerateKeyPair(crypto.Secp256k1, 0)
// 	if err != nil {
// 		return nil, err
// 	}

// 	bs, err := privkey.Bytes()
// 	if err != nil {
// 		return nil, err
// 	}

// 	err = ioutil.WriteFile(keyfile, bs, 0400)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return privkey, nil
// }

// type blankValidator struct{}

// func (blankValidator) Validate(_ string, _ []byte) error        { return nil }
// func (blankValidator) Select(_ string, _ [][]byte) (int, error) { return 0, nil }

// func (h *host) handlePeerStream(stream netp2p.Stream) {
// 	defer stream.Close()

// 	var tx Tx
// 	err := ReadMsg(stream, &tx)
// 	if err != nil {
// 		panic(err)
// 	}

// 	err = h.store.AddTx(tx)
// 	if err != nil {
// 		panic(err)
// 	}
// }

// // Adds a peer to the Node's address book and attempts to .Connect to it using the libp2p Host.
// func (h *host) AddPeer(ctx context.Context, multiaddrString string) error {
// 	// The following code extracts the peer ID from the given multiaddress
// 	addr, err := ma.NewMultiaddr(multiaddrString)
// 	if err != nil {
// 		return errors.Wrapf(err, "could not parse multiaddr '%v'", multiaddrString)
// 	}

// 	pinfo, err := pstore.InfoFromP2pAddr(addr)
// 	if err != nil {
// 		return errors.Wrapf(err, "could not parse PeerInfo from multiaddr '%v'", multiaddrString)
// 	}

// 	err = h.libp2pHost.Connect(ctx, *pinfo)
// 	if err != nil {
// 		return errors.Wrapf(err, "could not connect to peer '%v'", multiaddrString)
// 	}
// 	return nil
// }

// func (h *host) RemovePeer(peerID peer.ID) error {
// 	if len(h.libp2pHost.Network().ConnsToPeer(peerID)) > 0 {
// 		err := h.libp2pHost.Network().ClosePeer(peerID)
// 		if err != nil {
// 			return err
// 		}
// 	}
// 	h.libp2pHost.Peerstore().ClearAddrs(peerID)
// 	return nil
// }

// func (h *host) Peers() []pstore.PeerInfo {
// 	return pstore.PeerInfos(h.libp2pHost.Peerstore(), h.libp2pHost.Peerstore().Peers())
// }

// func (h *host) BroadcastTx(ctx context.Context, tx Tx) error {
// 	wg := &sync.WaitGroup{}

// 	for _, p := range h.Peers() {
// 		wg.Add(1)

// 		go func(peerID peer.ID) {
// 			defer wg.Done()

// 			stream, err := h.libp2pHost.NewStream(ctx, peerID, MAIN_PROTO)
// 			if err != nil {
// 				panic(err)
// 			}

// 			err = WriteMsg(stream, tx)
// 			if err != nil {
// 				panic(err)
// 			}

// 		}(p.ID)
// 	}

// 	wg.Wait()

// 	return nil
// }
