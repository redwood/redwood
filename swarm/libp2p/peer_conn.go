package libp2p

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"sync"
	"time"

	netp2p "github.com/libp2p/go-libp2p-core/network"
	peerstore "github.com/libp2p/go-libp2p-peerstore"

	"github.com/pkg/errors"

	"redwood.dev/blob"
	"redwood.dev/state"
	"redwood.dev/swarm"
	"redwood.dev/swarm/protoauth"
	"redwood.dev/swarm/protoblob"
	"redwood.dev/swarm/prototree"
	"redwood.dev/tree"
	"redwood.dev/types"
)

type peerConn struct {
	swarm.PeerDetails
	t      *transport
	pinfo  peerstore.PeerInfo
	stream netp2p.Stream
	mu     sync.Mutex
}

var (
	_ protoauth.AuthPeerConn = (*peerConn)(nil)
	_ protoblob.BlobPeerConn = (*peerConn)(nil)
	_ prototree.TreePeerConn = (*peerConn)(nil)
)

func (peer *peerConn) Transport() swarm.Transport {
	return peer.t
}

func (peer *peerConn) EnsureConnected(ctx context.Context) (err error) {
	if peer.stream == nil {
		defer func() { peer.UpdateConnStats(err == nil) }()

		if len(peer.t.libp2pHost.Network().ConnsToPeer(peer.pinfo.ID)) == 0 {
			err = peer.t.libp2pHost.Connect(ctx, peer.pinfo)
			if err != nil {
				return errors.Wrapf(types.ErrConnection, "(peer %v): %v", peer.pinfo.ID, err)
			}
		}

		var stream netp2p.Stream
		stream, err = peer.t.libp2pHost.NewStream(ctx, peer.pinfo.ID, PROTO_MAIN)
		if err != nil {
			return err
		}

		peer.stream = stream
	}
	return nil
}

func (peer *peerConn) Subscribe(ctx context.Context, stateURI string) (_ prototree.ReadableSubscription, err error) {
	defer func() { peer.UpdateConnStats(err == nil) }()

	err = peer.EnsureConnected(ctx)
	if err != nil {
		peer.t.Errorf("error connecting to peer: %v", err)
		return nil, err
	}

	err = peer.writeMsg(Msg{Type: msgType_Subscribe, Payload: stateURI})
	if err != nil {
		return nil, err
	}

	return &readableSubscription{peer}, nil
}

func (peer *peerConn) Put(ctx context.Context, tx *tree.Tx, state state.Node, leaves []types.ID) error {
	// Note: libp2p peers ignore `state` and `leaves`
	if tx.IsPrivate() {
		marshalledTx, err := json.Marshal(tx)
		if err != nil {
			return errors.WithStack(err)
		}

		peerAddrs := types.OverlappingAddresses(tx.Recipients, peer.Addresses())
		if len(peerAddrs) == 0 {
			return errors.New("tx not intended for this peer")
		}
		peerSigPubkey, peerEncPubkey := peer.PublicKeys(peerAddrs[0])

		identity, err := peer.t.keyStore.DefaultPublicIdentity()
		if err != nil {
			return errors.WithStack(err)
		}

		encryptedTxBytes, err := peer.t.keyStore.SealMessageFor(identity.Address(), peerEncPubkey, marshalledTx)
		if err != nil {
			return errors.WithStack(err)
		}

		etx := prototree.EncryptedTx{
			TxID:             tx.ID,
			EncryptedPayload: encryptedTxBytes,
			SenderPublicKey:  identity.Encrypting.EncryptingPublicKey.Bytes(),
			RecipientAddress: peerSigPubkey.Address(),
		}
		return peer.writeMsg(Msg{Type: msgType_EncryptedTx, Payload: etx})
	}
	return peer.writeMsg(Msg{Type: msgType_Tx, Payload: tx})
}

func (p *peerConn) Ack(stateURI string, txID types.ID) error {
	return p.writeMsg(Msg{Type: msgType_Ack, Payload: ackMsg{stateURI, txID}})
}

func (p *peerConn) ChallengeIdentity(challengeMsg protoauth.ChallengeMsg) error {
	return p.writeMsg(Msg{Type: msgType_ChallengeIdentityRequest, Payload: challengeMsg})
}

