package redwood

import (
	"bytes"
	"crypto/sha1"
	goerrors "errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/dgraph-io/badger/v2"
	"github.com/pkg/errors"
	"golang.org/x/crypto/sha3"

	"github.com/brynbellomy/redwood/ctx"
	"github.com/brynbellomy/redwood/types"
)

type RefStore interface {
	Ctx() *ctx.Context
	Start() error

	HaveObject(refID types.RefID) (bool, error)
	Object(refID types.RefID) (io.ReadCloser, int64, error)
	ObjectFilepath(refID types.RefID) (string, error)
	StoreObject(reader io.ReadCloser) (sha1Hash types.Hash, sha3Hash types.Hash, err error)
	AllHashes() ([]types.RefID, error)
}

type refStore struct {
	*ctx.Context

	rootPath string
	metadata *badger.DB
	fileMu   sync.Mutex
}

func NewRefStore(rootPath string) RefStore {
	return &refStore{
		Context:  &ctx.Context{},
		rootPath: rootPath,
	}
}

func (s *refStore) Start() error {
	return s.CtxStart(
		// on startup
		func() error {
			s.SetLogLabel("refstore")

			opts := badger.DefaultOptions(filepath.Join(s.rootPath, "metadata"))
			opts.Logger = nil

			db, err := badger.Open(opts)
			if err != nil {
				return err
			}
			s.metadata = db
			return nil
		},
		nil,
		nil,
		// on shutdown
		func() {
			s.metadata.Close()
		},
	)
}

func (s *refStore) ensureRootPath() error {
	return os.MkdirAll(filepath.Join(s.rootPath, "blobs"), 0700)
}

func (s *refStore) HaveObject(refID types.RefID) (bool, error) {
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

func (s *refStore) Object(refID types.RefID) (io.ReadCloser, int64, error) {
	s.fileMu.Lock()
	defer s.fileMu.Unlock()

	err := s.ensureRootPath()
	if err != nil {
		return nil, 0, err
	}

	switch refID.HashAlg {
	case types.SHA1:
		return s.objectBySHA1(refID.Hash)
	case types.SHA3:
		return s.objectBySHA3(refID.Hash)
	default:
		return nil, 0, errors.Errorf("unknown hash type '%v'", refID.HashAlg)
	}
}

func (s *refStore) ObjectFilepath(refID types.RefID) (string, error) {
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

func (s *refStore) objectBySHA1(hash types.Hash) (io.ReadCloser, int64, error) {
	sha3, err := s.sha3ForSHA1(hash)
	if err != nil {
		return nil, 0, err
	}
	return s.objectBySHA3(sha3)
}

func (s *refStore) objectBySHA3(sha3Hash types.Hash) (io.ReadCloser, int64, error) {
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

func (s *refStore) StoreObject(reader io.ReadCloser) (sha1Hash types.Hash, sha3Hash types.Hash, err error) {
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	defer annotate(&err, "refStore.StoreObject")

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

	bs := sha1Hasher.Sum(nil)
	copy(sha1Hash[:], bs)

	bs = sha3Hasher.Sum(nil)
	copy(sha3Hash[:], bs)

	err = tmpFile.Close()
	if err != nil {
		return types.Hash{}, types.Hash{}, err
	}

	err = os.Rename(tmpFile.Name(), s.filepathForSHA3Blob(sha3Hash))
	if err != nil {
		return sha1Hash, sha3Hash, err
	}

	err = s.metadata.Update(func(txn *badger.Txn) error {
		err := txn.Set(append(sha1Hash[:20], []byte(":sha3")...), sha3Hash[:])
		if err != nil {
			return err
		}
		return txn.Set(append(sha3Hash[:], []byte(":sha1")...), sha1Hash[:20])
	})
	if err != nil {
		return sha1Hash, sha3Hash, errors.Wrap(err, "error saving sha1<->sha3 mapping for ref")
	}

	s.Successf("saved ref (sha1: %v, sha3: %v)", sha1Hash.Hex(), sha3Hash.Hex())

	return sha1Hash, sha3Hash, nil
}

func (s *refStore) AllHashes() ([]types.RefID, error) {
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

func (s *refStore) sha3ForSHA1(hash types.Hash) (types.Hash, error) {
	sha1 := hash[:20]
	var sha3 types.Hash
	err := s.metadata.View(func(txn *badger.Txn) error {
		item, err := txn.Get(append(sha1, []byte(":sha3")...))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			copy(sha3[:], val)
			return nil
		})
	})
	if err == badger.ErrKeyNotFound {
		return types.Hash{}, types.Err404
	}
	return sha3, err
}

func (s *refStore) sha1ForSHA3(hash types.Hash) (types.Hash, error) {
	sha3 := hash[:]
	var sha1 types.Hash
	err := s.metadata.View(func(txn *badger.Txn) error {
		item, err := txn.Get(append(sha3, []byte(":sha1")...))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			copy(sha1[:], val)
			return nil
		})
	})
	if err == badger.ErrKeyNotFound {
		return types.Hash{}, types.Err404
	}
	return sha1, err
}

func (s *refStore) filepathForSHA3Blob(sha3Hash types.Hash) string {
	return filepath.Join(s.rootPath, "blobs", sha3Hash.Hex())
}

func (s *refStore) DebugPrint() {
	err := s.metadata.View(func(txn *badger.Txn) error {
		iter := txn.NewIterator(badger.DefaultIteratorOptions)
		defer iter.Close()
		for iter.Rewind(); iter.Valid(); iter.Next() {
			key := iter.Item().Key()
			val, err := iter.Item().ValueCopy(nil)
			if err != nil {
				return err
			}
			var keyStr string
			if bytes.HasSuffix(key, []byte(":sha3")) {
				keyStr = fmt.Sprintf("%0x:sha3", key[:len(key)-5])
			} else if bytes.HasSuffix(key, []byte(":sha1")) {
				keyStr = fmt.Sprintf("%0x:sha1", key[:len(key)-5])
			}
			s.Debugf("%s = %0x", keyStr, val)
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
}
