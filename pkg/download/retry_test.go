package download

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRetryPolicy_Retry(t *testing.T) {
	tests := []struct {
		name                  string
		maxRetries            int
		jitter                time.Duration
		initialDelay          time.Duration
		backoffFactor         float64
		shouldRetry           func(error) bool
		maxTotalRetryDuration time.Duration
		task                  func() error
		wantErr               error
	}{
		{
			name:                  "no_retry_success_task",
			maxRetries:            3,
			jitter:                1,
			initialDelay:          time.Millisecond,
			backoffFactor:         1.5,
			shouldRetry:           nil,
			maxTotalRetryDuration: 0,
			task:                  func() error { return nil },
			wantErr:               nil,
		},
		{
			name:          "no_retry_fail_task",
			maxRetries:    3,
			jitter:        1,
			initialDelay:  time.Millisecond,
			backoffFactor: 1.5,
			shouldRetry: func(err error) bool {
				return false
			},
			maxTotalRetryDuration: 0,
			task:                  func() error { return errors.New("error") },
			wantErr:               errors.New("error"),
		},
		{
			name:                  "retry_success_task",
			maxRetries:            3,
			jitter:                1,
			initialDelay:          time.Millisecond,
			backoffFactor:         1.5,
			shouldRetry:           func(err error) bool { return true },
			maxTotalRetryDuration: 0,
			task:                  func() error { return errors.New("error") },
			wantErr:               errors.New("error"),
		},
		{
			name:                  "retry_fail_due_to_exceeded_max_duration",
			maxRetries:            3,
			jitter:                1,
			initialDelay:          time.Second,
			backoffFactor:         1.5,
			shouldRetry:           func(err error) bool { return true },
			maxTotalRetryDuration: time.Millisecond,
			task:                  func() error { return errors.New("error") },
			wantErr:               ErrMaxTotalRetryDurationExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rp := NewRetryPolicy(
				tt.maxRetries,
				WithJitter(tt.jitter),
				WithRetryDelay(tt.initialDelay),
				WithBackoffFactor(tt.backoffFactor),
				WithMaxTotalRetryDuration(tt.maxTotalRetryDuration),
				WithShouldRetryPolicy(tt.shouldRetry),
			)
			ctx := context.Background()
			err := rp.Retry(ctx, 1, tt.task)
			assert.Equal(t, tt.wantErr, err)
		})
	}
}
