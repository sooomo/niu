package niu

import "time"

type RetryStrategy interface {
	Next() time.Duration
}

type linearRetryStrategy time.Duration

// LinearRetryStrategy allows retries regularly with customized intervals
func LinearRetryStrategy(backoff time.Duration) RetryStrategy {
	return linearRetryStrategy(backoff)
}

// NoRetry acquire the lock only once.
func NoRetry() RetryStrategy {
	return linearRetryStrategy(0)
}

func (r linearRetryStrategy) Next() time.Duration {
	return time.Duration(r)
}
