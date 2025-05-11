package slog

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
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
