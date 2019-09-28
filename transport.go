package main

import (
	"context"
	"io"

	peerstore "github.com/libp2p/go-libp2p-peerstore"
	// ma "github.com/multiformats/go-multiaddr"
)

type Transport interface {
	GetPeerConn(ctx context.Context, url string) (io.ReadWriteCloser, error)
	OnIncomingStream(handler func(stream io.ReadWriteCloser))
	AddPeer(ctx context.Context, multiaddrString string) error
	Peers() []peerstore.PeerInfo
}
