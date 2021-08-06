package blob

import (
	"io"
	"sync"

	"github.com/pkg/errors"

	"redwood.dev/log"
	"redwood.dev/state"
	"redwood.dev/types"
)

type badgerStore struct {
	log.Logger

	dbFilename       string
	db               *state.DBTree
	encryptionConfig *state.EncryptionConfig

	blobsNeededListeners   []func(blobs []ID)
	blobsNeededListenersMu sync.RWMutex
	blobsSavedListeners    []func()
	blobsSavedListenersMu  sync.RWMutex
}

var _ Store = (*badgerStore)(nil)

type Manifest struct {
	Size       uint64       `tree:"size"`
	ChunkSHA3s []types.Hash `tree:"chunkSHA3s"`
}

func NewBadgerStore(dbFilename string, encryptionConfig *state.EncryptionConfig) *badgerStore {
	return &badgerStore{
		Logger:           log.NewLogger("blobstore"),
		dbFilename:       dbFilename,
		encryptionConfig: encryptionConfig,
	}
}

func (s *badgerStore) Start() error {
	s.Infof(0, "opening blob store at %v", s.dbFilename)

	db, err := state.NewDBTree(s.dbFilename, s.encryptionConfig)
	if err != nil {
		return err
	}
	s.db = db

	return nil
}

func (s *badgerStore) Close() {
	_ = s.db.Close()
}

func (s *badgerStore) HaveBlob(blobID ID) (bool, error) {
	var sha3 types.Hash
	switch blobID.HashAlg {
	case types.SHA1:
		var err error
		sha3, err = s.sha3ForSHA1(blobID.Hash)
		if err == types.Err404 {
			return false, nil
		} else if err != nil {
			return false, err
		}

	case types.SHA3:
		sha3 = blobID.Hash

	default:
		return false, errors.Errorf("unknown hash type '%v'", blobID.HashAlg)
	}

	node := s.db.State(false)
	defer node.Close()
	return node.Exists(state.Keypath(sha3[:]).Pushs("manifest"))
}

func (s *badgerStore) BlobReader(blobID ID) (io.ReadCloser, int64, error) {
	switch blobID.HashAlg {
	case types.SHA1:
		return s.blobWithSHA1(blobID.Hash)
	case types.SHA3:
		return s.blobWithSHA3(blobID.Hash)
	default:
		return nil, 0, errors.Errorf("unknown hash type '%v'", blobID.HashAlg)
	}
}

func (s *badgerStore) blobWithSHA1(hash types.Hash) (io.ReadCloser, int64, error) {
	sha3, err := s.sha3ForSHA1(hash)
	if err != nil {
		return nil, 0, err
	}
	return s.blobWithSHA3(sha3)
}

func (s *badgerStore) blobWithSHA3(sha3Hash types.Hash) (io.ReadCloser, int64, error) {
	node := s.db.State(false)
	defer node.Close()

	var manifest Manifest
	err := node.NodeAt(state.Keypath(sha3Hash.Hex()).Pushs("manifest"), nil).Scan(&manifest)
	if err != nil {
		return nil, 0, err
	}
	return &blobReader{db: s.db, manifest: manifest}, int64(manifest.Size), nil
}

