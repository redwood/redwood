package blob

import (
	"crypto/sha1"
	"io"
	"io/ioutil"
	"sync"

	"github.com/dgraph-io/badger/v3"
	"golang.org/x/crypto/sha3"

	"redwood.dev/errors"
	"redwood.dev/log"
	"redwood.dev/state"
	"redwood.dev/types"
)

type badgerStore struct {
	log.Logger

	mu         *sync.RWMutex
	db         *state.DBTree
	badgerOpts badger.Options

	blobsNeededListeners   []func(blobs []ID)
	blobsNeededListenersMu sync.RWMutex
	blobsSavedListeners    []func()
	blobsSavedListenersMu  sync.RWMutex
}

var _ Store = (*badgerStore)(nil)

var (
	manifestKey     = state.Keypath("manifest")
	chunkKey        = state.Keypath("chunk")
	missingBlobsKey = state.Keypath("missing").Pushs("blobs")
)

const (
	DefaultMaxFetchConns uint64 = 4
)

func NewBadgerStore(badgerOpts badger.Options) *badgerStore {
	return &badgerStore{
		Logger:     log.NewLogger("blobstore"),
		mu:         &sync.RWMutex{},
		badgerOpts: badgerOpts,
	}
}

func (s *badgerStore) Start() error {
	s.Infof(0, "opening blob store at %v", s.badgerOpts.Dir)

	db, err := state.NewDBTree(s.badgerOpts)
	if err != nil {
		return err
	}
	s.db = db

	return nil
}

func (s *badgerStore) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.db.Close()
}

func (s *badgerStore) BlobReader(blobID ID, rng *types.Range) (io.ReadCloser, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	have, err := s.HaveBlob(blobID)
	if err != nil {
		return nil, 0, err
	} else if !have {
		return nil, 0, errors.WithStack(errors.Err404)
	}

	sha3, err := s.sha3ForBlobID(blobID)
	if err != nil {
		return nil, 0, err
	}

	node := s.db.State(false)
	defer node.Close()

	manifestKeypath := manifestKeypath(sha3)

	exists, err := node.Exists(manifestKeypath)
	if err != nil {
		return nil, 0, err
	} else if !exists {
		return nil, 0, errors.WithStack(errors.Err404)
	}

	var manifest Manifest
	err = node.NodeAt(manifestKeypath, nil).Scan(&manifest)
	if err != nil {
		return nil, 0, err
	}
	return NewReader(s.db, manifest, rng), int64(manifest.TotalSize), nil
}

func (s *badgerStore) StoreBlob(reader io.ReadCloser) (types.Hash, types.Hash, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	chunker := NewChunker(reader)
	defer chunker.Close()

	for !chunker.Done() {
		chunkBytes, chunk, err := chunker.Next()
		if err != nil {
			return types.Hash{}, types.Hash{}, err
		}

		err = s.storePrehashedChunk(chunk.SHA3, chunkBytes)
		if err != nil {
			return types.Hash{}, types.Hash{}, err
		}
	}

	manifest, sha1, sha3 := chunker.Summary()

	err := s.StoreManifest(ID{types.SHA3, sha3}, manifest)
	if err != nil {
		return types.Hash{}, types.Hash{}, err
	}

	err = s.markBlobPresentAndValid(sha1, sha3)
	if err != nil {
		return types.Hash{}, types.Hash{}, err
	}
	return sha1, sha3, nil
}

func (s *badgerStore) markBlobPresentAndValid(sha1, sha3 types.Hash) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	node := s.db.State(true)
	defer node.Close()

	err := node.Set(state.Keypath(sha1.Hex()).Pushs("sha3"), nil, sha3[:])
	if err != nil {
		return err
	}
	err = node.Set(state.Keypath(sha3.Hex()).Pushs("sha1"), nil, sha1[:])
	if err != nil {
		return err
	}
	err = node.Save()
	if err != nil {
		return err
	}
	s.Successf("saved blob (sha1: %v, sha3: %v)", sha1.Hex(), sha3.Hex())

	s.unmarkBlobsAsNeeded([]ID{
		{HashAlg: types.SHA1, Hash: sha1},
		{HashAlg: types.SHA3, Hash: sha3},
	})
	s.notifyBlobsSavedListeners()
	return nil
}

func (s *badgerStore) HaveBlob(blobID ID) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sha3, err := s.sha3ForBlobID(blobID)
	if errors.Cause(err) == errors.Err404 {
		return false, nil
	} else if err != nil {
		return false, err
	}

	rootNode := s.db.State(false)
	defer rootNode.Close()

	keypath := manifestKeypath(sha3)

	exists, err := rootNode.Exists(keypath)
	if err != nil {
		return false, err
	} else if !exists {
		return false, nil
	}

	var manifest Manifest
	err = rootNode.NodeAt(keypath, nil).Scan(&manifest)
	if errors.Cause(err) == errors.Err404 {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return haveAllChunks(rootNode, manifest.Chunks)
}

