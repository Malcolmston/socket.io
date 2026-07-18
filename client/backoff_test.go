package client

import (
	"testing"
	"time"
)

func TestBackoffDefaultsExponential(t *testing.T) {
	b := NewBackoff(BackoffOptions{}) // defaults: 100ms, x2, max 10s
	want := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		400 * time.Millisecond,
		800 * time.Millisecond,
		1600 * time.Millisecond,
	}
	for i, w := range want {
		if got := b.Duration(); got != w {
			t.Errorf("attempt %d: Duration = %v, want %v", i, got, w)
		}
	}
	if b.Attempts() != len(want) {
		t.Errorf("Attempts = %d, want %d", b.Attempts(), len(want))
	}
}

func TestBackoffClampsToMax(t *testing.T) {
	b := NewBackoff(BackoffOptions{Min: time.Second, Max: 3 * time.Second, Factor: 2})
	got := []time.Duration{b.Duration(), b.Duration(), b.Duration(), b.Duration()}
	want := []time.Duration{time.Second, 2 * time.Second, 3 * time.Second, 3 * time.Second}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("attempt %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestBackoffReset(t *testing.T) {
	b := NewBackoff(BackoffOptions{Min: 50 * time.Millisecond, Factor: 2})
	b.Duration()
	b.Duration()
	if b.Attempts() != 2 {
		t.Fatalf("Attempts = %d", b.Attempts())
	}
	b.Reset()
	if b.Attempts() != 0 {
		t.Fatalf("after Reset Attempts = %d", b.Attempts())
	}
	if got := b.Duration(); got != 50*time.Millisecond {
		t.Fatalf("after Reset Duration = %v, want 50ms", got)
	}
}

func TestBackoffJitterDeterministic(t *testing.T) {
	// A fixed Rand makes jitter reproducible. r=0.5, jitter=0.5:
	// base=100ms, deviation = 0.5*0.5*100ms = 25ms.
	// int(0.5*10)=5, 5&1==1 -> add: 125ms.
	b := NewBackoff(BackoffOptions{
		Min:    100 * time.Millisecond,
		Factor: 2,
		Jitter: 0.5,
		Rand:   func() float64 { return 0.5 },
	})
	if got := b.Duration(); got != 125*time.Millisecond {
		t.Fatalf("jittered Duration = %v, want 125ms", got)
	}
}

func TestBackoffNoJitterWithoutRand(t *testing.T) {
	// Jitter set but no Rand -> deterministic, no jitter applied.
	b := NewBackoff(BackoffOptions{Min: 100 * time.Millisecond, Jitter: 0.9})
	if got := b.Duration(); got != 100*time.Millisecond {
		t.Fatalf("Duration = %v, want 100ms (jitter suppressed without Rand)", got)
	}
}

func BenchmarkBackoffDuration(b *testing.B) {
	bo := NewBackoff(BackoffOptions{})
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		bo.Duration()
		if i%10 == 0 {
			bo.Reset()
		}
	}
}
