package blob

import (
	"io"

	"github.com/pkg/errors"

	"redwood.dev/state"
)

type blobReader struct {
	db       *state.DBTree
	manifest Manifest
	i, j     int
}

func (r *blobReader) Read(buf []byte) (int, error) {
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

func (r *blobReader) Close() error {
	return nil
}
