package vault

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/sha3"

	"redwood.dev/errors"
	"redwood.dev/log"
	"redwood.dev/utils"
	. "redwood.dev/utils/generics"
)

type Store interface {
	Collections() ([]string, error)
	Items(collectionID string, oldestMtime time.Time, start, end uint64) ([]Item, error)
	ReadCloser(collectionID, itemID string) (io.ReadCloser, time.Time, error)
	Upsert(collectionID, itemID string, reader io.Reader) (bool, error)
	Delete(collectionID, itemID string) error
	DeleteAll() error
}

type DiskStore struct {
	log.Logger
	root string
	mu   *sync.RWMutex
}

type Item struct {
	CollectionID string
	ItemID       string
	Mtime        time.Time
}

var _ Store = (*DiskStore)(nil)

func NewDiskStore(root string) *DiskStore {
	return &DiskStore{
		Logger: log.NewLogger("diskstore"),
		root:   root,
		mu:     &sync.RWMutex{},
	}
}

func (s *DiskStore) Collections() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items, err := os.ReadDir(s.root)
	if err != nil {
		return nil, err
	}
	items = Filter(items, func(item os.DirEntry) bool { return item.IsDir() })
	collections := Map(items, func(item os.DirEntry) string { return item.Name() })
	return collections, nil
}

func (s *DiskStore) Items(collectionID string, oldestMtime time.Time, start, end uint64) ([]Item, error) {
	var manifest []string
	var err error
	func() {
		s.mu.RLock()
		defer s.mu.RUnlock()
		manifest, err = s.getManifest(collectionID)
	}()
	if err != nil {
		return nil, err
	} else if len(manifest) == 0 {
		return nil, nil
	} else if start > uint64(len(manifest)) {
		return nil, errors.Errorf("invalid start index (start=%v length=%v)", start, len(manifest))
	} else if end < start && end != 0 {
		return nil, errors.Errorf("invalid end index (end=%v length=%v)", end, len(manifest))
	}

	items, _ := MapWithError(manifest, func(itemID string) (Item, error) {
		stat, err := os.Stat(s.blobPath(collectionID, itemID))
		if err != nil {
			return Item{}, err
		}
		return Item{collectionID, itemID, stat.ModTime()}, nil
	})

	if !oldestMtime.IsZero() {
		items = Filter(items, func(item Item) bool { return item.Mtime.After(oldestMtime) })
	}

	if end == 0 {
		end = uint64(len(items))
	}
	return items[start:end], nil
}

func (s *DiskStore) ReadCloser(collectionID, itemID string) (io.ReadCloser, time.Time, error) {
	file, err := os.Open(s.blobPath(collectionID, itemID))
	if os.IsNotExist(err) {
		return nil, time.Time{}, errors.Err404
	} else if err != nil {
		return nil, time.Time{}, err
	}
	stat, err := file.Stat()
	if err != nil {
		return nil, time.Time{}, err
	}
	return file, stat.ModTime(), nil
}

func (s *DiskStore) Upsert(collectionID, itemID string, reader io.Reader) (alreadyKnown bool, err error) {
	defer errors.AddStack(&err)

	tmpRoot := filepath.Join(s.root, "tmp")
	err = os.MkdirAll(tmpRoot, 0700)
	if err != nil {
		s.Errorf("while creating tmpRoot: %v", err)
		return false, err
	}

	tmpFile, err := os.CreateTemp(tmpRoot, collectionID+"-"+itemID)
	if err != nil {
		s.Errorf("while creating temp file: %v", err)
		return false, err
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	hasher := sha3.New512()
	_, err = io.Copy(tmpFile, io.TeeReader(reader, hasher))
	if err != nil {
		s.Errorf("while copying from client: %v", err)
		return false, err
	}

	err = tmpFile.Close()
	if err != nil {
		s.Errorf("while closing temp file: %v", err)
		return false, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	newHash := hasher.Sum(nil)

	oldFile, err := os.Open(s.blobPath(collectionID, itemID))
	if os.IsNotExist(err) {
		// no-op
	} else if err != nil {
		s.Errorf("while opening oldFile: %v", err)
		return false, err
	} else {
		defer oldFile.Close()

		hasher = sha3.New512()
		_, err = io.Copy(hasher, oldFile)
		if err != nil {
			s.Errorf("while copying from oldFile: %v", err)
			return false, err
		}
		oldHash := hasher.Sum(nil)

		// If the file contents haven't changed, just delete the temp file
		if bytes.Equal(newHash, oldHash) {
			return true, nil
		}

		// If they have, move the temp file to its proper place and update the manifest
		if s.exists(collectionID, itemID) {
			err := s.Delete(collectionID, itemID)
			if err != nil {
				s.Errorf("while deleting existing file: %v", err)
				return false, err
			}
		}
	}

	err = utils.EnsureDirAndMaxPerms(s.collectionPath(collectionID), os.FileMode(0700))
	if err != nil {
		s.Errorf("while ensuring collection path: %v", err)
		return false, err
	}

	err = os.Rename(tmpFile.Name(), s.blobPath(collectionID, itemID))
	if err != nil {
		s.Errorf("while renaming temp file: %v", err)
		return false, err
	}

	err = s.maybeAppendToManifest(collectionID, itemID)
	if err != nil {
		s.Errorf("while appending to manifest: %v", err)
		return false, err
	}
	return false, nil
}

func (s *DiskStore) Delete(collectionID, itemID string) error {
	return os.Remove(s.blobPath(collectionID, itemID))
}

func (s *DiskStore) DeleteAll() error {
	return os.Remove(s.root)
}

func (s *DiskStore) exists(collectionID, itemID string) bool {
	_, err := os.Stat(s.blobPath(collectionID, itemID))
	return err == nil
}

func (s *DiskStore) collectionPath(collectionID string) string {
	return filepath.Join(s.root, collectionID)
}

func (s *DiskStore) blobPath(collectionID, itemID string) string {
	return filepath.Join(s.collectionPath(collectionID), itemID)
}

func (s *DiskStore) manifestPath(collectionID string) string {
	return filepath.Join(s.collectionPath(collectionID), "manifest.json")
}

func (s *DiskStore) maybeAppendToManifest(collectionID, itemID string) error {
	manifest, err := s.getManifest(collectionID)
	if err != nil {
		return err
	}

	if !Contains(manifest, itemID) {
		manifest = append(manifest, itemID)
	}
	return s.saveManifest(collectionID, manifest)
}

func (s *DiskStore) getManifest(collectionID string) ([]string, error) {
	var manifest []string
	f, err := os.Open(s.manifestPath(collectionID))
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	defer f.Close()

	err = json.NewDecoder(f).Decode(&manifest)
	if err != nil {
		return nil, err
	}
	return manifest, nil
}

func (s *DiskStore) saveManifest(collectionID string, manifest []string) error {
	f, err := os.Create(s.manifestPath(collectionID))
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(manifest)
}
