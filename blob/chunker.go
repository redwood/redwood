package blob

import (
	"bytes"
	"crypto/sha1"
	"hash"
	"io"

	"github.com/aclements/go-rabin/rabin"
	"golang.org/x/crypto/sha3"

	"redwood.dev/types"
)

type Chunker struct {
	reader      io.ReadCloser
	chunker     *rabin.Chunker
	sha1Hasher  hash.Hash
	sha3Hasher  hash.Hash
	tee         io.Reader
	chunkHashes []types.Hash
	chunkBuf    *bytes.Buffer
	size        uint64
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

func (c *Chunker) Next() ([]byte, types.Hash, error) {
	chunkLen, err := c.chunker.Next()
	if err != nil {
		return nil, types.Hash{}, err
	}
	c.size += uint64(chunkLen)

	// @@TODO: reuse buffer
	var chunkBuf bytes.Buffer
	chunkHasher := sha3.NewLegacyKeccak256()
	chunkTee := io.TeeReader(c.chunkBuf, chunkHasher)

	_, err = io.CopyN(&chunkBuf, chunkTee, int64(chunkLen))
	if err != nil {
		return nil, types.Hash{}, err
	}

	var chunkHash types.Hash
	chunkHasher.Sum(chunkHash[:0])
	c.chunkHashes = append(c.chunkHashes, chunkHash)

	return chunkBuf.Bytes(), chunkHash, nil
}

func (c *Chunker) Hashes() (sha1 types.Hash, sha3 types.Hash, chunkHashes []types.Hash) {
	c.sha1Hasher.Sum(sha1[:0])
	c.sha3Hasher.Sum(sha3[:0])
	return sha1, sha3, c.chunkHashes
}

func (c *Chunker) Size() uint64 {
	return c.size
}

func (c *Chunker) Close() error {
	return c.reader.Close()
}
