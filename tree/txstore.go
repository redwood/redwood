package tree

import (
	"redwood.dev/state"
)

type TxStore interface {
	Start() error
	Close()

	AddTx(tx Tx) error
	RemoveTx(stateURI string, txID state.Version) error
	TxExists(stateURI string, txID state.Version) (bool, error)
	FetchTx(stateURI string, txID state.Version) (Tx, error)
	AllTxsForStateURI(stateURI string, fromTxID state.Version) TxIterator
	KnownStateURIs() ([]string, error)
	MarkLeaf(stateURI string, txID state.Version) error
	UnmarkLeaf(stateURI string, txID state.Version) error
	Leaves(stateURI string) ([]state.Version, error)
}

type TxIterator interface {
	Next() *Tx
	Cancel()
	Error() error
}

type txIterator struct {
	ch       chan *Tx
	chCancel chan struct{}
	err      error
}

func (i *txIterator) Next() *Tx {
	select {
	case tx := <-i.ch:
		return tx
	case <-i.chCancel:
		return nil
	}
}

func (i *txIterator) Cancel() {
	close(i.chCancel)
}

func (i *txIterator) Error() error {
	return i.err
}
