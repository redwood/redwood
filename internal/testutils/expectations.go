package testutils

import (
	"testing"
	"time"
)

type Awaiter chan struct{}

func NewAwaiter() Awaiter { return make(Awaiter, 10) }

func (a Awaiter) ItHappened() { a <- struct{}{} }

func (a Awaiter) AwaitOrFail(t testing.TB, params ...interface{}) {
	t.Helper()

	duration := 10 * time.Second
	msg := ""
	for _, p := range params {
		switch p := p.(type) {
		case time.Duration:
			duration = p
		case string:
			msg = p
		}
	}

	select {
	case <-a:
	case <-time.After(duration):
		t.Fatalf("Timed out waiting for Awaiter to get ItHappened: %v", msg)
	}
}

func (a Awaiter) NeverHappenedOrFail(t testing.TB, params ...interface{}) {
	t.Helper()

	duration := 10 * time.Second
	msg := ""
	for _, p := range params {
		switch p := p.(type) {
		case time.Duration:
			duration = p
		case string:
			msg = p
		}
	}

	select {
	case <-a:
		t.Fatalf("should not happen: %v", msg)
	case <-time.After(duration):
	}
}
