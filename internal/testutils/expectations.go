package testutils

import (
	"testing"
	"time"
)

type Awaiter chan struct{}

func NewAwaiter() Awaiter { return make(Awaiter, 10) }

func (a Awaiter) ItHappened() { a <- struct{}{} }

func (a Awaiter) AwaitOrFail(t testing.TB, durationParams ...time.Duration) {
	t.Helper()

	duration := 10 * time.Second
	if len(durationParams) > 0 {
		duration = durationParams[0]
	}

	select {
	case <-a:
	case <-time.After(duration):
		t.Fatal("Timed out waiting for Awaiter to get ItHappened")
	}
}

func (a Awaiter) NeverHappenedOrFail(t testing.TB, durationParams ...time.Duration) {
	t.Helper()

	duration := 10 * time.Second
	if len(durationParams) > 0 {
		duration = durationParams[0]
	}

	select {
	case <-a:
		t.Fatal("should not happen")
	case <-time.After(duration):
	}
}
