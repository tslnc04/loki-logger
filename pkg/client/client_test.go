package client

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tslnc04/loki-logger/pkg/internal/fake"
)

var (
	testPushStatusError = PushStatusError{
		StatusCode: 500,
		Status:     "500 Internal Server Error",
		Body:       []byte("Internal Server Error"),
	}
)

func TestNewLokiClient(t *testing.T) {
	t.Parallel()

	const url = "http://localhost:3100"

	lokiClient := NewLokiClient(url)
	require.NotNil(t, lokiClient)
	require.Equal(t, url, lokiClient.url)
	require.NotNil(t, lokiClient.client)
}

func TestLokiClient_WithHTTPClient(t *testing.T) {
	t.Parallel()

	const url = "http://localhost:3100"

	lokiClient := NewLokiClient(url)
	require.NotNil(t, lokiClient)

	withHTTP := lokiClient.WithHTTPClient(&http.Client{Timeout: time.Second})
	require.Equal(t, &http.Client{Timeout: time.Second}, withHTTP.client)

	// The original client should not be modified.
	require.Zero(t, lokiClient.client.Timeout)
}

func TestLokiClient_Push(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		entry         Entry
		expectedError error
	}{
		{
			name: "success",
			entry: Entry{
				Timestamp:          time.Now(),
				Labels:             LabelMap{"foo": "bar"},
				Line:               "test message",
				StructuredMetadata: map[string]string{"key": "value"},
			},
			expectedError: nil,
		},
		{
			name:          "error",
			entry:         Entry{},
			expectedError: &testPushStatusError,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			sendError := uint(0)
			if testCase.expectedError != nil {
				sendError = 1
			}

			fakeServer := fake.NewServer(sendError)
			httpServer := fakeServer.Start()

			defer httpServer.Close()

			lokiClient := NewLokiClient(httpServer.URL + PushPath)
			err := lokiClient.Push(context.Background(), testCase.entry)

			if testCase.expectedError != nil {
				require.Error(t, err)
				require.Exactly(t, testCase.expectedError, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPushStatusError_Error(t *testing.T) {
	t.Parallel()

	err := &testPushStatusError

	require.EqualError(t, err, "push request failed with status 500 Internal Server Error: Internal Server Error")
}

func TestPushStatusError_Is(t *testing.T) {
	t.Parallel()

	err := &testPushStatusError

	require.True(t, err.Is(&PushStatusError{}))
	require.False(t, err.Is(nil))
}
