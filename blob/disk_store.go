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

	refsNeededListeners   []func(refs []types.RefID)
	refsNeededListenersMu sync.RWMutex
	refsSavedListeners    []func()
	refsSavedListenersMu  sync.RWMutex
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
	return os.MkdirAll(filepath.Join(s.rootPath, "blobs"), 0777|os.ModeDir)
}

func (s *diskStore) HaveObject(refID types.RefID) (bool, error) {
	s.fileMu.Lock()
	defer s.fileMu.Unlock()

	var sha3 types.Hash
	switch refID.HashAlg {
	case types.SHA1:
		var err error
		sha3, err = s.sha3ForSHA1(refID.Hash)
		if err == types.Err404 {
			return false, nil
		} else if err != nil {
			return false, err
		}

	case types.SHA3:
		sha3 = refID.Hash

	default:
		return false, errors.Errorf("unknown hash type '%v'", refID.HashAlg)
	}

	_, err := os.Stat(s.filepathForSHA3Blob(sha3))
	if os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, errors.WithStack(err)
	}
	return true, nil
}

func (s *diskStore) Object(refID types.RefID) (io.ReadCloser, int64, error) {
	s.fileMu.Lock()
	defer s.fileMu.Unlock()

	err := s.ensureRootPath()
	if err != nil {
		return nil, 0, err
	}

	switch refID.HashAlg {
	case types.SHA1:
		return s.objectWithSHA1(refID.Hash)
	case types.SHA3:
		return s.objectWithSHA3(refID.Hash)
	default:
		return nil, 0, errors.Errorf("unknown hash type '%v'", refID.HashAlg)
	}
}

func (s *diskStore) ObjectFilepath(refID types.RefID) (string, error) {
	s.fileMu.Lock()
	defer s.fileMu.Unlock()

	switch refID.HashAlg {
	case types.SHA1:
		sha3Hash, err := s.sha3ForSHA1(refID.Hash)
		if err != nil {
			return "", err
		}
		return s.filepathForSHA3Blob(sha3Hash), nil

	case types.SHA3:
		return s.filepathForSHA3Blob(refID.Hash), nil
	default:
		return "", errors.Errorf("unknown hash type '%v'", refID.HashAlg)
	}
}

func (s *diskStore) objectWithSHA1(hash types.Hash) (io.ReadCloser, int64, error) {
	sha3, err := s.sha3ForSHA1(hash)
	if err != nil {
		return nil, 0, err
	}
	return s.objectWithSHA3(sha3)
}

func (s *diskStore) objectWithSHA3(sha3Hash types.Hash) (io.ReadCloser, int64, error) {
	filename := s.filepathForSHA3Blob(sha3Hash)
	stat, err := os.Stat(filename)
	if err != nil {
		return nil, 0, err
	}

	f, err := os.Open(filename)
	if err != nil {
		return nil, 0, err
	}

	return f, stat.Size(), nil
}

func (s *diskStore) StoreObject(reader io.ReadCloser) (sha1Hash types.Hash, sha3Hash types.Hash, err error) {
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

	s.Successf("saved ref (sha1: %v, sha3: %v)", sha1Hash.Hex(), sha3Hash.Hex())

	s.unmarkRefsAsNeeded([]types.RefID{
		{HashAlg: types.SHA1, Hash: sha1Hash},
		{HashAlg: types.SHA3, Hash: sha3Hash},
	})
	s.notifyRefsSavedListeners()

	return sha1Hash, sha3Hash, nil
}

func (s *diskStore) AllHashes() ([]types.RefID, error) {
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

	var refIDs []types.RefID
	for _, match := range matches {
		sha3Hash, err := types.HashFromHex(filepath.Base(match))
		if err != nil {
			// ignore (@@TODO: delete?  notify?)
			continue
		}
		refIDs = append(refIDs, types.RefID{HashAlg: types.SHA3, Hash: sha3Hash})

		sha1Hash, err := s.sha1ForSHA3(sha3Hash)
		if err != nil {
			continue
		}
		refIDs = append(refIDs, types.RefID{HashAlg: types.SHA1, Hash: sha1Hash})
	}
	return refIDs, nil
}

