package client

import (
	"sync"
	"time"
)

// BackoffOptions configures a Backoff. The zero value yields sensible defaults
// (100ms initial, 10s ceiling, factor 2, no jitter).
type BackoffOptions struct {
	// Min is the initial delay (default 100ms).
	Min time.Duration
	// Max is the ceiling the delay is clamped to (default 10s).
	Max time.Duration
	// Factor is the exponential growth base per attempt (default 2).
	Factor float64
	// Jitter is the randomization fraction in [0,1]; 0 disables jitter. A value
	// of 0.5 spreads each delay by up to ±50%. Jitter is only applied when Rand
	// is non-nil, keeping a default Backoff fully deterministic.
	Jitter float64
	// Rand supplies the randomness used for jitter; it must return a value in
	// [0,1). Inject a deterministic function in tests, or math/rand's Float64 in
	// production. When nil, no jitter is applied.
	Rand func() float64
}

// Backoff computes an exponentially increasing delay with optional jitter,
// mirroring the JavaScript client's backo2 module used for reconnection. Each
// call to Duration returns the delay for the current attempt and advances the
// attempt counter; Reset returns to the initial delay. A Backoff is safe for
// concurrent use.
type Backoff struct {
	mu       sync.Mutex
	min      time.Duration
	max      time.Duration
	factor   float64
	jitter   float64
	rand     func() float64
	attempts int
}

// NewBackoff creates a Backoff from the supplied options, filling in defaults
// for any zero field.
func NewBackoff(opts BackoffOptions) *Backoff {
	b := &Backoff{
		min:    opts.Min,
		max:    opts.Max,
		factor: opts.Factor,
		jitter: opts.Jitter,
		rand:   opts.Rand,
	}
	if b.min <= 0 {
		b.min = 100 * time.Millisecond
	}
	if b.max <= 0 {
		b.max = 10 * time.Second
	}
	if b.factor <= 0 {
		b.factor = 2
	}
	if b.jitter < 0 {
		b.jitter = 0
	}
	if b.jitter > 1 {
		b.jitter = 1
	}
	return b
}

// Duration returns the delay for the current attempt and advances to the next.
// The base delay is min * factor^attempt, clamped to max. When jitter and a Rand
// source are configured, the delay is randomized by up to ±(jitter fraction)
// before clamping.
func (b *Backoff) Duration() time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()

	ms := float64(b.min)
	for i := 0; i < b.attempts; i++ {
		ms *= b.factor
		if ms >= float64(b.max) {
			ms = float64(b.max)
			break
		}
	}
	b.attempts++

	if b.jitter > 0 && b.rand != nil {
		r := b.rand()
		deviation := r * b.jitter * ms
		// Alternate sign deterministically off the same draw, matching backo2.
		if int(r*10)&1 == 0 {
			ms -= deviation
		} else {
			ms += deviation
		}
	}

	d := time.Duration(ms)
	if d > b.max {
		d = b.max
	}
	if d < 0 {
		d = 0
	}
	return d
}

// Reset returns the backoff to its initial state, so the next Duration call
// yields the minimum delay again.
func (b *Backoff) Reset() {
	b.mu.Lock()
	b.attempts = 0
	b.mu.Unlock()
}

// Attempts returns how many times Duration has been called since the last Reset.
func (b *Backoff) Attempts() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.attempts
}
