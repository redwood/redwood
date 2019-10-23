// Package ctx provides project-agnostic utility code for Go projects
package ctx

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"sync"
	"syscall"
	"time"
)

// Ctx is an abstraction for a context that can stopped/aborted.
//
// Ctx serves as a vehicle between a Content expressed as *struct or as an interface (which can be upcast).
type Ctx interface {
	BaseContext() *Context
}

type childCtx struct {
	CtxID   []byte
	Ctx     Ctx
	Context *Context
}

// Context is a service helper.
type Context struct {
	context.Context
	logger

	FaultLog   []error
	FaultLimit int

	// internal
	ctxCancel     context.CancelFunc
	stopComplete  sync.WaitGroup
	stopMutex     sync.Mutex
	stopReason    string
	parent        *Context
	children      []childCtx
	childrenMutex sync.RWMutex

	// externally provided callbacks
	onAboutToStop      func()
	onChildAboutToStop func(inChild Ctx)
	onStopping         func()
}

func (c *Context) Ctx() *Context {
	return c
}

// CtxStopping allows callers to wait until this context is stopping.
func (c *Context) CtxStopping() <-chan struct{} {
	return c.Context.Done()
}

// CtxStart blocks until the given onStartup proc is complete  started up or stopped (if an err is returned).
//
// See notes for CtxInitiateStop()
func (c *Context) CtxStart(
	onStartup func() error,
	onAboutToStop func(),
	onChildAboutToStop func(inChild Ctx),
	onStopping func(),
) error {

	if c.CtxRunning() {
		panic("plan.Context already running")
	}

	c.onAboutToStop = onAboutToStop
	c.onChildAboutToStop = onChildAboutToStop
	c.FaultLimit = 1
	c.Context, c.ctxCancel = context.WithCancel(context.Background())

	var err error
	if onStartup != nil {
		err = onStartup()
	}

	// Even if onStopping == nil, we still need this call since the resulting c.stopComplete.Add(1) ensures
	// that this context's CtxStop() has actually somethong to wait on.
	Go(c, func(inCtx Ctx) {

		// This whole onStopping is a convenience as well as a self-documenting thing,  When looking at code,
		// some can make sense of a function passed as an "onStopping" proc more easily than some generic call that
		// is less discernable.
		select {
		case <-inCtx.BaseContext().CtxStopping():
		}

		if onStopping != nil {
			onStopping()
		}
	})

	if err != nil {
		c.Errorf("CtxStart failed: %v", err)
		c.CtxStop("CtxStart failed", nil)
		c.CtxWait()
	}

	return err
}

// CtxStop initiates a stop of this Context, calling inReleaseOnComplete.Done() when this context is fully stopped (if provided).
//
// If c.CtxStop() is being called for the first time, true is returned.
// If it has already been called (or is in-flight), then false is returned and this call effectively is a no-op (but still honors in inReleaseOnComplete).
//
//  The following are equivalent:
//
//          initiated := c.CtxStop(reason, waitGroup)
//
//          initiated := c.CtxStop(reason, nil)
//          c.CtxWait()
//          if waitGroup != nil {
//              waitGroup.Done()
//          }
//
// When c.CtxStop() is called (for the first time):
//  1. c.onAboutToStop() is called (if provided to c.CtxStart)
//  2. if c has a parent, c is detached and the parent's onChildAboutToStop(c) is called (if provided to c.CtxStart).
//  3. c.CtxStopChildren() is called, blocking until all children are stopped (recursive)
//  4. c.Context is cancelled, causing onStopping() to be called (if provided) and any <-CtxStopping() calls to be unblocked.
//  5. c's onStopping() is called (if provided to c.CtxStart)
//  6. After the last call to c.CtxGo() has completed, c.CtxWait() is released.
//
func (c *Context) CtxStop(
	inReason string,
	inReleaseOnComplete *sync.WaitGroup,
) bool {

	initiated := false

	c.stopMutex.Lock()
	if ctxCancel := c.ctxCancel; c.CtxRunning() && ctxCancel != nil {
		c.ctxCancel = nil
		c.stopReason = inReason
		c.Infof(2, "CtxStop (%s)", c.stopReason)

		// Hand it over to client-level execution to finish stopping/cleanup.
		if onAboutToStop := c.onAboutToStop; onAboutToStop != nil {
			c.onAboutToStop = nil
			onAboutToStop()
		}

		if c.parent != nil {
			c.parent.childStopping(c)
		}

		// Stop the all children so that leaf children are stopped first.
		c.CtxStopChildren("parent is stopping")

		// Calling this *after* stopping children causes the entire hierarchy to be closed/cancelled leaf-first.
		ctxCancel()

		initiated = true
	}
	c.stopMutex.Unlock()

	if inReleaseOnComplete != nil {
		c.CtxWait()
		inReleaseOnComplete.Done()
	}

	return initiated
}

