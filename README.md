# retrykit-go

`retrykit-go` is a tiny, idiomatic Go library for retrying operations with simple, safe defaults.

It is intentionally small and named `retrykit-go` to distinguish it from sibling implementations such as `retrykit-py`: one main function, a few options, and basic backoff strategies. It is meant to be easy to understand and useful within a few minutes, not a full retry framework.

## Installation

```sh
go get github.com/coderps/retrykit-go
```

## Quick start

```go
package main

import (
    "context"

    "github.com/coderps/retrykit-go"
)

func main() {
    ctx := context.Background()

    err := retrykit.Do(ctx, func(ctx context.Context) error {
        return callExternalService(ctx)
    })
    if err != nil {
        // handle error
    }
}
```

By default, `retrykit-go` tries an operation up to 3 times, including the initial attempt. It uses exponential backoff and does not retry `context.Canceled` or `context.DeadlineExceeded`.

## Context-aware retries

The operation receives the context passed to `Do`, so your code can respect cancellation and deadlines.

```go
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel()

err := retrykit.Do(ctx, func(ctx context.Context) error {
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
    if err != nil {
        return err
    }

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 500 {
        return fmt.Errorf("server error: %s", resp.Status)
    }

    return nil
})
```

If the context is already cancelled, the operation is not run. If the context is cancelled while waiting between attempts, `Do` returns immediately with the context error.

## Fixed backoff

```go
err := retrykit.Do(ctx, operation,
    retrykit.WithMaxAttempts(5),
    retrykit.WithBackoff(retrykit.FixedBackoff(200*time.Millisecond)),
)
```

## Exponential backoff

```go
err := retrykit.Do(ctx, operation,
    retrykit.WithMaxAttempts(4),
    retrykit.WithBackoff(retrykit.ExponentialBackoff(100*time.Millisecond, 2*time.Second)),
)
```

For exponential backoff, retry delay 1 is `base`, retry delay 2 is `base * 2`, retry delay 3 is `base * 4`, and delays are capped at `max`.

## Custom retry condition

Return `true` for errors that should be retried. Non-retryable errors are returned directly.

```go
var ErrTemporary = errors.New("temporary")

err := retrykit.Do(ctx, operation,
    retrykit.WithMaxAttempts(5),
    retrykit.WithBackoff(retrykit.FixedBackoff(200*time.Millisecond)),
    retrykit.WithRetryIf(func(err error) bool {
        return errors.Is(err, ErrTemporary)
    }),
)
```

Parent context cancellation always stops retries, even if a custom retry condition would retry the operation error.

## OnRetry callback

Use `WithOnRetry` to observe retry attempts. The callback runs after a failed retryable attempt and before sleeping for the next attempt. It is not called after the final failed attempt.

```go
err := retrykit.Do(ctx, operation,
    retrykit.WithOnRetry(func(attempt retrykit.Attempt) {
        log.Printf(
            "retrying after attempt %d/%d failed: %v; next delay: %s",
            attempt.Number,
            attempt.MaxAttempts,
            attempt.Err,
            attempt.Delay,
        )
    }),
)
```

`retrykit-go` does not log by itself.

## Error handling

When all attempts are exhausted, `Do` returns a `*retrykit.RetryError` that records the number of attempts and wraps the last operation error.

```go
err := retrykit.Do(ctx, operation)
if err != nil {
    var retryErr *retrykit.RetryError
    if errors.As(err, &retryErr) {
        fmt.Println("attempts:", retryErr.Attempts)
        fmt.Println("last error:", retryErr.LastErr)
    }
}
```

Because `RetryError` unwraps the last error, `errors.Is` and `errors.As` work with the original error.

```go
if errors.Is(err, ErrTemporary) {
    // the last operation error matched ErrTemporary
}
```

Non-retryable errors are returned directly, and context cancellation returns the context error directly.

## Design philosophy

- Small API surface.
- Safe defaults.
- Context-aware by default.
- No external dependencies.
- Easy to test.
- Easy to understand.
- Production-friendly, but not over-engineered.

## Non-goals

`retrykit-go` intentionally does not include:

- HTTP-specific helpers.
- Database-specific helpers.
- Logging or metrics.
- Background retry workers.
- A large policy engine.
- Complex scheduling or circuit breaker behavior.

For complex retry policies, distributed systems tooling, or advanced resilience patterns, a larger mature library may be a better fit.
