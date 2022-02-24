package pb

import (
	"bytes"

	"redwood.dev/errors"
	"redwood.dev/types"
)

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
	return errors.Errorf("bad blob ID: '%v'", string(bs))
}
