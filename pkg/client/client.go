// Package client provides a client for pushing logs to a Loki instance.
package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// PushPath is the path to the Loki push endpoint. It is not appended to the URL automatically, but left as a constant
// for the caller to use if needed.
const PushPath = "/loki/api/v1/push"

const (
	// contentTypeProtobuf is the value of the Content-Type header for protobuf requests. It represents data
	// serialized as a protobuf and compressed using Snappy.
	contentTypeProtobuf = "application/x-protobuf"
	// userAgent is the value of the User-Agent header for requests to Loki. It is specific to this library.
	userAgent = "loki-logger/0.0"
)

// Client is an interface that abstracts the sending of log entries to Loki. Each call to Push represents a single log
// entry being sent to Loki.
//
// Implementations of this interface should be safe to use concurrently.
type Client interface {
	Push(ctx context.Context, entry Entry) error
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

// WithHTTPClient sets the HTTP client to use for the LokiClient. It is safe to call concurrently from multiple
// goroutines as it returns a new LokiClient struct.
func (client *LokiClient) WithHTTPClient(httpClient *http.Client) *LokiClient {
	return &LokiClient{
		url:    client.url,
		client: httpClient,
	}
}

// Assert that LokiClient implements the Client interface.
var _ Client = (*LokiClient)(nil)

// Push implements the [Client] interface. It sends the given Entry to Loki.
func (client *LokiClient) Push(ctx context.Context, entry Entry) error {
	buf, err := entry.Encode()
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", client.url, bytes.NewReader(buf))
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
			return errors.Join(&PushStatusError{
				StatusCode: resp.StatusCode,
				Status:     resp.Status,
			}, fmt.Errorf("failed to read response body: %w", err))
		}

		return &PushStatusError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       body,
		}
	}

	return nil
}

// PushStatusError is an error that represents a failed push request to Loki. It contains the status code, status
// message, and body of the response. It implements the [error] interface.
type PushStatusError struct {
	// StatusCode is the status code of the response.
	StatusCode int
	// Status is the status message of the response.
	Status string
	// Body is the body of the response.
	Body []byte
}

var _ error = (*PushStatusError)(nil)

func (e *PushStatusError) Error() string {
	return fmt.Sprintf("push request failed with status %s: %s", e.Status, e.Body)
}

// Is checks if the target error is a PushStatusError. It is used internally by [errors.Is].
func (e *PushStatusError) Is(target error) bool {
	if target == nil {
		return false
	}

	_, ok := target.(*PushStatusError)

	return ok
}
