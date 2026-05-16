package retrykit

import (
	"testing"
	"time"
)

func TestFixedBackoff(t *testing.T) {
	backoff := FixedBackoff(150 * time.Millisecond)
	for attempt := 1; attempt <= 3; attempt++ {
		if got := backoff.Delay(attempt); got != 150*time.Millisecond {
			t.Fatalf("Delay(%d) = %v, want 150ms", attempt, got)
		}
	}

	if got := FixedBackoff(-time.Second).Delay(1); got != 0 {
		t.Fatalf("negative fixed backoff Delay(1) = %v, want 0", got)
	}
}

func TestExponentialBackoff(t *testing.T) {
	backoff := ExponentialBackoff(100*time.Millisecond, 350*time.Millisecond)

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: 1, want: 100 * time.Millisecond},
		{attempt: 2, want: 200 * time.Millisecond},
		{attempt: 3, want: 350 * time.Millisecond},
		{attempt: 4, want: 350 * time.Millisecond},
	}

	for _, tt := range tests {
		if got := backoff.Delay(tt.attempt); got != tt.want {
			t.Fatalf("Delay(%d) = %v, want %v", tt.attempt, got, tt.want)
		}
	}
}

func TestExponentialBackoffDefaults(t *testing.T) {
	backoff := ExponentialBackoff(0, 0)
	if got := backoff.Delay(1); got != defaultExponentialBase {
		t.Fatalf("Delay(1) = %v, want default base %v", got, defaultExponentialBase)
	}

	capped := ExponentialBackoff(time.Hour, 0)
	if got := capped.Delay(1); got != defaultExponentialMax {
		t.Fatalf("Delay(1) = %v, want default max %v", got, defaultExponentialMax)
	}
}

func TestExponentialBackoffAvoidsOverflow(t *testing.T) {
	backoff := ExponentialBackoff(time.Duration(1<<62), time.Duration(1<<62))
	if got := backoff.Delay(100); got != time.Duration(1<<62) {
		t.Fatalf("Delay(100) = %v, want max", got)
	}
}

func TestNoBackoff(t *testing.T) {
	backoff := NoBackoff()
	for attempt := 1; attempt <= 3; attempt++ {
		if got := backoff.Delay(attempt); got != 0 {
			t.Fatalf("Delay(%d) = %v, want 0", attempt, got)
		}
	}
}
