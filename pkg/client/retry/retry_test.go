package retry

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tslnc04/loki-logger/pkg/client"
	"github.com/tslnc04/loki-logger/pkg/internal/fake"
)

func TestExponentialBackoff_Clone(t *testing.T) {
	t.Parallel()

	backoff := &ExponentialBackoff{
		Delay:  1 * time.Second,
		Factor: 2.0,
		Max:    10 * time.Second,
	}

	clonedBackoff := backoff.Clone()
	require.IsType(t, &ExponentialBackoff{}, clonedBackoff)
	require.Equal(t, backoff, clonedBackoff)
}

//nolint:funlen // The length is mostly tests, so function length is not a concern.
func TestExponentialBackoff_Next(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		backoff   ExponentialBackoff
		completes bool
		cycles    int
		totalTime time.Duration
	}{
		{
			name: "once",
			backoff: ExponentialBackoff{
				Delay:  1 * time.Second,
				Factor: 2.0,
				Max:    10 * time.Second,
			},
			completes: false,
			cycles:    1,
			totalTime: 1 * time.Second,
		},
		{
			name: "multiple",
			backoff: ExponentialBackoff{
				Delay:  1 * time.Second,
				Factor: 2.0,
				Max:    10 * time.Second,
			},
			completes: false,
			cycles:    2,
			totalTime: 3 * time.Second,
		},
		{
			name: "completes",
			backoff: ExponentialBackoff{
				Delay:  500 * time.Millisecond,
				Factor: 2.0,
				Max:    1 * time.Second,
			},
			completes: true,
			cycles:    3,
			totalTime: 1500 * time.Millisecond,
		},
		{
			name:      "defaults",
			backoff:   ExponentialBackoff{Max: time.Second},
			completes: true,
			cycles:    5,
			totalTime: (100 + 200 + 400 + 800) * time.Millisecond,
		},
		{
			name: "custom-factor",
			backoff: ExponentialBackoff{
				Delay:  1 * time.Second,
				Factor: 1.5,
			},
			completes: false,
			cycles:    2,
			totalTime: 2500 * time.Millisecond,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			startTime := time.Now()

			for cycleCount := range testCase.cycles {
				next := testCase.backoff.Next()
				select {
				case _, ok := <-next:
					if cycleCount == testCase.cycles-1 && testCase.completes {
						require.False(t, ok)
					} else {
						require.True(t, ok)
					}
				case <-t.Context().Done():
					require.Fail(t, "context done before next")
				}
			}

			timeElapsed := time.Since(startTime)
			require.InDelta(t, testCase.totalTime, timeElapsed, float64(50*time.Millisecond))
		})
	}
}

func TestRetryClient_WithBackoff(t *testing.T) {
	t.Parallel()

	client := NewRetryClient(nil)
	backoff := &ExponentialBackoff{
		Delay:  1 * time.Second,
		Factor: 2.0,
		Max:    10 * time.Second,
	}

	retryClient := client.WithBackoff(backoff)
	require.Equal(t, backoff, retryClient.backoff)

	// Ensure the original client is not modified.
	require.NotEqual(t, client.backoff, retryClient.backoff)
}

func TestRetryClient_PushWithHandle(t *testing.T) {
	t.Parallel()

	var testEntry = client.Entry{
		Timestamp:          time.Now(),
		Labels:             client.LabelMap{"foo": "bar"},
		Line:               "test message",
		StructuredMetadata: map[string]string{"key": "value"},
	}

	testCases := []struct {
		name      string
		sendError uint
	}{
		{
			name:      "success",
			sendError: 0,
		},
		{
			name: "errors",

			sendError: 4,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			fakeServer := fake.NewServer(testCase.sendError)
			httpServer := fakeServer.Start()

			defer httpServer.Close()

			lokiClient := client.NewLokiClient(httpServer.URL + client.PushPath)
			retryClient := NewRetryClient(lokiClient)

			errChan := retryClient.PushWithHandle(t.Context(), testEntry)
			require.NoError(t, <-errChan)

			streams := fakeServer.Streams()
			defer fakeServer.Close()

			require.Len(t, streams, 1, "Expected one push stream to be sent to the server")
			client.AssertStreamMatchesEntry(t, testEntry, streams[0])
		})
	}
}
