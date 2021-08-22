package process

import (
	"context"
	"time"

	"redwood.dev/utils"
)

type PeriodicTask struct {
	Process
	interval time.Duration
	mailbox  *utils.Mailbox
	taskFn   func(ctx context.Context)
}

func NewPeriodicTask(name string, interval time.Duration, taskFn func(ctx context.Context)) *PeriodicTask {
	return &PeriodicTask{
		Process:  *New(name),
		interval: interval,
		mailbox:  utils.NewMailbox(1),
		taskFn:   taskFn,
	}
}

func (task *PeriodicTask) Start() error {
	err := task.Process.Start()
	if err != nil {
		return err
	}

	task.Process.Go("ticker", func(ctx context.Context) {
		ticker := time.NewTicker(task.interval)
		defer ticker.Stop()

	Loop:
		for {
			select {
			case <-ctx.Done():
				return

			case <-ticker.C:
				task.Enqueue()

			case <-task.mailbox.Notify():
				xs := task.mailbox.RetrieveAll()
				if len(xs) == 0 {
					continue Loop
				}
				// func() {
				//  ctx, cancel := CombinedContext(task.chStop, interval)
				//  defer cancel()
				task.taskFn(ctx)
				// }()
			}
		}
	})
	return nil
}

func (task *PeriodicTask) Enqueue() {
	task.mailbox.Deliver(struct{}{})
}
