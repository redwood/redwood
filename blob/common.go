package blob

import (
	"bytes"
	"io"

	"github.com/pkg/errors"

	"redwood.dev/types"
)

type Store interface {
	Start() error
	Close()

	HaveBlob(id ID) (bool, error)
	BlobReader(id ID) (io.ReadCloser, int64, error)
	StoreBlob(reader io.ReadCloser) (sha1Hash types.Hash, sha3Hash types.Hash, err error)
	AllHashes() ([]ID, error)

	BlobsNeeded() ([]ID, error)
	MarkBlobsAsNeeded(refs []ID)
	OnBlobsNeeded(fn func(refs []ID))
	OnBlobsSaved(fn func())

	DebugPrint()
}

type ID struct {
	HashAlg types.HashAlg
	Hash    types.Hash
}

func (id ID) String() string {
	hashStr := id.Hash.Hex()
	if id.HashAlg == types.SHA1 {
		hashStr = hashStr[:40]
	}
	return id.HashAlg.String() + ":" + hashStr
}

func (id ID) MarshalText() ([]byte, error) {
	return []byte(id.String()), nil
}

func (id *ID) UnmarshalText(bs []byte) error {
	if bytes.HasPrefix(bs, []byte("sha1:")) && len(bs) >= 45 {
		hash, err := types.HashFromHex(string(bs[5:]))
		if err != nil {
			return err
		}
		copy(id.Hash[:], hash[:20])
		id.HashAlg = types.SHA1
		return nil

	} else if bytes.HasPrefix(bs, []byte("sha3:")) && len(bs) == 69 {
		hash, err := types.HashFromHex(string(bs[5:]))
		if err != nil {
			return err
		}
		copy(id.Hash[:], hash[:])
		id.HashAlg = types.SHA3
		return nil
	}
	return errors.Errorf("bad ref ID: '%v' (hasPrefix: %v, len: %v)", string(bs), bytes.HasPrefix(bs, []byte("sha3:")), len(bs))
}