func (s *badgerStore) VerifyBlobOrPrune(blobID ID) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	valid, sha1, sha3, err := s.verifyBlob(blobID)
	if err != nil {
		return errors.Wrapf(err, "while verifying blob %v: %v", blobID, err)
	}

	if !valid {
		// @@TODO
		// s.DeleteBlob()
		return errors.Err404
	}

	err = s.markBlobPresentAndValid(sha1, sha3)
	if err != nil {
		return err
	}
	s.notifyBlobsSavedListeners()
	return nil
}

func (s *badgerStore) verifyBlob(blobID ID) (valid bool, sha1Hash types.Hash, sha3Hash types.Hash, err error) {
	blobReader, length, err := s.BlobReader(blobID, nil)
	if err != nil {
		return false, types.Hash{}, types.Hash{}, err
	}
	defer blobReader.Close()

	sha1Hasher := sha1.New()
	sha3Hasher := sha3.NewLegacyKeccak256()
	tee := io.TeeReader(io.TeeReader(blobReader, sha1Hasher), sha3Hasher)

	bs, err := ioutil.ReadAll(tee)
	if err != nil {
		return false, types.Hash{}, types.Hash{}, err
	} else if int64(len(bs)) != length {
		return false, types.Hash{}, types.Hash{}, err
	}

	sha1Hasher.Sum(sha1Hash[:0])
	sha3Hasher.Sum(sha3Hash[:0])

	if blobID.HashAlg == types.SHA1 && sha1Hash != blobID.Hash {
		s.Errorf("blob %v has incorrect hash (got %v)", blobID, sha1Hash.Hex())
		return false, types.Hash{}, types.Hash{}, nil
	} else if blobID.HashAlg == types.SHA3 && sha3Hash != blobID.Hash {
		s.Errorf("blob %v has incorrect hash (got %v)", blobID, sha3Hash.Hex())
		return false, types.Hash{}, types.Hash{}, nil
	}
	return true, sha1Hash, sha3Hash, nil
}

func (s *badgerStore) Manifest(blobID ID) (Manifest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sha3, err := s.sha3ForBlobID(blobID)
	if err != nil {
		return Manifest{}, err
	}

	node := s.db.State(false)
	defer node.Close()

	keypath := manifestKeypath(sha3)

	exists, err := node.Exists(keypath)
	if err != nil {
		return Manifest{}, err
	} else if !exists {
		return Manifest{}, errors.Err404
	}

	var manifest Manifest
	err = node.NodeAt(keypath, nil).Scan(&manifest)
	if err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func (s *badgerStore) StoreManifest(blobID ID, manifest Manifest) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sha3, err := s.sha3ForBlobID(blobID)
	if err != nil {
		return err
	}

	node := s.db.State(true)
	defer node.Close()

	err = node.Set(manifestKeypath(sha3), nil, manifest)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *badgerStore) HaveManifest(blobID ID) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sha3, err := s.sha3ForBlobID(blobID)
	if err != nil {
		return false, err
	}

	node := s.db.State(false)
	defer node.Close()
	return node.Exists(manifestKeypath(sha3))
}

func (s *badgerStore) Chunk(sha3 types.Hash) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	node := s.db.State(false)
	defer node.Close()

	keypath := chunkKeypath(sha3)

	exists, err := node.Exists(keypath)
	if err != nil {
		return nil, err
	} else if !exists {
		return nil, errors.Err404
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
	s.mu.RLock()
	defer s.mu.RUnlock()

	sha3 := types.HashBytes(chunkBytes)
	if sha3 != expectedSHA3 {
		return errors.Wrapf(ErrWrongHash, "expected %v, got %v (len: %v)", expectedSHA3.Hex(), sha3, len(chunkBytes))
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
	return node.Save()
}

func haveAllChunks(rootNode state.Node, chunks []ManifestChunk) (bool, error) {
	for _, chunk := range chunks {
		exists, err := rootNode.Exists(chunkKeypath(chunk.SHA3))
		if err != nil {
			return false, err
		} else if !exists {
			return false, nil
		}
	}
	return true, nil
}

func (s *badgerStore) HaveChunk(sha3 types.Hash) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	node := s.db.State(false)
	defer node.Close()
	return node.Exists(chunkKeypath(sha3))
}

func (s *badgerStore) BlobIDs() (sha1s, sha3s []ID, _ error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	contents, err := s.Contents()
	if err != nil {
		return nil, nil, err
	}

Outer:
	for sha3, chunks := range contents {
		for _, have := range chunks {
			if !have {
				continue Outer
			}
		}
		sha3s = append(sha3s, ID{HashAlg: types.SHA3, Hash: sha3})
	}

	node := s.db.State(false)
	defer node.Close()

	for _, id := range sha3s {
		sha1, err := sha1ForSHA3(id.Hash, node)
		if err != nil {
			return nil, nil, err
		}
		sha1s = append(sha1s, ID{HashAlg: types.SHA1, Hash: sha1})
	}
	return sha1s, sha3s, nil
}

