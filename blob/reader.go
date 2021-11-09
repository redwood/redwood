package blob

import (
	"io"

	"redwood.dev/errors"
	"redwood.dev/state"
)

type Reader struct {
	db       *state.DBTree
	manifest Manifest
	i, j     int
}

func NewReader(db *state.DBTree, manifest Manifest) *Reader {
	return &Reader{db: db, manifest: manifest}
}

func (r *Reader) Read(buf []byte) (int, error) {
	if r.i == len(r.manifest.ChunkSHA3s) {
		return 0, io.EOF
	}
	node := r.db.State(false)
	defer node.Close()

	chunkSHA3 := r.manifest.ChunkSHA3s[r.i]

	bytes, is, err := node.BytesValue(state.Keypath(chunkSHA3.Hex()).Pushs("chunk"))
	if err != nil {
		return 0, err
	} else if !is {
		return 0, errors.New("value is not bytes")
	}
	n := copy(buf, bytes[r.j:])
	r.j += n
	if r.j >= len(bytes) {
		r.i++
		r.j = 0
	}
	return n, nil
}

func (r *Reader) Close() error {
	return nil
}
