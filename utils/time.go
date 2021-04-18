package utils

import (
	"time"
)

type ExponentialBackoff struct {
	Min     time.Duration
	Max     time.Duration
	current time.Duration
}

func (eb *ExponentialBackoff) Wait() {
	if eb.current == 0 {
		eb.current = eb.Min
	}
	time.Sleep(eb.current)
	eb.current *= 2
	if eb.current > eb.Max {
		eb.current = eb.Max
	}
}
