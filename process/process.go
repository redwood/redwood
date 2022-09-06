package process

import (
	"context"
	"errors"
	"sync"

	"redwood.dev/log"
	"redwood.dev/utils"
	. "redwood.dev/utils/generics"
)

type Process struct {
	log.Logger

	name       string
	mu         sync.RWMutex
	children   map[Spawnable]struct{}
	goroutines Set[*goroutine]
	state      State

	closeOnce   sync.Once
	doneOnce    sync.Once
	chStop      chan struct{}
	chDone      chan struct{}
	chAutoclose chan struct{}
	wg          sync.WaitGroup

	debug bool
}

var _ Interface = (*Process)(nil)

type Interface interface {
	Spawnable
	ProcessTreer
	Autoclose()
	AutocloseWithCleanup(closeFn func())
	Ctx() context.Context
	NewChild(ctx context.Context, name string) *Process
	SpawnChild(ctx context.Context, child Spawnable) error
	Go(ctx context.Context, name string, fn func(ctx context.Context)) <-chan struct{}
}

type Spawnable interface {
	Name() string
	Start() error
	Close() error
	Done() <-chan struct{}
	State() State
}

type ProcessTreer interface {
	Spawnable
	ProcessTree() map[string]interface{}
}

var _ Spawnable = (*Process)(nil)

func New(name string) *Process {
	name = name + " #" + utils.RandomNumberString()
	return &Process{
		name:       name,
		children:   make(map[Spawnable]struct{}),
		goroutines: NewSet[*goroutine](nil),
		chStop:     make(chan struct{}),
		chDone:     make(chan struct{}),
		Logger:     log.NewLogger(""),
	}
}

func (p *Process) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state != Unstarted {
		panic(p.name + " already started")
	}
	p.state = Started

	return nil
}

func (p *Process) Close() error {
	p.closeOnce.Do(func() {
		// p.Infof(p.name+" shutting down...")
		func() {
			p.mu.Lock()
			defer p.mu.Unlock()

			if p.state != Started {
				panic(p.name + " not started")
			}
			p.state = Closed
		}()

		close(p.chStop)
		p.wg.Wait()
		close(p.chDone)
	})
	return nil
}

func (p *Process) Autoclose() {
	go func() {
		p.wg.Wait()
		p.Close()
	}()
}

func (p *Process) AutocloseWithCleanup(closeFn func()) {
	go func() {
		p.wg.Wait()
		closeFn()
		p.Close()
	}()
}

func (p *Process) Name() string {
	return p.name
}

func (p *Process) State() State {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state
}

func (p *Process) Ctx() context.Context {
	return utils.ChanContext(p.chStop)
}

func (p *Process) ProcessTree() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var goroutines []string
	for goroutine := range p.goroutines {
		goroutines = append(goroutines, goroutine.name)
	}

	children := make(map[string]interface{}, len(p.children))
	for child := range p.children {
		if child, is := child.(ProcessTreer); is {
			children[child.Name()] = child.ProcessTree()
		} else {
			children[child.Name()] = nil
		}
	}
	return map[string]interface{}{
		"status":     p.state.String(),
		"goroutines": goroutines,
		"children":   children,
	}
}

func (p *Process) NewChild(ctx context.Context, name string) *Process {
	child := New(name)
	_ = p.SpawnChild(ctx, child) // @@TODO: probably should return this error
	return child
}

var (
	ErrUnstarted = errors.New("unstarted")
	ErrClosed    = errors.New("closed")
)

func (p *Process) SpawnChild(ctx context.Context, child Spawnable) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state == Unstarted {
		panic(p.name + " can't spawn children; not started")

	} else if p.state == Closed {
		return nil
	}

	err := child.Start()
	if err != nil {
		return err
	}
	p.children[child] = struct{}{}

	p.wg.Add(1)
	go func() {
		defer func() {
			p.wg.Done()
			p.mu.Lock()
			delete(p.children, child)
			p.mu.Unlock()
		}()

		var ctxDone <-chan struct{}
		if ctx != nil {
			ctxDone = ctx.Done()
		}

		select {
		case <-p.chStop:
			child.Close()
		case <-ctxDone:
			child.Close()
		case <-child.Done():
		}
	}()

	return nil
}

type goroutine struct {
	name   string
	chDone chan struct{}
}

func (p *Process) Go(ctx context.Context, name string, fn func(ctx context.Context)) <-chan struct{} {
	p.mu.Lock()
	defer p.mu.Unlock()

	chDone := make(chan struct{})

	if p.state == Unstarted {
		panic(p.name + " can't spawn goroutines; not started")

	} else if p.state == Closed {
		ch := make(chan struct{})
		close(ch)
		return ch
	}

	g := &goroutine{name: name + " #" + utils.RandomNumberString(), chDone: chDone}
	p.goroutines.Add(g)

	p.wg.Add(1)
	go func() {
		defer func() {
			p.wg.Done()
			p.mu.Lock()
			defer p.mu.Unlock()
			p.goroutines.Remove(g)
			close(g.chDone)
		}()

		ctx, cancel := utils.CombinedContext(ctx, p.chStop)
		defer cancel()

		fn(ctx)
	}()

	return g.chDone
}

func (p *Process) Done() <-chan struct{} {
	return p.chDone
}

type State int

const (
	Unstarted State = iota
	Started
	Closed
)

func (s State) String() string {
	switch s {
	case Unstarted:
		return "unstarted"
	case Started:
		return "started"
	case Closed:
		return "closed"
	default:
		return "(err: unknown)"
	}
}
