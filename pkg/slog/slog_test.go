package slog

import (
	"log/slog"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tslnc04/loki-logger/pkg/client"
	"github.com/tslnc04/loki-logger/pkg/internal/fake"
)

var (
	// currentPackage is the package name of the current file.
	currentPackage = "github.com/tslnc04/loki-logger/pkg/slog"
	// currentFile is the file name of the current file.
	_, currentFile, _, _ = runtime.Caller(0)
)

func TestNewLogger(t *testing.T) {
	t.Parallel()

	logger := NewLogger(nil, nil)
	require.Equal(t, slog.New(NewHandler(nil, nil)), logger)
}

func TestNewHandler(t *testing.T) {
	t.Parallel()

	options := &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	}

	handler := NewHandler(nil, options)
	require.NotNil(t, handler)
	require.Nil(t, handler.client)
	require.Equal(t, *options, handler.options)
	require.Empty(t, handler.labels)
	require.Empty(t, handler.groups)
}

func TestHandler_Enabled(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		level    slog.Level
		expected bool
	}{
		{
			name:     "enabled",
			level:    slog.LevelInfo,
			expected: true,
		},
		{
			name:     "disabled",
			level:    slog.LevelDebug,
			expected: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			handler := NewHandler(nil, &slog.HandlerOptions{
				Level: slog.LevelInfo,
			})
			enabled := handler.Enabled(t.Context(), testCase.level)

			require.Equal(t, testCase.expected, enabled)
		})
	}
}

//nolint:funlen // This function is long because it tests multiple cases, so not a code quality issue.
func TestHandlerLogging(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name            string
		level           slog.Level
		expected        client.Entry
		generateHandler func(lokiClient client.Client) slog.Handler
	}{
		{
			name:  "basic",
			level: slog.LevelInfo,
			expected: client.Entry{
				Timestamp:          time.Now(),
				Labels:             client.LabelMap{slog.LevelKey: slog.LevelInfo.String()},
				Line:               "test",
				StructuredMetadata: map[string]string{"attrKey": "attrValue"},
			},
			generateHandler: func(lokiClient client.Client) slog.Handler {
				return NewHandler(lokiClient, &slog.HandlerOptions{
					Level: slog.LevelInfo,
				})
			},
		},
		{
			name:  "with-attr",
			level: slog.LevelInfo,
			expected: client.Entry{
				Timestamp:          time.Now(),
				Labels:             client.LabelMap{slog.LevelKey: slog.LevelInfo.String(), "testKey": "testValue"},
				Line:               "test",
				StructuredMetadata: map[string]string{"attrKey": "attrValue"},
			},
			generateHandler: func(lokiClient client.Client) slog.Handler {
				return NewHandler(lokiClient, &slog.HandlerOptions{Level: slog.LevelInfo}).
					WithAttrs([]slog.Attr{slog.String("testKey", "testValue")})
			},
		},
		{
			name:  "with-group",
			level: slog.LevelInfo,
			expected: client.Entry{
				Timestamp:          time.Now(),
				Labels:             client.LabelMap{slog.LevelKey: slog.LevelInfo.String()},
				Line:               "test",
				StructuredMetadata: map[string]string{"testGroup_attrKey": "attrValue"},
			},
			generateHandler: func(lokiClient client.Client) slog.Handler {
				return NewHandler(lokiClient, &slog.HandlerOptions{Level: slog.LevelInfo}).
					WithGroup("testGroup")
			},
		},
		{
			name:  "with-group-and-attr",
			level: slog.LevelInfo,
			expected: client.Entry{
				Timestamp:          time.Now(),
				Labels:             client.LabelMap{slog.LevelKey: slog.LevelInfo.String(), "testGroup_testKey": "testValue"},
				Line:               "test",
				StructuredMetadata: map[string]string{"testGroup_attrKey": "attrValue"},
			},
			generateHandler: func(lokiClient client.Client) slog.Handler {
				return NewHandler(lokiClient, &slog.HandlerOptions{Level: slog.LevelInfo}).
					WithGroup("testGroup").
					WithAttrs([]slog.Attr{slog.String("testKey", "testValue")})
			},
		},
		{
			name:  "with-source",
			level: slog.LevelInfo,
			expected: client.Entry{
				Timestamp: time.Now(),
				Labels:    client.LabelMap{slog.LevelKey: slog.LevelInfo.String()},
				Line:      "test",
				StructuredMetadata: map[string]string{
					"attrKey":                    "attrValue",
					slog.SourceKey + "_file":     currentFile,
					slog.SourceKey + "_function": currentPackage + ".TestHandlerLogging.func7",
					slog.SourceKey + "_line":     "189",
				},
			},
			generateHandler: func(lokiClient client.Client) slog.Handler {
				return NewHandler(lokiClient, &slog.HandlerOptions{
					Level:     slog.LevelInfo,
					AddSource: true,
				})
			},
		},
		{
			name:     "not-enabled",
			level:    slog.LevelDebug,
			expected: client.Entry{},
			generateHandler: func(lokiClient client.Client) slog.Handler {
				return NewHandler(lokiClient, &slog.HandlerOptions{Level: slog.LevelInfo})
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			fakeServer := fake.NewServer(0)
			httpServer := fakeServer.Start()

			defer httpServer.Close()

			lokiClient := client.NewLokiClient(httpServer.URL + client.PushPath)
			logger := slog.New(testCase.generateHandler(lokiClient))

			logger.LogAttrs(t.Context(), testCase.level, "test", slog.String("attrKey", "attrValue"))

			streams := fakeServer.Streams()
			defer fakeServer.Close()

			if testCase.name == "not-enabled" {
				require.Empty(t, streams, "Expected no streams to be sent")

				return
			}

			require.Len(t, streams, 1, "Expected number of streams to match")
			client.AssertStreamMatchesEntry(t, testCase.expected, streams[0])
		})
	}
}
