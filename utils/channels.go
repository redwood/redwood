package utils

import (
	"context"
	"sync/atomic"
	"time"
)

type PeriodicTask struct {
	interval time.Duration
	mailbox  *Mailbox
	taskFn   func(ctx context.Context)
	chStop   chan struct{}
	chDone   chan struct{}
}

func NewPeriodicTask(interval time.Duration, taskFn func(ctx context.Context)) *PeriodicTask {
	task := &PeriodicTask{
		interval,
		NewMailbox(1),
		taskFn,
		make(chan struct{}),
		make(chan struct{}),
	}

	go func() {
		defer close(task.chDone)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				task.Enqueue()

			case <-task.mailbox.Notify():
				for {
					x := task.mailbox.Retrieve()
					if x == nil {
						break
					}
					func() {
						ctx, cancel := context.WithTimeout(context.Background(), interval)
						defer cancel()
						task.taskFn(ctx)
					}()
				}

			case <-task.chStop:
				return
			}
		}
	}()

	return task
}

func (task PeriodicTask) Enqueue() {
	task.mailbox.Deliver(struct{}{})
}

func (task PeriodicTask) Close() {
	close(task.chStop)
	<-task.chDone
}

// ContextFromChan creates a context that finishes when the provided channel
// receives or is closed.
func ContextFromChan(chStop <-chan struct{}) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		select {
		case <-chStop:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}

// WaitGroupChan creates a channel that closes when the provided sync.WaitGroup is done.
type WaitGroupChan struct {
	i         int
	chAdd     chan int
	chWait    chan struct{}
	waitCalls uint64
}

func NewWaitGroupChan() WaitGroupChan {
	wg := WaitGroupChan{
		chAdd:  make(chan int),
		chWait: make(chan struct{}),
	}

	go func() {
		defer close(wg.chWait)
		for {
			select {
			case i := <-wg.chAdd:
				wg.i += i
				if wg.i < 0 {
					panic("")
				} else if wg.i == 0 && atomic.LoadUint64(&wg.waitCalls) > 0 {
					return
				}

				// case
			}
		}
	}()

	return wg
}

func (wg WaitGroupChan) Add(i int) {
	select {
	case <-wg.chWait:
		panic("")
	case wg.chAdd <- i:
	}
}

func (wg WaitGroupChan) Done() {
	select {
	case <-wg.chWait:
		panic("")
	case wg.chAdd <- -1:
	}
}

func (wg WaitGroupChan) Wait() <-chan struct{} {
	atomic.AddUint64(&wg.waitCalls, 1)
	return wg.chWait
}
