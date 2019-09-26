package main

import (
	"encoding/json"
	"fmt"
	"sync"
)

type Store interface {
	AddTx(tx Tx) error
	RemoveTx(txID ID) error
	FetchTxs() ([]Tx, error)

	State() interface{}
	StateJSON() ([]byte, error)

	RegisterResolverForKeypath(keypath []string, resolver Resolver)
}

type store struct {
	mu           sync.RWMutex
	txs          map[ID]Tx
	resolverTree resolverTree
	currentState interface{}
}

func NewStore() Store {
	return &store{
		mu:           sync.RWMutex{},
		txs:          map[ID]Tx{},
		resolverTree: resolverTree{},
		currentState: nil,
	}
}

func (s *store) State() interface{} {
	return s.currentState
}

func (s *store) StateJSON() ([]byte, error) {
	return json.MarshalIndent(s.currentState, "", "    ")
}

func (s *store) RegisterResolverForKeypath(keypath []string, resolver Resolver) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.resolverTree.addResolver(keypath, resolver)
}

func (s *store) AddTx(tx Tx) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.txs[tx.ID] = tx

	for _, p := range tx.Patches {
		fmt.Println("patch:", p.String())

		var patch Patch = p
		var newState interface{}
		var err error
		for {
			resolver, parentResolverKeypath := s.resolverTree.resolverForKeypath(patch.Keys)
			fmt.Printf("resolver for keypath %v = %T\n", patch.Keys, resolver)

			patchCopy := patch
			patchCopy.Keys = patchCopy.Keys[len(parentResolverKeypath):]

			newState, err = resolver.ResolveState(patchCopy)
			if err != nil {
				return err
			}

			if len(parentResolverKeypath) == 0 {
				break
			}

			patch = Patch{Keys: parentResolverKeypath, Val: newState}
		}
		s.currentState = newState
	}

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

	return txs, nil
}
