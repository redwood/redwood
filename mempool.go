package redwood

import (
	"sync"

	"github.com/brynbellomy/redwood/ctx"
	"github.com/brynbellomy/redwood/types"
)

type Mempool interface {
	Ctx() *ctx.Context
	Start() error

	Add(tx *Tx)
	Get() *txSortedSet
	ForceReprocess()
}

type mempool struct {
	*ctx.Context

	sync.RWMutex
	txs   *txSortedSet
	chAdd chan *Tx

	processMempoolWorkQueue WorkQueue
	processCallback         func(tx *Tx) processTxOutcome
}

func NewMempool(processCallback func(tx *Tx) processTxOutcome) Mempool {
	mp := &mempool{
		Context:         &ctx.Context{},
		txs:             newTxSortedSet(),
		chAdd:           make(chan *Tx, 100),
		processCallback: processCallback,
	}

	mp.processMempoolWorkQueue = NewWorkQueue(1, mp.processMempool)

	return mp
}

func (m *mempool) Start() error {
	return m.CtxStart(
		// on startup
		func() error {
			m.SetLogLabel("mempool")

			go m.mempoolLoop()

			return nil
		},
		nil,
		nil,
		// on shutdown
		func() {
			m.processMempoolWorkQueue.Stop()
		},
	)
}

func (m *mempool) Get() *txSortedSet {
	return m.txs.copy()
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
			m.txs.add(tx)
			m.processMempoolWorkQueue.Enqueue()
		}
	}
}

func (m *mempool) ForceReprocess() {
	m.processMempoolWorkQueue.Enqueue()
}

type processTxOutcome int

const (
	processTxOutcome_Succeeded processTxOutcome = iota
	processTxOutcome_Failed
	processTxOutcome_Retry
)

func (m *mempool) processMempool() {
	txs := m.txs.takeAll()

	var retry []*Tx
	for {
		var anySucceeded bool
		retry = nil

		for _, tx := range txs {
			outcome := m.processCallback(tx)

			switch outcome {
			case processTxOutcome_Failed:
				// Discard it

			case processTxOutcome_Retry:
				// Leave it in the mempool
				retry = append(retry, tx)

			case processTxOutcome_Succeeded:
				anySucceeded = true

			default:
				panic("this should never happen")
			}
		}
		txs = retry
		if !anySucceeded {
			break
		}
	}
	if len(retry) > 0 {
		m.txs.addMany(retry)
	}
}

type txSortedSet struct {
	sync.RWMutex
	txs   map[types.Hash]*Tx
	order []types.Hash
}

func newTxSortedSet() *txSortedSet {
	return &txSortedSet{
		txs:   make(map[types.Hash]*Tx, 0),
		order: make([]types.Hash, 0),
	}
}

func (s *txSortedSet) takeAll() []*Tx {
	s.Lock()
	defer s.Unlock()

	cp := make([]*Tx, len(s.order))
	for i, hash := range s.order {
		cp[i] = s.txs[hash].Copy()
	}

	s.txs = make(map[types.Hash]*Tx, 0)
	s.order = make([]types.Hash, 0)

	return cp
}

func (s *txSortedSet) copy() *txSortedSet {
	s.RLock()
	defer s.RUnlock()

	order := make([]types.Hash, len(s.order))
	for i, hash := range s.order {
		order[i] = hash
	}
	txs := make(map[types.Hash]*Tx, len(s.txs))
	for hash, tx := range s.txs {
		txs[hash] = tx.Copy()
	}
	return &txSortedSet{txs: txs, order: order}
}

func (s *txSortedSet) add(tx *Tx) {
	s.Lock()
	defer s.Unlock()
	if _, exists := s.txs[tx.Hash()]; !exists {
		s.txs[tx.Hash()] = tx.Copy()
		s.order = append(s.order, tx.Hash())
	}
}

func (s *txSortedSet) addMany(txs []*Tx) {
	s.Lock()
	defer s.Unlock()
	for _, tx := range txs {
		s.txs[tx.Hash()] = tx.Copy()
		s.order = append(s.order, tx.Hash())
	}
}