func (s *badgerStore) StoreManifest(sha3 types.Hash, manifest Manifest) error {
	node := s.db.State(true)
	defer node.Close()

	err := node.Set(state.Keypath(sha3.Hex()).Pushs("manifest"), nil, manifest)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *badgerStore) StoreChunk(chunkBytes []byte, chunkSha3 types.Hash) error {
	node := s.db.State(true)
	defer node.Close()

	chunkKeypath := state.Keypath(chunkSha3.Hex()).Pushs("chunk")

	err := node.Set(chunkKeypath, nil, chunkBytes)
	if err != nil {
		return err
	}
	err = node.Delete(missingChunksKeypath.Pushs(chunkSha3.Hex()), nil)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *badgerStore) StoreBlob(reader io.ReadCloser) (types.Hash, types.Hash, error) {
	chunker := NewChunker(reader)
	defer chunker.Close()
	for {
		chunkBytes, chunkSha3, err := chunker.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return types.Hash{}, types.Hash{}, err
		}

		err = s.StoreChunk(chunkBytes, chunkSha3)
		if err != nil {
			return types.Hash{}, types.Hash{}, err
		}
	}

	sha1, sha3, chunkSha3s := chunker.Hashes()
	size := chunker.Size()

	err := s.StoreManifest(sha3, Manifest{Size: size, ChunkSHA3s: chunkSha3s})
	if err != nil {
		return types.Hash{}, types.Hash{}, err
	}

	node := s.db.State(true)
	defer node.Close()

	err = node.Set(state.Keypath(sha1.Hex()).Pushs("sha3"), nil, sha3[:])
	if err != nil {
		return types.Hash{}, types.Hash{}, err
	}
	err = node.Set(state.Keypath(sha3.Hex()).Pushs("sha1"), nil, sha1[:])
	if err != nil {
		return types.Hash{}, types.Hash{}, err
	}
	err = node.Delete(missingBlobsKeypath.Pushs(sha3.Hex()), nil)
	if err != nil {
		return types.Hash{}, types.Hash{}, err
	}
	err = node.Save()
	if err != nil {
		return types.Hash{}, types.Hash{}, err
	}

	s.Successf("saved blob (sha1: %v, sha3: %v)", sha1.Hex(), sha3.Hex())

	s.unmarkBlobsAsNeeded([]ID{
		{HashAlg: types.SHA1, Hash: sha1},
		{HashAlg: types.SHA3, Hash: sha3},
	})
	s.notifyBlobsSavedListeners()

	return sha1, sha3, nil
}

func (s *badgerStore) AllHashes() ([]ID, error) {
	node := s.db.State(false)
	defer node.Close()

	iter := node.Iterator(nil, false, 0)
	defer iter.Close()

	var blobIDs []ID
	for iter.Rewind(); iter.Valid(); iter.Next() {
		manifestNode := iter.Node()
		if !manifestNode.Keypath().Part(-1).Equals(manifestKeypath) {
			continue
		}
		exists, err := func() (bool, error) {
			chunksIter := manifestNode.ChildIterator(manifestKeypath, true, 10)
			defer chunksIter.Close()

			for chunksIter.Rewind(); chunksIter.Valid(); chunksIter.Next() {
				shaBytes := chunksIter.Node().Keypath().Part(-1)
				exists, err := node.Exists(shaBytes.Pushs("chunk"))
				if err != nil {
					return false, err
				} else if !exists {
					return false, nil
				}
			}
			return true, nil
		}()
		if err != nil {
			s.Errorf("error checking for existence of blob chunks: %v", err)
			continue
		} else if !exists {
			continue
		}

		sha3Hex := manifestNode.Keypath().Part(-2)
		sha3Hash, err := types.HashFromHex(sha3Hex.String())
		if err != nil {
			continue
		}

		blobIDs = append(blobIDs, ID{HashAlg: types.SHA3, Hash: sha3Hash})

		sha1Hash, err := s.sha1ForSHA3(sha3Hash)
		if err != nil {
			continue
		}
		blobIDs = append(blobIDs, ID{HashAlg: types.SHA1, Hash: sha1Hash})
	}
	return blobIDs, nil
}

var (
	manifestKeypath      = state.Keypath("manifest")
	missingBlobsKeypath  = state.Keypath("missing").Pushs("blobs")
	missingChunksKeypath = state.Keypath("missing").Pushs("chunks")
)

func (s *badgerStore) BlobsNeeded() ([]ID, error) {
	node := s.db.State(false).NodeAt(missingBlobsKeypath, nil)
	defer node.Close()

	iter := node.ChildIterator(nil, false, 0)
	defer iter.Close()

	var missingBlobsSlice []ID
	for iter.Rewind(); iter.Valid(); iter.Next() {
		var blobID ID
		err := blobID.UnmarshalText(iter.Node().Keypath().RelativeTo(node.Keypath()))
		if err != nil {
			s.Errorf("error unmarshaling blobID: %v", err)
			continue
		}
		missingBlobsSlice = append(missingBlobsSlice, blobID)
	}
	return missingBlobsSlice, nil
}