// CtxWait blocks until this Context has completed stopping (following a CtxInitiateStop). Returns true if this call initiated the shutdown (vs another cause)
//
// THREADSAFE
func (c *Context) CtxWait() {
	c.stopComplete.Wait()
}

// CtxStopChildren initiates a stop on each child and blocks until complete.
func (c *Context) CtxStopChildren(inReason string) {

	childrenRunning := &sync.WaitGroup{}

	logInfo := c.LogV(2)

	c.childrenMutex.RLock()
	N := len(c.children)
	if logInfo {
		c.Infof(2, "%d children to stop", N)
	}
	if N > 0 {
		childrenRunning.Add(N)
		for i := N - 1; i >= 0; i-- {
			child := c.children[i].Ctx
			childC := child.BaseContext()
			if logInfo {
				c.Infof(2, "stopping child %s(%s)", childC.GetLogPrefix(), reflect.TypeOf(child).Elem().Name())
			}
			go childC.CtxStop(inReason, childrenRunning)
		}
	}
	c.childrenMutex.RUnlock()

	childrenRunning.Wait()
	if N > 0 && logInfo {
		c.Infof(2, "children stopped")
	}
}

// CtxGo is lesser more convenient variant of Go().
func (c *Context) CtxGo(
	inProcess func(),
) {
	c.stopComplete.Add(1)
	go func(c *Context) {
		inProcess()
		c.stopComplete.Done()
	}(c)
}

// CtxAddChild adds the given Context as a "child", where c will initiate CtxStop() on each child before c carries out its own stop.
func (c *Context) CtxAddChild(
	inChild Ctx,
	inID []byte,
) {

	c.childrenMutex.Lock()
	c.children = append(c.children, childCtx{
		CtxID: inID,
		Ctx:   inChild,
	})
	inChild.BaseContext().setParent(c)
	c.childrenMutex.Unlock()
}

// CtxGetChildByID returns the child Context with the match ID (or nil if not found).
func (c *Context) CtxGetChildByID(
	inChildID []byte,
) Ctx {
	var ctx Ctx

	c.childrenMutex.RLock()
	N := len(c.children)
	for i := 0; i < N; i++ {
		if bytes.Equal(c.children[i].CtxID, inChildID) {
			ctx = c.children[i].Ctx
		}
	}
	c.childrenMutex.RUnlock()

	return ctx
}

// CtxStopReason returns the reason provided by the stop initiator.
func (c *Context) CtxStopReason() string {
	return c.stopReason
}

// CtxStatus returns an error if this Context has not yet started, is stopping, or has stopped.
//
// THREADSAFE
func (c *Context) CtxStatus() error {

	if c.Context == nil {
		return ErrCtxNotRunning
	}

	return c.Context.Err()
}

// CtxRunning returns true has been started and has not stopped. See also CtxStopped().
// This is typically used to detect if/when a dependent workflow should cease.
//
//
// THREADSAFE
func (c *Context) CtxRunning() bool {
	if c.Context == nil {
		return false
	}

	select {
	case <-c.Context.Done():
		return false
	default:
	}

	return true
}

