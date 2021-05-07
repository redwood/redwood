package blob

import (
	"crypto/sha1"
	goerrors "errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"golang.org/x/crypto/sha3"

	"redwood.dev/log"
	"redwood.dev/state"
	"redwood.dev/types"
)

type diskStore struct {
	log.Logger

	rootPath string
	metadata *state.DBTree
	fileMu   sync.Mutex

	blobsNeededListeners   []func(blobs []ID)
	blobsNeededListenersMu sync.RWMutex
	blobsSavedListeners    []func()
	blobsSavedListenersMu  sync.RWMutex
}

var _ Store = (*diskStore)(nil)

func NewDiskStore(rootPath string, metadataDB *state.DBTree) *diskStore {
	return &diskStore{
		Logger:   log.NewLogger("blobstore"),
		rootPath: rootPath,
		metadata: metadataDB,
	}
}

func (s *diskStore) Start() error {
	return s.ensureRootPath()
}

func (s *diskStore) Close() {}

func (s *diskStore) ensureRootPath() error {
	return os.MkdirAll(s.rootPath, 0777|os.ModeDir)
}

func (s *diskStore) HaveBlob(blobID ID) (bool, error) {
	s.fileMu.Lock()
	defer s.fileMu.Unlock()

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

	_, err := os.Stat(s.filepathForSHA3Blob(sha3))
	if os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, errors.WithStack(err)
	}
	return true, nil
}

func (s *diskStore) BlobReader(blobID ID) (io.ReadCloser, int64, error) {
	s.fileMu.Lock()
	defer s.fileMu.Unlock()

	err := s.ensureRootPath()
	if err != nil {
		return nil, 0, err
	}

	switch blobID.HashAlg {
	case types.SHA1:
		return s.blobWithSHA1(blobID.Hash)
	case types.SHA3:
		return s.blobWithSHA3(blobID.Hash)
	default:
		return nil, 0, errors.Errorf("unknown hash type '%v'", blobID.HashAlg)
	}
}

func (s *diskStore) blobWithSHA1(hash types.Hash) (io.ReadCloser, int64, error) {
	sha3, err := s.sha3ForSHA1(hash)
	if err != nil {
		return nil, 0, err
	}
	return s.blobWithSHA3(sha3)
}

func (s *diskStore) blobWithSHA3(sha3Hash types.Hash) (io.ReadCloser, int64, error) {
	filename := s.filepathForSHA3Blob(sha3Hash)
	stat, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return nil, 0, errors.Wrapf(types.Err404, "while looking up blob %v at %v", sha3Hash, filename)
	} else if err != nil {
		return nil, 0, err
	}

	f, err := os.Open(filename)
	if err != nil {
		return nil, 0, err
	}

	return f, stat.Size(), nil
}

func (s *diskStore) StoreBlob(reader io.ReadCloser) (sha1Hash types.Hash, sha3Hash types.Hash, err error) {
	s.fileMu.Lock()
	defer s.fileMu.Unlock()

	err = s.ensureRootPath()
	if err != nil {
		return types.Hash{}, types.Hash{}, err
	}

	tmpFile, err := ioutil.TempFile(s.rootPath, "temp-")
	if err != nil {
		return types.Hash{}, types.Hash{}, err
	}
	defer func() {
		closeErr := tmpFile.Close()
		if closeErr != nil && !goerrors.Is(closeErr, os.ErrClosed) {
			err = closeErr
		}
	}()

	sha1Hasher := sha1.New()
	sha3Hasher := sha3.NewLegacyKeccak256()
	tee := io.TeeReader(io.TeeReader(reader, sha1Hasher), sha3Hasher)

	_, err = io.Copy(tmpFile, tee)
	if err != nil {
		return types.Hash{}, types.Hash{}, err
	}

	sha1 := sha1Hasher.Sum(nil)
	copy(sha1Hash[:], sha1)

	sha3 := sha3Hasher.Sum(nil)
	copy(sha3Hash[:], sha3)

	err = tmpFile.Close()
	if err != nil {
		return types.Hash{}, types.Hash{}, err
	}

	err = os.Rename(tmpFile.Name(), s.filepathForSHA3Blob(sha3Hash))
	if err != nil {
		return sha1Hash, sha3Hash, err
	}

	err = s.updateMetadata(func(node state.Node) error {
		return multierr.Combine(
			node.Set(state.Keypath(sha1).Pushs("sha3"), nil, sha3),
			node.Set(state.Keypath(sha3).Pushs("sha1"), nil, sha1),
		)
	})
	if err != nil {
		return sha1Hash, sha3Hash, err
	}

	s.Successf("saved blob (sha1: %v, sha3: %v)", sha1Hash.Hex(), sha3Hash.Hex())

	s.unmarkBlobsAsNeeded([]ID{
		{HashAlg: types.SHA1, Hash: sha1Hash},
		{HashAlg: types.SHA3, Hash: sha3Hash},
	})
	s.notifyBlobsSavedListeners()

	return sha1Hash, sha3Hash, nil
}

