package blob

import (
	"io"

	"redwood.dev/blob/pb"
	"redwood.dev/errors"
	"redwood.dev/types"
)

type ID = pb.ID
type Manifest = pb.Manifest
type ManifestChunk = pb.ManifestChunk

var (
	ErrWrongHash = errors.New("wrong hash")
)

type Store interface {
	Start() error
	Close()

	MaxFetchConns() (uint64, error)
	SetMaxFetchConns(maxFetchConns uint64) error

	BlobReader(blobID ID, rng *types.Range) (io.ReadCloser, int64, error)
	HaveBlob(id ID) (bool, error)
	StoreBlob(reader io.ReadCloser) (sha1Hash types.Hash, sha3Hash types.Hash, err error)
	VerifyBlobOrPrune(blobID ID) error

	Manifest(id ID) (Manifest, error)
	HaveManifest(blobID ID) (bool, error)
	StoreManifest(blobID ID, manifest Manifest) error

	Chunk(sha3 types.Hash) ([]byte, error)
	HaveChunk(sha3 types.Hash) (bool, error)
	StoreChunkIfHashMatches(expectedSHA3 types.Hash, chunkBytes []byte) error

	// BlobIDs() *IDIterator
	BlobIDs() (sha1s, sha3s []ID, _ error)

	BlobsNeeded() ([]ID, error)
	MarkBlobsAsNeeded(refs []ID) error
	OnBlobsNeeded(fn func(refs []ID))
	OnBlobsSaved(fn func())

	Contents() (map[types.Hash]map[types.Hash]bool, error)
	DebugPrint()
}