// CtxStopped returns true if CtxStop() is currently in flight (or has completed).
//
// Warning: this is NOT the opposite of CtxRunning().  c.CtxStopped() will return true
// when c.CtxStop() is called vs c.CtxRunning() will return true until all of c's children have
// stopped.
//
// THREADSAFE
func (c *Context) CtxStopped() bool {
	return c.Context == nil
}

// CtxOnFault is called when the given error has occurred an represents an unexpected fault that alone doesn't
// justify an emergency condition. (e.g. a db error while accessing a record).  Call this on unexpected errors.
//
// If inErr == nil, this call has no effect.
//
// THREADSAFE
func (c *Context) CtxOnFault(inErr error, inDesc string) {

	if inErr == nil {
		return
	}

	c.Error(inDesc, ": ", inErr)

	c.stopMutex.Lock()
	c.FaultLog = append(c.FaultLog, inErr)
	faultCount := len(c.FaultLog)
	c.stopMutex.Unlock()

	if faultCount < c.FaultLimit {
		return
	}

	c.CtxStop("fault limit reached", nil)
}

// setParent is internally called wheen attaching/detaching a child.
func (c *Context) setParent(inNewParent *Context) {
	if inNewParent != nil && c.parent != nil {
		panic("Context already has parent")
	}
	c.parent = inNewParent
}

// BaseContext allows the holder of a Ctx to get the raw/underlying Context.
func (c *Context) BaseContext() *Context {
	return c
}

// childStopping is internally called once the given child has been stopped and its c.onAboutToStop() has completed.
func (c *Context) childStopping(
	inChild *Context,
) {

	var native Ctx

	// Detach the child
	c.childrenMutex.Lock()
	inChild.setParent(nil)
	N := len(c.children)
	for i := 0; i < N; i++ {

		// A downside of Go is that a struct that embeds ctx.Context can show up as two interfaces:
		//    Ctx(&item.Context) and Ctx(&item)
		// Since all the callbacks need to be the latter "native" Ctx (so that it can be upcast to a client type),
		//    we must ensure that we search for ptr matches using the "base" Context but callback with the native.
		native = c.children[i].Ctx
		if native.BaseContext() == inChild {
			copy(c.children[i:], c.children[i+1:N])
			N--
			c.children[N].Ctx = nil
			c.children = c.children[:N]
			break
		}
		native = nil
	}
	c.childrenMutex.Unlock()

	if c.onChildAboutToStop != nil {
		c.onChildAboutToStop(native)
	}
}

// AttachInterruptHandler creates a new interupt handler and fuses it w/ the given context
func (c *Context) AttachInterruptHandler() {
	sigInbox := make(chan os.Signal, 1)

	signal.Notify(sigInbox, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		count := 0
		firstTime := int64(0)

		//timer := time.NewTimer(30 * time.Second)

		for sig := range sigInbox {
			count++
			curTime := time.Now().Unix()

			// Prevent un-terminated ^c character in terminal
			fmt.Println()

			if count == 1 {
				firstTime = curTime

				reason := "received " + sig.String()
				c.CtxStop(reason, nil)
			} else {
				if curTime > firstTime+3 {
					fmt.Println("\nReceived interrupt before graceful shutdown, terminating...")
					os.Exit(-1)
				}
			}
		}
	}()

	go func() {
		c.CtxWait()
		signal.Stop(sigInbox)
		close(sigInbox)
	}()

	c.Infof(0, "for graceful shutdown, \x1b[1m^c\x1b[0m or \x1b[1mkill -s SIGINT %d\x1b[0m", os.Getpid())
}

// Go starts inProcess() in its own go routine while preventing inHostCtx from stopping until inProcess has completed.
//
//  In effect, CtxGo(inHostCtx, inProcess) replaces:
//
//      go inProcess(inHostCtx)
//
// The presumption here is that inProcess will exit from some trigger *other* than inHost.CtxWait()
func Go(
	inHost Ctx,
	inProcess func(inHost Ctx),
) {
	inHost.BaseContext().stopComplete.Add(1)
	go func(inHost Ctx) {
		inProcess(inHost)
		inHost.BaseContext().stopComplete.Done()
	}(inHost)
}
