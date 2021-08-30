package blob

import (
	"bytes"
	"io"

	"redwood.dev/blob/pb"
	"redwood.dev/errors"
	"redwood.dev/types"
)

type Store interface {
	Start() error
	Close()

	MaxFetchConns() (uint64, error)
	SetMaxFetchConns(maxFetchConns uint64) error

	BlobReader(blobID ID) (io.ReadCloser, int64, error)
	HaveBlob(id ID) (bool, error)
	StoreBlob(reader io.ReadCloser) (sha1Hash types.Hash, sha3Hash types.Hash, err error)
	VerifyBlobOrPrune(blobID ID) error

	Manifest(id ID) (Manifest, error)
	HaveManifest(blobID ID) (bool, error)
	StoreManifest(blobID ID, manifest Manifest) error

	Chunk(sha3 types.Hash) ([]byte, error)
	HaveChunk(sha3 types.Hash) (bool, error)
	StoreChunkIfHashMatches(expectedSHA3 types.Hash, chunkBytes []byte) error

	BlobIDs() *IDIterator

	BlobsNeeded() ([]ID, error)
	MarkBlobsAsNeeded(refs []ID) error
	OnBlobsNeeded(fn func(refs []ID))
	OnBlobsSaved(fn func())

	Contents() (map[types.Hash]map[types.Hash]bool, error)
	DebugPrint()
}

var (
	ErrWrongHash = errors.New("wrong hash")
)

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

func IDFromProtobuf(proto *pb.BlobID) (ID, error) {
	hash, err := types.HashFromBytes(proto.Hash)
	if err != nil {
		return ID{}, err
	}
	return ID{
		HashAlg: types.HashAlgFromProtobuf(proto.HashAlg),
		Hash:    hash,
	}, nil
}

func (id ID) ToProtobuf() *pb.BlobID {
	hash := id.Hash.Copy()
	return &pb.BlobID{
		HashAlg: id.HashAlg.ToProtobuf(),
		Hash:    hash[:],
	}
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

type Manifest struct {
	Size       uint64       `tree:"size"       json:"size"`
	ChunkSHA3s []types.Hash `tree:"chunkSHA3s" json:"chunkSHA3s"`
}

func ManifestFromProtobuf(proto *pb.Manifest) (Manifest, error) {
	var manifest Manifest
	for _, sha3 := range proto.ChunkSHA3S {
		hash, err := types.HashFromBytes(sha3)
		if err != nil {
			return Manifest{}, errors.Wrap(err, "while decoding manifest from protobuf")
		}
		manifest.ChunkSHA3s = append(manifest.ChunkSHA3s, hash)
	}
	manifest.Size = proto.Size_
	return manifest, nil
}

func (m Manifest) ToProtobuf() *pb.Manifest {
	out := &pb.Manifest{Size_: m.Size}
	for _, chunkSHA3 := range m.ChunkSHA3s {
		out.ChunkSHA3S = append(out.ChunkSHA3S, chunkSHA3.Bytes())
	}
	return out
}
