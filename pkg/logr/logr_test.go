package logr

import (
	"runtime"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	"github.com/tslnc04/loki-logger/pkg/client"
	"github.com/tslnc04/loki-logger/pkg/internal/fake"
)

const (
	currentPackage = "github.com/tslnc04/loki-logger/pkg/logr"
	defaultMessage = "Hello, world!"
	maxLevel       = 10
)

var _, currentFile, _, _ = runtime.Caller(0)

func TestInfoVerbosityLevels(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		level    int
		expected []client.Entry
	}{
		{
			name:     "log-higher-than-logger-verbosity",
			level:    1,
			expected: []client.Entry{},
		},
		{
			name:  "log-same-as-logger-verbosity",
			level: 0,
			expected: []client.Entry{{
				Labels: client.LabelMap{
					LevelKey: "0",
				},
				Line: defaultMessage,
				StructuredMetadata: map[string]string{
					SourceKey + "_function": currentPackage + ".TestInfoVerbosityLevels.func1",
					SourceKey + "_file":     currentFile,
					SourceKey + "_line":     "63",
				},
			}},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			fakeServer := fake.NewServer(0)
			httpServer := fakeServer.Start()

			defer httpServer.Close()

			lokiClient := client.NewLokiClient(httpServer.URL + client.PushPath)
			logger := New(lokiClient, 0)

			logger.V(testCase.level).Info(defaultMessage)

			streams := fakeServer.Streams()
			defer fakeServer.Close()

			require.Len(t, streams, len(testCase.expected), "Expected number of streams to match")

			for i, expectedEntry := range testCase.expected {
				client.AssertStreamMatchesEntry(t, expectedEntry, streams[i])
			}
		})
	}
}

func TestErrorVerbosityLevels(t *testing.T) {
	t.Parallel()

	expectedEntry := client.Entry{
		Labels: client.LabelMap{
			LevelKey: "-1",
		},
		Line: defaultMessage,
		StructuredMetadata: map[string]string{
			ErrorKey:                "<nil>",
			SourceKey + "_function": currentPackage + ".TestErrorVerbosityLevels.func1",
			SourceKey + "_file":     currentFile,
			SourceKey + "_line":     "122",
		},
	}

	testCases := []struct {
		name     string
		level    int
		expected []client.Entry
	}{
		{
			name:     "log-higher-than-logger-verbosity",
			level:    1,
			expected: []client.Entry{expectedEntry},
		},
		{
			name:     "log-same-as-logger-verbosity",
			level:    0,
			expected: []client.Entry{expectedEntry},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			fakeServer := fake.NewServer(0)
			httpServer := fakeServer.Start()

			defer httpServer.Close()

			lokiClient := client.NewLokiClient(httpServer.URL + client.PushPath)
			logger := New(lokiClient, 0)

			logger.V(testCase.level).Error(nil, defaultMessage)

			streams := fakeServer.Streams()
			defer fakeServer.Close()

			require.Len(t, streams, len(testCase.expected), "Expected number of streams to match")

			for i, expected := range testCase.expected {
				client.AssertStreamMatchesEntry(t, expected, streams[i])
			}
		})
	}
}

func TestNewLokiSink(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		levels []int
	}{
		{
			name:   "no-levels",
			levels: []int{},
		},
		{
			name:   "one-level",
			levels: []int{1},
		},
		{
			name:   "multiple-levels",
			levels: []int{1, 2, 3},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			lokiSink := NewLokiSink(nil, testCase.levels...)
			require.NotNil(t, lokiSink)
			require.Nil(t, lokiSink.lokiClient)

			if len(testCase.levels) == 0 {
				require.Zero(t, lokiSink.level)
			} else {
				require.Equal(t, testCase.levels[0], lokiSink.level)
			}
		})
	}
}

func TestLokiSink_WithLevel(t *testing.T) {
	t.Parallel()

	lokiSink := NewLokiSink(nil, 0)
	require.NotNil(t, lokiSink)
	require.Equal(t, 0, lokiSink.level)

	modifiedSink := lokiSink.WithLevel(1)
	require.NotNil(t, modifiedSink)
	require.Equal(t, 1, modifiedSink.level)

	// Ensure the original sink is not modified.
	require.Equal(t, 0, lokiSink.level)
}

func TestLokiSink_Clone(t *testing.T) {
	t.Parallel()

	lokiSink := NewLokiSink(nil, 0)
	require.NotNil(t, lokiSink)

	clonedSink := lokiSink.Clone()
	require.NotNil(t, clonedSink)
	require.Equal(t, lokiSink.level, clonedSink.level)

	// Ensure the original sink is not modified.
	lokiSink.level = 1
	require.NotEqual(t, lokiSink.level, clonedSink.level)
}

func TestLokiSink_Init(t *testing.T) {
	t.Parallel()

	lokiSink := NewLokiSink(nil, 0)
	require.NotNil(t, lokiSink)

	info := logr.RuntimeInfo{
		CallDepth: 1,
	}

	lokiSink.Init(info)
	require.Equal(t, info, lokiSink.info)
}

func TestLokiSink_Enabled(t *testing.T) {
	t.Parallel()

	lokiSink := NewLokiSink(nil, 0)
	require.NotNil(t, lokiSink)

	enabled := lokiSink.Enabled(1)
	require.False(t, enabled)

	enabled = lokiSink.Enabled(0)
	require.True(t, enabled)
}

func TestLokiSink_WithValues(t *testing.T) {
	t.Parallel()

	lokiSink := NewLokiSink(nil, 0)
	require.NotNil(t, lokiSink)

	modifiedSink := lokiSink.WithValues("key", "value")

	modifiedLokiSink, ok := modifiedSink.(*LokiSink)
	require.True(t, ok)

	require.Len(t, modifiedLokiSink.labels, 1)
	require.Equal(t, "value", modifiedLokiSink.labels["key"])

	// Ensure the original sink is not modified.
	require.Empty(t, lokiSink.labels)
}

func TestLokiSink_WithName(t *testing.T) {
	t.Parallel()

	lokiSink := NewLokiSink(nil, 0)
	require.NotNil(t, lokiSink)

	modifiedSink := lokiSink.WithName("test")
	modifiedLokiSink, ok := modifiedSink.(*LokiSink)
	require.True(t, ok)

	require.Equal(t, "test", modifiedLokiSink.labels[NameKey])

	// Ensure the original sink is not modified.
	require.Empty(t, lokiSink.labels)

	twiceModifiedSink := modifiedSink.WithName("test2")
	twiceModifiedLokiSink, ok := twiceModifiedSink.(*LokiSink)
	require.True(t, ok)
	require.Equal(t, "test/test2", twiceModifiedLokiSink.labels[NameKey])

	// Ensure the original sinks are not modified.
	require.Empty(t, lokiSink.labels)
	require.Len(t, modifiedLokiSink.labels, 1)
	require.Equal(t, "test", modifiedLokiSink.labels[NameKey])
}

func TestLokiSink_WithCallDepth(t *testing.T) {
	t.Parallel()

	lokiSink := NewLokiSink(nil, 0)
	require.NotNil(t, lokiSink)

	modifiedSink := lokiSink.WithCallDepth(1)
	modifiedLokiSink, ok := modifiedSink.(*LokiSink)
	require.True(t, ok)

	require.Equal(t, 1, modifiedLokiSink.callDepth)

	// Ensure the original sink is not modified.
	require.Equal(t, 0, lokiSink.callDepth)
}
