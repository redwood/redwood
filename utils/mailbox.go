package utils

import (
	"sync"

	"golang.org/x/net/context"

	. "redwood.dev/utils/generics"
)

type Mailbox[T any] struct {
	chNotify chan struct{}
	mu       sync.Mutex
	queue    []T
	capacity uint64
}

func NewMailbox[T any](capacity uint64) *Mailbox[T] {
	queueCap := capacity
	if queueCap == 0 {
		queueCap = 100
	}
	return &Mailbox[T]{
		chNotify: make(chan struct{}, 1),
		queue:    make([]T, 0, queueCap),
		capacity: capacity,
	}
}

func (m *Mailbox[T]) Notify() chan struct{} {
	return m.chNotify
}

func (m *Mailbox[T]) Deliver(x T) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.queue = append([]T{x}, m.queue...)
	if uint64(len(m.queue)) > m.capacity && m.capacity > 0 {
		m.queue = m.queue[:len(m.queue)-1]
	}

	select {
	case m.chNotify <- struct{}{}:
	default:
	}
}

func (m *Mailbox[T]) DeliverAll(x []T) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.queue = append(Reverse(x), m.queue...)
	if uint64(len(m.queue)) > m.capacity && m.capacity > 0 {
		m.queue = m.queue[:len(m.queue)-1]
	}

	select {
	case m.chNotify <- struct{}{}:
	default:
	}
}

func (m *Mailbox[T]) IngestFromChannel(ctx context.Context, ch <-chan T) {
	for {
		select {
		case <-ctx.Done():
			return

		case x, ok := <-ch:
			if !ok {
				return
			}
			m.Deliver(x)
		}
	}
}

func (m *Mailbox[T]) Retrieve() (T, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.queue) == 0 {
		var x T
		return x, false
	}
	x := m.queue[len(m.queue)-1]
	m.queue = m.queue[:len(m.queue)-1]
	return x, true
}

func (m *Mailbox[T]) RetrieveAll() []T {
	m.mu.Lock()
	defer m.mu.Unlock()
	queue := m.queue
	m.queue = nil
	for i, j := 0, len(queue)-1; i < j; i, j = i+1, j-1 {
		queue[i], queue[j] = queue[j], queue[i]
	}
	return queue
}

func (m *Mailbox[T]) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	queueCap := m.capacity
	if queueCap == 0 {
		queueCap = 100
	}
	m.queue = make([]T, 0, queueCap)
}
