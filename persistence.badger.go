package redwood

import (
	"encoding/json"

	"github.com/dgraph-io/badger"

	"github.com/brynbellomy/redwood/ctx"
)

type badgerPersistence struct {
	*ctx.Context
	db         *badger.DB
	dbFilename string
	address    Address
}

func NewBadgerPersistence(dbFilename string, address Address) Persistence {
	return &badgerPersistence{
		Context:    &ctx.Context{},
		dbFilename: dbFilename,
		address:    address,
	}
}

func (p *badgerPersistence) Start() error {
	return p.CtxStart(
		// on startup,
		func() error {
			p.SetLogLabel(p.address.Pretty() + " persistence:badger")
			p.Infof(0, "opening badger persistent store at %v", p.dbFilename)
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

func (p *badgerPersistence) AddTx(tx *Tx) error {
	bs, err := json.Marshal(tx)
	if err != nil {
		return err
	}

	hash := tx.Hash()
	key := append([]byte("tx:"), hash[:]...)

	err = p.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, []byte(bs))
	})
	if err != nil {
		p.Errorf("failed to write tx %v", hash)
		return err
	}
	p.Infof(0, "wrote tx %v", hash)
	return nil
}

func (p *badgerPersistence) RemoveTx(txHash Hash) error {
	key := append([]byte("tx:"), txHash[:]...)
	return p.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}

func (p *badgerPersistence) FetchTx(txHash Hash) (*Tx, error) {
	key := append([]byte("tx:"), txHash[:]...)

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

	var tx Tx
	err = json.Unmarshal(bs, &tx)
	return &tx, err
}

func (p *badgerPersistence) AllTxs() TxIterator {
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

			prefix := []byte("tx:")
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
