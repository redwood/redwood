package process_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"redwood.dev/internal/testutils"
	"redwood.dev/process"
)

func TestProcess(t *testing.T) {
	t.Run("it does a thing", func(t *testing.T) {
		p := process.New("")
		p.Start()

		p.Go(nil, "foo", func(ctx context.Context) { time.Sleep(1 * time.Second) })
		p.Go(nil, "bar", func(ctx context.Context) { time.Sleep(1 * time.Second) })
		p.Go(nil, "baz", func(ctx context.Context) { time.Sleep(1 * time.Second) })

		p.Autoclose()

		select {
		case <-time.After(5 * time.Second):
			t.Fatal("fail")
		case <-p.Done():
		}
	})

	t.Run("it does a thing", func(t *testing.T) {
		p := process.New("")

		p.Start()

		p.Go(nil, "foo", func(ctx context.Context) { time.Sleep(1 * time.Second) })
		p.Go(nil, "bar", func(ctx context.Context) { time.Sleep(1 * time.Second) })
		p.Go(nil, "baz", func(ctx context.Context) { time.Sleep(1 * time.Second) })

		select {
		case <-time.After(5 * time.Second):
		case <-p.Done():
			t.Fatal("fail")
		}

		p.Autoclose()

		select {
		case <-time.After(5 * time.Second):
			t.Fatal("fail")
		case <-p.Done():
		}
	})

	t.Run("child", func(t *testing.T) {
		p := process.New("")
		p.Start()
		defer p.Close()

		child := p.NewChild(context.Background(), "child")

		child.Go(nil, "foo", func(ctx context.Context) { time.Sleep(1 * time.Second) })
		child.Go(nil, "bar", func(ctx context.Context) { time.Sleep(1 * time.Second) })
		child.Go(nil, "baz", func(ctx context.Context) { time.Sleep(1 * time.Second) })

		child.Autoclose()

		select {
		case <-time.After(5 * time.Second):
			t.Fatal("fail")
		case <-child.Done():
		}
	})

	t.Run("child", func(t *testing.T) {
		p := process.New("")
		p.Start()
		defer p.Close()

		child := p.NewChild(context.Background(), "child")

		child.Go(nil, "foo", func(ctx context.Context) { time.Sleep(1 * time.Second) })
		child.Go(nil, "bar", func(ctx context.Context) { time.Sleep(1 * time.Second) })
		child.Go(nil, "baz", func(ctx context.Context) { time.Sleep(1 * time.Second) })

		select {
		case <-time.After(5 * time.Second):
		case <-child.Done():
			t.Fatal("fail")
		}

		child.Autoclose()

		select {
		case <-time.After(5 * time.Second):
			t.Fatal("fail")
		case <-child.Done():
		}
	})

	t.Run(".Close cancels child contexts", func(t *testing.T) {
		p := process.New("")
		p.Start()

		child := p.NewChild(context.Background(), "child")

		canceled1 := testutils.NewAwaiter()
		canceled2 := testutils.NewAwaiter()

		chDone1 := p.Go(nil, "foo", func(ctx context.Context) {
			select {
			case <-ctx.Done():
				canceled1.ItHappened()
			case <-time.After(5 * time.Second):
				t.Fatal("context wasn't canceled")
			}
		})

		chDone2 := child.Go(nil, "foo", func(ctx context.Context) {
			select {
			case <-ctx.Done():
				canceled2.ItHappened()
			case <-time.After(5 * time.Second):
				t.Fatal("context wasn't canceled")
			}
		})

		requireDone(t, p.Done(), false)
		requireDone(t, child.Done(), false)
		requireDone(t, chDone1, false)
		requireDone(t, chDone2, false)

		go p.Close()

		canceled1.AwaitOrFail(t)
		canceled2.AwaitOrFail(t)

		require.Eventually(t, func() bool { return isDone(t, p.Done()) }, 5*time.Second, 100*time.Millisecond)
		require.Eventually(t, func() bool { return isDone(t, child.Done()) }, 5*time.Second, 100*time.Millisecond)
		require.Eventually(t, func() bool { return isDone(t, chDone1) }, 5*time.Second, 100*time.Millisecond)
		require.Eventually(t, func() bool { return isDone(t, chDone2) }, 5*time.Second, 100*time.Millisecond)
	})
}

func requireDone(t *testing.T, chDone <-chan struct{}, done bool) {
	t.Helper()
	require.Equal(t, done, isDone(t, chDone))
}

func isDone(t *testing.T, chDone <-chan struct{}) bool {
	t.Helper()
	select {
	case <-chDone:
		return true
	default:
		return false
	}
}