func (s *badgerStore) BlobsNeeded() ([]ID, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	node := s.db.State(false).NodeAt(missingBlobsKey, nil)
	defer node.Close()

	iter := node.ChildIterator(nil, false, 0)
	defer iter.Close()

	var missingBlobsSlice []ID
	for iter.Rewind(); iter.Valid(); iter.Next() {
		childNode := iter.Node()
		var blobID ID
		err := blobID.UnmarshalText(childNode.Keypath().RelativeTo(node.Keypath()))
		if err != nil {
			return nil, errors.Wrapf(err, "while unmarshaling blobID from database (keypath: %v)", childNode.Keypath())
		}
		missingBlobsSlice = append(missingBlobsSlice, blobID)
	}
	return missingBlobsSlice, nil
}

func (s *badgerStore) MarkBlobsAsNeeded(blobs []ID) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var actuallyNeeded []ID
	for _, blobID := range blobs {
		have, err := s.HaveBlob(blobID)
		if err != nil {
			return errors.Wrapf(err, "while checking store for blob %v", blobID)
		}
		if !have {
			actuallyNeeded = append(actuallyNeeded, blobID)
		}
	}

	if len(actuallyNeeded) == 0 {
		return nil
	}

	node := s.db.State(true)
	defer node.Close()

	for _, blobID := range actuallyNeeded {
		err := node.Set(missingBlobKeypath(blobID), nil, true)
		if err != nil {
			return errors.Wrap(err, "while updating list of missing blobs")
			continue
		}
	}
	err := node.Save()
	if err != nil {
		return errors.Wrap(err, "while updating list of missing blobs")
	}

	s.notifyBlobsNeededListeners(actuallyNeeded)
	return nil
}

func (s *badgerStore) unmarkBlobsAsNeeded(blobs []ID) {
	node := s.db.State(true)
	defer node.Close()

	for _, blobID := range blobs {
		err := node.Delete(missingBlobKeypath(blobID), nil)
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

	keypath := state.Keypath(sha1.Hex()).Pushs("sha3")
	shaNode := node.NodeAt(keypath, nil)

	exists, err := shaNode.Exists(nil)
	if err != nil {
		return types.Hash{}, err
	} else if !exists {
		return types.Hash{}, errors.Err404
	}

	sha3Bytes, exists, err := shaNode.BytesValue(nil)
	if err != nil {
		return types.Hash{}, err
	} else if !exists {
		return types.Hash{}, errors.WithStack(errors.Err404)
	}
	return types.HashFromBytes(sha3Bytes)
}

func (s *badgerStore) sha1ForSHA3(sha3 types.Hash) (types.Hash, error) {
	node := s.db.State(false)
	defer node.Close()
	return sha1ForSHA3(sha3, node)
}

func sha1ForSHA3(sha3 types.Hash, node state.Node) (types.Hash, error) {
	sha1Bytes, exists, err := node.NodeAt(state.Keypath(sha3.Hex()).Pushs("sha1"), nil).BytesValue(nil)
	if err != nil {
		return types.Hash{}, err
	} else if !exists {
		return types.Hash{}, errors.WithStack(errors.Err404)
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

func manifestKeypath(sha3 types.Hash) state.Keypath {
	return state.Keypath(sha3.Hex()).Pushs("manifest")
}

func chunkKeypath(sha3 types.Hash) state.Keypath {
	return state.Keypath(sha3.Hex()).Pushs("chunk")
}

func missingBlobKeypath(blobID ID) state.Keypath {
	return missingBlobsKey.Pushs(blobID.String())
}

func (s *badgerStore) Contents() (map[types.Hash]map[types.Hash]bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

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

		for _, chunk := range manifest.Chunks {
			exists, err := rootNode.Exists(state.Keypath(chunk.SHA3.Hex()).Push(chunkKey))
			if err != nil {
				return nil, err
			}
			m[sha3Hash][chunk.SHA3] = exists
		}
	}
	return m, nil
}

func (s *badgerStore) MaxFetchConns() (uint64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	node := s.db.State(false)
	defer node.Close()

	maxFetchConns, is, err := node.UintValue(state.Keypath("maxFetchConns"))
	if err != nil && errors.Cause(err) != errors.Err404 {
		return 0, err
	} else if !is || errors.Cause(err) == errors.Err404 {
		return DefaultMaxFetchConns, nil
	}
	return maxFetchConns, nil
}

func (s *badgerStore) SetMaxFetchConns(maxFetchConns uint64) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	node := s.db.State(true)
	defer node.Close()

	err := node.Set(state.Keypath("maxFetchConns"), nil, maxFetchConns)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *badgerStore) DebugPrint() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keypaths, values, err := s.db.DebugPrint(nil, nil)
	if err != nil {
		panic(err)
	}

	for i := range keypaths {
		kp := keypaths[i]
		val := values[i]

		if kp.ContainsPart(chunkKey) {
			s.Debugf("%v: (chunk of length %v)\n", kp, len(val.([]byte)))
		} else if kp.ContainsPart(state.Keypath("manifest")) {
			switch val := val.(type) {
			case []interface{}:
				s.Debugf("%v: (manifest with %v entries)\n", kp, len(val))
			default:
				s.Debugf("%v: %0x\n", kp, val)
			}
		} else {
			s.Debugf("%v: %v\n", kp, val)
		}
	}
}
