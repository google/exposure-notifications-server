// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package retry provides generic function-based retry helpers.
package retry

import (
	"context"
	"sync"
	"time"
)

// RetryFunc is a function passed to retry.
type RetryFunc func() error

type retryableError struct{ error }

// RetryableError marks an error as retryable.
func RetryableError(err error) error {
	if err == nil {
		return nil
	}
	return &retryableError{err}
}

// Retry wraps a function with a backoff to retry.
func Retry(ctx context.Context, b Backoff, f RetryFunc) error {
	for {
		err := f()
		if err == nil {
			return nil
		}

		// Not retryable
		if _, ok := err.(*retryableError); !ok {
			return err
		}

		next, stop := b.Next()
		if stop {
			return err
		}

		select {
		case <-ctx.Done():
		case <-time.After(next):
			continue
		}
	}
}

// RetryFib is a wrapper around Retry that uses a fibonacci backoff.
func RetryFib(ctx context.Context, base time.Duration, maxAttempts int, f RetryFunc) error {
	return Retry(ctx, FibonacciBackoff(base, maxAttempts), f)
}

// RetryExp is a wrapper around Retry that uses an exponential backoff.
func RetryExp(ctx context.Context, base time.Duration, maxAttempts int, f RetryFunc) error {
	return Retry(ctx, ExponentialBackoff(base, maxAttempts), f)
}

// Backoff is an interface that backs off.
type Backoff interface {
	Next() (next time.Duration, stop bool)
}

// FibonacciBackoff creates a new fibonacci backoff.
func FibonacciBackoff(base time.Duration, maxAttempts int) Backoff {
	return &fibonacciBackoff{
		prev2:       base,
		maxAttempts: maxAttempts,
	}
}

type fibonacciBackoff struct {
	sync.Mutex
	prev1       time.Duration
	prev2       time.Duration
	maxAttempts int
	attempts    int
}

func (b *fibonacciBackoff) Next() (time.Duration, bool) {
	b.Lock()
	defer b.Unlock()

	b.attempts++
	if b.attempts > b.maxAttempts {
		return 0, true
	}

	next := b.prev1 + b.prev2
	b.prev1, b.prev2 = b.prev2, next

	return next, false
}

// ExponentialBackoff creates a new exponential backoff.
func ExponentialBackoff(base time.Duration, maxAttempts int) Backoff {
	return &exponentialBackoff{
		prev:        base,
		maxAttempts: maxAttempts,
	}
}

type exponentialBackoff struct {
	sync.Mutex
	prev        time.Duration
	maxAttempts int
	attempts    int
}

func (b *exponentialBackoff) Next() (time.Duration, bool) {
	b.Lock()
	defer b.Unlock()

	b.attempts++
	if b.attempts > b.maxAttempts {
		return 0, true
	}

	next := b.prev * b.prev
	b.prev = next

	return next, false
}
