package libp2p

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"strings"
	"sync"
	"time"

	netp2p "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	protocol "github.com/libp2p/go-libp2p-protocol"

	"redwood.dev/blob"
	"redwood.dev/crypto"
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
	. "redwood.dev/utils/generics"
)

type peerConn struct {
	swarm.PeerEndpoint // @@TODO: Consider changing this to hold a swarm.PeerDevice
	t                  *transport
	pinfo              peer.AddrInfo
	stream             netp2p.Stream
	mu                 sync.Mutex
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
	if err != nil && strings.Contains(err.Error(), "NO_RESERVATION") {
		for _, multiaddr := range multiaddrsFromPeerInfo(peer.pinfo) {
			relay, _ := splitRelayAndPeer(multiaddr)
			if relay == nil {
				continue
			}
			addrInfo, ok := addrInfoFromMultiaddr(relay)
			if !ok {
				continue
			}
			peer.t.relayClient.RenewReservation(addrInfo)
		}
	} else if err != nil {
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

	ctx = netp2p.WithUseTransient(ctx, "because circuitv2 seems to require it?")

	peer.stream, err = peer.t.libp2pHost.NewStream(ctx, peer.pinfo.ID, p)
	if err != nil && strings.Contains(err.Error(), "NO_RESERVATION") {
		for _, multiaddr := range multiaddrsFromPeerInfo(peer.pinfo) {
			relay, _ := splitRelayAndPeer(multiaddr)
			if relay == nil {
				continue
			}
			addrInfo, ok := addrInfoFromMultiaddr(relay)
			if !ok {
				continue
			}
			peer.t.relayClient.RenewReservation(addrInfo)
		}
		return errors.Wrapf(errors.ErrConnection, "(peer %v): %v", peer.pinfo.ID, err)
	} else if err != nil {
		return errors.Wrapf(errors.ErrConnection, "(peer %v): %v", peer.pinfo.ID, err)
	}
	return nil
}

func (peer *peerConn) Subscribe(ctx context.Context, stateURI string) (prototree.ReadableSubscription, error) {
	err := peer.EnsureConnected(ctx)
	if err != nil {
		return nil, err
	}

	err = peer.ensureStreamWithProtocol(ctx, PROTO_TREE)
	if err != nil {
		return nil, err
	}

	err = peer.writeProtobuf(ctx, pb.MakeTreeProtobuf_Subscribe(stateURI))
	if err != nil {
		return nil, err
	}
	return &readableSubscription{peer}, nil
}

func (peer *peerConn) SendTx(ctx context.Context, tx tree.Tx) error {
	err := peer.ensureStreamWithProtocol(ctx, PROTO_TREE)
	if err != nil {
		return err
	}
	return peer.writeProtobuf(ctx, pb.MakeTreeProtobuf_Tx(tx))
}

func (peer *peerConn) SendEncryptedTx(ctx context.Context, tx prototree.EncryptedTx) error {
	err := peer.ensureStreamWithProtocol(ctx, PROTO_TREE)
	if err != nil {
		return err
	}
	return peer.writeProtobuf(ctx, pb.MakeTreeProtobuf_EncryptedTx(tx))
}

func (peer *peerConn) Ack(ctx context.Context, stateURI string, txID state.Version) error {
	err := peer.ensureStreamWithProtocol(ctx, PROTO_TREE)
	if err != nil {
		return err
	}
	return peer.writeProtobuf(ctx, pb.MakeTreeProtobuf_Ack(stateURI, txID))
}

func (peer *peerConn) AnnounceP2PStateURI(ctx context.Context, stateURI string) error {
	err := peer.ensureStreamWithProtocol(ctx, PROTO_TREE)
	if err != nil {
		return err
	}
	return peer.writeProtobuf(ctx, pb.MakeTreeProtobuf_AnnounceP2PStateURI(stateURI))
}

// func (peer *peerConn) RequestIdentityChallenge(ctx context.Context) (crypto.ChallengeMsg, error) {
// 	err := peer.ensureStreamWithProtocol(peer.t.Process.Ctx(), PROTO_AUTH)
// 	if err != nil {
// 		return nil, err
// 	}
// 	err = peer.writeProtobuf(ctx, pb.MakeAuthProtobuf_ChallengeRequest())
// 	if err != nil {
// 		return nil, err
// 	}
// 	var msg pb.AuthMessage
// 	err = peer.readProtobuf(ctx, &msg)
// 	if err != nil {
// 		return nil, err
// 	}
// 	payload := msg.GetChallenge()
// 	if payload == nil {
// 		return nil, swarm.ErrProtocol
// 	}
// 	return payload.Challenge, nil
// }

func (peer *peerConn) ChallengeIdentity(ctx context.Context, challengeMsg crypto.ChallengeMsg) error {
	err := peer.ensureStreamWithProtocol(ctx, PROTO_AUTH)
	if err != nil {
		return err
	}
	defer peer.Close()

	err = peer.writeProtobuf(ctx, pb.MakeAuthProtobuf_Challenge(challengeMsg))
	if err != nil {
		return err
	}
	var msg pb.AuthMessage
	err = peer.readProtobuf(ctx, &msg)
	if err != nil {
		return err
	}
	payload := msg.GetSignatures()
	if payload == nil {
		return swarm.ErrProtocol
	}

	response := ZipFunc3(payload.Challenges, payload.Signatures, payload.AsymEncPubkeys,
		func(c crypto.ChallengeMsg, s crypto.Signature, k crypto.AsymEncPubkey, _ int) protoauth.ChallengeIdentityResponse {
			return protoauth.ChallengeIdentityResponse{c, s, k}
		},
	)

	return peer.t.HandleIncomingIdentityChallengeResponse(response, peer)
}

func (peer *peerConn) RespondChallengeIdentity(ctx context.Context, response []protoauth.ChallengeIdentityResponse) error {
	challenges, signatures, asymEncPubkeys := UnzipFunc3(response,
		func(resp protoauth.ChallengeIdentityResponse, _ int) (crypto.ChallengeMsg, crypto.Signature, crypto.AsymEncPubkey) {
			return resp.Challenge, resp.Signature, resp.AsymEncPubkey
		},
	)
	err := peer.writeProtobuf(ctx, pb.MakeAuthProtobuf_Signatures(challenges, signatures, asymEncPubkeys))
	if err != nil {
		return err
	}
	return nil
}

func (peer *peerConn) SendUcan(ctx context.Context, ucan string) error {
	return peer.writeProtobuf(ctx, pb.MakeAuthProtobuf_Ucan(ucan))
}

func (peer *peerConn) FetchBlobManifest(ctx context.Context, blobID blob.ID) (blob.Manifest, error) {
	err := peer.ensureStreamWithProtocol(ctx, PROTO_BLOB)
	if err != nil {
		return blob.Manifest{}, err
	}
	defer peer.Close()

	err = peer.writeProtobuf(ctx, pb.MakeBlobProtobuf_FetchManifest(blobID))
	if err != nil {
		return blob.Manifest{}, err
	}

	var msg pb.BlobMessage
	err = peer.readProtobuf(ctx, &msg)
	if err != nil {
		return blob.Manifest{}, err
	}

	payload := msg.GetSendManifest()
	if payload == nil {
		return blob.Manifest{}, swarm.ErrProtocol
	}
	return payload.Manifest, nil
}

func (peer *peerConn) SendBlobManifest(ctx context.Context, manifest blob.Manifest, exists bool) error {
	err := peer.ensureStreamWithProtocol(ctx, PROTO_BLOB)
	if err != nil {
		return err
	}
	defer peer.Close()
	return peer.writeProtobuf(ctx, pb.MakeBlobProtobuf_SendManifest(manifest, exists))
}

func (peer *peerConn) FetchBlobChunk(ctx context.Context, sha3 types.Hash) ([]byte, error) {
	err := peer.ensureStreamWithProtocol(ctx, PROTO_BLOB)
	if err != nil {
		return nil, err
	}
	defer peer.Close()

	err = peer.writeProtobuf(ctx, pb.MakeBlobProtobuf_FetchChunk(sha3))
	if err != nil {
		return nil, err
	}

	var msg pb.BlobMessage
	err = peer.readProtobuf(ctx, &msg)
	if err != nil {
		return nil, err
	}

	payload := msg.GetSendChunk()
	if payload == nil {
		return nil, swarm.ErrProtocol
	}
	return payload.Chunk, nil
}

func (peer *peerConn) SendBlobChunk(ctx context.Context, chunk []byte, exists bool) error {
	err := peer.ensureStreamWithProtocol(ctx, PROTO_BLOB)
	if err != nil {
		return err
	}
	defer peer.Close()
	return peer.writeProtobuf(ctx, pb.MakeBlobProtobuf_SendChunk(chunk, exists))
}

func (peer *peerConn) SendPubkeyBundles(ctx context.Context, bundles []protohush.PubkeyBundle) error {
	err := peer.ensureStreamWithProtocol(ctx, PROTO_HUSH)
	if err != nil {
		return err
	}
	defer peer.Close()
	return peer.writeProtobuf(ctx, pb.MakeHushProtobuf_PubkeyBundles(bundles))
}

func (peer *peerConn) AnnouncePeers(ctx context.Context, dialInfos []swarm.PeerDialInfo) error {
	err := peer.ensureStreamWithProtocol(ctx, PROTO_PEER)
	if err != nil {
		return err
	}
	defer peer.Close()
	return peer.writeProtobuf(ctx, pb.MakePeerProtobuf_AnnouncePeers(dialInfos))
}

func (peer *peerConn) readUint64() (uint64, error) {
	buf := make([]byte, 8)
	_, err := io.ReadFull(peer.stream, buf)
	if err == io.EOF {
		return 0, err
	} else if err != nil {
		return 0, errors.Wrap(err, "readUint64")
	}
	return binary.LittleEndian.Uint64(buf), nil
}

func (peer *peerConn) writeUint64(n uint64) error {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, n)
	written, err := peer.stream.Write(buf)
	if err != nil {
		return err
	} else if written < 8 {
		return errors.New("writeUint64: wrote too few bytes")
	}
	return nil
}

