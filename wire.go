package redwood

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"

	"github.com/pkg/errors"
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
	bs, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	buflen := uint64(len(bs))

	err = WriteUint64(w, buflen)
	if err != nil {
		return err
	}
	n, err := io.Copy(w, bytes.NewReader(bs))
	if err != nil {
		return err
	} else if n != int64(buflen) {
		return fmt.Errorf("WriteMsg: could not write entire packet")
	}
	return nil
}

func ReadMsg(r io.Reader, msg *Msg) error {
	size, err := ReadUint64(r)
	if err != nil {
		return err
	}

	buf := &bytes.Buffer{}
	_, err = io.CopyN(buf, r, int64(size))
	if err != nil {
		return err
	}

	err = json.NewDecoder(buf).Decode(msg)
	if err != nil {
		return err
	}
	return nil
}

func ReadUint64(r io.Reader) (uint64, error) {
	buf := make([]byte, 8)
	_, err := io.ReadFull(r, buf)
	if err == io.EOF {
		return 0, err
	} else if err != nil {
		return 0, errors.Wrap(err, "ReadUint64")
	}
	return binary.LittleEndian.Uint64(buf), nil
}

func WriteUint64(w io.Writer, n uint64) error {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, n)
	written, err := w.Write(buf)
	if err != nil {
		return err
	} else if written < 8 {
		return errors.Wrap(err, "WriteUint64")
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
