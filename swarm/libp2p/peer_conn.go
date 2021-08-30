package libp2p

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"sync"

	netp2p "github.com/libp2p/go-libp2p-core/network"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	protocol "github.com/libp2p/go-libp2p-protocol"
	"github.com/pkg/errors"

	"redwood.dev/blob"
	"redwood.dev/state"
	"redwood.dev/swarm"
	"redwood.dev/swarm/libp2p/pb"
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

func (peer *peerConn) DeviceSpecificID() string {
	return peer.pinfo.ID.Pretty()
}

func (peer *peerConn) Transport() swarm.Transport {
	return peer.t
}

func (peer *peerConn) EnsureConnected(ctx context.Context) (err error) {
	defer func() { peer.UpdateConnStats(err == nil) }()

	err = peer.t.libp2pHost.Connect(ctx, peer.pinfo)
	if err != nil {
		return errors.Wrapf(types.ErrConnection, "(peer %v): %v", peer.pinfo.ID, err)
	}
	return nil
}

func (peer *peerConn) ensureStreamWithProtocol(ctx context.Context, p protocol.ID) (err error) {
	if peer.stream != nil {
		if peer.stream.Protocol() != p {
			return errors.Wrapf(ErrWrongProtocol, "got %v", p)
		}
		return nil
	}

	peer.stream, err = peer.t.libp2pHost.NewStream(ctx, peer.pinfo.ID, p)
	if err != nil {
		return errors.Wrapf(types.ErrConnection, "(peer %v): %v", peer.pinfo.ID, err)
	}
	return nil
}

func (peer *peerConn) Subscribe(ctx context.Context, stateURI string) (prototree.ReadableSubscription, error) {
	err := peer.EnsureConnected(ctx)
	if err != nil {
		return nil, err
	}

	err = peer.ensureStreamWithProtocol(ctx, PROTO_MAIN)
	if err != nil {
		return nil, err
	}

	leaves, err := peer.t.controllerHub.Leaves(stateURI)
	if err != nil {
		return nil, errors.Errorf("while subscribing: couldn't fetch leaves for state URI %v: %v", stateURI, err)
	}

	err = peer.writeMsg(Msg{Type: msgType_Subscribe, Payload: subscribeMsg{StateURI: stateURI, FromTxIDs: leaves.Slice()}})
	if err != nil {
		return nil, err
	}

	return &readableSubscription{peer}, nil
}

func (peer *peerConn) Put(ctx context.Context, tx *tree.Tx, state state.Node, leaves []types.ID) error {
	err := peer.ensureStreamWithProtocol(ctx, PROTO_MAIN)
	if err != nil {
		return err
	}

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
			SenderPublicKey:  identity.AsymEncKeypair.AsymEncPubkey.Bytes(),
			RecipientAddress: peerSigPubkey.Address(),
		}
		return peer.writeMsg(Msg{Type: msgType_EncryptedTx, Payload: etx})
	}
	return peer.writeMsg(Msg{Type: msgType_Tx, Payload: tx})
}

func (peer *peerConn) Ack(stateURI string, txID types.ID) (err error) {
	err = peer.ensureStreamWithProtocol(peer.t.Process.Ctx(), PROTO_MAIN)
	if err != nil {
		return err
	}
	return peer.writeMsg(Msg{Type: msgType_Ack, Payload: ackMsg{stateURI, txID}})
}

