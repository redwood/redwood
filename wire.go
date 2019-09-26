package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/lunixbochs/struc"
	"github.com/pkg/errors"
)

type (
	EncodeToWirer interface {
		EncodeToWire() error
	}

	DecodeFromWirer interface {
		DecodeFromWire() error
	}
)

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

func WriteMsg(w io.Writer, obj interface{}) error {
	if asEncodable, ok := obj.(EncodeToWirer); ok {
		err := asEncodable.EncodeToWire()
		if err != nil {
			return err
		}
	}

	buf := &bytes.Buffer{}

	err := struc.Pack(buf, obj)
	if err != nil {
		return err
	}

	buflen := buf.Len()
	err = WriteUint64(w, uint64(buflen))
	if err != nil {
		return err
	}

	n, err := io.Copy(w, buf)
	if err != nil {
		return err
	} else if n != int64(buflen) {
		return fmt.Errorf("WriteMsg: could not write entire packet")
	}
	return nil
}

func ReadMsg(r io.Reader, obj interface{}) error {
	size, err := ReadUint64(r)
	if err != nil {
		return err
	}

	buf := &bytes.Buffer{}
	_, err = io.CopyN(buf, r, int64(size))
	if err != nil {
		return err
	}

	err = struc.Unpack(buf, obj)
	if err != nil {
		return err
	}

	if asDecodable, ok := obj.(DecodeFromWirer); ok {
		err = asDecodable.DecodeFromWire()
		if err != nil {
			return err
		}
	}

	return nil
}
