package retrykit

import "time"

const (
	defaultExponentialBase = 100 * time.Millisecond
	defaultExponentialMax  = 5 * time.Second
)

// Backoff calculates how long to wait before a retry attempt.
//
// The attempt argument is 1-based and represents the retry delay after a
// failed operation attempt. For example, Delay(1) is used between operation
// attempts 1 and 2.
type Backoff interface {
	Delay(attempt int) time.Duration
}

type fixedBackoff struct {
	delay time.Duration
}

// FixedBackoff returns a Backoff that always uses the same delay.
//
// Negative delays are treated as zero.
func FixedBackoff(delay time.Duration) Backoff {
	if delay < 0 {
		delay = 0
	}

	return fixedBackoff{delay: delay}
}

func (b fixedBackoff) Delay(int) time.Duration {
	return b.delay
}

type exponentialBackoff struct {
	base time.Duration
	max  time.Duration
}

// ExponentialBackoff returns a Backoff that doubles the delay after each
// failed attempt until it reaches max.
//
// Delay(1) returns base, Delay(2) returns base*2, Delay(3) returns base*4,
// and so on. If base is less than or equal to zero, a default of 100ms is
// used. If max is less than or equal to zero, a default of 5s is used.
func ExponentialBackoff(base, max time.Duration) Backoff {
	if base <= 0 {
		base = defaultExponentialBase
	}
	if max <= 0 {
		max = defaultExponentialMax
	}

	return exponentialBackoff{base: base, max: max}
}

func (b exponentialBackoff) Delay(attempt int) time.Duration {
	if attempt <= 1 {
		if b.base > b.max {
			return b.max
		}
		return b.base
	}

	delay := b.base
	if delay >= b.max {
		return b.max
	}

	for i := 1; i < attempt; i++ {
		if delay > b.max/2 {
			return b.max
		}
		delay *= 2
		if delay >= b.max {
			return b.max
		}
	}

	return delay
}

type noBackoff struct{}

// NoBackoff returns a Backoff that never waits between attempts.
func NoBackoff() Backoff {
	return noBackoff{}
}

func (noBackoff) Delay(int) time.Duration {
	return 0
}