func (s *diskStore) RefsNeeded() ([]types.RefID, error) {
	node := s.metadata.State(false)
	defer node.Close()

	iter := node.ChildIterator(state.Keypath("blob").Pushs("missing"), false, 0)
	defer iter.Close()

	var missingRefsSlice []types.RefID
	for iter.Rewind(); iter.Valid(); iter.Next() {
		var refID types.RefID
		err := refID.UnmarshalText(iter.Node().Keypath())
		if err != nil {
			s.Errorf("error unmarshaling refID: %v", err)
			continue
		}
		missingRefsSlice = append(missingRefsSlice, refID)
	}
	return missingRefsSlice, nil
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

func (s *diskStore) MarkRefsAsNeeded(refs []types.RefID) {
	var actuallyNeeded []types.RefID
	for _, refID := range refs {
		have, err := s.HaveObject(refID)
		if err != nil {
			s.Errorf("error checking ref diskStore for ref %v: %v", refID, err)
			continue
		}
		if !have {
			actuallyNeeded = append(actuallyNeeded, refID)
		}
	}

	if len(actuallyNeeded) == 0 {
		return
	}

	err := s.updateMetadata(func(node state.Node) error {
		node = node.NodeAt(state.Keypath("missing"), nil)

		for _, refID := range actuallyNeeded {
			refIDKey, err := refID.MarshalText()
			if err != nil {
				s.Errorf("can't marshal refID %+v to bytes: %v", refID, err)
				continue
			}

			err = node.Set(state.Keypath(refIDKey), nil, true)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		s.Errorf("error updating list of missing refs: %v", err)
		// don't error out
	}

	allNeeded, err := s.RefsNeeded()
	if err != nil {
		s.Errorf("error fetching list of missing refs: %v", err)
		return
	}

	s.notifyRefsNeededListeners(allNeeded)
}

func (s *diskStore) unmarkRefsAsNeeded(refs []types.RefID) {
	err := s.updateMetadata(func(node state.Node) error {
		node = node.NodeAt(state.Keypath("missing"), nil)

		for _, refID := range refs {
			refIDKey, err := refID.MarshalText()
			if err != nil {
				s.Errorf("can't marshal refID %+v to string: %v", refID, err)
				continue
			}

			err = node.Delete(state.Keypath(refIDKey), nil)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		s.Errorf("error updating list of needed refs: %v", err)
	}
}

func (s *diskStore) OnRefsNeeded(fn func(refs []types.RefID)) {
	s.refsNeededListenersMu.Lock()
	defer s.refsNeededListenersMu.Unlock()
	s.refsNeededListeners = append(s.refsNeededListeners, fn)
}

func (s *diskStore) notifyRefsNeededListeners(refs []types.RefID) {
	s.refsNeededListenersMu.RLock()
	defer s.refsNeededListenersMu.RUnlock()

	var wg sync.WaitGroup
	wg.Add(len(s.refsNeededListeners))

	for _, handler := range s.refsNeededListeners {
		handler := handler
		go func() {
			defer wg.Done()
			handler(refs)
		}()
	}
	wg.Wait()
}

func (s *diskStore) OnRefsSaved(fn func()) {
	s.refsSavedListenersMu.Lock()
	defer s.refsSavedListenersMu.Unlock()
	s.refsSavedListeners = append(s.refsSavedListeners, fn)
}

func (s *diskStore) notifyRefsSavedListeners() {
	s.refsSavedListenersMu.RLock()
	defer s.refsSavedListenersMu.RUnlock()

	var wg sync.WaitGroup
	wg.Add(len(s.refsSavedListeners))

	for _, handler := range s.refsSavedListeners {
		handler := handler
		go func() {
			defer wg.Done()
			handler()
		}()
	}
	wg.Wait()
}

func (s *diskStore) sha3ForSHA1(hash types.Hash) (types.Hash, error) {
	node := s.metadata.State(false).NodeAt(state.Keypath("metadata"), nil)
	defer node.Close()

	sha1 := hash[:20]

	sha3Bytes, exists, err := node.NodeAt(state.Keypath(sha1).Pushs("sha3"), nil).BytesValue(nil)
	if err != nil {
		return types.Hash{}, err
	} else if !exists {
		return types.Hash{}, types.Err404
	}
	var sha3 types.Hash
	copy(sha3[:], sha3Bytes)
	return sha3, nil
}

func (s *diskStore) sha1ForSHA3(hash types.Hash) (types.Hash, error) {
	node := s.metadata.State(false).NodeAt(state.Keypath("metadata"), nil)
	defer node.Close()

	sha3 := hash[:]

	sha1Bytes, exists, err := node.NodeAt(state.Keypath(sha3).Pushs("sha1"), nil).BytesValue(nil)
	if err != nil {
		return types.Hash{}, err
	} else if !exists {
		return types.Hash{}, types.Err404
	}
	var sha1 types.Hash
	copy(sha1[:], sha1Bytes)
	return sha1, nil
}

func (s *diskStore) filepathForSHA3Blob(sha3Hash types.Hash) string {
	return filepath.Join(s.rootPath, "blobs", sha3Hash.Hex())
}

// func (s *diskStore) DebugPrint() {
// 	err := s.metadata.View(func(txn *badger.Txn) error {
// 		iter := txn.NewIterator(badger.DefaultIteratorOptions)
// 		defer iter.Close()
// 		for iter.Rewind(); iter.ValidForPrefix([]byte("blob:")); iter.Next() {
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
