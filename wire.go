package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"sort"

	"github.com/lunixbochs/struc"
	"github.com/pkg/errors"
)

type (
	ID [32]byte

	Patch struct {
		TextLen int `struc:"sizeof=Text"`
		Text    string
	}

	Transaction struct {
		ID         ID
		ParentsLen int `struc:"sizeof=Parents"`
		Parents    []ID
		PatchesLen int `struc:"sizeof=Patches"`
		Patches    []Patch
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
	return nil
}

type blankValidator struct{}

func (blankValidator) Validate(_ string, _ []byte) error        { return nil }
func (blankValidator) Select(_ string, _ [][]byte) (int, error) { return 0, nil }
