package blob

import (
	"redwood.dev/errors"
	"redwood.dev/state"
	"redwood.dev/types"
)

type IDIterator struct {
	node        state.Node
	iter        state.Iterator
	currentSHA1 types.Hash
	currentSHA3 types.Hash
	err         error
}

func newIDIterator(db *state.DBTree) *IDIterator {
	node := db.State(false)
	return &IDIterator{
		node: node,
		iter: node.Iterator(nil, false, 0),
	}
}

func (iter *IDIterator) Rewind() {
	iter.iter.Rewind()
	iter.Next()
}

func (iter *IDIterator) Current() (sha1, sha3 ID) {
	return ID{types.SHA1, iter.currentSHA1}, ID{types.SHA3, iter.currentSHA3}
}

func (iter *IDIterator) Err() error {
	return iter.err
}

func (iter *IDIterator) Next() {
	if !iter.iter.Valid() {
		return
	}
	iter.iter.Next()

	var manifestNode state.Node
	for ; iter.iter.Valid(); iter.iter.Next() {
		manifestNode = iter.iter.Node()
		if !manifestNode.Keypath().Part(-1).Equals(manifestKey) {
			continue
		}

		var manifest Manifest
		err := manifestNode.Scan(&manifest)
		if errors.Cause(err) == errors.Err404 {
			iter.err = err
			iter.currentSHA1 = types.Hash{}
			iter.currentSHA3 = types.Hash{}
			return
		} else if err != nil {
			iter.err = err
			iter.currentSHA1 = types.Hash{}
			iter.currentSHA3 = types.Hash{}
			return
		}
		have, err := haveAllChunks(iter.node, manifest.ChunkSHA3s)
		if err != nil {
			iter.err = err
			return
		} else if !have {
			continue
		}
		break
	}

	var err error

	sha3Hex := manifestNode.Keypath().Part(-2)
	iter.currentSHA3, err = types.HashFromHex(sha3Hex.String())
	if err != nil {
		iter.err = err
		return
	}
	iter.currentSHA1, err = sha1ForSHA3(iter.currentSHA3, iter.node)
	if err != nil {
		iter.err = err
		return
	}
}

func (iter *IDIterator) Valid() bool {
	return iter.iter.Valid() && iter.err == nil
}

func (iter *IDIterator) Close() {
	iter.iter.Close()
	iter.node.Close()
}
