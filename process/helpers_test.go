package process_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"redwood.dev/internal/testutils"
	"redwood.dev/process"
)

func makeItems() (item [5]*workItem) {
	item[0] = &workItem{id: "a", processed: testutils.NewAwaiter()}
	item[1] = &workItem{id: "b1", processed: testutils.NewAwaiter()}
	item[2] = &workItem{id: "b2", processed: testutils.NewAwaiter()}
	item[3] = &workItem{id: "c", processed: testutils.NewAwaiter()}
	item[4] = &workItem{id: "d", processed: testutils.NewAwaiter()}
	return
}

type workItem struct {
	id        string
	retry     bool
	processed testutils.Awaiter
	block     chan struct{}
}

func (i workItem) ID() process.PoolUniqueID { return i.id[:1] }

func (i *workItem) Work(ctx context.Context) (retry bool) {
	i.processed.ItHappened()
	if i.block != nil {
		select {
		case <-i.block:
			i.block = nil
		case <-ctx.Done():
			return false
		}
	}
	retry = i.retry
	i.retry = false
	return retry
}

func requireHappened(t *testing.T, item *workItem, times int, wg *sync.WaitGroup) {
	t.Helper()
	defer wg.Done()
	for i := 0; i < times; i++ {
		item.processed.AwaitOrFail(t, 5*time.Second, item.id)
	}
	item.processed.NeverHappenedOrFail(t, 5*time.Second, item.id)
}

func requireNeverHappened(t *testing.T, item *workItem, wg *sync.WaitGroup) {
	t.Helper()
	defer wg.Done()
	item.processed.NeverHappenedOrFail(t, 5*time.Second, item.id)
}
