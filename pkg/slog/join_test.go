package slog

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
	"testing/slogtest"

	"github.com/stretchr/testify/require"
)

func TestJoinedHandler(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		concurrency bool
	}{
		{
			name:        "concurrency-enabled",
			concurrency: true,
		},
		{
			name:        "concurrency-disabled",
			concurrency: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var (
				output1 bytes.Buffer
				output2 bytes.Buffer
			)

			newHandler := generateNewHandlerFunc(&output1, &output2, testCase.concurrency)
			resultFunc := generateResultFunc(&output1, &output2)

			slogtest.Run(t, newHandler, resultFunc)
		})
	}
}

func TestJoinedHandlerWithHandlers(t *testing.T) {
	t.Parallel()

	var (
		output1 bytes.Buffer
		output2 bytes.Buffer
	)

	newHandlerFunc := func(t *testing.T) slog.Handler {
		t.Helper()

		handler1 := slog.NewJSONHandler(&output1, nil)
		handler2 := slog.NewJSONHandler(&output2, nil)

		return NewJoinedHandler().WithHandlers(handler1, handler2)
	}

	resultFunc := generateResultFunc(&output1, &output2)

	slogtest.Run(t, newHandlerFunc, resultFunc)
}

func generateNewHandlerFunc(output1, output2 *bytes.Buffer, concurrency bool) func(t *testing.T) slog.Handler {
	return func(t *testing.T) slog.Handler {
		t.Helper()

		handler1 := slog.NewJSONHandler(output1, nil)
		handler2 := slog.NewJSONHandler(output2, nil)

		return NewJoinedHandler(handler1, handler2).WithConcurrency(concurrency)
	}
}

func generateResultFunc(output1, output2 *bytes.Buffer) func(t *testing.T) map[string]any {
	return func(t *testing.T) map[string]any {
		t.Helper()

		var parsed1 map[string]any
		err := json.Unmarshal(output1.Bytes(), &parsed1)
		require.NoError(t, err, "failed to unmarshal first output")

		output1.Reset()

		var parsed2 map[string]any
		err = json.Unmarshal(output2.Bytes(), &parsed2)
		require.NoError(t, err, "failed to unmarshal second output")

		output2.Reset()

		require.Equal(t, parsed1, parsed2, "outputs should be equal")

		return parsed1
	}
}
