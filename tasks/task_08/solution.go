package main

import (
	"sync"
	"time"
)

type Clock interface{ Now() time.Time }
type Limiter struct {
	mu     sync.Mutex
	clock  Clock
	rate   float64
	burst  int
	tokens float64
	last   time.Time
}

func NewLimiter(clock Clock, ratePerSec float64, burst int) *Limiter {
	l := &Limiter{
		clock: clock,
		rate: ratePerSec,
		burst: burst,
	}

	if burst <= 0 || clock == nil {
		return l
	}

	l.last = clock.Now()
	l.tokens = float64(burst)
	return l
}
func (l *Limiter) AllowN(n int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if n > l.burst || n <= 0 || l.clock == nil {
		return false
	}
	t := l.clock.Now()

	if e := t.Sub(l.last); e > 0 {
		if l.rate > 0 {
			l.tokens += l.rate * e.Seconds()
			l.tokens = min(l.tokens, float64(l.burst))
		}
		l.last = t
	}

	if float64(n) <= l.tokens {
		l.tokens -= float64(n)
		return true
	}
	return false

}
