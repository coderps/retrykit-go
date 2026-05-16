package retrykit

import (
	"context"
	"errors"
	"time"
)

const defaultMaxAttempts = 3

var errNilOperation = errors.New("retrykit: operation is nil")

// Operation is a function that can be retried.
//
// The context passed to Operation is the same context supplied to Do, allowing
// the operation to respect cancellation and deadlines.
type Operation func(context.Context) error

// RetryIf decides whether an operation error should be retried.
type RetryIf func(error) bool

// Option customizes retry behavior.
type Option func(*Config)

// Config contains retry settings.
//
// Invalid or nil values are normalized to safe defaults by Do.
type Config struct {
	MaxAttempts int
	Backoff     Backoff
	RetryIf     RetryIf
	OnRetry     func(Attempt)

	sleep func(context.Context, time.Duration) error
}

// Attempt describes a failed attempt that is about to be retried.
type Attempt struct {
	Number      int
	MaxAttempts int
	Err         error
	Delay       time.Duration
}

// Do runs operation until it succeeds, returns a non-retryable error, exhausts
// its attempts, or the context is cancelled.
//
// MaxAttempts includes the initial operation attempt. Non-retryable errors are
// returned directly. Exhausted retries return a *RetryError wrapping the last
// operation error. Context cancellation returns the context error directly.
func Do(ctx context.Context, operation Operation, options ...Option) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if operation == nil {
		return errNilOperation
	}

	cfg := defaultConfig()
	for _, option := range options {
		if option != nil {
			option(&cfg)
		}
	}
	cfg.normalize()

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		err := operation(ctx)
		if err == nil {
			return nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if !cfg.RetryIf(err) {
			return err
		}
		if attempt == cfg.MaxAttempts {
			return &RetryError{Attempts: attempt, LastErr: err}
		}

		delay := cfg.Backoff.Delay(attempt)
		if delay < 0 {
			delay = 0
		}

		if cfg.OnRetry != nil {
			cfg.OnRetry(Attempt{
				Number:      attempt,
				MaxAttempts: cfg.MaxAttempts,
				Err:         err,
				Delay:       delay,
			})
		}

		if err := cfg.sleep(ctx, delay); err != nil {
			return err
		}
	}

	return nil
}

// WithMaxAttempts sets the maximum number of operation attempts, including the
// initial attempt. Values less than or equal to zero fall back to the default.
func WithMaxAttempts(max int) Option {
	return func(cfg *Config) {
		cfg.MaxAttempts = max
	}
}

// WithBackoff sets the backoff used between failed attempts. A nil backoff
// falls back to the default exponential backoff.
func WithBackoff(backoff Backoff) Option {
	return func(cfg *Config) {
		cfg.Backoff = backoff
	}
}

// WithRetryIf sets the retry predicate. A nil predicate falls back to the
// default retry behavior.
func WithRetryIf(fn RetryIf) Option {
	return func(cfg *Config) {
		cfg.RetryIf = fn
	}
}

// WithOnRetry sets a callback that runs after a failed retryable attempt and
// before waiting for the next attempt. It is not called after the final failed
// attempt.
func WithOnRetry(fn func(Attempt)) Option {
	return func(cfg *Config) {
		cfg.OnRetry = fn
	}
}

func defaultConfig() Config {
	return Config{
		MaxAttempts: defaultMaxAttempts,
		Backoff:     ExponentialBackoff(0, 0),
		RetryIf:     defaultRetryIf,
		sleep:       sleep,
	}
}

func (cfg *Config) normalize() {
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = defaultMaxAttempts
	}
	if cfg.Backoff == nil {
		cfg.Backoff = ExponentialBackoff(0, 0)
	}
	if cfg.RetryIf == nil {
		cfg.RetryIf = defaultRetryIf
	}
	if cfg.sleep == nil {
		cfg.sleep = sleep
	}
}

func defaultRetryIf(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	return true
}

func sleep(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
