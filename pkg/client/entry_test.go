package client

import (
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/stretchr/testify/require"
)

var testTimestamp = time.Date(2025, 05, 27, 0, 0, 0, 0, time.UTC)

//nolint:funlen // Most of the function is test cases, no need to worry about length.
func TestEntry_AsPushRequest(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		entry    Entry
		expected push.PushRequest
	}{
		{
			name: "basic-entry",
			entry: Entry{
				Timestamp:          testTimestamp,
				Labels:             LabelMap{"foo": "bar"},
				Line:               "test message",
				StructuredMetadata: map[string]string{"key": "value"},
			},
			expected: push.PushRequest{
				Streams: []push.Stream{
					{
						Labels: `{foo="bar"}`,
						Entries: []push.Entry{
							{
								Timestamp:          testTimestamp,
								Line:               "test message",
								StructuredMetadata: []push.LabelAdapter{{Name: "key", Value: "value"}},
							},
						},
					},
				},
			},
		},
		{
			name:  "empty-entry",
			entry: Entry{},
			expected: push.PushRequest{
				Streams: []push.Stream{
					{
						Labels: `{}`,
						Entries: []push.Entry{
							{
								Timestamp:          time.Time{},
								Line:               "",
								StructuredMetadata: nil,
							},
						},
					},
				},
			},
		},
		{name: "no-structured-metadata",
			entry: Entry{
				Timestamp: testTimestamp,
				Labels:    LabelMap{"foo": "bar"},
				Line:      "test message",
			},
			expected: push.PushRequest{
				Streams: []push.Stream{
					{
						Labels: `{foo="bar"}`,
						Entries: []push.Entry{
							{
								Timestamp:          testTimestamp,
								Line:               "test message",
								StructuredMetadata: nil,
							},
						},
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			pushRequest := testCase.entry.AsPushRequest()
			require.Equal(t, testCase.expected, pushRequest)
		})
	}
}
