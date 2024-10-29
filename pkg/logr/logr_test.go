package logr

import (
	"testing"

	"github.com/tslnc04/loki-logger/pkg/client"
	"github.com/tslnc04/loki-logger/pkg/client/fake"
)

const (
	defaultMessage = "Hello, world!"
	maxLevel       = 10
)

func TestInfoVerbosityLevels(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		level    int
		expected []client.Entry
	}{
		{
			name:     "LogHigherThanLoggerVerbosity",
			level:    1,
			expected: []client.Entry{},
		},
		{
			name:  "LogSameAsLoggerVerbosity",
			level: 0,
			expected: []client.Entry{{
				Labels: client.LabelMap{
					LevelKey: "0",
				},
				Line: defaultMessage,
				StructuredMetadata: map[string]string{
					SourceKey + "_function": "TestInfoVerbosityLevels.func1",
					SourceKey + "_file":     "logr_test.go",
					SourceKey + "_line":     "52",
				},
			}},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			lokiClient := fake.New()
			logger := New(lokiClient, 0)

			logger.V(testCase.level).Info(defaultMessage)

			lokiClient.AssertEntries(t, testCase.expected)
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
			SourceKey + "_function": "TestErrorVerbosityLevels.func1",
			SourceKey + "_file":     "logr_test.go",
			SourceKey + "_line":     "99",
		},
	}

	testCases := []struct {
		name     string
		level    int
		expected []client.Entry
	}{
		{
			name:     "LogHigherThanLoggerVerbosity",
			level:    1,
			expected: []client.Entry{expectedEntry},
		},
		{
			name:     "LogSameAsLoggerVerbosity",
			level:    0,
			expected: []client.Entry{expectedEntry},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			lokiClient := fake.New()
			logger := New(lokiClient, 0)

			logger.V(testCase.level).Error(nil, defaultMessage)

			lokiClient.AssertEntries(t, testCase.expected)
		})
	}
}
