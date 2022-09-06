package tree

import (
	"bytes"
	"sync"

	"github.com/dgraph-io/badger/v3"

	"redwood.dev/errors"
	"redwood.dev/log"
	"redwood.dev/state"
	"redwood.dev/utils"
	"redwood.dev/utils/badgerutils"
	. "redwood.dev/utils/generics"
)

type badgerTxStore struct {
	log.Logger
	db                             *badger.DB
	badgerOpts                     badger.Options
	newStateURIWithDataListeners   []NewStateURIWithDataCallback
	newStateURIWithDataListenersMu sync.RWMutex
}

func NewBadgerTxStore(badgerOpts badger.Options) TxStore {
	return &badgerTxStore{
		Logger:     log.NewLogger("txstore"),
		badgerOpts: badgerOpts,
	}
}

func (p *badgerTxStore) Start() error {
	p.Infof("opening txstore at %v", p.badgerOpts.Dir)

	db, err := badger.Open(p.badgerOpts)
	if err != nil {
		return err
	}
	p.db = db
	return nil
}

func (s *badgerTxStore) Close() {
	if s.db != nil {
		s.Debugf("closing txstore")
		err := s.db.Close()
		if err != nil {
			s.Errorf("could not close txstore: %v", err)
		}
	}
}

func makeTxPrefixForStateURI(stateURI string) []byte {
	return []byte("tx:" + stateURI + ":")
}

func makeTxKey(stateURI string, txID state.Version) []byte {
	return append(makeTxPrefixForStateURI(stateURI), txID[:]...)
}

func decodeTxKey(key []byte) (stateURI string, txID state.Version, _ error) {
	parts := bytes.SplitN(key, []byte(":"), 3)
	if len(parts) != 3 {
		return "", state.Version{}, errors.Errorf("bad tx key: %v (%0x)", string(key), key)
	}
	stateURI = string(parts[1])
	copy(txID[:], parts[2])
	return
}

func makeStateURIKey(stateURI string) []byte {
	return []byte("stateuri:" + stateURI)
}

func (p *badgerTxStore) AddTx(tx Tx) (err error) {
	defer errors.Annotate(&err, "badgerTxStore#AddTx")

	bs, err := tx.Marshal()
	if err != nil {
		return err
	}

	var isNewStateURI bool

	key := makeTxKey(tx.StateURI, tx.ID)
	err = badgerutils.UpdateWithRetryOnConflict(p.db, func(txn *badger.Txn) error {
		// Add the tx to the DB
		err := txn.Set(key, []byte(bs))
		if err != nil {
			return err
		}

		// Add the new tx to the `.Children` slice on each of its parents
		if tx.Status == TxStatusValid {
			isNewStateURI, err = p.isStateURIWithData(txn, tx.StateURI)
			if err != nil {
				return err
			}

			for _, parentID := range tx.Parents {
				item, err := txn.Get(makeTxKey(tx.StateURI, parentID))
				if err != nil {
					return errors.Wrapf(err, "can't find parent %v of tx %v", parentID, tx.ID)
				}
				var parentTx Tx
				err = item.Value(func(val []byte) error {
					return parentTx.Unmarshal(val)
				})
				if err != nil {
					return err
				}

				newChildren := NewSet(parentTx.Children)
				newChildren.Add(tx.ID)
				parentTx.Children = newChildren.Slice()

				parentBytes, err := parentTx.Marshal()
				if err != nil {
					return err
				}

				err = txn.Set(makeTxKey(tx.StateURI, parentID), parentBytes)
				if err != nil {
					return err
				}
			}
		}

		// We need to keep track of all of the state URIs we know about
		err = txn.Set(makeStateURIKey(tx.StateURI), nil)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		p.Errorf("failed to write tx %v: %v", tx.ID.Pretty(), err)
		return err
	}

	if isNewStateURI {
		p.newStateURIWithDataListenersMu.RLock()
		defer p.newStateURIWithDataListenersMu.RUnlock()
		for _, fn := range p.newStateURIWithDataListeners {
			fn(tx.StateURI)
		}
	}

	p.Infof("wrote tx %v %v (status: %v)", tx.StateURI, tx.ID.Pretty(), tx.Status)
	return nil
}

func (p *badgerTxStore) RemoveTx(stateURI string, txID state.Version) error {
	key := makeTxKey(stateURI, txID)
	return badgerutils.UpdateWithRetryOnConflict(p.db, func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}

func (p *badgerTxStore) TxExists(stateURI string, txID state.Version) (bool, error) {
	key := makeTxKey(stateURI, txID)

	var exists bool
	err := p.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			exists = false
			return nil
		} else if err != nil {
			return errors.WithStack(err)
		}

		exists = true
		return nil
	})
	return exists, err
}

