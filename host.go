package main

import (
	"context"

	peer "github.com/libp2p/go-libp2p-peer"
)

type Host interface {
	AddPeer(ctx context.Context, multiaddrString string) error
	RemovePeer(peerID peer.ID) error
	Peers() []pstore.PeerInfo

	BroadcastTx(ctx context.Context, tx Tx) error
}