type protobufUnmarshaler interface {
	Unmarshal(bs []byte) error
}

func (peer *peerConn) readProtobuf(ctx context.Context, proto protobufUnmarshaler) (err error) {
	defer func() { peer.UpdateConnStats(err == nil) }()

	if ctx != nil {
		deadline, ok := ctx.Deadline()
		if !ok {
			deadline = time.Now().Add(10 * time.Second)
		}
		err = peer.stream.SetReadDeadline(deadline)
		if err != nil {
			return err
		}
	}

	size, err := peer.readUint64()
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

func (peer *peerConn) writeProtobuf(ctx context.Context, msg protobufMarshaler) error {
	bs, err := msg.Marshal()
	if err != nil {
		return err
	}

	defer func() { peer.UpdateConnStats(err == nil) }()

	if ctx != nil {
		deadline, ok := ctx.Deadline()
		if !ok {
			deadline = time.Now().Add(10 * time.Second)
		}
		err = peer.stream.SetWriteDeadline(deadline)
		if err != nil {
			return err
		}
	}

	buflen := uint64(len(bs))
	err = peer.writeUint64(buflen)
	if err != nil {
		return err
	}

	n, err := io.CopyN(peer.stream, bytes.NewReader(bs), int64(buflen))
	if err != nil {
		return err
	} else if n != int64(buflen) {
		return errors.New("writeProtobuf:ctx,  could not write entire packet")
	}
	return nil
}

func (p *peerConn) Close() error {
	if p.stream != nil {
		err := p.stream.Close()
		p.stream = nil
		return err
	}
	return nil
}
