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

func (eb *ExponentialBackoff) Reset() {
	eb.current = eb.Min
}

type ExponentialBackoffTicker struct {
	backoff ExponentialBackoff
	chTick  chan time.Time
	chStop  chan struct{}
	chDone  chan struct{}
}

func NewExponentialBackoffTicker(min, max time.Duration) *ExponentialBackoffTicker {
	return &ExponentialBackoffTicker{
		backoff: ExponentialBackoff{Min: min, Max: max},
		chTick:  make(chan time.Time),
		chStop:  make(chan struct{}),
		chDone:  make(chan struct{}),
	}
}

func (t *ExponentialBackoffTicker) Start() {
	go func() {
		defer close(t.chDone)
		for {
			duration := t.backoff.Next()
			select {
			case <-t.chStop:
				return

			case tick := <-time.After(duration):
				select {
				case <-t.chStop:
					return
				case t.chTick <- tick:
				case <-time.After(duration):
				}
			}
		}
	}()
}

func (t *ExponentialBackoffTicker) Stop() {
	close(t.chStop)
	<-t.chDone
}

func (t *ExponentialBackoffTicker) Tick() <-chan time.Time {
	return t.chTick
}
