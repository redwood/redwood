package redwood

import (
	"encoding/json"

	"github.com/dgraph-io/badger"

	"github.com/brynbellomy/redwood/ctx"
)

type badgerStore struct {
	*ctx.Context
	db         *badger.DB
	dbFilename string
	address    Address
}

func NewBadgerStore(dbFilename string, address Address) Store {
	return &badgerStore{
		Context:    &ctx.Context{},
		dbFilename: dbFilename,
		address:    address,
	}
}

func (p *badgerStore) Start() error {
	return p.CtxStart(
		// on startup,
		func() error {
			p.SetLogLabel(p.address.Pretty() + " store:badger")
			p.Infof(0, "opening badger store at %v", p.dbFilename)
			db, err := badger.Open(badger.DefaultOptions(p.dbFilename))
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

func makeTxKey(stateURI string, txID ID) []byte {
	return append([]byte("tx:"+stateURI+":"), txID[:]...)
}

func (p *badgerStore) AddTx(tx *Tx) error {
	bs, err := json.Marshal(tx)
	if err != nil {
		return err
	}

	key := makeTxKey(tx.URL, tx.ID)
	err = p.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, []byte(bs))
	})
	if err != nil {
		p.Errorf("failed to write tx %v", tx.ID)
		return err
	}
	p.Infof(0, "wrote tx %v", tx.ID)
	return nil
}

func (p *badgerStore) RemoveTx(stateURI string, txID ID) error {
	key := makeTxKey(stateURI, txID)
	return p.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}

func (p *badgerStore) FetchTx(stateURI string, txID ID) (*Tx, error) {
	key := makeTxKey(stateURI, txID)

	var bs []byte
	err := p.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			return Err404
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

func (p *badgerStore) AllTxs() TxIterator {
	return p.allTxs("tx:")
}

func (p *badgerStore) AllTxsForStateURI(stateURI string) TxIterator {
	return p.allTxs("tx:" + stateURI + ":")
}

func (p *badgerStore) allTxs(prefix string) TxIterator {
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

func (p *badgerStore) AddState(version ID, state interface{}) error {
	bs, err := json.Marshal(state)
	if err != nil {
		return err
	}

	key := append([]byte("state:"), version[:]...)

	err = p.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, []byte(bs))
	})
	if err != nil {
		p.Errorf("failed to write state %v", version.Hex())
		return err
	}
	p.Infof(0, "wrote state %v", version.Hex())
	return nil
}

func (p *badgerStore) FetchState(version ID) (interface{}, error) {
	key := append([]byte("state:"), version[:]...)

	var bs []byte
	err := p.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
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

	var state interface{}
	err = json.Unmarshal(bs, &state)
	return state, err
}
