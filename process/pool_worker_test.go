package process_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"redwood.dev/process"
)

func TestPoolWorker(t *testing.T) {
	t.Run("disallows simultaneous processing of items with the same UniqueID", func(t *testing.T) {
		t.Parallel()

		var wg sync.WaitGroup
		item := makeItems()
		item[1].block = make(chan struct{})
		item[1].retry = true

		w := process.NewPoolWorker("", 2, process.NewStaticScheduler(100*time.Millisecond, 2*time.Second))
		err := w.Start()
		require.NoError(t, err)
		defer w.Close()

		w.Add(item[1])
		w.Add(item[2])

		var which *workItem
		wg.Add(1)
		go func() {
			select {
			case <-item[1].processed:
				which = item[1]
			case <-item[2].processed:
				which = item[2]
			}

			select {
			case <-item[1].processed:
				t.Fatalf("nope")
			case <-item[2].processed:
				t.Fatalf("nope")
			case <-time.After(1 * time.Second):
			}
			wg.Done()
		}()
		wg.Wait()

		close(which.block)
		wg.Add(1)
		go func() {
			select {
			case <-item[1].processed:
			case <-item[2].processed:
			}

			select {
			case <-item[1].processed:
				t.Fatalf("nope")
			case <-item[2].processed:
				t.Fatalf("nope")
			case <-time.After(1 * time.Second):
			}
			wg.Done()
		}()
		wg.Wait()
	})
}
