package main

import (
	"sync"
)

type Store interface {
	AddTx(tx Tx) error
	RemoveTx(txID ID) error
	FetchTxs() ([]Tx, error)

	RegisterResolverForKey(key string, resolver Resolver)
}

type store struct {
	mu        sync.RWMutex
	txs       map[ID]Tx
	resolvers map[string]Resolver
}

func (s *store) RegisterResolverForKey(key string, resolver Resolver) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.resolvers[key] = resolver
}

func (s *store) AddTx(tx Tx) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.txs[tx.ID] = tx

	return nil
}

func (s *store) RemoveTx(txID ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.txs, txID)

	return nil
}

func (s *store) FetchTxs() ([]Tx, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var txs []Tx
	for _, tx := range s.txs {
		txs = append(txs, tx)
	}

	return cp
}
