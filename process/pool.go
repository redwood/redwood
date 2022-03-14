package process

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"

	"redwood.dev/utils"
)

type Pool struct {
	Process
	itemsAvailable *utils.Mailbox[PoolUniqueID]
	chItems        chan PoolUniqueID
	sem            *semaphore.Weighted
	retryInterval  time.Duration
	poolItems      map[PoolUniqueID]poolItem
	poolItemsMu    sync.RWMutex
}

func NewPool(name string, concurrency uint64, retryInterval time.Duration) *Pool {
	return &Pool{
		Process:        *New(name),
		itemsAvailable: utils.NewMailbox[PoolUniqueID](1000),
		chItems:        make(chan PoolUniqueID),
		sem:            semaphore.NewWeighted(int64(concurrency)),
		retryInterval:  retryInterval,
		poolItems:      make(map[PoolUniqueID]poolItem),
	}
}

type PoolUniqueID interface{}

type poolItem struct {
	item      PoolUniqueIDer
	state     poolItemState
	retryWhen time.Time
}

type PoolUniqueIDer interface {
	ID() PoolUniqueID
}

type poolItemState int

const (
	poolItemState_Available poolItemState = iota
	poolItemState_InUse
	poolItemState_Done
	poolItemState_InRetryPool
)

func (p *Pool) Start() error {
	err := p.Process.Start()
	if err != nil {
		return err
	}
	p.Process.Go(nil, "deliverAvailableItems", p.deliverAvailableItems)
	p.Process.Go(nil, "handleItemsAwaitingRetry", p.handleItemsAwaitingRetry)
	return nil
}

func (p *Pool) NumItemsPending() int {
	p.poolItemsMu.RLock()
	defer p.poolItemsMu.RUnlock()

	var n int
	for _, item := range p.poolItems {
		if item.state == poolItemState_Available || item.state == poolItemState_InRetryPool {
			n++
		}
	}
	return n
}

func (p *Pool) Add(item PoolUniqueIDer) {
	p.poolItemsMu.Lock()
	defer p.poolItemsMu.Unlock()

	_, exists := p.poolItems[item.ID()]
	if exists {
		return
	}
	p.poolItems[item.ID()] = poolItem{item, poolItemState_Available, time.Time{}}
	p.itemsAvailable.Deliver(item.ID())
}

func (p *Pool) Get(ctx context.Context) (item interface{}, err error) {
	err = p.sem.Acquire(ctx, 1)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			p.sem.Release(1)
		}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()

	case id := <-p.chItems:
		p.poolItemsMu.RLock()
		defer p.poolItemsMu.RUnlock()
		entry, exists := p.poolItems[id]
		if !exists {
			panic(fmt.Sprintf("(%T) %v", id, id))
		} else if entry.state != poolItemState_Available {
			panic(fmt.Sprintf("(%T) %v", id, id))
		}
		return entry.item, nil
	}
}

func (p *Pool) deliverAvailableItems(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.itemsAvailable.Notify():
			for _, id := range p.itemsAvailable.RetrieveAll() {
				var entry poolItem
				var exists bool
				func() {
					p.poolItemsMu.RLock()
					defer p.poolItemsMu.RUnlock()
					entry, exists = p.poolItems[id]
				}()
				if !exists {
					continue
				} else if entry.state != poolItemState_Available {
					panic("no")
				}

				select {
				case <-ctx.Done():
					return
				case p.chItems <- id:
				}
			}
		}
	}
}

func (p *Pool) setState(id PoolUniqueID, state poolItemState, retryWhen time.Time) {
	entry, exists := p.poolItems[id]
	if !exists {
		panic(fmt.Sprintf("(%T) %v", id, id))
		return
	}
	entry.state = state
	entry.retryWhen = retryWhen
	p.poolItems[id] = entry
}

func (p *Pool) RetryLater(id PoolUniqueID, when time.Time) {
	p.poolItemsMu.Lock()
	defer p.poolItemsMu.Unlock()
	p.setState(id, poolItemState_InRetryPool, when)
	p.sem.Release(1)
}

func (p *Pool) ForceRetry(id PoolUniqueID) {
	p.poolItemsMu.Lock()
	defer p.poolItemsMu.Unlock()

	entry, exists := p.poolItems[id]
	if !exists {
		panic(fmt.Sprintf("(%T) %v", id, id))
		return
	}

	switch entry.state {
	case poolItemState_Available:
	case poolItemState_InUse:
	case poolItemState_InRetryPool:
		p.setState(id, poolItemState_Available, time.Time{})
		p.itemsAvailable.Deliver(id)
	case poolItemState_Done:
	}
}

func (p *Pool) Complete(id PoolUniqueID) {
	p.poolItemsMu.Lock()
	defer p.poolItemsMu.Unlock()
	p.setState(id, poolItemState_Done, time.Time{})
	p.sem.Release(1)
}

func (p *Pool) handleItemsAwaitingRetry(ctx context.Context) {
	ticker := time.NewTicker(p.retryInterval)
	for {
		select {
		case <-p.Process.Done():
			return
		case <-ctx.Done():
			return

		case <-ticker.C:
			func() {
				p.poolItemsMu.Lock()
				defer p.poolItemsMu.Unlock()

				now := time.Now()

				for id, entry := range p.poolItems {
					if entry.state != poolItemState_InRetryPool {
						continue
					} else if !entry.retryWhen.Before(now) {
						continue
					}
					p.setState(id, poolItemState_Available, time.Time{})
					p.itemsAvailable.Deliver(id)
				}
			}()
		}
	}
}
