package blob

import (
	"bytes"
	"crypto/sha1"
	"hash"
	"io"

	"github.com/aclements/go-rabin/rabin"
	"golang.org/x/crypto/sha3"

	"redwood.dev/errors"
	"redwood.dev/types"
)

type Chunker struct {
	reader     io.ReadCloser
	chunker    *rabin.Chunker
	sha1Hasher hash.Hash
	sha3Hasher hash.Hash
	tee        io.Reader
	manifest   Manifest
	chunkBuf   *bytes.Buffer
	done       bool
}

const (
	KB          = 1024
	WINDOW_SIZE = 2 * KB
	MIN         = 128 * KB
	AVG         = 256 * KB
	MAX         = 512 * KB
)

func NewChunker(reader io.ReadCloser) *Chunker {
	sha1Hasher := sha1.New()
	sha3Hasher := sha3.NewLegacyKeccak256()
	chunkBuf := &bytes.Buffer{}
	tee := io.TeeReader(io.TeeReader(io.TeeReader(reader, sha1Hasher), sha3Hasher), chunkBuf)

	return &Chunker{
		reader:     reader,
		chunker:    rabin.NewChunker(rabin.NewTable(rabin.Poly64, WINDOW_SIZE), tee, MIN, AVG, MAX),
		sha1Hasher: sha1Hasher,
		sha3Hasher: sha3Hasher,
		chunkBuf:   chunkBuf,
		tee:        tee,
	}
}

func (c *Chunker) Done() bool {
	return c.done
}

func (c *Chunker) Next() ([]byte, ManifestChunk, error) {
	chunkLen, err := c.chunker.Next()
	if errors.Cause(err) == io.EOF {
		c.done = true
		return nil, ManifestChunk{}, nil
	} else if err != nil {
		c.done = true
		return nil, ManifestChunk{}, err
	}

	rangeStart := c.manifest.TotalSize
	c.manifest.TotalSize += uint64(chunkLen)

	// @@TODO: reuse buffer
	var chunkBuf bytes.Buffer
	chunkHasher := sha3.NewLegacyKeccak256()
	chunkTee := io.TeeReader(c.chunkBuf, chunkHasher)

	_, err = io.CopyN(&chunkBuf, chunkTee, int64(chunkLen))
	if err != nil {
		return nil, ManifestChunk{}, err
	}

	var chunkHash types.Hash
	chunkHasher.Sum(chunkHash[:0])

	chunk := ManifestChunk{
		SHA3:  chunkHash,
		Range: types.Range{Start: rangeStart, End: c.manifest.TotalSize},
	}
	c.manifest.Chunks = append(c.manifest.Chunks, chunk)

	return chunkBuf.Bytes(), chunk, nil
}

func (c *Chunker) Summary() (_ Manifest, sha1, sha3 types.Hash) {
	c.sha1Hasher.Sum(sha1[:0])
	c.sha3Hasher.Sum(sha3[:0])
	return c.manifest, sha1, sha3
}

func (c *Chunker) Close() error {
	return c.reader.Close()
}