func (p *badgerTxStore) FetchTx(stateURI string, txID state.Version) (Tx, error) {
	var bs []byte
	err := p.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(makeTxKey(stateURI, txID))
		if err == badger.ErrKeyNotFound {
			return errors.WithStack(errors.Err404)
		} else if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			bs = append([]byte{}, val...)
			return nil
		})
	})
	if err != nil {
		return Tx{}, err
	}

	var tx Tx
	err = tx.Unmarshal(bs)
	return tx, err
}

func (p *badgerTxStore) AllTxsForStateURI(stateURI string, fromTxID state.Version) (TxIterator, error) {
	if fromTxID == (state.Version{}) {
		fromTxID = GenesisTxID
	}

	var txIDs []state.Version
	err := p.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		iter := txn.NewIterator(opts)
		defer iter.Close()

		prefix := makeTxPrefixForStateURI(stateURI)

		for iter.Seek(prefix); iter.ValidForPrefix(prefix); iter.Next() {
			uri, txID, err := decodeTxKey(iter.Item().Key())
			if err != nil {
				return err
			} else if uri != stateURI {
				return errors.New("wrong stateURI: " + uri)
			}

			txIDs = append(txIDs, txID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	txIter := NewAllTxsIterator(p, stateURI, txIDs)
	return txIter, nil
}

func (p *badgerTxStore) AllValidTxsForStateURIOrdered(stateURI string, fromTxID state.Version) TxIterator {
	if fromTxID == (state.Version{}) {
		fromTxID = GenesisTxID
	}
	return NewAllValidTxsForStateURIOrderedIterator(p, stateURI, fromTxID)
}

func (s *badgerTxStore) StateURIsWithData() (Set[string], error) {
	stateURIs := NewSet[string](nil)
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		iter := txn.NewIterator(opts)
		defer iter.Close()

		prefix := []byte("stateuri:")

		for iter.Seek(prefix); iter.ValidForPrefix(prefix); iter.Next() {
			stateURIs.Add(string(iter.Item().Key()[len("stateuri:"):]))
		}
		return nil
	})
	return stateURIs, err
}

func (s *badgerTxStore) IsStateURIWithData(stateURI string) (is bool, err error) {
	err = s.db.View(func(txn *badger.Txn) error {
		is, err = s.isStateURIWithData(txn, stateURI)
		return err
	})
	return
}

func (s *badgerTxStore) isStateURIWithData(txn *badger.Txn, stateURI string) (is bool, err error) {
	_, err = txn.Get(makeStateURIKey(stateURI))
	if errors.Cause(err) == badger.ErrKeyNotFound {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (s *badgerTxStore) OnNewStateURIWithData(fn NewStateURIWithDataCallback) {
	s.newStateURIWithDataListenersMu.Lock()
	defer s.newStateURIWithDataListenersMu.Unlock()
	s.newStateURIWithDataListeners = append(s.newStateURIWithDataListeners, fn)
}

func (s *badgerTxStore) MarkLeaf(stateURI string, txID state.Version) error {
	return badgerutils.UpdateWithRetryOnConflict(s.db, func(txn *badger.Txn) error {
		return txn.Set(append([]byte("leaf:"+stateURI+":"), txID[:]...), nil)
	})
}

func (s *badgerTxStore) UnmarkLeaf(stateURI string, txID state.Version) error {
	return badgerutils.UpdateWithRetryOnConflict(s.db, func(txn *badger.Txn) error {
		return txn.Delete(append([]byte("leaf:"+stateURI+":"), txID[:]...))
	})
}

func (s *badgerTxStore) Leaves(stateURI string) ([]state.Version, error) {
	var leaves []state.Version
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		iter := txn.NewIterator(opts)
		defer iter.Close()

		prefix := []byte("leaf:" + stateURI + ":")

		for iter.Seek(prefix); iter.ValidForPrefix(prefix); iter.Next() {
			txID := state.VersionFromBytes(iter.Item().Key()[len("leaf:"+stateURI+":"):])
			leaves = append(leaves, txID)
		}
		return nil
	})
	return leaves, err
}

func (s *badgerTxStore) DebugPrint() {
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Reverse = false
		opts.PrefetchValues = true
		opts.PrefetchSize = 10
		iter := txn.NewIterator(opts)
		defer iter.Close()

		for iter.Rewind(); iter.Valid(); iter.Next() {
			key := iter.Item().KeyCopy(nil)
			val, err := iter.Item().ValueCopy(nil)
			if err != nil {
				return err
			}

			var tx Tx
			err = tx.Unmarshal(val)
			if err != nil {
				return err
			}
			s.Debugf("%v: %v", string(key), utils.PrettyJSON(tx))
		}
		return nil
	})
	if err != nil {
		s.Errorf("error: %v", err)
	}
}
