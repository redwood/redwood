package redwood

import (
	"context"
	"sync"
	"time"
)

type jobQueue struct {
	ctx                context.Context
	cancel             func()
	chJobs             chan job
	chBatches          chan []job
	chUncapBatchSize   chan struct{}
	chFinishedJobs     chan int
	unfinishedJobs     int
	batchSize          uint
	batchTimeout       time.Duration
	batchSizeUncapped  bool
	uncapBatchSizeOnce *sync.Once
}

type job struct {
	objectID []byte
	size     int64
	// failedPeers map[peer.ID]bool
}

func newJobQueue(ctx context.Context, jobs []job, batchSize uint, batchTimeout time.Duration) *jobQueue {
	ctxInner, cancel := context.WithCancel(ctx)

	q := &jobQueue{
		ctx:                ctxInner,
		cancel:             cancel,
		chJobs:             make(chan job, len(jobs)),
		chBatches:          make(chan []job),
		chUncapBatchSize:   make(chan struct{}, 1),
		chFinishedJobs:     make(chan int),
		unfinishedJobs:     len(jobs),
		batchSize:          batchSize,
		batchTimeout:       batchTimeout,
		batchSizeUncapped:  false,
		uncapBatchSizeOnce: &sync.Once{},
	}

	// Load up the job queue
	for _, job := range jobs {
		q.chJobs <- job
	}

	go q.aggregateWork()

	return q
}

func (q *jobQueue) Close() {
	q.cancel()
}

func (q *jobQueue) UncapBatchSize() {
	// if this isn't limited to a single execution, it can cause deadlocks
	q.uncapBatchSizeOnce.Do(func() {
		q.chUncapBatchSize <- struct{}{}
	})
}

func (q *jobQueue) GetBatch() []job {
	batch, open := <-q.chBatches
	if !open {
		return nil
	}
	return batch
}

func (q *jobQueue) ReturnFailed(jobs []job) {
	for _, j := range jobs {
		select {
		case q.chJobs <- j:
		case <-q.ctx.Done():
			return
		}
	}
}

func (q *jobQueue) MarkDone(num int) {
	q.chFinishedJobs <- num
}

// Batches received jobs up to `batchSize`.  Batches are also time-constrained.
// If `batchSize` jobs aren't received within `batchTimeout`, the batch is sent anyway.
func (q *jobQueue) aggregateWork() {
	defer close(q.chBatches)

	batchSizeUncapped := false

Outer:
	for {
		// We don't wait more than this amount of time
		timeout := time.After(q.batchTimeout)
		current := make([]job, 0)

	ReceiveJobs:
		for {
			select {
			case j := <-q.chJobs:
				current = append(current, j)

				if !batchSizeUncapped && uint(len(current)) >= q.batchSize {
					// We have to try to receive from chUncapBatchSize here, rather than in the
					// outer select, because the signal on chUncapBatchSize can't occur until after
					// the 2nd batch is assembled (the 2nd batch is assembled instantly, as soon as
					// the first one is sent off).  Therefore, in order to intercept it and unbound
					// its size (in the 1 peer case, which is precisely what batchSizeUncapped was
					// built to improve), we have to check right before sending.
					select {
					case <-q.chUncapBatchSize:
						q.batchSizeUncapped = true
						continue ReceiveJobs

					case q.chBatches <- current:
						continue Outer

					case <-q.ctx.Done():
						return
					}
				}

			case <-timeout:
				if len(current) > 0 {
					select {
					case q.chBatches <- current:
					case <-q.ctx.Done():
						return
					}
				}
				continue Outer

			case i := <-q.chFinishedJobs:
				q.unfinishedJobs -= i
				if q.unfinishedJobs <= 0 {
					return
				}

			case <-q.ctx.Done():
				return
			}
		}
	}
}
