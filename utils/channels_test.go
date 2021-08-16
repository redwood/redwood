package utils_test

import (
	"context"
	"testing"
	"time"

	"redwood.dev/utils"
)

func TestWaitGroupChan(t *testing.T) {
	t.Parallel()

	t.Run("with no context cancellation, releases only after sufficiently many calls to Done()", func(t *testing.T) {
		t.Parallel()

		wg := utils.NewWaitGroupChan(context.Background())
		defer wg.Close()

		wg.Add(5)

		select {
		case <-wg.Wait():
			t.Fatal("ended too soon")
		case <-time.After(1 * time.Second):
		}

		wg.Done()

		select {
		case <-wg.Wait():
			t.Fatal("ended too soon")
		case <-time.After(1 * time.Second):
		}

		wg.Done()

		select {
		case <-wg.Wait():
			t.Fatal("ended too soon")
		case <-time.After(1 * time.Second):
		}

		wg.Done()

		select {
		case <-wg.Wait():
			t.Fatal("ended too soon")
		case <-time.After(1 * time.Second):
		}

		wg.Done()

		select {
		case <-wg.Wait():
			t.Fatal("ended too soon")
		case <-time.After(1 * time.Second):
		}

		wg.Done()

		select {
		case <-wg.Wait():
		case <-time.After(1 * time.Second):
			t.Fatal("did not end")
		}
	})

	t.Run("releases after the context expires, even if Done() has not been called enough", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		wg := utils.NewWaitGroupChan(ctx)
		defer wg.Close()

		wg.Add(5)

		select {
		case <-wg.Wait():
			t.Fatal("ended too soon")
		case <-time.After(1 * time.Second):
		}

		wg.Done()

		select {
		case <-wg.Wait():
			t.Fatal("ended too soon")
		case <-time.After(1 * time.Second):
		}

		cancel()

		select {
		case <-wg.Wait():
		case <-time.After(1 * time.Second):
			t.Fatal("did not end")
		}
	})
}

func TestCombinedContext(t *testing.T) {
	t.Parallel()

	t.Run("cancels when an inner context is canceled", func(t *testing.T) {
		t.Parallel()

		innerCtx, innerCancel := context.WithCancel(context.Background())
		defer innerCancel()

		chStop := make(chan struct{})

		ctx, cancel := utils.CombinedContext(innerCtx, chStop, 1*time.Hour)
		defer cancel()

		innerCancel()

		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Second):
			t.Fatal("context didn't cancel")
		}
	})

	t.Run("cancels when a channel is closed", func(t *testing.T) {
		t.Parallel()

		innerCtx, innerCancel := context.WithCancel(context.Background())
		defer innerCancel()

		chStop := make(chan struct{})

		ctx, cancel := utils.CombinedContext(innerCtx, chStop, 1*time.Hour)
		defer cancel()

		close(chStop)

		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Second):
			t.Fatal("context didn't cancel")
		}
	})

	t.Run("cancels when a duration elapses", func(t *testing.T) {
		t.Parallel()

		innerCtx, innerCancel := context.WithCancel(context.Background())
		defer innerCancel()

		chStop := make(chan struct{})

		ctx, cancel := utils.CombinedContext(innerCtx, chStop, 1*time.Second)
		defer cancel()

		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Second):
			t.Fatal("context didn't cancel")
		}
	})

	t.Run("doesn't cancel if none of its children cancel", func(t *testing.T) {
		t.Parallel()

		innerCtx, innerCancel := context.WithCancel(context.Background())
		defer innerCancel()

		chStop := make(chan struct{})

		ctx, cancel := utils.CombinedContext(innerCtx, chStop, 1*time.Hour)
		defer cancel()

		select {
		case <-ctx.Done():
			t.Fatal("context canceled")
		case <-time.After(5 * time.Second):
		}
	})
}

func TestChanContext(t *testing.T) {
	ctx := utils.ChanContext(make(chan struct{}))

	go func() {
		time.Sleep(1 * time.Second)
		close(ctx)
	}()

	select {
	case <-time.After(5 * time.Second):
		t.Fatal("fail")
	case <-ctx.Done():
	}

	ctx = utils.ChanContext(make(chan struct{}))
	ctx2, _ := context.WithTimeout(ctx, 1*time.Second)

	select {
	case <-time.After(5 * time.Second):
		t.Fatal("fail")
	case <-ctx2.Done():
	}

	ctx = utils.ChanContext(make(chan struct{}))
	ctx2, cancel := context.WithTimeout(ctx, 3*time.Second)

	go func() {
		time.Sleep(1 * time.Second)
		cancel()
	}()

	select {
	case <-time.After(5 * time.Second):
		t.Fatal("fail")
	case <-ctx2.Done():
	}
}
