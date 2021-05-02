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

	"redwood.dev/state"
	"redwood.dev/swarm"
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

func (peer *peerConn) Subscribe(ctx context.Context, stateURI string) (_ swarm.ReadableSubscription, err error) {
	defer func() { peer.UpdateConnStats(err == nil) }()

	err = peer.EnsureConnected(ctx)
	if err != nil {
		peer.t.Errorf("error connecting to peer: %v", err)
		return nil, err
	}

	err = peer.writeMsg(Msg{Type: MsgType_Subscribe, Payload: stateURI})
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

		etx := swarm.EncryptedTx{
			TxID:             tx.ID,
			EncryptedPayload: encryptedTxBytes,
			SenderPublicKey:  identity.Encrypting.EncryptingPublicKey.Bytes(),
			RecipientAddress: peerSigPubkey.Address(),
		}
		return peer.writeMsg(Msg{Type: MsgType_Private, Payload: etx})
	}
	return peer.writeMsg(Msg{Type: MsgType_Put, Payload: tx})
}

func (p *peerConn) Ack(stateURI string, txID types.ID) error {
	return p.writeMsg(Msg{Type: MsgType_Ack, Payload: swarm.AckMsg{stateURI, txID}})
}

func (p *peerConn) ChallengeIdentity(challengeMsg types.ChallengeMsg) error {
	return p.writeMsg(Msg{Type: MsgType_ChallengeIdentityRequest, Payload: challengeMsg})
}

func (p *peerConn) ReceiveChallengeIdentityResponse() ([]swarm.ChallengeIdentityResponse, error) {
	msg, err := p.readMsg()
	if err != nil {
		return nil, err
	}
	resp, ok := msg.Payload.([]swarm.ChallengeIdentityResponse)
	if !ok {
		return nil, swarm.ErrProtocol
	}
	return resp, nil
}

func (p *peerConn) RespondChallengeIdentity(challengeIdentityResponse []swarm.ChallengeIdentityResponse) error {
	return p.writeMsg(Msg{Type: MsgType_ChallengeIdentityResponse, Payload: challengeIdentityResponse})
}

func (p *peerConn) FetchRef(refID types.RefID) error {
	return p.writeMsg(Msg{Type: MsgType_FetchRef, Payload: refID})
}

func (p *peerConn) SendRefHeader() error {
	return p.writeMsg(Msg{Type: MsgType_FetchRefResponse, Payload: swarm.FetchRefResponse{Header: &swarm.FetchRefResponseHeader{}}})
}

func (p *peerConn) SendRefPacket(data []byte, end bool) error {
	return p.writeMsg(Msg{Type: MsgType_FetchRefResponse, Payload: swarm.FetchRefResponse{Body: &swarm.FetchRefResponseBody{Data: data, End: end}}})
}

func (p *peerConn) ReceiveRefPacket() (swarm.FetchRefResponseBody, error) {
	msg, err := p.readMsg()
	if err != nil {
		return swarm.FetchRefResponseBody{}, errors.Errorf("error reading from peer: %v", err)
	} else if msg.Type != MsgType_FetchRefResponse {
		return swarm.FetchRefResponseBody{}, swarm.ErrProtocol
	}

	resp, is := msg.Payload.(swarm.FetchRefResponse)
	if !is {
		return swarm.FetchRefResponseBody{}, swarm.ErrProtocol
	} else if resp.Body == nil {
		return swarm.FetchRefResponseBody{}, swarm.ErrProtocol
	}
	return *resp.Body, nil
}

func (p *peerConn) ReceiveRefHeader() (swarm.FetchRefResponseHeader, error) {
	msg, err := p.readMsg()
	if err != nil {
		return swarm.FetchRefResponseHeader{}, errors.Errorf("error reading from peer: %v", err)
	} else if msg.Type != MsgType_FetchRefResponse {
		return swarm.FetchRefResponseHeader{}, swarm.ErrProtocol
	}

	resp, is := msg.Payload.(swarm.FetchRefResponse)
	if !is {
		return swarm.FetchRefResponseHeader{}, swarm.ErrProtocol
	} else if resp.Header == nil {
		return swarm.FetchRefResponseHeader{}, swarm.ErrProtocol
	}
	return *resp.Header, nil
}

func (p *peerConn) AnnouncePeers(ctx context.Context, peerDialInfos []swarm.PeerDialInfo) error {
	return p.writeMsg(Msg{Type: MsgType_AnnouncePeers, Payload: peerDialInfos})
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
	return readMsg(p.stream)
}

func (p *peerConn) Close() error {
	if p.stream != nil {
		return p.stream.Close()
	}
	return nil
}
