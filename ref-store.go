package redwood

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/pkg/errors"
	"golang.org/x/crypto/sha3"
)

type RefStore interface {
	Object(hash Hash) (io.ReadCloser, string, error)
	StoreObject(reader io.ReadCloser, contentType string) (Hash, error)
	HaveObject(hash Hash) bool
	AllHashes() ([]Hash, error)
}

type refStore struct {
	rootPath   string
	metadataMu sync.Mutex
}

func NewRefStore(rootPath string) RefStore {
	return &refStore{rootPath: rootPath}
}

func (s *refStore) ensureRootPath() error {
	return os.MkdirAll(s.rootPath, 0755)
}

func (s *refStore) Object(hash Hash) (io.ReadCloser, string, error) {
	err := s.ensureRootPath()
	if err != nil {
		return nil, "", err
	}

	f, err := os.Open(filepath.Join(s.rootPath, "ref-"+hash.String()))
	if err != nil {
		return nil, "", err
	}

	contentType, err := s.contentType(hash)
	if err != nil {
		return nil, "", err
	}

	return f, contentType, nil
}

func (s *refStore) StoreObject(reader io.ReadCloser, contentType string) (h Hash, err error) {
	defer annotate(&err, "refStore.StoreObject")

	err = s.ensureRootPath()
	if err != nil {
		return Hash{}, err
	}

	f, err := ioutil.TempFile(s.rootPath, "temp-")
	if err != nil {
		return Hash{}, err
	}
	defer f.Close()

	hasher := sha3.NewLegacyKeccak256()
	tee := io.TeeReader(reader, hasher)

	_, err = io.Copy(f, tee)
	if err != nil {
		return Hash{}, err
	}

	bs := hasher.Sum(nil)
	var hash Hash
	copy(hash[:], bs)

	f.Close()

	err = os.Rename(f.Name(), filepath.Join(s.rootPath, "ref-"+hash.String()))
	if err != nil {
		return hash, err
	}

	err = s.setContentType(hash, contentType)
	if err != nil {
		return hash, err
	}

	return hash, nil
}

func (s *refStore) HaveObject(hash Hash) bool {
	return fileExists(filepath.Join(s.rootPath, "ref-"+hash.String()))
}

func (s *refStore) contentType(hash Hash) (string, error) {
	s.metadataMu.Lock()
	defer s.metadataMu.Unlock()

	f, err := os.Open(filepath.Join(s.rootPath, "metadata.json"))
	if err != nil {
		return "", err
	}
	defer f.Close()

	var metadata map[string]interface{}
	err = json.NewDecoder(f).Decode(&metadata)
	if err != nil {
		return "", err
	}

	contentType, exists := getString(metadata, []string{hash.String(), "contentType"})
	if !exists {
		return "", nil
	}

	return contentType, nil
}

func (s *refStore) setContentType(hash Hash, contentType string) error {
	s.metadataMu.Lock()
	defer s.metadataMu.Unlock()

	f, err := os.OpenFile(filepath.Join(s.rootPath, "metadata.json"), os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return err
	}
	defer f.Close()

	var metadata map[string]interface{}
	err = json.NewDecoder(f).Decode(&metadata)
	if errors.Cause(err) == io.EOF {
		metadata = make(map[string]interface{})
	} else if err != nil {
		return err
	}

	setValueAtKeypath(metadata, []string{hash.String(), "contentType"}, contentType, true)

	_, err = f.Seek(0, 0)
	if err != nil {
		return err
	}

	err = json.NewEncoder(f).Encode(metadata)
	if err != nil {
		return err
	}
	return nil
}

func (s *refStore) AllHashes() ([]Hash, error) {
	err := s.ensureRootPath()
	if err != nil {
		return nil, err
	}

	matches, err := filepath.Glob(filepath.Join(s.rootPath, "ref-*"))
	if err != nil {
		return nil, err
	}

	var refHashes []Hash
	for _, match := range matches {
		hash, err := HashFromHex(filepath.Base(match)[4:])
		if err != nil {
			// ignore (@@TODO: delete?  notify?)
			continue
		}
		refHashes = append(refHashes, hash)
	}
	return refHashes, nil
}
