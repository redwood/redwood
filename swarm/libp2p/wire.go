package libp2p

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"

	"github.com/pkg/errors"

	"redwood.dev/swarm"
	"redwood.dev/tree"
	"redwood.dev/types"
)

type Msg struct {
	Type    MsgType     `json:"type"`
	Payload interface{} `json:"payload"`
}

type MsgType string

const (
	MsgType_Subscribe                 MsgType = "subscribe"
	MsgType_Unsubscribe               MsgType = "unsubscribe"
	MsgType_Put                       MsgType = "put"
	MsgType_Private                   MsgType = "private"
	MsgType_Ack                       MsgType = "ack"
	MsgType_Error                     MsgType = "error"
	MsgType_ChallengeIdentityRequest  MsgType = "challenge identity"
	MsgType_ChallengeIdentityResponse MsgType = "challenge identity response"
	MsgType_FetchRef                  MsgType = "fetch ref"
	MsgType_FetchRefResponse          MsgType = "fetch ref response"
	MsgType_AnnouncePeers             MsgType = "announce peers"
)

func readMsg(r io.Reader) (msg Msg, err error) {
	size, err := readUint64(r)
	if err != nil {
		return Msg{}, err
	}

	buf := &bytes.Buffer{}
	_, err = io.CopyN(buf, r, int64(size))
	if err != nil {
		return Msg{}, err
	}

	err = json.NewDecoder(buf).Decode(&msg)
	return msg, err
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

	msg.Type = MsgType(m.Type)

	switch msg.Type {
	case MsgType_Subscribe:
		url := string(m.PayloadBytes)
		msg.Payload = url[1 : len(url)-1] // remove quotes

	case MsgType_Put:
		var tx tree.Tx
		err := json.Unmarshal(m.PayloadBytes, &tx)
		if err != nil {
			return err
		}
		msg.Payload = tx

	case MsgType_Ack:
		var payload swarm.AckMsg
		err := json.Unmarshal(m.PayloadBytes, &payload)
		if err != nil {
			return err
		}
		msg.Payload = payload

	case MsgType_Private:
		var ep swarm.EncryptedTx
		err := json.Unmarshal(m.PayloadBytes, &ep)
		if err != nil {
			return err
		}
		msg.Payload = ep

	case MsgType_ChallengeIdentityRequest:
		var challenge types.ChallengeMsg
		err := json.Unmarshal(m.PayloadBytes, &challenge)
		if err != nil {
			return err
		}
		msg.Payload = challenge

	case MsgType_ChallengeIdentityResponse:
		var resp []swarm.ChallengeIdentityResponse
		err := json.Unmarshal([]byte(m.PayloadBytes), &resp)
		if err != nil {
			return err
		}

		msg.Payload = resp

	case MsgType_FetchRef:
		var refID types.RefID
		err := json.Unmarshal([]byte(m.PayloadBytes), &refID)
		if err != nil {
			return err
		}
		msg.Payload = refID

	case MsgType_FetchRefResponse:
		var resp swarm.FetchRefResponse
		err := json.Unmarshal([]byte(m.PayloadBytes), &resp)
		if err != nil {
			return err
		}
		msg.Payload = resp

	case MsgType_AnnouncePeers:
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