func (s *diskStore) AllHashes() ([]ID, error) {
	s.fileMu.Lock()
	defer s.fileMu.Unlock()

	err := s.ensureRootPath()
	if err != nil {
		return nil, err
	}

	matches, err := filepath.Glob(filepath.Join(s.rootPath, "blobs", "*"))
	if err != nil {
		return nil, err
	}

	var blobIDs []ID
	for _, match := range matches {
		sha3Hash, err := types.HashFromHex(filepath.Base(match))
		if err != nil {
			// ignore (@@TODO: delete?  notify?)
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

func (s *diskStore) BlobsNeeded() ([]ID, error) {
	var missingBlobsSlice []ID
	err := s.viewMetadata(func(node state.Node) error {
		node = node.NodeAt(state.Keypath("missing"), nil)

		iter := node.ChildIterator(nil, false, 0)
		defer iter.Close()

		for iter.Rewind(); iter.Valid(); iter.Next() {
			var blobID ID
			err := blobID.UnmarshalText(iter.Node().Keypath().RelativeTo(node.Keypath()))
			if err != nil {
				s.Errorf("error unmarshaling blobID: %v", err)
				continue
			}
			missingBlobsSlice = append(missingBlobsSlice, blobID)
		}
		return nil
	})
	return missingBlobsSlice, err
}

func (s *diskStore) MarkBlobsAsNeeded(blobs []ID) {
	var actuallyNeeded []ID
	for _, blobID := range blobs {
		have, err := s.HaveBlob(blobID)
		if err != nil {
			s.Errorf("error checking blob diskStore for blob %v: %v", blobID, err)
			continue
		}
		if !have {
			actuallyNeeded = append(actuallyNeeded, blobID)
		}
	}

	if len(actuallyNeeded) == 0 {
		return
	}

	err := s.updateMetadata(func(node state.Node) error {
		node = node.NodeAt(state.Keypath("missing"), nil)

		for _, blobID := range actuallyNeeded {
			blobIDKey, err := blobID.MarshalText()
			if err != nil {
				s.Errorf("can't marshal blobID %+v to bytes: %v", blobID, err)
				continue
			}

			err = node.Set(state.Keypath(blobIDKey), nil, true)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		s.Errorf("error updating list of missing blobs: %v", err)
		// don't error out
	}

	allNeeded, err := s.BlobsNeeded()
	if err != nil {
		s.Errorf("error fetching list of missing blobs: %v", err)
		return
	}

	s.notifyBlobsNeededListeners(allNeeded)
}

func (s *diskStore) unmarkBlobsAsNeeded(blobs []ID) {
	err := s.updateMetadata(func(node state.Node) error {
		node = node.NodeAt(state.Keypath("missing"), nil)

		for _, blobID := range blobs {
			blobIDKey, err := blobID.MarshalText()
			if err != nil {
				s.Errorf("can't marshal blobID %+v to string: %v", blobID, err)
				continue
			}

			err = node.Delete(state.Keypath(blobIDKey), nil)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		s.Errorf("error updating list of needed blobs: %v", err)
	}
}

func (s *diskStore) OnBlobsNeeded(fn func(blobs []ID)) {
	s.blobsNeededListenersMu.Lock()
	defer s.blobsNeededListenersMu.Unlock()
	s.blobsNeededListeners = append(s.blobsNeededListeners, fn)
}

func (s *diskStore) notifyBlobsNeededListeners(blobs []ID) {
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

func (s *diskStore) OnBlobsSaved(fn func()) {
	s.blobsSavedListenersMu.Lock()
	defer s.blobsSavedListenersMu.Unlock()
	s.blobsSavedListeners = append(s.blobsSavedListeners, fn)
}

func (s *diskStore) notifyBlobsSavedListeners() {
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

func (s *diskStore) sha3ForSHA1(hash types.Hash) (types.Hash, error) {
	var sha3 types.Hash
	err := s.viewMetadata(func(node state.Node) error {
		sha1 := hash[:20]

		sha3Bytes, exists, err := node.NodeAt(state.Keypath(sha1).Pushs("sha3"), nil).BytesValue(nil)
		if err != nil {
			return err
		} else if !exists {
			return types.Err404
		}
		copy(sha3[:], sha3Bytes)
		return nil
	})
	return sha3, err
}

func (s *diskStore) sha1ForSHA3(hash types.Hash) (types.Hash, error) {
	var sha1 types.Hash
	err := s.viewMetadata(func(node state.Node) error {
		sha3 := hash[:]

		sha1Bytes, exists, err := node.NodeAt(state.Keypath(sha3).Pushs("sha1"), nil).BytesValue(nil)
		if err != nil {
			return err
		} else if !exists {
			return types.Err404
		}
		copy(sha1[:], sha1Bytes)
		return nil
	})
	return sha1, err
}

func (s *diskStore) viewMetadata(fn func(node state.Node) error) error {
	node := s.metadata.State(false)
	defer node.Close()

	return fn(node.NodeAt(state.Keypath("blob"), nil))
}

func (s *diskStore) updateMetadata(fn func(node state.Node) error) error {
	node := s.metadata.State(true)
	defer node.Close()

	err := fn(node.NodeAt(state.Keypath("blob"), nil))
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *diskStore) filepathForSHA3Blob(sha3Hash types.Hash) string {
	return filepath.Join(s.rootPath, sha3Hash.Hex())
}

// func (s *diskStore) DebugPrint() {
// 	err := s.metadata.View(func(txn *badger.Txn) error {
// 		iter := txn.NewIterator(badger.DefaultIteratorOptions)
// 		defer iter.Close()
// 		for iter.Rewind(); iter.ValidForPblobix([]byte("blob:")); iter.Next() {
// 			key := iter.Item().Key()
// 			val, err := iter.Item().ValueCopy(nil)
// 			if err != nil {
// 				return err
// 			}
// 			var keyStr string
// 			if bytes.HasSuffix(key, []byte(":sha3")) {
// 				keyStr = fmt.Sprintf("%0x:sha3", key[:len(key)-5])
// 			} else if bytes.HasSuffix(key, []byte(":sha1")) {
// 				keyStr = fmt.Sprintf("%0x:sha1", key[:len(key)-5])
// 			}
// 			s.Debugf("%s = %0x", keyStr, val)
// 		}
// 		return nil
// 	})
// 	if err != nil {
// 		panic(err)
// 	}
// }
