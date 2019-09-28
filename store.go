package main

import (
	"encoding/json"
	"sync"

	log "github.com/sirupsen/logrus"
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
	ID           ID
	mu           sync.RWMutex
	txs          map[ID]Tx
	resolverTree resolverTree
	currentState interface{}
	timeDAG      map[ID]map[ID]bool
	leaves       map[ID]bool
}

func NewStore(id ID) Store {
	s := &store{
		ID:           id,
		mu:           sync.RWMutex{},
		txs:          map[ID]Tx{},
		resolverTree: resolverTree{},
		currentState: nil,
		timeDAG:      make(map[ID]map[ID]bool),
		leaves:       make(map[ID]bool),
	}

	s.RegisterResolverForKeypath([]string{}, NewDumbResolver())

	return s
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

	log.Infof("[store %v] new tx %v", s.ID, tx.ID)

	// Ignore duplicates
	if _, exists := s.txs[tx.ID]; exists {
		return nil
	}

	// Unmark parents as leaves
	for _, parentID := range tx.Parents {
		delete(s.leaves, parentID)
	}

	// Store the tx
	s.txs[tx.ID] = tx

	// Apply its changes to the state tree
	for _, p := range tx.Patches {
		var patch Patch = p
		var newState interface{}
		var err error
		for {
			resolver, parentResolverKeypath := s.resolverTree.resolverForKeypath(patch.Keys)

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

	j, err := s.StateJSON()
	if err != nil {
		return err
	}
	log.Infof("[store %v] state = %v", s.ID, string(j))

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

func (s *store) getAncestors(vids map[ID]bool) map[ID]bool {
	ancestors := map[ID]bool{}

	var mark_ancestors func(id ID)
	mark_ancestors = func(id ID) {
		if !ancestors[id] {
			ancestors[id] = true
			for parentID := range s.timeDAG[id] {
				mark_ancestors(parentID)
			}
		}
	}
	for parentID := range vids {
		mark_ancestors(parentID)
	}

	return ancestors
}
