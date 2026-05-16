package retrykit

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDoSuccessOnFirstAttempt(t *testing.T) {
	calls := 0

	err := Do(context.Background(), func(context.Context) error {
		calls++
		return nil
	})

	if err != nil {
		t.Fatalf("Do() error = %v, want nil", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestDoSuccessAfterRetries(t *testing.T) {
	calls := 0
	temporaryErr := errors.New("temporary")

	err := Do(context.Background(), func(context.Context) error {
		calls++
		if calls < 3 {
			return temporaryErr
		}
		return nil
	}, WithMaxAttempts(3), WithBackoff(NoBackoff()))

	if err != nil {
		t.Fatalf("Do() error = %v, want nil", err)
	}
	if calls != 3 {
		t.Fatalf("calls = %d, want 3", calls)
	}
}

func TestDoFailureAfterMaxAttempts(t *testing.T) {
	lastErr := errors.New("connection refused")
	calls := 0

	err := Do(context.Background(), func(context.Context) error {
		calls++
		return lastErr
	}, WithMaxAttempts(3), WithBackoff(NoBackoff()))

	var retryErr *RetryError
	if !errors.As(err, &retryErr) {
		t.Fatalf("Do() error = %T %v, want *RetryError", err, err)
	}
	if retryErr.Attempts != 3 {
		t.Fatalf("RetryError.Attempts = %d, want 3", retryErr.Attempts)
	}
	if !errors.Is(err, lastErr) {
		t.Fatalf("errors.Is(err, lastErr) = false, want true")
	}
	if calls != 3 {
		t.Fatalf("calls = %d, want 3", calls)
	}
}

func TestDoNonRetryableErrorReturnsOriginal(t *testing.T) {
	nonRetryableErr := errors.New("bad request")
	calls := 0

	err := Do(context.Background(), func(context.Context) error {
		calls++
		return nonRetryableErr
	}, WithRetryIf(func(error) bool { return false }))

	if !errors.Is(err, nonRetryableErr) {
		t.Fatalf("Do() error = %v, want original non-retryable error", err)
	}
	var retryErr *RetryError
	if errors.As(err, &retryErr) {
		t.Fatalf("Do() error is RetryError, want original error")
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestDoContextAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Do(ctx, func(context.Context) error {
		t.Fatal("operation should not run")
		return nil
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Do() error = %v, want context.Canceled", err)
	}
}

func TestDoContextCancelledDuringSleep(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var calls int32
	onRetry := make(chan struct{})

	errCh := make(chan error, 1)
	go func() {
		errCh <- Do(ctx, func(context.Context) error {
			atomic.AddInt32(&calls, 1)
			return errors.New("temporary")
		},
			WithMaxAttempts(3),
			WithBackoff(FixedBackoff(time.Hour)),
			WithOnRetry(func(Attempt) { close(onRetry) }),
		)
	}()

	select {
	case <-onRetry:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for retry sleep to start")
	}
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Do() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Do to return")
	}

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("calls = %d, want 1", got)
	}
}

func TestDoMaxAttemptsIncludesInitialAttempt(t *testing.T) {
	calls := 0
	operationErr := errors.New("failed")

	err := Do(context.Background(), func(context.Context) error {
		calls++
		return operationErr
	}, WithMaxAttempts(1))

	var retryErr *RetryError
	if !errors.As(err, &retryErr) {
		t.Fatalf("Do() error = %T %v, want *RetryError", err, err)
	}
	if retryErr.Attempts != 1 {
		t.Fatalf("RetryError.Attempts = %d, want 1", retryErr.Attempts)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestDoInvalidMaxAttemptsFallsBackToDefault(t *testing.T) {
	tests := []struct {
		name string
		max  int
	}{
		{name: "zero", max: 0},
		{name: "negative", max: -2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			operationErr := errors.New("failed")

			err := Do(context.Background(), func(context.Context) error {
				calls++
				return operationErr
			}, WithMaxAttempts(tt.max), WithBackoff(NoBackoff()))

			var retryErr *RetryError
			if !errors.As(err, &retryErr) {
				t.Fatalf("Do() error = %T %v, want *RetryError", err, err)
			}
			if retryErr.Attempts != defaultMaxAttempts {
				t.Fatalf("RetryError.Attempts = %d, want %d", retryErr.Attempts, defaultMaxAttempts)
			}
			if calls != defaultMaxAttempts {
				t.Fatalf("calls = %d, want %d", calls, defaultMaxAttempts)
			}
		})
	}
}

func TestDoOnRetryCallback(t *testing.T) {
	operationErr := errors.New("failed")
	var attempts []Attempt

	err := Do(context.Background(), func(context.Context) error {
		return operationErr
	},
		WithMaxAttempts(3),
		WithBackoff(FixedBackoff(25*time.Millisecond)),
		WithOnRetry(func(attempt Attempt) {
			attempts = append(attempts, attempt)
		}),
	)

	var retryErr *RetryError
	if !errors.As(err, &retryErr) {
		t.Fatalf("Do() error = %T %v, want *RetryError", err, err)
	}
	if len(attempts) != 2 {
		t.Fatalf("OnRetry calls = %d, want 2", len(attempts))
	}
	for i, attempt := range attempts {
		wantNumber := i + 1
		if attempt.Number != wantNumber {
			t.Fatalf("attempts[%d].Number = %d, want %d", i, attempt.Number, wantNumber)
		}
		if attempt.MaxAttempts != 3 {
			t.Fatalf("attempts[%d].MaxAttempts = %d, want 3", i, attempt.MaxAttempts)
		}
		if !errors.Is(attempt.Err, operationErr) {
			t.Fatalf("attempts[%d].Err = %v, want operationErr", i, attempt.Err)
		}
		if attempt.Delay != 25*time.Millisecond {
			t.Fatalf("attempts[%d].Delay = %v, want 25ms", i, attempt.Delay)
		}
	}
}

func TestDoNilOperation(t *testing.T) {
	err := Do(context.Background(), nil)
	if err == nil {
		t.Fatal("Do() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "operation is nil") {
		t.Fatalf("Do() error = %q, want clear nil operation error", err.Error())
	}
}

func TestDoNilContext(t *testing.T) {
	calls := 0

	err := Do(nil, func(ctx context.Context) error {
		calls++
		if ctx == nil {
			t.Fatal("operation context is nil")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Do() error = %v, want nil", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestDoDefaultRetryIfDoesNotRetryContextErrors(t *testing.T) {
	for _, operationErr := range []error{context.Canceled, context.DeadlineExceeded} {
		calls := 0

		err := Do(context.Background(), func(context.Context) error {
			calls++
			return operationErr
		}, WithBackoff(NoBackoff()))

		if !errors.Is(err, operationErr) {
			t.Fatalf("Do() error = %v, want %v", err, operationErr)
		}
		if calls != 1 {
			t.Fatalf("calls = %d, want 1", calls)
		}
	}
}
