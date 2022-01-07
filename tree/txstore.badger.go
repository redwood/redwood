package tree

import (
	"github.com/dgraph-io/badger/v2"

	"redwood.dev/errors"
	"redwood.dev/log"
	"redwood.dev/state"
)

type badgerTxStore struct {
	log.Logger
	db         *badger.DB
	badgerOpts badger.Options
}

func NewBadgerTxStore(badgerOpts badger.Options) TxStore {
	return &badgerTxStore{
		Logger:     log.NewLogger("txstore"),
		badgerOpts: badgerOpts,
	}
}

func (p *badgerTxStore) Start() error {
	p.Infof(0, "opening txstore at %v", p.badgerOpts.Dir)

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

func makeTxKey(stateURI string, txID state.Version) []byte {
	return append([]byte("tx:"+stateURI+":"), txID[:]...)
}

func (p *badgerTxStore) AddTx(tx Tx) (err error) {
	defer errors.Annotate(&err, "badgerTxStore#AddTx")

	bs, err := tx.Marshal()
	if err != nil {
		return err
	}

	key := makeTxKey(tx.StateURI, tx.ID)
	err = p.db.Update(func(txn *badger.Txn) error {
		// Add the tx to the DB
		err := txn.Set(key, []byte(bs))
		if err != nil {
			return err
		}

		// Add the new tx to the `.Children` slice on each of its parents
		if tx.Status == TxStatusValid {
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

				parentTx.Children = state.NewVersionSet(parentTx.Children).Add(tx.ID).Slice()

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
		err = txn.Set([]byte("stateuri:"+tx.StateURI), nil)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		p.Errorf("failed to write tx %v: %v", tx.ID.Pretty(), err)
		return err
	}
	p.Infof(0, "wrote tx %v %v (status: %v)", tx.StateURI, tx.ID.Pretty(), tx.Status)
	return nil
}

func (p *badgerTxStore) RemoveTx(stateURI string, txID state.Version) error {
	key := makeTxKey(stateURI, txID)
	return p.db.Update(func(txn *badger.Txn) error {
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

func (p *badgerTxStore) AllTxsForStateURI(stateURI string, fromTxID state.Version) TxIterator {
	if fromTxID == (state.Version{}) {
		fromTxID = GenesisTxID
	}

	txIter := NewTxIterator()

	go func() {
		defer close(txIter.ch)

		stack := []state.Version{fromTxID}
		sent := make(map[state.Version]struct{})

		txIter.err = p.db.View(func(txn *badger.Txn) error {
			for len(stack) > 0 {
				txID := stack[0]
				stack = stack[1:]

				if _, wasSent := sent[txID]; wasSent {
					continue
				}

				item, err := txn.Get(makeTxKey(stateURI, txID))
				if err != nil {
					return err
				}

				var tx Tx
				err = item.Value(func(val []byte) error {
					return tx.Unmarshal(val)
				})
				if err != nil {
					return err
				}

				// for _, parentID := range tx.Parents {
				// 	if _, wasSent := sent[parentID]; !wasSent {
				// 		stack = append(stack, tx.ID)
				// 		continue OuterLoop
				// 	}
				// }

				select {
				case <-txIter.chClose:
					return nil
				case txIter.ch <- &tx:
				}

				sent[txID] = struct{}{}
				stack = append(stack, tx.Children...)
			}
			return nil
		})
	}()

	return txIter
}

func (s *badgerTxStore) KnownStateURIs() ([]string, error) {
	var stateURIs []string
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		iter := txn.NewIterator(opts)
		defer iter.Close()

		prefix := []byte("stateuri:")

		for iter.Seek(prefix); iter.ValidForPrefix(prefix); iter.Next() {
			stateURIs = append(stateURIs, string(iter.Item().Key()[len("stateuri:"):]))
		}
		return nil
	})
	return stateURIs, err
}

func (s *badgerTxStore) MarkLeaf(stateURI string, txID state.Version) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(append([]byte("leaf:"+stateURI+":"), txID[:]...), nil)
	})
}

func (s *badgerTxStore) UnmarkLeaf(stateURI string, txID state.Version) error {
	return s.db.Update(func(txn *badger.Txn) error {
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

			// var tx Tx
			// err = tx.Unmarshal(val)
			// if err != nil {
			// 	return err
			// }
			s.Debugf("%v: %0x", string(key), val)
		}
		return nil
	})
	if err != nil {
		s.Errorf("error: %v", err)
	}
}
