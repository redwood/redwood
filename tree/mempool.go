package tree

import (
	"context"
	"sync"

	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/types"
	"redwood.dev/utils"
)

type Mempool interface {
	process.Interface
	Add(tx Tx)
	Get() *txSortedSet
	ForceReprocess()
}

type mempool struct {
	process.Process
	log.Logger
	chStop chan struct{}
	chDone chan struct{}

	sync.RWMutex
	txs   *txSortedSet
	chAdd chan Tx

	processMempoolWorkQueue *utils.Mailbox
	processCallback         func(tx Tx) processTxOutcome
}

func NewMempool(processCallback func(tx Tx) processTxOutcome) *mempool {
	return &mempool{
		Process:                 *process.New("Mempool"),
		Logger:                  log.NewLogger("mempool"),
		chStop:                  make(chan struct{}),
		chDone:                  make(chan struct{}),
		txs:                     newTxSortedSet(),
		chAdd:                   make(chan Tx, 100),
		processMempoolWorkQueue: utils.NewMailbox(0),
		processCallback:         processCallback,
	}
}

func (m *mempool) Start() error {
	err := m.Process.Start()
	if err != nil {
		return err
	}

	m.Process.Go(nil, "mempool", func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			case <-m.processMempoolWorkQueue.Notify():
				for {
					x := m.processMempoolWorkQueue.Retrieve()
					if x == nil {
						break
					}
					switch tx := x.(type) {
					case Tx:
						m.txs.add(tx)
					case struct{}:
					}
				}
				m.processMempool(ctx)
			}
		}
	})

	return nil
}

func (m *mempool) Get() *txSortedSet {
	return m.txs.copy()
}

func (m *mempool) Add(tx Tx) {
	m.processMempoolWorkQueue.Deliver(tx)
}

func (m *mempool) ForceReprocess() {
	m.processMempoolWorkQueue.Deliver(struct{}{})
}

type processTxOutcome int

const (
	processTxOutcome_Succeeded processTxOutcome = iota
	processTxOutcome_Failed
	processTxOutcome_Retry
)

func (m *mempool) processMempool(ctx context.Context) {
	txs := m.txs.takeAll()

	var retry []Tx
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		var anySucceeded bool
		retry = nil

		for _, tx := range txs {
			select {
			case <-ctx.Done():
				return
			default:
			}

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
	txs   map[types.Hash]Tx
	order []types.Hash
}

func newTxSortedSet() *txSortedSet {
	return &txSortedSet{
		txs:   make(map[types.Hash]Tx, 0),
		order: make([]types.Hash, 0),
	}
}

func (s *txSortedSet) takeAll() []Tx {
	s.Lock()
	defer s.Unlock()

	cp := make([]Tx, len(s.order))
	for i, hash := range s.order {
		cp[i] = s.txs[hash].Copy()
	}

	s.txs = make(map[types.Hash]Tx, 0)
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
	txs := make(map[types.Hash]Tx, len(s.txs))
	for hash, tx := range s.txs {
		txs[hash] = tx.Copy()
	}
	return &txSortedSet{txs: txs, order: order}
}

func (s *txSortedSet) add(tx Tx) {
	s.Lock()
	defer s.Unlock()
	if _, exists := s.txs[tx.Hash()]; !exists {
		s.txs[tx.Hash()] = tx.Copy()
		s.order = append(s.order, tx.Hash())
	}
}

func (s *txSortedSet) addMany(txs []Tx) {
	s.Lock()
	defer s.Unlock()
	for _, tx := range txs {
		s.txs[tx.Hash()] = tx.Copy()
		s.order = append(s.order, tx.Hash())
	}
}
