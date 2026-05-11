package taskserver

import (
	"math"
	"time"
)

// DefaultRetryFactor is the backoff factor used when RetryConfig.Factor is unset.
const DefaultRetryFactor float32 = 2.0

// GetFactor returns the configured exponential backoff factor, falling back to
// DefaultRetryFactor when the receiver or Factor is nil.
func (r *RetryConfig) GetFactor() float32 {
	if r == nil || r.Factor == nil {
		return DefaultRetryFactor
	}
	return *r.Factor
}

// GetWaitDurationMs returns the configured initial wait between retries in
// milliseconds, or 0 when the receiver or WaitDurationMs is nil.
func (r *RetryConfig) GetWaitDurationMs() int64 {
	if r == nil || r.WaitDurationMs == nil {
		return 0
	}
	return *r.WaitDurationMs
}

func (r *RetryConfig) GetMaxRetries() int {
	if r == nil || r.MaxRetries == nil {
		return 0
	}
	return *r.MaxRetries
}

// ShouldRetry reports whether another attempt is permitted given the number of
// retries already completed.
func (r *RetryConfig) ShouldRetry(retryCount int) bool {
	return retryCount < r.GetMaxRetries()
}

// GetSleepDuration returns the backoff before the given retry attempt:
// waitDuration * factor^retryCount.
func (r *RetryConfig) GetSleepDuration(retryCount int) time.Duration {
	wait := float64(r.GetWaitDurationMs()) * float64(time.Millisecond)
	return time.Duration(wait * math.Pow(float64(r.GetFactor()), float64(retryCount)))
}
