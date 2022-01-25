package process

import (
	"context"
	"sync"

	"redwood.dev/utils"
)

type PeriodicTask struct {
	Process
	ticker  utils.Ticker
	mailbox *utils.Mailbox
	abort   context.CancelFunc
	abortMu sync.Mutex
	taskFn  func(ctx context.Context)
}

func NewPeriodicTask(name string, ticker utils.Ticker, taskFn func(ctx context.Context)) *PeriodicTask {
	return &PeriodicTask{
		Process: *New(name),
		ticker:  ticker,
		mailbox: utils.NewMailbox(1),
		taskFn:  taskFn,
	}
}

func (task *PeriodicTask) Start() error {
	err := task.Process.Start()
	if err != nil {
		return err
	}
	task.ticker.Start()

	task.Process.Go(nil, "ticker", func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			func() {
				select {
				case <-ctx.Done():
					return

				case <-task.ticker.Notify():
					task.Enqueue()

				case <-task.mailbox.Notify():
					x := task.mailbox.Retrieve()
					if x == nil {
						return
					}

					task.abortMu.Lock()
					innerCtx, innerCancel := context.WithCancel(ctx)
					task.abort = innerCancel
					task.abortMu.Unlock()

					task.taskFn(innerCtx)
				}
			}()
		}
	})
	return nil
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
	defer task.abortMu.Unlock()
	if task.abort != nil {
		task.abort()
	}
}

func (task *PeriodicTask) ForceRerun() {
	task.AbortIfRunning()
	task.Enqueue()
}
