package blob

import (
	"bytes"
	"io"
	"sync"

	"github.com/pkg/errors"
	"golang.org/x/crypto/sha3"

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

var (
	manifestKey      = state.Keypath("manifest")
	chunkKey         = state.Keypath("chunk")
	missingBlobsKey  = state.Keypath("missing").Pushs("blobs")
	missingChunksKey = state.Keypath("missing").Pushs("chunks")
)

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

func (s *badgerStore) BlobReader(blobID ID) (io.ReadCloser, int64, error) {
	sha3, err := s.sha3ForBlobID(blobID)
	if err != nil {
		return nil, 0, err
	}

	node := s.db.State(false)
	defer node.Close()

	manifestKeypath := s.manifestKeypath(sha3)

	exists, err := node.Exists(manifestKeypath)
	if err != nil {
		return nil, 0, err
	} else if !exists {
		return nil, 0, errors.WithStack(types.Err404)
	}

	var manifest Manifest
	err = node.NodeAt(manifestKeypath, nil).Scan(&manifest)
	if err != nil {
		return nil, 0, err
	}
	return &blobReader{db: s.db, manifest: manifest}, int64(manifest.Size), nil
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

		err = s.storePrehashedChunk(chunkSha3, chunkBytes)
		if err != nil {
			return types.Hash{}, types.Hash{}, err
		}
	}

	sha1, sha3, chunkSha3s := chunker.Hashes()
	size := chunker.Size()

	err := s.StoreManifest(ID{HashAlg: types.SHA3, Hash: sha3}, Manifest{Size: size, ChunkSHA3s: chunkSha3s})
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
	err = node.Delete(s.missingBlobKeypath(sha3), nil)
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

func (s *badgerStore) HaveBlob(blobID ID) (bool, error) {
	sha3, err := s.sha3ForBlobID(blobID)
	if err != nil {
		return false, err
	}

	rootNode := s.db.State(false)
	defer rootNode.Close()

	manifestNode := rootNode.NodeAt(s.manifestKeypath(sha3), nil)

	exists, err := manifestNode.Exists(nil)
	if err != nil {
		return false, err
	} else if !exists {
		return false, nil
	}
	return s.haveAllChunksInManifest(rootNode, manifestNode)
}

func (s *badgerStore) VerifyBlobOrPrune(blobID ID) error {
	blobReader, length, err := s.BlobReader(blobID)
	if err != nil {
		return errors.Wrapf(err, "while verifying blob hash %v: %v", blobID, err)
	}
	defer blobReader.Close()

	var shouldPrune bool
	hasher := sha3.NewLegacyKeccak256()
	n, err := io.Copy(hasher, blobReader)
	if err != nil {
		return errors.Wrapf(err, "while verifying blob hash %v: %v", blobID, err)
	} else if n != length {
		s.Errorf("blob %v is incorrect length (expected %v, got %v)", blobID, length, n)
		shouldPrune = true
	} else if hash, _ := types.HashFromBytes(hasher.Sum(nil)); hash != blobID.Hash {
		s.Errorf("blob %v has incorrect hash (got %v)", blobID, hash.Hex())
		shouldPrune = true
	}

	if shouldPrune {
		// @@TODO
		// s.DeleteBlob()
	} else {
		s.unmarkBlobsAsNeeded([]ID{blobID})
	}
	return nil
}

func (s *badgerStore) Manifest(blobID ID) (Manifest, error) {
	sha3, err := s.sha3ForBlobID(blobID)
	if err != nil {
		return Manifest{}, err
	}

	node := s.db.State(false)
	defer node.Close()

	keypath := s.manifestKeypath(sha3)

	exists, err := node.Exists(keypath)
	if err != nil {
		return Manifest{}, err
	} else if !exists {
		return Manifest{}, types.Err404
	}

	var manifest Manifest
	err = node.NodeAt(keypath, nil).Scan(&manifest)
	if err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func (s *badgerStore) StoreManifest(blobID ID, manifest Manifest) error {
	sha3, err := s.sha3ForBlobID(blobID)
	if err != nil {
		return err
	}

	node := s.db.State(true)
	defer node.Close()

	err = node.Set(s.manifestKeypath(sha3), nil, manifest)
	if err != nil {
		return err
	}

	for _, chunkSHA3 := range manifest.ChunkSHA3s {
		exists, err := node.Exists(s.chunkKeypath(chunkSHA3))
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		err = node.Set(s.missingChunkKeypath(chunkSHA3), nil, true)
		if err != nil {
			return err
		}
	}

	return node.Save()
}

func (s *badgerStore) HaveManifest(blobID ID) (bool, error) {
	sha3, err := s.sha3ForBlobID(blobID)
	if err != nil {
		return false, err
	}

	node := s.db.State(false)
	defer node.Close()

	return node.Exists(s.manifestKeypath(sha3))
}

func (s *badgerStore) Chunk(sha3 types.Hash) ([]byte, error) {
	node := s.db.State(false)
	defer node.Close()

	keypath := s.chunkKeypath(sha3)

	exists, err := node.Exists(keypath)
	if err != nil {
		return nil, err
	} else if !exists {
		return nil, types.Err404
	}

	bytesVal, is, err := node.BytesValue(keypath)
	if err != nil {
		return nil, err
	} else if !is {
		return nil, errors.New("chunk is not a []byte")
	}
	return bytesVal, nil
}

func (s *badgerStore) StoreChunkIfHashMatches(expectedSHA3 types.Hash, chunkBytes []byte) error {
	sha3Hasher := sha3.NewLegacyKeccak256()
	_, err := io.Copy(sha3Hasher, bytes.NewReader(chunkBytes))
	if err != nil {
		return err
	}
	sha3Bytes := sha3Hasher.Sum(nil)
	if !bytes.Equal(expectedSHA3[:], sha3Bytes) {
		return errors.Wrapf(ErrWrongHash, "expected %v, got %0x", expectedSHA3.Hex(), sha3Bytes)
	}
	return s.storePrehashedChunk(expectedSHA3, chunkBytes)
}

func (s *badgerStore) storePrehashedChunk(chunkSha3 types.Hash, chunkBytes []byte) error {
	node := s.db.State(true)
	defer node.Close()

	chunkKeypath := state.Keypath(chunkSha3.Hex()).Pushs("chunk")

	err := node.Set(chunkKeypath, nil, chunkBytes)
	if err != nil {
		return err
	}
	err = node.Delete(s.missingChunkKeypath(chunkSha3), nil)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *badgerStore) haveAllChunksInManifest(rootNode, manifestNode state.Node) (bool, error) {
	chunksIter := manifestNode.ChildIterator(manifestKey, false, 0)
	defer chunksIter.Close()

	for chunksIter.Rewind(); chunksIter.Valid(); chunksIter.Next() {
		shaBytes := chunksIter.Node().Keypath().Part(-1)
		var chunkSHA3 types.Hash
		copy(chunkSHA3[:], shaBytes)
		exists, err := rootNode.Exists(s.chunkKeypath(chunkSHA3))
		if err != nil {
			return false, err
		} else if !exists {
			return false, nil
		}
	}
	return true, nil
}

func (s *badgerStore) HaveChunk(sha3 types.Hash) (bool, error) {
	node := s.db.State(false)
	defer node.Close()
	return node.Exists(s.chunkKeypath(sha3))
}

func (s *badgerStore) AllHashes() ([]ID, error) {
	rootNode := s.db.State(false)
	defer rootNode.Close()

	iter := rootNode.Iterator(nil, false, 0)
	defer iter.Close()

	var blobIDs []ID
	for iter.Rewind(); iter.Valid(); iter.Next() {
		manifestNode := iter.Node()
		if !manifestNode.Keypath().Part(-1).Equals(manifestKey) {
			continue
		}
		have, err := s.haveAllChunksInManifest(rootNode, manifestNode)
		if err != nil {
			s.Errorf("error checking for existence of blob chunks: %v", err)
			continue
		} else if !have {
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

func (s *badgerStore) BlobsNeeded() ([]ID, error) {
	node := s.db.State(false).NodeAt(missingBlobsKey, nil)
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

	node = node.NodeAt(missingBlobsKey, nil).(*state.DBNode)

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
	node := s.db.State(true).NodeAt(missingBlobsKey, nil).(*state.DBNode)
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

	sha3Bytes, exists, err := node.NodeAt(state.Keypath(sha1.Hex()).Pushs("sha3"), nil).BytesValue(nil)
	if err != nil {
		return types.Hash{}, err
	} else if !exists {
		return types.Hash{}, errors.WithStack(types.Err404)
	}
	var sha3 types.Hash
	copy(sha3[:], sha3Bytes)
	return sha3, err
}

func (s *badgerStore) sha1ForSHA3(sha3 types.Hash) (types.Hash, error) {
	node := s.db.State(false)
	defer node.Close()

	sha1Bytes, exists, err := node.NodeAt(state.Keypath(sha3.Hex()).Pushs("sha1"), nil).BytesValue(nil)
	if err != nil {
		return types.Hash{}, err
	} else if !exists {
		return types.Hash{}, errors.WithStack(types.Err404)
	}
	var sha1 types.Hash
	copy(sha1[:], sha1Bytes)
	return sha1, err
}

func (s *badgerStore) sha3ForBlobID(blobID ID) (types.Hash, error) {
	switch blobID.HashAlg {
	case types.SHA1:
		return s.sha3ForSHA1(blobID.Hash)
	case types.SHA3:
		return blobID.Hash, nil
	default:
		return types.Hash{}, errors.Errorf("unknown hash type '%v'", blobID.HashAlg)
	}
}

func (s *badgerStore) manifestKeypath(sha3 types.Hash) state.Keypath {
	return state.Keypath(sha3.Hex()).Pushs("manifest")
}

func (s *badgerStore) chunkKeypath(sha3 types.Hash) state.Keypath {
	return state.Keypath(sha3.Hex()).Pushs("chunk")
}

func (s *badgerStore) missingBlobKeypath(sha3 types.Hash) state.Keypath {
	return missingBlobsKey.Pushs(sha3.Hex())
}

func (s *badgerStore) missingChunkKeypath(sha3 types.Hash) state.Keypath {
	return missingChunksKey.Pushs(sha3.Hex())
}

func (s *badgerStore) Contents() (map[types.Hash]map[types.Hash]bool, error) {
	m := make(map[types.Hash]map[types.Hash]bool)

	rootNode := s.db.State(false)
	defer rootNode.Close()

	iter := rootNode.Iterator(nil, false, 0)
	defer iter.Close()

	for iter.Rewind(); iter.Valid(); iter.Next() {
		manifestNode := iter.Node()
		if !manifestNode.Keypath().Part(-1).Equals(manifestKey) {
			continue
		}

		sha3Hex := manifestNode.Keypath().Part(-2)
		sha3Hash, err := types.HashFromHex(sha3Hex.String())
		if err != nil {
			continue
		}

		if _, exists := m[sha3Hash]; !exists {
			m[sha3Hash] = make(map[types.Hash]bool)
		}

		manifest, err := s.Manifest(ID{HashAlg: types.SHA3, Hash: sha3Hash})
		if err != nil {
			return nil, err
		}

		for _, chunkSHA3 := range manifest.ChunkSHA3s {
			exists, err := rootNode.Exists(state.Keypath(chunkSHA3.Hex()).Push(chunkKey))
			if err != nil {
				return nil, err
			}
			m[sha3Hash][chunkSHA3] = exists
		}
	}
	return m, nil
}

func (s *badgerStore) DebugPrint() {
	keypaths, values, err := s.db.DebugPrint(nil, nil)
	if err != nil {
		panic(err)
	}

	for i := range keypaths {
		kp := keypaths[i]
		val := values[i]

		if kp.ContainsPart(chunkKey) {
			s.Debugf("%v: (chunk of length %v)", kp, len(val.([]byte)))
		} else if kp.ContainsPart(state.Keypath("manifest")) {
			switch val := val.(type) {
			case []interface{}:
				s.Debugf("%v: (manifest with %v entries)", kp, len(val))
			default:
				s.Debugf("%v: %0x", kp, val)
			}
		} else {
			s.Debugf("%v: %v", kp, val)
		}
	}
}