func (p *peerConn) ReceiveChallengeIdentityResponse() ([]protoauth.ChallengeIdentityResponse, error) {
	msg, err := p.readMsg()
	if err != nil {
		return nil, err
	}
	resp, ok := msg.Payload.([]protoauth.ChallengeIdentityResponse)
	if !ok {
		return nil, swarm.ErrProtocol
	}
	return resp, nil
}

func (p *peerConn) RespondChallengeIdentity(challengeIdentityResponse []protoauth.ChallengeIdentityResponse) error {
	return p.writeMsg(Msg{Type: msgType_ChallengeIdentityResponse, Payload: challengeIdentityResponse})
}

func (p *peerConn) FetchBlob(refID blob.ID) error {
	return p.writeMsg(Msg{Type: msgType_FetchBlob, Payload: refID})
}

func (p *peerConn) SendBlobHeader(haveBlob bool) error {
	return p.writeMsg(Msg{Type: msgType_FetchBlobResponse, Payload: protoblob.FetchBlobResponse{Header: &protoblob.FetchBlobResponseHeader{}}})
}

func (p *peerConn) SendBlobPacket(data []byte, end bool) error {
	return p.writeMsg(Msg{Type: msgType_FetchBlobResponse, Payload: protoblob.FetchBlobResponse{Body: &protoblob.FetchBlobResponseBody{Data: data, End: end}}})
}

func (p *peerConn) ReceiveBlobPacket() (protoblob.FetchBlobResponseBody, error) {
	msg, err := p.readMsg()
	if err != nil {
		return protoblob.FetchBlobResponseBody{}, errors.Errorf("error reading from peer: %v", err)
	} else if msg.Type != msgType_FetchBlobResponse {
		return protoblob.FetchBlobResponseBody{}, swarm.ErrProtocol
	}

	resp, is := msg.Payload.(protoblob.FetchBlobResponse)
	if !is {
		return protoblob.FetchBlobResponseBody{}, swarm.ErrProtocol
	} else if resp.Body == nil {
		return protoblob.FetchBlobResponseBody{}, swarm.ErrProtocol
	}
	return *resp.Body, nil
}

func (p *peerConn) ReceiveBlobHeader() (protoblob.FetchBlobResponseHeader, error) {
	msg, err := p.readMsg()
	if err != nil {
		return protoblob.FetchBlobResponseHeader{}, errors.Errorf("error reading from peer: %v", err)
	} else if msg.Type != msgType_FetchBlobResponse {
		return protoblob.FetchBlobResponseHeader{}, swarm.ErrProtocol
	}

	resp, is := msg.Payload.(protoblob.FetchBlobResponse)
	if !is {
		return protoblob.FetchBlobResponseHeader{}, swarm.ErrProtocol
	} else if resp.Header == nil {
		return protoblob.FetchBlobResponseHeader{}, swarm.ErrProtocol
	}
	return *resp.Header, nil
}

func (p *peerConn) AnnouncePeers(ctx context.Context, peerDialInfos []swarm.PeerDialInfo) error {
	return p.writeMsg(Msg{Type: msgType_AnnouncePeers, Payload: peerDialInfos})
}

func (p *peerConn) writeMsg(msg Msg) (err error) {
	defer func() { p.UpdateConnStats(err == nil) }()

	bs, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	buflen := uint64(len(bs))

	err = p.stream.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err != nil {
		return err
	}

	err = writeUint64(p.stream, buflen)
	if err != nil {
		return err
	}

	n, err := io.Copy(p.stream, bytes.NewReader(bs))
	if err != nil {
		return err
	} else if n != int64(buflen) {
		return errors.New("WriteMsg: could not write entire packet")
	}
	return nil
}

func (p *peerConn) readMsg() (msg Msg, err error) {
	defer func() { p.UpdateConnStats(err == nil) }()
	p.stream.SetReadDeadline(time.Now().Add(10 * time.Second))
	return readMsg(p.stream)
}

func (p *peerConn) Close() error {
	if p.stream != nil {
		return p.stream.Close()
	}
	return nil
}
