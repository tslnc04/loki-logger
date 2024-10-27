// Package client provides a client for pushing logs to a Loki instance.
package client

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/grafana/loki/pkg/push"
)

// PushPath is the path to the Loki push endpoint.
const PushPath = "/loki/api/v1/push"

const (
	// contentTypeProtobuf is the value of the Content-Type header for protobuf requests. It represents data
	// serialized as a protobuf and compressed using Snappy.
	contentTypeProtobuf = "application/x-protobuf"
	// userAgent is the value of the User-Agent header for requests to Loki. It is specific to this library.
	userAgent = "loki-logger/0.0"
)

// Labeler is an interface that abstracts the conversion of labels to a string for sending to Loki. For now, it is best
// to use the [LabelMap] type, which implements this interface.
type Labeler interface {
	// Label returns a single string that represents all the labels to add to the stream.
	//
	// # Format
	//
	// The labels follow the format `{key="value", key2="value2"}`. The values should be properly escaped as Go
	// strings, such as by [strconv.Quote]. The keys should also be sorted alphabetically.
	Label() LabelString
}

// LabelMap is a map of labels that can be converted to a string for sending to Loki. It implements the [Labeler]
// interface.
type LabelMap map[string]string

// Label returns the string representation of the LabelMap.
func (lm LabelMap) Label() LabelString {
	return LabelString(labelsToString(lm))
}

// LabelString is a string that contains labels already formatted as a string. It implements the [Labeler] interface.
type LabelString string

// Label returns the string representation of the LabelsString. It is effectively a no-op.
func (ls LabelString) Label() LabelString {
	return ls
}

// Entry is a struct that represents a single log entry to be sent to Loki. It contains the timestamp, labels, line, and
// structured metadata. It does not have any knowledge of streams.
type Entry struct {
	Timestamp          time.Time
	Labels             Labeler
	Line               string
	StructuredMetadata map[string]string
}

// AsPushRequest converts the Entry to a [push.PushRequest] that can be marshaled, compressed, and sent to Loki. This
// method does not modify the Entry.
func (entry *Entry) AsPushRequest() push.PushRequest {
	return push.PushRequest{
		Streams: []push.Stream{
			{
				Labels: string(entry.Labels.Label()),
				Entries: []push.Entry{{
					Timestamp:          entry.Timestamp,
					Line:               entry.Line,
					StructuredMetadata: metadataToLabelsAdapter(entry.StructuredMetadata),
				}},
			},
		},
	}
}

// Encode converts the Entry to a byte slice that can be sent to Loki. It first serializes the Entry to a protobuf and
// then encodes it using Snappy compression. This method does not modify the Entry.
func (entry *Entry) Encode() ([]byte, error) {
	pushRequest := entry.AsPushRequest()

	buf, err := proto.Marshal(&pushRequest)
	if err != nil {
		return nil, err
	}

	buf = snappy.Encode(nil, buf)

	return buf, nil
}

// Client is an interface that abstracts the sending of log entries to Loki. Each call to Push represents a single log
// entry being sent to Loki.
//
// Implementations of this interface should be safe to use concurrently.
type Client interface {
	Push(entry Entry) error
}

// LokiClient is a client for pushing log entries to a Loki instance. It implements the [Client] interface.
type LokiClient struct {
	url    string
	client *http.Client
}

// NewLokiClient creates a new LokiClient with the given URL.
func NewLokiClient(url string) *LokiClient {
	return &LokiClient{
		url:    url,
		client: &http.Client{},
	}
}

// Push implements the [Client] interface. It sends the given Entry to Loki.
func (client *LokiClient) Push(entry Entry) error {
	buf, err := entry.Encode()
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", client.url, bytes.NewReader(buf))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", contentTypeProtobuf)
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		return fmt.Errorf("push request failed with status code %d: %s", resp.StatusCode, body)
	}

	return nil
}

// labelsToString converts a map of labels to a string that can be added to a stream. It follows the format required by
// Loki and thus the [Labeler] interface. It does not modify the labels map.
func labelsToString(labels map[string]string) string {
	// This code is based heavily on the labelsMapToString function in the Promtail client, which is licensed under
	// the Apache 2.0 license.
	builder := strings.Builder{}
	totalSize := 2
	keys := make([]string, 0, len(labels))

	for key, value := range labels {
		keys = append(keys, key)
		// add 2 for `, ` between labels and 3 for `=` and quotes around the value
		totalSize += len(key) + 2 + len(value) + 3
	}

	builder.Grow(totalSize)
	builder.WriteByte('{')
	slices.Sort(keys)

	for i, key := range keys {
		if i > 0 {
			builder.WriteString(", ")
		}

		builder.WriteString(key)
		builder.WriteByte('=')
		builder.WriteString(strconv.Quote(labels[key]))
	}

	builder.WriteByte('}')

	return builder.String()
}

// metadataToLabelsAdapter converts the map of structured metadata to a slice of [push.LabelAdapter] that can be added
// to a stream. It does not modify the metadata map.
func metadataToLabelsAdapter(metadata map[string]string) push.LabelsAdapter {
	labels := make([]push.LabelAdapter, 0, len(metadata))

	for key, value := range metadata {
		labels = append(labels, push.LabelAdapter{
			Name:  key,
			Value: value,
		})
	}

	return labels
}
