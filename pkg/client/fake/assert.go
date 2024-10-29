package fake

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tslnc04/loki-logger/pkg/client"
)

// AssertEntries asserts that the Client has the given entries. It handles the locking of the Client for reading and
// unlocking it after the assertion. It is safe to call concurrently from multiple goroutines.
//
// The entries are compared so that the timestamps are within a second of this function being called and all other
// fields are equal.
//
// Like the testify/assert package, this function returns true if the assertion is successful.
func (client *Client) AssertEntries(t *testing.T, expected []client.Entry) bool {
	t.Helper()

	actual := client.Entries()
	defer client.Close()

	if !assert.Len(t, actual, len(expected)) {
		return false
	}

	ok := true
	for i := range expected {
		ok = ok && assertEntry(t, expected[i], actual[i])
	}

	return ok
}

// assertEntry asserts that the actual Entry is equal to the expected Entry. It does this by asserting that the
// timestamp is within a second of the expected time or current time if the expected time is zero. Additionally, it
// asserts that all other fields are equal.
//
// Like the testify/assert package, this function returns true if the assertion is successful.
func assertEntry(t *testing.T, expected client.Entry, actual client.Entry) bool {
	t.Helper()

	if expected.Timestamp.IsZero() {
		expected.Timestamp = time.Now()
	}

	//nolint:varnamelen
	ok := true
	ok = ok && assert.WithinDuration(t, expected.Timestamp, actual.Timestamp, time.Second)
	ok = ok && assert.Equal(t, expected.Labels.Label(), actual.Labels.Label())
	ok = ok && assert.Equal(t, expected.Line, actual.Line)
	ok = ok && assertMetadata(t, expected.StructuredMetadata, actual.StructuredMetadata)

	return ok
}

func assertMetadata(t *testing.T, expected map[string]string, actual map[string]string) bool {
	t.Helper()

	if !assert.Len(t, actual, len(expected)) {
		return false
	}

	for key, expectedValue := range expected {
		actualValue, ok := actual[key]
		if !assert.Truef(t, ok, "expected key `%s` to exist", key) {
			return false
		}

		if strings.HasPrefix(key, "source_") {
			if !assert.Conditionf(t, func() bool {
				return strings.HasSuffix(actualValue, expectedValue)
			}, "expected source key `%s` with value `%s` to end with `%s`", key, actualValue, expectedValue) {
				return false
			}

			continue
		}

		if assert.Equal(t, expectedValue, actualValue) {
			return false
		}
	}

	return true
}
