// Package retry provides a thin wrapper around the [client.Client] interface that retries the push request with
// exponential backoff if it fails.
package retry

import (
	"context"
	"errors"
	"time"

	"github.com/tslnc04/loki-logger/pkg/client"
)

// Backoff is a struct that defines the backoff strategy for the retry client. Since a single backoff is expected to
// contain mutable state and cannot be called concurrently, Clone provides a way to create a new backoff and will be
// called for each push to [Client].
//
// Next returns a new channel that has a single value sent when the backoff is complete. If the time from the Next
// channel is zero-valued, the backoff has completed and the caller should stop retrying.
type Backoff interface {
	Next() <-chan time.Time
	Clone() Backoff
}

const (
	// DefaultInitialDelay is the default initial delay for [ExponentialBackoff] when none is provided.
	DefaultInitialDelay = 100 * time.Millisecond
	// DefaultFactor is the default factor for [ExponentialBackoff] when none is provided. It is used to multiply
	// the delay each time the backoff is called.
	DefaultFactor = 2.0
)

// ExponentialBackoff is a struct that implements the [Backoff] interface. It uses exponential backoff with a
// configurable starting delay, factor, and maximum delay. The package constants provide the default values for these
// fields. A maximum delay of 0 means no maximum delay.
type ExponentialBackoff struct {
	Delay  time.Duration
	Factor float64
	Max    time.Duration
}

// Assert that ExponentialBackoff implements the [Backoff] interface.
var _ Backoff = (*ExponentialBackoff)(nil)

// Next returns a new channel that has a single value sent when the backoff is complete. If the time from the Next
// channel is zero-valued, the backoff has completed and the caller should stop retrying. The delay is multiplied by the
// factor each time it is called.
func (b *ExponentialBackoff) Next() <-chan time.Time {
	if b.Max != 0 && b.Delay > b.Max {
		timeChan := make(chan time.Time)
		close(timeChan)

		return timeChan
	}

	if b.Delay == 0 {
		b.Delay = DefaultInitialDelay
	}

	if b.Factor == 0 {
		b.Factor = DefaultFactor
	}

	delay := b.Delay
	b.Delay = time.Duration(float64(b.Delay) * b.Factor)

	return time.After(delay)
}

// Clone returns a new backoff with the same configuration as the original. It is safe to call concurrently from
// multiple goroutines. Although the new backoff has its own state, if the original backoff has already been used, the
// new backoff will also appear to have been used.
//
//nolint:ireturn // Necessary to implement the Backoff interface.
func (b *ExponentialBackoff) Clone() Backoff {
	return &ExponentialBackoff{
		Delay:  b.Delay,
		Factor: b.Factor,
		Max:    b.Max,
	}
}

// Client is a client that retries the push request with exponential backoff if it fails. It implements the
// [Client] interface. It is safe to call concurrently from multiple goroutines, although this may result in multiple
// requests being in flight and retrying at the same time.
type Client struct {
	inner   client.Client
	backoff Backoff
}

// NewRetryClient creates a new RetryClient with the given client. It defaults to using the default values for
// [ExponentialBackoff].
func NewRetryClient(client client.Client) *Client {
	return &Client{
		inner:   client,
		backoff: &ExponentialBackoff{},
	}
}

// WithBackoff sets the backoff strategy for the RetryClient. It is safe to call concurrently from multiple goroutines
// and will return a new RetryClient with the same inner client and the given backoff strategy.
func (retryClient *Client) WithBackoff(backoff Backoff) *Client {
	return &Client{
		inner:   retryClient.inner,
		backoff: backoff.Clone(),
	}
}

// Assert that RetryClient implements the [client.Client] interface.
var _ client.Client = (*Client)(nil)

// Push implements the [Client] interface. It retries the push request with exponential backoff if it fails.
func (retryClient *Client) Push(ctx context.Context, entry client.Entry) error {
	retryClient.PushWithHandle(ctx, entry)

	return nil
}

// PushWithHandle is similar to [Push] but returns a channel that will have a single error sent when the push exhausts
// all retries. If the push succeeds before the retries are exhausted, the channel will be closed without sending an
// error.
func (retryClient *Client) PushWithHandle(ctx context.Context, entry client.Entry) <-chan error {
	errChan := make(chan error, 1)
	clonedBackoff := retryClient.backoff.Clone()

	go func() {
		err := retryClient.inner.Push(ctx, entry)

		for errors.Is(err, &client.PushStatusError{}) {
			select {
			case _, ok := <-clonedBackoff.Next():
				if !ok {
					return
				}

				err = retryClient.inner.Push(ctx, entry)
			case <-ctx.Done():
				return
			}
		}

		if err != nil {
			errChan <- err
		}

		close(errChan)
	}()

	return errChan
}
