package client

import (
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/stretchr/testify/require"
)

// AssertStreamMatchesEntry asserts that the given stream matches the given entry. It checks that the stream has exactly
// one entry, that the labels match (after converting expected to string), and that the line matches. It also checks
// that the timestamps are within 1 second of each other. If the expected entry has a zero timestamp, it uses
// time.Now() as the expected timestamp.
func AssertStreamMatchesEntry(t *testing.T, expected Entry, actual push.Stream) {
	t.Helper()

	expectedTimestamp := expected.Timestamp
	if expectedTimestamp.IsZero() {
		expectedTimestamp = time.Now()
	}

	require.Len(t, actual.Entries, 1, "expected exactly one entry in the stream")
	require.Equal(t, string(expected.Labels.Label()), actual.Labels, "expected labels to match")
	require.Equal(t, expected.Line, actual.Entries[0].Line, "expected lines to match")
	require.WithinDuration(t, expectedTimestamp, actual.Entries[0].Timestamp, time.Second,
		"expected timestamps to be within 1 second of each other")
	require.Equal(t, expected.StructuredMetadata, convertLabelsAdapterToMap(actual.Entries[0].StructuredMetadata),
		"expected structured metadata to match")
}

func convertLabelsAdapterToMap(labelsAdapter push.LabelsAdapter) map[string]string {
	if len(labelsAdapter) == 0 {
		return nil
	}

	labels := make(map[string]string, len(labelsAdapter))
	for _, label := range labelsAdapter {
		labels[label.Name] = label.Value
	}

	return labels
}
