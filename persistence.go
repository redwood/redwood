package redwood

import (
	"github.com/brynbellomy/redwood/ctx"
)

type Persistence interface {
	Ctx() *ctx.Context
	Start() error

	AddTx(tx *Tx) error
	RemoveTx(txHash Hash) error
	FetchTx(txHash Hash) (*Tx, error)
	AllTxs() TxIterator
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
