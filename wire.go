package main

import (
	"encoding/json"
	"io"

	"github.com/pkg/errors"
	// log "github.com/sirupsen/logrus"
)

type Msg struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

const (
	MsgType_Subscribe   = "subscribe"
	MsgType_Unsubscribe = "unsubscribe"
	MsgType_Put         = "put"
	MsgType_Ack         = "ack"
	MsgType_Error       = "error"
)

func WriteMsg(w io.Writer, msg Msg) error {
	return json.NewEncoder(w).Encode(msg)
}

func ReadMsg(r io.Reader, msg *Msg) error {
	return json.NewDecoder(r).Decode(msg)
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

	msg.Type = m.Type

	switch m.Type {
	case MsgType_Subscribe:
		url := string(m.PayloadBytes)
		msg.Payload = url[1 : len(url)-1] // remove quotes

	case MsgType_Put:
		var tx Tx
		err := json.Unmarshal(m.PayloadBytes, &tx)
		if err != nil {
			return err
		}
		msg.Payload = tx

	case MsgType_Ack:
		var id ID
		bs := []byte(m.PayloadBytes[1 : len(m.PayloadBytes)-1]) // remove quotes
		copy(id[:], bs)
		msg.Payload = id

	default:
		return errors.New("bad msg")
	}

	return nil
}
