package libp2p

import (
	"encoding/binary"
	"encoding/json"
	"io"

	"redwood.dev/errors"
	"redwood.dev/state"
	"redwood.dev/swarm"
	"redwood.dev/swarm/protoauth"
	"redwood.dev/swarm/prototree"
	"redwood.dev/tree"
)

type Msg struct {
	Type    msgType     `json:"type"`
	Payload interface{} `json:"payload"`
}

type msgType string

const (
	msgType_Subscribe                 msgType = "subscribe"
	msgType_Unsubscribe               msgType = "unsubscribe"
	msgType_Tx                        msgType = "tx"
	msgType_EncryptedTx               msgType = "encrypted tx"
	msgType_Ack                       msgType = "ack"
	msgType_Error                     msgType = "error"
	msgType_ChallengeIdentityRequest  msgType = "challenge identity"
	msgType_ChallengeIdentityResponse msgType = "challenge identity response"
	msgType_AnnouncePeers             msgType = "announce peers"
	msgType_AnnounceP2PStateURI       msgType = "announce p2p stateURI"
)

type ackMsg struct {
	StateURI string        `json:"stateURI"`
	TxID     state.Version `json:"txID"`
}

func readUint64(r io.Reader) (uint64, error) {
	buf := make([]byte, 8)
	_, err := io.ReadFull(r, buf)
	if err == io.EOF {
		return 0, err
	} else if err != nil {
		return 0, errors.Wrap(err, "readUint64")
	}
	return binary.LittleEndian.Uint64(buf), nil
}

func writeUint64(w io.Writer, n uint64) error {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, n)
	written, err := w.Write(buf)
	if err != nil {
		return err
	} else if written < 8 {
		return errors.New("writeUint64: wrote too few bytes")
	}
	return nil
}

func (msg *Msg) UnmarshalJSON(bs []byte) error {
	var m struct {
		Type         string          `json:"type"`
		PayloadBytes json.RawMessage `json:"payload"`
	}

	err := json.Unmarshal(bs, &m)
	if err != nil {
		return err
	}

	msg.Type = msgType(m.Type)

	switch msg.Type {
	case msgType_Subscribe:
		url := string(m.PayloadBytes)
		msg.Payload = url[1 : len(url)-1] // remove quotes

	case msgType_Tx:
		var tx tree.Tx
		err := json.Unmarshal(m.PayloadBytes, &tx)
		if err != nil {
			return err
		}
		msg.Payload = tx

	case msgType_EncryptedTx:
		var ep prototree.EncryptedTx
		err := json.Unmarshal(m.PayloadBytes, &ep)
		if err != nil {
			return err
		}
		msg.Payload = ep

	case msgType_Ack:
		var payload ackMsg
		err := json.Unmarshal(m.PayloadBytes, &payload)
		if err != nil {
			return err
		}
		msg.Payload = payload

	case msgType_AnnounceP2PStateURI:
		stateURI := string(m.PayloadBytes)
		msg.Payload = stateURI[1 : len(stateURI)-1] // remove quotes

	case msgType_ChallengeIdentityRequest:
		var challenge protoauth.ChallengeMsg
		err := json.Unmarshal(m.PayloadBytes, &challenge)
		if err != nil {
			return err
		}
		msg.Payload = challenge

	case msgType_ChallengeIdentityResponse:
		var resp []protoauth.ChallengeIdentityResponse
		err := json.Unmarshal([]byte(m.PayloadBytes), &resp)
		if err != nil {
			return err
		}

		msg.Payload = resp

	case msgType_AnnouncePeers:
		var peerDialInfos []swarm.PeerDialInfo
		err := json.Unmarshal([]byte(m.PayloadBytes), &peerDialInfos)
		if err != nil {
			return err
		}
		msg.Payload = peerDialInfos

	default:
		return errors.Errorf("bad msg: %v", msg.Type)
	}

	return nil
}
