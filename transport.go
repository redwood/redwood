package main

import (
	"context"
	"io"

	peerstore "github.com/libp2p/go-libp2p-peerstore"
	// ma "github.com/multiformats/go-multiaddr"
)

type Transport interface {
	// OpenStream(ctx context.Context, multiaddr ma.Multiaddr) (io.ReadWriteCloser, error)
	GetPeerConn(ctx context.Context, url string) (io.ReadWriteCloser, error)
	OnIncomingStream(handler func(stream io.ReadWriteCloser))
	AddPeer(ctx context.Context, multiaddrString string) error
	Peers() []peerstore.PeerInfo
}
