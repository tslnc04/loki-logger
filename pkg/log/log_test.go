package log

import (
	"log"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tslnc04/loki-logger/pkg/client"
	"github.com/tslnc04/loki-logger/pkg/internal/fake"
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
		expected client.Entry
	}{
		{
			name:   "without-labels",
			labels: nil,
			flags:  0,
			prefix: "",
			expected: client.Entry{
				Labels: client.LabelMap(map[string]string{}).Label(),
				Line:   defaultMessage,
			},
		},
		{
			name: "with-labels",
			labels: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			flags:  0,
			prefix: "",
			expected: client.Entry{
				Labels: client.LabelMap(map[string]string{
					"key1": "value1",
					"key2": "value2",
				}).Label(),
				Line: defaultMessage,
			},
		},
		{
			name:   "with-flags",
			labels: nil,
			flags:  log.Lshortfile,
			prefix: "",
			expected: client.Entry{
				Labels: client.LabelMap(map[string]string{}).Label(),
				Line:   "log_test.go:88: " + defaultMessage,
			},
		},
		{
			name:   "with-prefix",
			labels: nil,
			flags:  0,
			prefix: "prefix: ",
			expected: client.Entry{
				Labels: client.LabelMap(map[string]string{}).Label(),
				Line:   "prefix: " + defaultMessage,
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
			writer := NewLokiWriter(lokiClient, nil).WithLabels(testCase.labels)
			logger := New(testCase.prefix, testCase.flags, writer)

			logger.Print(defaultMessage)

			streams := fakeServer.Streams()
			defer fakeServer.Close()

			require.Len(t, streams, 1, "Expected one push stream to be sent to the server")
			client.AssertStreamMatchesEntry(t, testCase.expected, streams[0])
		})
	}
}
