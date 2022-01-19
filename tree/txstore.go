package tree

import (
	"redwood.dev/state"
	"redwood.dev/types"
)

type TxStore interface {
	Start() error
	Close()

	AddTx(tx Tx) error
	RemoveTx(stateURI string, txID state.Version) error
	TxExists(stateURI string, txID state.Version) (bool, error)
	FetchTx(stateURI string, txID state.Version) (Tx, error)
	AllTxsForStateURI(stateURI string, fromTxID state.Version) TxIterator
	IsStateURIWithData(stateURI string) (bool, error)
	StateURIsWithData() (types.StringSet, error)
	OnNewStateURIWithData(fn NewStateURIWithDataCallback)
	MarkLeaf(stateURI string, txID state.Version) error
	UnmarkLeaf(stateURI string, txID state.Version) error
	Leaves(stateURI string) ([]state.Version, error)

	DebugPrint()
}

type NewStateURIWithDataCallback func(stateURI string)

type TxIterator interface {
	Next() *Tx
	Close()
	Error() error
}

type txIterator struct {
	ch      chan *Tx
	chClose chan struct{}
	err     error
}

func NewTxIterator() *txIterator {
	return &txIterator{
		ch:      make(chan *Tx),
		chClose: make(chan struct{}),
	}
}

func (i *txIterator) Next() *Tx {
	select {
	case tx := <-i.ch:
		return tx
	case <-i.chClose:
		return nil
	}
}

func (i *txIterator) Close() {
	close(i.chClose)
}

func (i *txIterator) Error() error {
	return i.err
}