func (s *badgerStore) MarkBlobsAsNeeded(blobs []ID) {
	var actuallyNeeded []ID
	for _, blobID := range blobs {
		have, err := s.HaveBlob(blobID)
		if err != nil {
			s.Errorf("error checking blob badgerStore for blob %v: %v", blobID, err)
			continue
		}
		if !have {
			actuallyNeeded = append(actuallyNeeded, blobID)
		}
	}

	if len(actuallyNeeded) == 0 {
		return
	}

	node := s.db.State(true)
	defer node.Close()

	node = node.NodeAt(missingBlobsKeypath, nil).(*state.DBNode)

	for _, blobID := range actuallyNeeded {
		blobIDKey, err := blobID.MarshalText()
		if err != nil {
			s.Errorf("can't marshal blobID %+v to bytes: %v", blobID, err)
			continue
		}

		err = node.Set(state.Keypath(blobIDKey), nil, true)
		if err != nil {
			s.Errorf("error updating list of missing blobs: %v", err)
			continue
		}
	}
	err := node.Save()
	if err != nil {
		s.Errorf("error updating list of missing blobs: %v", err)
	}

	allNeeded, err := s.BlobsNeeded()
	if err != nil {
		s.Errorf("error fetching list of missing blobs: %v", err)
		return
	}

	s.notifyBlobsNeededListeners(allNeeded)
}

func (s *badgerStore) unmarkBlobsAsNeeded(blobs []ID) {
	node := s.db.State(true).NodeAt(missingBlobsKeypath, nil).(*state.DBNode)
	defer node.Close()

	for _, blobID := range blobs {
		blobIDKey, err := blobID.MarshalText()
		if err != nil {
			s.Errorf("can't marshal blobID %+v to string: %v", blobID, err)
			continue
		}

		err = node.Delete(state.Keypath(blobIDKey), nil)
		if err != nil {
			s.Errorf("error updating list of needed blobs: %v", err)
			continue
		}
	}
	err := node.Save()
	if err != nil {
		s.Errorf("error updating list of needed blobs: %v", err)
	}
}

func (s *badgerStore) OnBlobsNeeded(fn func(blobs []ID)) {
	s.blobsNeededListenersMu.Lock()
	defer s.blobsNeededListenersMu.Unlock()
	s.blobsNeededListeners = append(s.blobsNeededListeners, fn)
}

func (s *badgerStore) notifyBlobsNeededListeners(blobs []ID) {
	s.blobsNeededListenersMu.RLock()
	defer s.blobsNeededListenersMu.RUnlock()

	var wg sync.WaitGroup
	wg.Add(len(s.blobsNeededListeners))

	for _, handler := range s.blobsNeededListeners {
		handler := handler
		go func() {
			defer wg.Done()
			handler(blobs)
		}()
	}
	wg.Wait()
}

func (s *badgerStore) OnBlobsSaved(fn func()) {
	s.blobsSavedListenersMu.Lock()
	defer s.blobsSavedListenersMu.Unlock()
	s.blobsSavedListeners = append(s.blobsSavedListeners, fn)
}

func (s *badgerStore) notifyBlobsSavedListeners() {
	s.blobsSavedListenersMu.RLock()
	defer s.blobsSavedListenersMu.RUnlock()

	var wg sync.WaitGroup
	wg.Add(len(s.blobsSavedListeners))

	for _, handler := range s.blobsSavedListeners {
		handler := handler
		go func() {
			defer wg.Done()
			handler()
		}()
	}
	wg.Wait()
}

func (s *badgerStore) sha3ForSHA1(sha1 types.Hash) (types.Hash, error) {
	node := s.db.State(false)
	defer node.Close()

	sha3Bytes, exists, err := node.NodeAt(state.Keypath(sha1[:20]).Pushs("sha3"), nil).BytesValue(nil)
	if err != nil {
		return types.Hash{}, err
	} else if !exists {
		return types.Hash{}, types.Err404
	}
	var sha3 types.Hash
	copy(sha3[:], sha3Bytes)
	return sha3, err
}

func (s *badgerStore) sha1ForSHA3(sha3 types.Hash) (types.Hash, error) {
	node := s.db.State(false)
	defer node.Close()

	sha1Bytes, exists, err := node.NodeAt(state.Keypath(sha3[:]).Pushs("sha1"), nil).BytesValue(nil)
	if err != nil {
		return types.Hash{}, err
	} else if !exists {
		return types.Hash{}, types.Err404
	}
	var sha1 types.Hash
	copy(sha1[:], sha1Bytes)
	return sha1, err
}
