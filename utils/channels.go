package utils

import (
	"context"
	"reflect"
	"sync/atomic"
	"time"
)

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
	x         int
	chAdd     chan wgAdd
	chWait    chan struct{}
	chCtxDone <-chan struct{}
	chStop    chan struct{}
	waitCalls uint32
}

type wgAdd struct {
	i   int
	err chan string
}

func NewWaitGroupChan(ctx context.Context) *WaitGroupChan {
	wg := &WaitGroupChan{
		chAdd:  make(chan wgAdd),
		chWait: make(chan struct{}),
		chStop: make(chan struct{}),
	}
	if ctx != nil {
		wg.chCtxDone = ctx.Done()
	}

	go func() {
		var done bool
		for {
			select {
			case <-wg.chCtxDone:
				if !done {
					close(wg.chWait)
				}
				return
			case <-wg.chStop:
				if !done {
					close(wg.chWait)
				}
				return
			case wgAdd := <-wg.chAdd:
				if done {
					wgAdd.err <- "WaitGroupChan already finished. Do you need to add a bounding wg.Add(1) and wg.Done()?"
					return
				}
				wg.i += wgAdd.i
				if wg.i < 0 {
					wgAdd.err <- "called Done() too many times"
					close(wg.chWait)
					return
				} else if wg.i == 0 {
					done = true
					close(wg.chWait)
				}
				wgAdd.err <- ""
			}
		}
	}()

	return wg
}

func (wg *WaitGroupChan) Close() {
	close(wg.chStop)
}

func (wg *WaitGroupChan) Add(i int) {
	if atomic.LoadUint32(&wg.waitCalls) > 0 {
		panic("cannot call Add() after Wait()")
	}
	ch := make(chan string)
	select {
	case <-wg.chCtxDone:
	case <-wg.chStop:
	case wg.chAdd <- wgAdd{i, ch}:
		err := <-ch
		if err != "" {
			panic(err)
		}
	}
}

func (wg *WaitGroupChan) Done() {
	ch := make(chan string)
	select {
	case <-wg.chCtxDone:
	case <-wg.chStop:
	case <-wg.chWait:
	case wg.chAdd <- wgAdd{-1, ch}:
		err := <-ch
		if err != "" {
			panic(err)
		}
	}
}

func (wg *WaitGroupChan) Wait() <-chan struct{} {
	atomic.StoreUint32(&wg.waitCalls, 1)
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

	var cases []reflect.SelectCase
	var cancel2 context.CancelFunc
	for _, signal := range signals {
		var ch reflect.Value

		switch sig := signal.(type) {
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
		_, _, _ = reflect.Select(cases)
	}()

	return context.WithCancel(ctx)
}

type ChanContext chan struct{}

var _ context.Context = ChanContext(nil)

func (ch ChanContext) Deadline() (deadline time.Time, ok bool) {
	return time.Time{}, false
}

func (ch ChanContext) Done() <-chan struct{} {
	return ch
}

func (ch ChanContext) Err() error {
	select {
	case <-ch:
		return context.Canceled
	default:
		return nil
	}
}

func (ch ChanContext) Value(key interface{}) interface{} {
	return nil
}
