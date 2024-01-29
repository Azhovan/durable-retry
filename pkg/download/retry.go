package download

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"
)

// ErrMaxTotalRetryDurationExceeded is an error indicating that the maximum total retry duration has been exceeded.
var ErrMaxTotalRetryDurationExceeded = errors.New("maximum total retry duration exceeded")

// RetryPolicy defines the configuration for retrying operations.
type RetryPolicy struct {
	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int

	// RetryDelay is the initial delay before the first retry
	RetryDelay time.Duration

	// BackoffFactor is the multiplier by which the retry delay is increased after each attempt.
	BackoffFactor float64

	// Jitter adds randomness to the retry delay to prevent synchronized retries.
	Jitter time.Duration

	// OnRetry is an optional callback function called before each retry attempt.
	// It can be used for logging or custom logic.
	OnRetry func(id int, attempt int, nextRetryIn time.Duration)

	// ShouldRetry is an optional callback that determines whether a retry should be attempted
	// after an error. If not set, all errors will trigger a retry.
	ShouldRetry func(err error) bool

	// MaxTotalRetryDuration is the maximum total time to spend on all retry attempts.
	// If zero, there is no limit on the total retry duration.
	MaxTotalRetryDuration time.Duration

	// seed is a source of random numbers used to generate jitter in retry intervals.
	// It ensures that each retry interval has some level of randomness,
	// reducing the chance of synchronized retries in distributed systems.
	// This random number generator is safe for concurrent use.
	seed *rand.Rand
}

// NewRetryPolicy creates a new RetryPolicy with the given parameters.
//
// This function initializes a new source of random numbers (seeded with the current time)
// used for generating jitter in retry intervals. This randomness helps to spread out retry
// attempts in distributed systems and prevent synchronized retries.
func NewRetryPolicy(maxRetries int, options ...RetryOption) *RetryPolicy {
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))

	retry := &RetryPolicy{
		MaxRetries: maxRetries,
		seed:       seed,
	}
	for _, opt := range options {
		opt(retry)
	}

	return retry
}

// RetryOption is a functional option for configuring a RetryPolicy.
type RetryOption func(policy *RetryPolicy)

// WithRetryDelay is a RetryOption that sets the initial delay
// before the first retry in the RetryPolicy.
func WithRetryDelay(delay time.Duration) RetryOption {
	return func(policy *RetryPolicy) {
		policy.RetryDelay = delay
	}
}

// WithBackoffFactor is a RetryOption function that sets the backoff factor of the RetryPolicy.
func WithBackoffFactor(factor float64) RetryOption {
	return func(policy *RetryPolicy) {
		policy.BackoffFactor = factor
	}
}

// WithJitter is a RetryOption that sets the Jitter value in the RetryPolicy.
func WithJitter(jitter time.Duration) RetryOption {
	return func(policy *RetryPolicy) {
		policy.Jitter = jitter
	}
}

// Retry runs the given function with the retry policy.
// It implements a retry mechanism based on the policy's configuration,
// such as maximum retries, retry delay, backoff factor, and jitter.
// Usage:
//
//	err := retryPolicy.Retry(ctx, segmentID, func() error {
//	    // Operation to retry
//	})
func (p *RetryPolicy) Retry(ctx context.Context, segmentID int, task func() error) error {
	var err error

	totalRetryDuration := time.Duration(0)

	for attempt := 1; attempt <= p.MaxRetries; attempt++ {
		// we don't want to end up with goroutines that are stuck waiting to retry even after the context is canceled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err = task(); err == nil {
			return nil
		}

		if p.ShouldRetry != nil && !p.ShouldRetry(err) {
			return err
		}

		nextRetryIn := p.RetryDelay + time.Duration(float64(attempt)*p.BackoffFactor)*time.Millisecond
		nextRetryIn += time.Duration(p.seed.Int63n(int64(p.Jitter)))

		// Check if exceeding the maximum total retry duration
		if p.MaxTotalRetryDuration > 0 {
			totalRetryDuration += nextRetryIn
			if totalRetryDuration > p.MaxTotalRetryDuration {
				return ErrMaxTotalRetryDurationExceeded
			}
		}

		if p.OnRetry != nil {
			p.OnRetry(segmentID, attempt+1, nextRetryIn)
		}

		time.Sleep(nextRetryIn)
	}

	return err
}

const defaultMaxRetries = 5

// DefaultRetryPolicy creates a retry policy with sensible defaults.
func DefaultRetryPolicy() *RetryPolicy {
	retry := NewRetryPolicy(
		defaultMaxRetries,                // Retry up to 5 times
		WithRetryDelay(1*time.Second),    // Start with a 1-second delay
		WithJitter(500*time.Millisecond), // Add up to 500ms of random jitter
		WithBackoffFactor(2),             // Double the delay with each retry
	)

	retry.OnRetry = func(id int, attempt int, nextRetryIn time.Duration) {
		fmt.Printf("segment ID:%d: retry attempt: %d, retring in: %v\n", id, attempt, nextRetryIn)
	}

	return retry
}
