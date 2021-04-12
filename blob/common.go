package blob

import (
	"io"

	"redwood.dev/types"
)

type Store interface {
	Start() error
	Close()

	HaveObject(refID types.RefID) (bool, error)
	Object(refID types.RefID) (io.ReadCloser, int64, error)
	ObjectFilepath(refID types.RefID) (string, error)
	StoreObject(reader io.ReadCloser) (sha1Hash types.Hash, sha3Hash types.Hash, err error)
	AllHashes() ([]types.RefID, error)

	RefsNeeded() ([]types.RefID, error)
	MarkRefsAsNeeded(refs []types.RefID)
	OnRefsNeeded(fn func(refs []types.RefID))
	OnRefsSaved(fn func())
}
