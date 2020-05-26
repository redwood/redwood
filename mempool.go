package redwood

import (
	"fmt"
	"sync"

	"github.com/brynbellomy/redwood/ctx"
	"github.com/brynbellomy/redwood/types"
)

type Mempool interface {
	Ctx() *ctx.Context
	Start() error

	Add(tx *Tx)
	Get() map[types.Hash]*Tx
	ForceReprocess()
}

type mempool struct {
	*ctx.Context
	address types.Address

	sync.RWMutex
	txSet            *txSet
	chAdd            chan *Tx
	chForceReprocess chan struct{}
	processCallback  func(tx *Tx) processTxOutcome
}

func NewMempool(address types.Address, processCallback func(tx *Tx) processTxOutcome) Mempool {
	return &mempool{
		Context:          &ctx.Context{},
		address:          address,
		txSet:            newTxSet(),
		chAdd:            make(chan *Tx, 100),
		chForceReprocess: make(chan struct{}, 1),
		processCallback:  processCallback,
	}
}

func (m *mempool) Start() error {
	return m.CtxStart(
		// on startup,
		func() error {
			m.SetLogLabel(m.address.Pretty() + " mempool")

			go m.mempoolLoop()

			return nil
		},
		nil,
		nil,
		// on shutdown
		func() {},
	)
}

func (m *mempool) Get() map[types.Hash]*Tx {
	return m.txSet.get()
}

func (m *mempool) Add(tx *Tx) {
	select {
	case <-m.Context.Done():
		return
	case m.chAdd <- tx:
	}
}

func (m *mempool) mempoolLoop() {
	for {
		select {
		case <-m.Context.Done():
			return

		case tx := <-m.chAdd:
			m.txSet.add(tx)
			m.ForceReprocess()

		case <-m.chForceReprocess:
			m.processMempool()
		}
	}
}

func (m *mempool) ForceReprocess() {
	select {
	case <-m.Context.Done():
		return
	case m.chForceReprocess <- struct{}{}:
	default:
	}
}

func (m *mempool) debugPrint() {
	var lines []string
	txs := m.txSet.get()
	for hash, tx := range txs {
		lines = append(lines, fmt.Sprintf("%v -> %v", hash, tx.ID))
	}
	m.Debugf("mempool = %v", PrettyJSON(lines))
}

type processTxOutcome int

const (
	processTxOutcome_Succeeded processTxOutcome = iota
	processTxOutcome_Failed
	processTxOutcome_Retry
)

func (m *mempool) processMempool() {
	txs := m.txSet.get()
	m.txSet.clear()

	for {
		var anySucceeded bool

		for _, tx := range txs {
			outcome := m.processCallback(tx)

			switch outcome {
			case processTxOutcome_Failed:
				delete(txs, tx.Hash())

			case processTxOutcome_Retry:
				// Leave it in the mempool

			case processTxOutcome_Succeeded:
				delete(txs, tx.Hash())
				anySucceeded = true

			default:
				panic("this should never happen")
			}
		}
		if !anySucceeded {
			break
		}
	}
	m.txSet.addMany(txs)
}

type txSet struct {
	sync.RWMutex
	txs map[types.Hash]*Tx
}

func newTxSet() *txSet {
	return &txSet{
		txs: make(map[types.Hash]*Tx),
	}
}

func (s *txSet) get() map[types.Hash]*Tx {
	s.RLock()
	defer s.RUnlock()

	cp := make(map[types.Hash]*Tx, len(s.txs))
	for _, tx := range s.txs {
		cp[tx.Hash()] = tx.Copy()
	}
	return cp
}

func (s *txSet) add(tx *Tx) {
	s.Lock()
	defer s.Unlock()
	s.txs[tx.Hash()] = tx.Copy()
}

func (s *txSet) addMany(txs map[types.Hash]*Tx) {
	if len(txs) == 0 {
		return
	}
	s.Lock()
	defer s.Unlock()
	for hash, tx := range txs {
		s.txs[hash] = tx.Copy()
	}
}

func (s *txSet) clear() {
	s.Lock()
	defer s.Unlock()
	s.txs = make(map[types.Hash]*Tx)
}
