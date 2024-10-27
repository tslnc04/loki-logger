package log

import (
	"log"
	"testing"

	"github.com/tslnc04/loki-logger/pkg/client"
	"github.com/tslnc04/loki-logger/pkg/client/fake"
)

const (
	defaultMessage = "Hello, world!"
)

//nolint:funlen
func TestLogging(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		labels   map[string]string
		flags    int
		prefix   string
		expected []client.Entry
	}{
		{
			name:   "WithoutLabels",
			labels: nil,
			flags:  0,
			prefix: "",
			expected: []client.Entry{{
				Labels: client.LabelMap(map[string]string{}).Label(),
				Line:   defaultMessage,
			}},
		},
		{
			name: "WithLabels",
			labels: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			flags:  0,
			prefix: "",
			expected: []client.Entry{{
				Labels: client.LabelMap(map[string]string{
					"key1": "value1",
					"key2": "value2",
				}).Label(),
				Line: defaultMessage,
			}},
		},
		{
			name:   "WithFlags",
			labels: nil,
			flags:  log.Lshortfile,
			prefix: "",
			expected: []client.Entry{{
				Labels: client.LabelMap(map[string]string{}).Label(),
				Line:   "log_test.go:82: " + defaultMessage,
			}},
		},
		{
			name:   "WithPrefix",
			labels: nil,
			flags:  0,
			prefix: "prefix: ",
			expected: []client.Entry{{
				Labels: client.LabelMap(map[string]string{}).Label(),
				Line:   "prefix: " + defaultMessage,
			}},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			lokiClient := fake.New()
			writer := NewLokiWriter(lokiClient, testCase.labels)
			logger := New(testCase.prefix, testCase.flags, writer)

			logger.Print(defaultMessage)

			lokiClient.AssertEntries(t, testCase.expected)
		})
	}
}
