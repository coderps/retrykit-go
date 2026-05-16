package retrykit

import "fmt"

// RetryError reports that an operation failed after all retry attempts were
// exhausted.
type RetryError struct {
	Attempts int
	LastErr  error
}

// Error returns a readable retry failure message.
func (e *RetryError) Error() string {
	if e == nil {
		return "retrykit: operation failed"
	}
	if e.LastErr == nil {
		return fmt.Sprintf("retrykit: operation failed after %d attempts", e.Attempts)
	}

	return fmt.Sprintf("retrykit: operation failed after %d attempts: %v", e.Attempts, e.LastErr)
}

// Unwrap returns the last error returned by the operation.
func (e *RetryError) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.LastErr
}
