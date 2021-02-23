package utils_test

import (
	"context"
	"testing"
	"time"

	"redwood.dev/utils"
)

func TestWaitGroupChan(t *testing.T) {
	t.Run("releases only after sufficiently many calls to Done()", func(t *testing.T) {
		wg := utils.NewWaitGroupChan()
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
}

func TestCombinedContext(t *testing.T) {
	t.Run("cancels when an inner context is canceled", func(t *testing.T) {
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
