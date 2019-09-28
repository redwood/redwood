package main

// import (
// 	"context"
// 	"fmt"
// 	"io/ioutil"
// 	"os"
// 	"sync"
// 	"time"

// 	"github.com/pkg/errors"

// 	"github.com/libp2p/go-libp2p-peerstore"
// 	ma "github.com/multiformats/go-multiaddr"
// )

// type httpTransport struct {
// 	libp2pHost host.Host
// 	*metrics.BandwidthCounter
// }

// func NewHTTPTransport(ctx context.Context, port uint, onRecvTxHandler func(Tx)) (Transport, error) {
// 	t := &httpTransport{
// 		onRecvTxHandler: onRecvTxHandler,
// 	}

// 	return t, nil
// }

// func (t *httpTransport) handlePeerStream(stream netp2p.Stream) {
// 	defer stream.Close()

// 	tx, err := handleIncomingTx(stream)
// 	if err != nil {
// 		panic(err)
// 	}

// 	t.onRecvTxHandler(tx)
// }

// func (t *httpTransport) OpenStream(ctx context.Context, multiaddr ma.Multiaddr) (io.ReadWriteCloser, error) {
// 	peerinfo, err := peerstore.InfoFromP2pAddr(multiaddr)
// 	if err != nil {
// 		return nil, errors.Wrapf(err, "could not parse PeerInfo from multiaddr '%v'", multiaddr.String())
// 	}

// 	if len(h.libp2pHost.Network().ConnsToPeer(peerinfo.ID)) == 0 {
// 		err = t.libp2pHost.Connect(ctx, *peerinfo)
// 		if err != nil {
// 			return nil, errors.Wrapf(err, "could not connect to peer '%v'", multiaddr.String())
// 		}
// 	}

// 	return t.libp2pHost.NewStream(ctx, peerinfo.ID, MAIN_PROTO)
// }
