package tree

import (
	"redwood.dev/log"
	"redwood.dev/state"
	"redwood.dev/types"
)

//go:generate mockery --name TxStore --output ./mocks/ --case=underscore
type TxStore interface {
	Start() error
	Close()

	AddTx(tx Tx) error
	RemoveTx(stateURI string, txID state.Version) error
	TxExists(stateURI string, txID state.Version) (bool, error)
	FetchTx(stateURI string, txID state.Version) (Tx, error)
	AllTxsForStateURI(stateURI string, fromTxID state.Version) (TxIterator, error)
	AllValidTxsForStateURIOrdered(stateURI string, fromTxID state.Version) TxIterator
	IsStateURIWithData(stateURI string) (bool, error)
	StateURIsWithData() (types.Set[string], error)
	OnNewStateURIWithData(fn NewStateURIWithDataCallback)
	MarkLeaf(stateURI string, txID state.Version) error
	UnmarkLeaf(stateURI string, txID state.Version) error
	Leaves(stateURI string) ([]state.Version, error)

	DebugPrint()
}

type NewStateURIWithDataCallback func(stateURI string)

type TxIterator interface {
	Rewind()
	Valid() bool
	Next()
	Tx() *Tx
	Err() error
}

type allTxsForStateURIIterator struct {
	txStore   TxStore
	stateURI  string
	txIDs     []state.Version
	i         int
	currentTx *Tx
	err       error
}

func NewAllTxsIterator(txStore TxStore, stateURI string, txIDs []state.Version) *allTxsForStateURIIterator {
	return &allTxsForStateURIIterator{txStore, stateURI, txIDs, 0, nil, nil}
}

func (iter *allTxsForStateURIIterator) Rewind() {
	iter.i = 0
	iter.fetchTx()
}

func (iter *allTxsForStateURIIterator) Valid() bool {
	return iter.i < len(iter.txIDs) && iter.err == nil
}

func (iter *allTxsForStateURIIterator) Next() {
	iter.i++
	iter.fetchTx()
}

func (iter *allTxsForStateURIIterator) fetchTx() {
	if !iter.Valid() {
		iter.currentTx = nil
		return
	}

	tx, err := iter.txStore.FetchTx(iter.stateURI, iter.txIDs[iter.i])
	if err != nil {
		iter.currentTx = nil
		iter.err = err
		return
	}
	iter.currentTx = &tx
}

func (iter *allTxsForStateURIIterator) Tx() *Tx {
	return iter.currentTx
}

func (iter *allTxsForStateURIIterator) Err() error {
	return iter.err
}

type allValidTxsForStateURIOrderedIterator struct {
	txStore   TxStore
	stateURI  string
	stack     []state.Version
	currentTx *Tx
	err       error
	fromTxID  state.Version
	sent      types.Set[state.Version]
}

func NewAllValidTxsForStateURIOrderedIterator(
	txStore TxStore,
	stateURI string,
	fromTxID state.Version,
) *allValidTxsForStateURIOrderedIterator {
	return &allValidTxsForStateURIOrderedIterator{
		txStore:  txStore,
		stateURI: stateURI,
		fromTxID: fromTxID,
	}
}

var l = log.NewLogger("ITER")

func (iter *allValidTxsForStateURIOrderedIterator) Rewind() {
	iter.currentTx = nil
	iter.stack = []state.Version{iter.fromTxID}
	iter.sent = types.NewSet[state.Version](nil)
	iter.Next()
}

func (iter *allValidTxsForStateURIOrderedIterator) Valid() bool {
	return iter.currentTx != nil && iter.err == nil
}

func (iter *allValidTxsForStateURIOrderedIterator) Next() {
	iter.currentTx = nil

	for len(iter.stack) > 0 {
		txID := iter.stack[0]
		iter.stack = iter.stack[1:]

		if iter.sent.Contains(txID) {
			continue
		}
		iter.sent.Add(txID)

		tx, err := iter.txStore.FetchTx(iter.stateURI, txID)
		if err != nil {
			iter.currentTx = nil
			iter.err = err
			return
		} else if tx.Status != TxStatusValid {
			iter.currentTx = nil
			continue
		}

		iter.currentTx = &tx
		iter.stack = append(iter.stack, tx.Children...)
		return
	}
}

func (iter *allValidTxsForStateURIOrderedIterator) Tx() *Tx {
	return iter.currentTx
}

func (iter *allValidTxsForStateURIOrderedIterator) Err() error {
	return iter.err
}
