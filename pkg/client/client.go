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

const (
	contentTypeProtobuf = "application/x-protobuf"
	userAgent           = "loki-logger/0.0"
)

type Labeler interface {
	Label() string
}

type LabelMap map[string]string

func (lm LabelMap) Label() string {
	return labelsToString(lm)
}

type Entry struct {
	Timestamp          time.Time
	Labels             Labeler
	Line               string
	StructuredMetadata map[string]string
}

func (entry *Entry) AsPushRequest() push.PushRequest {
	return push.PushRequest{
		Streams: []push.Stream{
			{
				Labels: entry.Labels.Label(),
				Entries: []push.Entry{{
					Timestamp:          entry.Timestamp,
					Line:               entry.Line,
					StructuredMetadata: metadataToLabelsAdapter(entry.StructuredMetadata),
				}},
			},
		},
	}
}

func (entry *Entry) Encode() ([]byte, error) {
	pushRequest := entry.AsPushRequest()

	buf, err := proto.Marshal(&pushRequest)
	if err != nil {
		return nil, err
	}

	buf = snappy.Encode(nil, buf)

	return buf, nil
}

type Client interface {
	Push(entry Entry) error
}

type lokiClient struct {
	url    string
	client *http.Client
}

func NewLokiClient(url string) *lokiClient {
	return &lokiClient{
		url:    url,
		client: &http.Client{},
	}
}

func (client *lokiClient) Push(entry Entry) error {
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
