package redwood

import (
	"encoding/json"

	"github.com/dgraph-io/badger/v2"
	"github.com/pkg/errors"

	"github.com/brynbellomy/redwood/ctx"
	"github.com/brynbellomy/redwood/types"
)

type badgerTxStore struct {
	*ctx.Context
	db         *badger.DB
	dbFilename string
	address    types.Address
}

func NewBadgerTxStore(dbFilename string, address types.Address) TxStore {
	return &badgerTxStore{
		Context:    &ctx.Context{},
		dbFilename: dbFilename,
		address:    address,
	}
}

func (p *badgerTxStore) Start() error {
	return p.CtxStart(
		// on startup
		func() error {
			p.SetLogLabel(p.address.Pretty() + " store:badger")
			p.Infof(0, "opening badger store at %v", p.dbFilename)
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

func (p *badgerTxStore) AddTx(tx *Tx) error {
	bs, err := json.Marshal(tx)
	if err != nil {
		return err
	}

	key := makeTxKey(tx.URL, tx.ID)
	err = p.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, []byte(bs))
	})
	if err != nil {
		p.Errorf("failed to write tx %v", tx.ID.Pretty())
		return err
	}
	p.Infof(0, "wrote tx %v", tx.ID.Pretty())
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
	key := makeTxKey(stateURI, txID)

	var bs []byte
	err := p.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
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
	err = json.Unmarshal(bs, &tx)
	return &tx, err
}

func (p *badgerTxStore) AllTxs() TxIterator {
	return p.allTxs("tx:")
}

func (p *badgerTxStore) AllTxsForStateURI(stateURI string) TxIterator {
	return p.allTxs("tx:" + stateURI + ":")
}

func (p *badgerTxStore) allTxs(prefix string) TxIterator {
	txIter := &txIterator{
		ch:       make(chan *Tx),
		chCancel: make(chan struct{}),
	}

	go func() {
		defer close(txIter.ch)

		txIter.err = p.db.View(func(txn *badger.Txn) error {
			opts := badger.DefaultIteratorOptions
			opts.PrefetchSize = 10
			badgerIter := txn.NewIterator(opts)
			defer badgerIter.Close()

			prefix := []byte(prefix)
			for badgerIter.Seek(prefix); badgerIter.ValidForPrefix(prefix); badgerIter.Next() {
				item := badgerIter.Item()

				var bs []byte
				err := item.Value(func(val []byte) error {
					bs = append([]byte{}, val...)
					return nil
				})
				if err != nil {
					return err
				}

				var tx Tx
				err = json.Unmarshal(bs, &tx)
				if err != nil {
					return err
				}

				select {
				case <-txIter.chCancel:
					return nil
				case txIter.ch <- &tx:
				}
			}
			return nil
		})
	}()

	return txIter
}
