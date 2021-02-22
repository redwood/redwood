package utils

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"sync/atomic"
	"time"
)

type PeriodicTask struct {
	interval time.Duration
	mailbox  *Mailbox
	taskFn   func(ctx context.Context)
	chStop   chan struct{}
	chDone   chan struct{}
	i        int
}

func NewPeriodicTask(interval time.Duration, taskFn func(ctx context.Context)) *PeriodicTask {
	i := rand.Intn(10000)
	task := &PeriodicTask{
		interval,
		NewMailbox(1),
		taskFn,
		make(chan struct{}),
		make(chan struct{}),
		i,
	}

	go func() {
		defer close(task.chDone)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			fmt.Println("PeriodicTask loop", i)
			select {
			case <-ticker.C:
				fmt.Println("PeriodicTask ticker", i)
				task.Enqueue()

			case <-task.mailbox.Notify():
				for {
					x := task.mailbox.Retrieve()
					if x == nil {
						break
					}
					func() {
						ctx, cancel := CombinedContext(task.chStop, interval)
						defer cancel()
						task.taskFn(ctx)
					}()
				}

			case <-task.chStop:
				fmt.Println("PeriodicTask GOT STOP", i)
				return
			}
		}
	}()

	return task
}

func (task *PeriodicTask) Enqueue() {
	task.mailbox.Deliver(struct{}{})
}

func (task *PeriodicTask) Close() {
	fmt.Println("PeriodicTask CLOSE", task.i)
	close(task.chStop)
	<-task.chDone
	fmt.Println("PeriodicTask CLOSED", task.i)
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

// CombinedContext creates a context that finishes when any of the provided
// signals finish.  A signal can be a `context.Context`, a `chan struct{}`, or
// a `time.Duration` (which is transformed into a `context.WithTimeout`).
func CombinedContext(signals ...interface{}) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	if len(signals) == 0 {
		return ctx, cancel
	}
	signals = append(signals, ctx)

	var label string

	var cases []reflect.SelectCase
	var cancel2 context.CancelFunc
	for _, signal := range signals {
		var ch reflect.Value

		switch sig := signal.(type) {
		case string:
			label = sig
			continue
		case context.Context:
			ch = reflect.ValueOf(sig.Done())
		case <-chan struct{}:
			ch = reflect.ValueOf(sig)
		case chan struct{}:
			ch = reflect.ValueOf(sig)
		case time.Duration:
			var ctxTimeout context.Context
			ctxTimeout, cancel2 = context.WithTimeout(ctx, sig)
			ch = reflect.ValueOf(ctxTimeout.Done())
		default:
			continue
		}
		cases = append(cases, reflect.SelectCase{Chan: ch, Dir: reflect.SelectRecv})
	}

	go func() {
		defer cancel()
		if cancel2 != nil {
			defer cancel2()
		}
		fmt.Println("COMBINED CTX waiting", label)
		_, _, _ = reflect.Select(cases)
		if label != "" {
			fmt.Println("COMBINED CTX done", label)
		}
	}()

	// return ctx, cancel
	return context.WithCancel(ctx)
}
