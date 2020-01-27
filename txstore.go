package redwood

import (
	"github.com/brynbellomy/redwood/ctx"
	"github.com/brynbellomy/redwood/types"
)

type TxStore interface {
	Ctx() *ctx.Context
	Start() error

	AddTx(tx *Tx) error
	RemoveTx(stateURI string, txID types.ID) error
	FetchTx(stateURI string, txID types.ID) (*Tx, error)
	AllTxs() TxIterator
	AllTxsForStateURI(stateURI string) TxIterator
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
