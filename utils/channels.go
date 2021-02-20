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
