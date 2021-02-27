package redwood

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/pkg/errors"

	"redwood.dev/ctx"
	"redwood.dev/types"
	"redwood.dev/utils"
)

type badgerTxStore struct {
	*ctx.Context
	db         *badger.DB
	dbFilename string
}

func NewBadgerTxStore(dbFilename string) TxStore {
	return &badgerTxStore{
		Context:    &ctx.Context{},
		dbFilename: dbFilename,
	}
}

func (p *badgerTxStore) Start() error {
	return p.CtxStart(
		// on startup
		func() error {
			p.SetLogLabel("txstore")
			p.Infof(0, "opening txstore at %v", p.dbFilename)
			opts := badger.DefaultOptions(p.dbFilename)
			opts.Logger = nil
			db, err := badger.Open(opts)
			if err != nil {
				return err
			}
			p.db = db
			return nil
		},
		nil,
		nil,
		// on shutdown
		func() {
			p.db.Close()
		},
	)
}

func makeTxKey(stateURI string, txID types.ID) []byte {
	return append([]byte("tx:"+stateURI+":"), txID[:]...)
}

func (p *badgerTxStore) AddTx(tx *Tx) (err error) {
	defer utils.Annotate(&err, "badgerTxStore#AddTx")

	bs, err := tx.MarshalProto()
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
					return parentTx.UnmarshalProto(val)
				})
				if err != nil {
					return err
				}

				parentTx.Children = utils.NewIDSet(parentTx.Children).Add(tx.ID).Slice()

				parentBytes, err := parentTx.MarshalProto()
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
	p.Infof(0, "wrote tx %v (status: %v)", tx.ID.Pretty(), tx.Status)
	return nil
}

func (p *badgerTxStore) RemoveTx(stateURI string, txID types.ID) error {
	key := makeTxKey(stateURI, txID)
	return p.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}

func (p *badgerTxStore) TxExists(stateURI string, txID types.ID) (bool, error) {
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

func (p *badgerTxStore) FetchTx(stateURI string, txID types.ID) (*Tx, error) {
	var bs []byte
	err := p.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(makeTxKey(stateURI, txID))
		if err == badger.ErrKeyNotFound {
			return errors.WithStack(types.Err404)
		} else if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			bs = append([]byte{}, val...)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	var tx Tx
	err = tx.UnmarshalProto(bs)
	return &tx, err
}

func (p *badgerTxStore) AllTxsForStateURI(stateURI string, fromTxID types.ID) TxIterator {
	if fromTxID == (types.ID{}) {
		fromTxID = GenesisTxID
	}

	txIter := &txIterator{
		ch:       make(chan *Tx),
		chCancel: make(chan struct{}),
	}

	go func() {
		defer close(txIter.ch)

		stack := []types.ID{fromTxID}
		sent := make(map[types.ID]struct{})

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
					return tx.UnmarshalProto(val)
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
				case <-txIter.chCancel:
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

func (s *badgerTxStore) MarkLeaf(stateURI string, txID types.ID) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(append([]byte("leaf:"+stateURI+":"), txID[:]...), nil)
	})
}

func (s *badgerTxStore) UnmarkLeaf(stateURI string, txID types.ID) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(append([]byte("leaf:"+stateURI+":"), txID[:]...))
	})
}

func (s *badgerTxStore) Leaves(stateURI string) ([]types.ID, error) {
	var leaves []types.ID
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		iter := txn.NewIterator(opts)
		defer iter.Close()

		prefix := []byte("leaf:" + stateURI + ":")

		for iter.Seek(prefix); iter.ValidForPrefix(prefix); iter.Next() {
			txID := types.IDFromBytes(iter.Item().Key()[len("leaf:"+stateURI+":"):])
			leaves = append(leaves, txID)
		}
		return nil
	})
	return leaves, err
}
