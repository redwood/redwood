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
	protocol "github.com/libp2p/go-libp2p-protocol"

	"redwood.dev/blob"
	"redwood.dev/errors"
	"redwood.dev/state"
	"redwood.dev/swarm"
	"redwood.dev/swarm/libp2p/pb"
	"redwood.dev/swarm/protoauth"
	"redwood.dev/swarm/protoblob"
	"redwood.dev/swarm/protohush"
	"redwood.dev/swarm/prototree"
	"redwood.dev/tree"
	"redwood.dev/types"
)

type peerConn struct {
	swarm.PeerEndpoint
	t      *transport
	pinfo  peerstore.PeerInfo
	stream netp2p.Stream
	mu     sync.Mutex
}

var (
	_ protoauth.AuthPeerConn = (*peerConn)(nil)
	_ protoblob.BlobPeerConn = (*peerConn)(nil)
	_ prototree.TreePeerConn = (*peerConn)(nil)
	_ protohush.HushPeerConn = (*peerConn)(nil)
)

func (peer *peerConn) DeviceUniqueID() string {
	return peer.pinfo.ID.Pretty()
}

func (peer *peerConn) Transport() swarm.Transport {
	return peer.t
}

func (peer *peerConn) EnsureConnected(ctx context.Context) (err error) {
	defer func() { peer.UpdateConnStats(err == nil) }()

	err = peer.t.libp2pHost.Connect(ctx, peer.pinfo)
	if err != nil {
		return errors.Wrapf(errors.ErrConnection, "(peer %v): %v", peer.pinfo.ID, err)
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

	defer func() { peer.UpdateConnStats(err == nil) }()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	peer.stream, err = peer.t.libp2pHost.NewStream(ctx, peer.pinfo.ID, p)
	if err != nil {
		return errors.Wrapf(errors.ErrConnection, "(peer %v): %v", peer.pinfo.ID, err)
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

	err = peer.writeMsg(Msg{Type: msgType_Subscribe, Payload: stateURI})
	if err != nil {
		return nil, err
	}

	return &readableSubscription{peer}, nil
}

func (peer *peerConn) SendTx(ctx context.Context, tx tree.Tx) error {
	err := peer.ensureStreamWithProtocol(ctx, PROTO_MAIN)
	if err != nil {
		return err
	}
	// Note: libp2p peers ignore `state` and `leaves`
	return peer.writeMsg(Msg{Type: msgType_Tx, Payload: tx})
}

func (peer *peerConn) SendPrivateTx(ctx context.Context, encryptedTx prototree.EncryptedTx) error {
	err := peer.ensureStreamWithProtocol(ctx, PROTO_MAIN)
	if err != nil {
		return err
	}
	// Note: libp2p peers ignore `state` and `leaves`
	return peer.writeMsg(Msg{Type: msgType_EncryptedTx, Payload: encryptedTx})
}

func (peer *peerConn) Ack(stateURI string, txID state.Version) (err error) {
	err = peer.ensureStreamWithProtocol(peer.t.Process.Ctx(), PROTO_MAIN)
	if err != nil {
		return err
	}
	return peer.writeMsg(Msg{Type: msgType_Ack, Payload: ackMsg{stateURI, txID}})
}

func (peer *peerConn) AnnounceP2PStateURI(ctx context.Context, stateURI string) (err error) {
	err = peer.ensureStreamWithProtocol(ctx, PROTO_MAIN)
	if err != nil {
		return err
	}
	return peer.writeMsg(Msg{Type: msgType_AnnounceP2PStateURI, Payload: stateURI})
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
	err := peer.ensureStreamWithProtocol(peer.t.Process.Ctx(), PROTO_BLOB)
	if err != nil {
		return blob.Manifest{}, err
	}
	defer peer.Close()

	err = peer.writeProtobuf(pb.MakeBlobProtobuf_FetchManifest(blobID))
	if err != nil {
		return blob.Manifest{}, err
	}

	var msg pb.BlobMessage
	err = peer.readProtobuf(&msg)
	if err != nil {
		return blob.Manifest{}, err
	}

	payload := msg.GetSendManifest()
	if payload == nil {
		return blob.Manifest{}, swarm.ErrProtocol
	}
	return payload.Manifest, nil
}

func (peer *peerConn) SendBlobManifest(manifest blob.Manifest, exists bool) error {
	err := peer.ensureStreamWithProtocol(peer.t.Process.Ctx(), PROTO_BLOB)
	if err != nil {
		return err
	}
	defer peer.Close()
	return peer.writeProtobuf(pb.MakeBlobProtobuf_SendManifest(manifest, exists))
}

func (peer *peerConn) FetchBlobChunk(sha3 types.Hash) ([]byte, error) {
	err := peer.ensureStreamWithProtocol(peer.t.Process.Ctx(), PROTO_BLOB)
	if err != nil {
		return nil, err
	}
	defer peer.Close()

	err = peer.writeProtobuf(pb.MakeBlobProtobuf_FetchChunk(sha3))
	if err != nil {
		return nil, err
	}

	var msg pb.BlobMessage
	err = peer.readProtobuf(&msg)
	if err != nil {
		return nil, err
	}

	payload := msg.GetSendChunk()
	if payload == nil {
		return nil, swarm.ErrProtocol
	}
	return payload.Chunk, nil
}

func (peer *peerConn) SendBlobChunk(chunk []byte, exists bool) error {
	err := peer.ensureStreamWithProtocol(peer.t.Process.Ctx(), PROTO_BLOB)
	if err != nil {
		return err
	}
	defer peer.Close()
	return peer.writeProtobuf(pb.MakeBlobProtobuf_SendChunk(chunk, exists))
}

func (peer *peerConn) SendDHPubkeyAttestations(ctx context.Context, attestations []protohush.DHPubkeyAttestation) error {
	err := peer.ensureStreamWithProtocol(ctx, PROTO_HUSH)
	if err != nil {
		return err
	}
	defer peer.Close()
	return peer.writeProtobuf(pb.MakeHushProtobuf_DHPubkeyAttestations(attestations))
}

func (peer *peerConn) ProposeIndividualSession(ctx context.Context, encryptedProposal []byte) error {
	err := peer.ensureStreamWithProtocol(ctx, PROTO_HUSH)
	if err != nil {
		return err
	}
	defer peer.Close()
	return peer.writeProtobuf(pb.MakeHushProtobuf_ProposeIndividualSession(encryptedProposal))
}

func (peer *peerConn) RespondToIndividualSession(ctx context.Context, response protohush.IndividualSessionResponse) error {
	err := peer.ensureStreamWithProtocol(ctx, PROTO_HUSH)
	if err != nil {
		return err
	}
	defer peer.Close()
	return peer.writeProtobuf(pb.MakeHushProtobuf_RespondToIndividualSessionProposal(response))
}

func (peer *peerConn) SendHushIndividualMessage(ctx context.Context, msg protohush.IndividualMessage) error {
	err := peer.ensureStreamWithProtocol(ctx, PROTO_HUSH)
	if err != nil {
		return err
	}
	defer peer.Close()
	return peer.writeProtobuf(pb.MakeHushProtobuf_SendIndividualMessage(msg))
}

func (peer *peerConn) SendHushGroupMessage(ctx context.Context, msg protohush.GroupMessage) error {
	err := peer.ensureStreamWithProtocol(ctx, PROTO_HUSH)
	if err != nil {
		return err
	}
	defer peer.Close()
	return peer.writeProtobuf(pb.MakeHushProtobuf_SendGroupMessage(msg))
}

func (peer *peerConn) AnnouncePeers(ctx context.Context, peerDialInfos []swarm.PeerDialInfo) error {
	err := peer.ensureStreamWithProtocol(ctx, PROTO_MAIN)
	if err != nil {
		return err
	}
	return peer.writeMsg(Msg{Type: msgType_AnnouncePeers, Payload: peerDialInfos})
}

type protobufUnmarshaler interface {
	Unmarshal(bs []byte) error
}

func (peer *peerConn) readProtobuf(proto protobufUnmarshaler) (err error) {
	defer func() { peer.UpdateConnStats(err == nil) }()

	// peer.stream.SetReadDeadline(time.Now().Add(10 * time.Second))

	size, err := readUint64(peer.stream)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	_, err = io.CopyN(&buf, peer.stream, int64(size))
	if err != nil {
		return err
	}

	err = proto.Unmarshal(buf.Bytes())
	if err != nil {
		return err
	}
	return nil
}

type protobufMarshaler interface {
	Marshal() ([]byte, error)
}

func (peer *peerConn) writeProtobuf(msg protobufMarshaler) error {
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