func (peer *peerConn) ChallengeIdentity(challengeMsg protoauth.ChallengeMsg) error {
	err := peer.ensureStreamWithProtocol(peer.t.Process.Ctx(), PROTO_MAIN)
	if err != nil {
		return err
	}
	return peer.writeMsg(Msg{Type: msgType_ChallengeIdentityRequest, Payload: challengeMsg})
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

func (peer *peerConn) RespondChallengeIdentity(challengeIdentityResponse []protoauth.ChallengeIdentityResponse) error {
	return peer.writeMsg(Msg{Type: msgType_ChallengeIdentityResponse, Payload: challengeIdentityResponse})
}

func (peer *peerConn) FetchBlobManifest(blobID blob.ID) (blob.Manifest, error) {
	err := peer.ensureStreamWithProtocol(peer.t.Process.Ctx(), PROTO_BLOB_MANIFEST)
	if err != nil {
		return blob.Manifest{}, err
	}
	defer peer.Close()

	err = peer.writeProtobuf(pb.FetchBlobManifest{Id: blobID.ToProtobuf()})
	if err != nil {
		return blob.Manifest{}, err
	}

	proto, err := peer.readProtobuf()
	if err != nil {
		return blob.Manifest{}, err
	}

	msg, is := proto.(*pb.SendBlobManifest)
	if !is {
		return blob.Manifest{}, swarm.ErrProtocol
	} else if !msg.Exists {
		return blob.Manifest{}, types.Err404
	}
	m, err := blob.ManifestFromProtobuf(msg.Manifest)
	if err != nil {
		return blob.Manifest{}, err
	}
	return m, nil
}

func (p *peerConn) ReadBlobManifestRequest() (blob.ID, error) {
	proto, err := p.readProtobuf()
	if err != nil {
		return blob.ID{}, err
	}
	msg, is := proto.(*pb.FetchBlobManifest)
	if !is {
		return blob.ID{}, swarm.ErrProtocol
	}
	return blob.IDFromProtobuf(msg.Id)
}

func (peer *peerConn) SendBlobManifest(manifest blob.Manifest, exists bool) error {
	defer peer.Close()
	if exists {
		return peer.writeProtobuf(pb.SendBlobManifest{Manifest: manifest.ToProtobuf(), Exists: true})
	}
	return peer.writeProtobuf(pb.SendBlobManifest{Exists: false})
}

func (peer *peerConn) FetchBlobChunk(sha3 types.Hash) ([]byte, error) {
	err := peer.ensureStreamWithProtocol(peer.t.Process.Ctx(), PROTO_BLOB_CHUNK)
	if err != nil {
		return nil, err
	}
	defer peer.Close()

	err = peer.writeProtobuf(pb.FetchBlobChunk{Sha3: sha3.Bytes()})
	if err != nil {
		return nil, err
	}

	proto, err := peer.readProtobuf()
	if err != nil {
		return nil, err
	}

	msg, is := proto.(*pb.SendBlobChunk)
	if !is {
		return nil, swarm.ErrProtocol
	} else if !msg.Exists {
		return nil, types.Err404
	}
	return msg.Chunk, nil
}

func (peer *peerConn) ReadBlobChunkRequest() (sha3 types.Hash, err error) {
	proto, err := peer.readProtobuf()
	if err != nil {
		return types.Hash{}, err
	}
	msg, is := proto.(*pb.FetchBlobChunk)
	if !is {
		return types.Hash{}, swarm.ErrProtocol
	}
	return types.HashFromBytes(msg.Sha3)
}

func (peer *peerConn) SendBlobChunk(chunk []byte, exists bool) error {
	if exists {
		return peer.writeProtobuf(pb.SendBlobChunk{Chunk: chunk, Exists: true})
	}
	return peer.writeProtobuf(pb.SendBlobChunk{Exists: false})
}

func (peer *peerConn) AnnouncePeers(ctx context.Context, peerDialInfos []swarm.PeerDialInfo) error {
	err := peer.ensureStreamWithProtocol(ctx, PROTO_MAIN)
	if err != nil {
		return err
	}
	return peer.writeMsg(Msg{Type: msgType_AnnouncePeers, Payload: peerDialInfos})
}

func (peer *peerConn) readProtobuf() (_ interface{}, err error) {
	defer func() { peer.UpdateConnStats(err == nil) }()

	// peer.stream.SetReadDeadline(time.Now().Add(10 * time.Second))

	size, err := readUint64(peer.stream)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	_, err = io.CopyN(&buf, peer.stream, int64(size))
	if err != nil {
		return nil, err
	}

	var msg pb.Msg
	err = msg.Unmarshal(buf.Bytes())
	if err != nil {
		return nil, err
	}

	if m := msg.GetFetchBlobManifest(); m != nil {
		return m, nil
	} else if m := msg.GetFetchBlobChunk(); m != nil {
		return m, nil
	} else if m := msg.GetSendBlobManifest(); m != nil {
		return m, nil
	} else if m := msg.GetSendBlobChunk(); m != nil {
		return m, nil
	}
	return nil, swarm.ErrProtocol
}

func (peer *peerConn) writeProtobuf(child interface{}) error {
	var msg pb.Msg
	switch child := child.(type) {
	case pb.FetchBlobManifest:
		msg.Msg = &pb.Msg_FetchBlobManifest{
			FetchBlobManifest: &child,
		}
	case pb.SendBlobManifest:
		msg.Msg = &pb.Msg_SendBlobManifest{
			SendBlobManifest: &child,
		}
	case pb.FetchBlobChunk:
		msg.Msg = &pb.Msg_FetchBlobChunk{
			FetchBlobChunk: &child,
		}
	case pb.SendBlobChunk:
		msg.Msg = &pb.Msg_SendBlobChunk{
			SendBlobChunk: &child,
		}
	default:
		panic("invariant violation")
	}

	bs, err := msg.Marshal()
	if err != nil {
		return err
	}

	defer func() { peer.UpdateConnStats(err == nil) }()

	buflen := uint64(len(bs))

	// err = peer.stream.SetWriteDeadline(time.Now().Add(10 * time.Second))
	// if err != nil {
	// 	return err
	// }

	err = writeUint64(peer.stream, buflen)
	if err != nil {
		return err
	}

	n, err := io.CopyN(peer.stream, bytes.NewReader(bs), int64(buflen))
	if err != nil {
		return err
	} else if n != int64(buflen) {
		return errors.New("WriteMsg: could not write entire packet")
	}
	return nil
}

func (p *peerConn) writeMsg(msg Msg) (err error) {
	defer func() { p.UpdateConnStats(err == nil) }()

	bs, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	buflen := uint64(len(bs))

	// err = p.stream.SetWriteDeadline(time.Now().Add(10 * time.Second))
	// if err != nil {
	// 	return err
	// }

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
		err := p.stream.Close()
		p.stream = nil
		return err
	}
	return nil
}
