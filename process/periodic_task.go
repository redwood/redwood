package process

import (
	"context"
	"sync"

	"redwood.dev/utils"
)

type PeriodicTask struct {
	Process
	ticker  utils.Ticker
	mailbox *utils.Mailbox[struct{}]
	abort   context.CancelFunc
	abortMu sync.Mutex
	taskFn  func(ctx context.Context)
	name    string
}

func NewPeriodicTask(name string, ticker utils.Ticker, taskFn func(ctx context.Context)) *PeriodicTask {
	return &PeriodicTask{
		Process: *New(name),
		ticker:  ticker,
		mailbox: utils.NewMailbox[struct{}](1),
		taskFn:  taskFn,
		name:    name,
	}
}

func (task *PeriodicTask) Start() error {
	err := task.Process.Start()
	if err != nil {
		return err
	}
	defer task.Process.Autoclose()

	task.ticker.Start()

	task.Process.Go(nil, "ticker", func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			case <-task.ticker.Notify():
				task.Enqueue()
			case <-task.mailbox.Notify():
				task.handle(ctx, task.mailbox.RetrieveAll())
			}
		}
	})
	return nil
}

func (task *PeriodicTask) handle(ctx context.Context, items []struct{}) {
	for range items {
		select {
		case <-ctx.Done():
			return
		default:
		}
		task.abortMu.Lock()
		innerCtx, innerCancel := context.WithCancel(ctx)
		task.abort = innerCancel
		task.abortMu.Unlock()

		task.taskFn(innerCtx)
	}
}

func (task *PeriodicTask) Close() error {
	task.ticker.Close()
	return nil
}

func (task *PeriodicTask) Enqueue() {
	task.mailbox.Deliver(struct{}{})
}

func (task *PeriodicTask) AbortIfRunning() {
	task.abortMu.Lock()
	if task.abort != nil {
		task.abort()
	}
	task.abortMu.Unlock()
}

func (task *PeriodicTask) ForceRerun() {
	task.AbortIfRunning()
	task.Enqueue()
}
