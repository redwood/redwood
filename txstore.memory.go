package redwood

import (
	"sync"

	"github.com/brynbellomy/redwood/ctx"
	"github.com/brynbellomy/redwood/types"
)

type memoryTxStore struct {
	sync.RWMutex
	txs map[string]map[types.ID]*Tx
}

func NewMemoryTxStore() TxStore {
	return &memoryTxStore{
		txs: make(map[string]map[types.ID]*Tx),
	}
}

func (s memoryTxStore) Ctx() *ctx.Context {
	return nil
}

func (s memoryTxStore) Start() error {
	return nil
}

func (s memoryTxStore) AddTx(tx *Tx) error {
	s.Lock()
	defer s.Unlock()
	if _, exists := s.txs[tx.URL]; !exists {
		s.txs[tx.URL] = make(map[types.ID]*Tx)
	}
	s.txs[tx.URL][tx.ID] = tx
	return nil
}

func (s memoryTxStore) RemoveTx(stateURI string, txID types.ID) error {
	s.Lock()
	defer s.Unlock()
	if _, exists := s.txs[stateURI]; !exists {
		return nil
	}
	delete(s.txs[stateURI], txID)
	if len(s.txs[stateURI]) == 0 {
		delete(s.txs, stateURI)
	}
	return nil
}

func (s memoryTxStore) TxExists(stateURI string, txID types.ID) (bool, error) {
	s.RLock()
	defer s.RUnlock()
	if _, exists := s.txs[stateURI]; !exists {
		return false, nil
	}
	_, exists := s.txs[stateURI][txID]
	return exists, nil
}

func (s memoryTxStore) FetchTx(stateURI string, txID types.ID) (*Tx, error) {
	s.RLock()
	defer s.RUnlock()
	if _, exists := s.txs[stateURI]; !exists {
		return nil, types.Err404
	}
	tx, exists := s.txs[stateURI][txID]
	if !exists {
		return nil, types.Err404
	}
	return tx, nil
}

func (s memoryTxStore) AllTxs() TxIterator {
	s.RLock()
	defer s.RUnlock()

	txIter := &txIterator{
		ch:       make(chan *Tx),
		chCancel: make(chan struct{}),
	}

	go func() {
		defer close(txIter.ch)

		for stateURI := range s.txs {
			for _, tx := range s.txs[stateURI] {
				select {
				case <-txIter.chCancel:
					return
				case txIter.ch <- tx:
				}
			}
		}
	}()

	return txIter
}

func (s memoryTxStore) AllTxsForStateURI(stateURI string) TxIterator {
	s.RLock()
	defer s.RUnlock()

	txIter := &txIterator{
		ch:       make(chan *Tx),
		chCancel: make(chan struct{}),
	}

	go func() {
		defer close(txIter.ch)

		for _, tx := range s.txs[stateURI] {
			select {
			case <-txIter.chCancel:
				return
			case txIter.ch <- tx:
			}
		}
	}()

	return txIter
}
