package utils

import (
	"time"
)

type ExponentialBackoff struct {
	Min          time.Duration
	Max          time.Duration
	current      time.Duration
	previousIncr time.Time
}

func (eb *ExponentialBackoff) Ready() (ready bool, until time.Duration) {
	if eb.previousIncr.IsZero() {
		return true, 0
	}
	whenReady := eb.previousIncr.Add(eb.current)
	ready = time.Now().After(whenReady)
	if !ready {
		until = whenReady.Sub(time.Now())
	}
	return
}

func (eb *ExponentialBackoff) Next() time.Duration {
	if eb.current == 0 {
		eb.current = eb.Min
	}
	current := eb.current
	eb.current *= 2
	if eb.current > eb.Max {
		eb.current = eb.Max
	}
	eb.previousIncr = time.Now()
	return current
}

func (eb *ExponentialBackoff) Wait() {
	time.Sleep(eb.Next())
}
