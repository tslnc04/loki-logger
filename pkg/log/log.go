// Package log provides an [io.Writer] that writes to a Loki instance. It is intended for use as an output for
// [log.Logger].
//
// As an additional convenience, it also provides a [New] function that creates a new [log.Logger] with the given
// writers. This function is intended to be used as a drop-in replacement for [log.New] and supports using multiple
// writers.
package log

import (
	"context"
	"io"
	"log"
	"maps"
	"time"

	"github.com/tslnc04/loki-logger/pkg/client"
)

// New creates a new logger with the given prefix and flags, combining the writers using io.MultiWriter. Any writes will
// be be sequentially written to the writers, stopping at the first error.
func New(prefix string, flag int, writers ...io.Writer) *log.Logger {
	multiWriter := io.MultiWriter(writers...)
	logger := log.New(multiWriter, prefix, flag)

	return logger
}

// LokiWriter is a writer that sends log entries to a Loki instance. It implements the [io.Writer] interface. Writes are
// assumed to always be a full log line.
type LokiWriter struct {
	lokiClient         client.Client
	labels             client.LabelMap
	preformattedLabels client.LabelString
}

// Assert that LokiWriter implements the io.Writer interface.
var _ io.Writer = (*LokiWriter)(nil)

// NewLokiWriter creates a new LokiWriter with the given client and labels. Labels may be nil.
func NewLokiWriter(lokiClient client.Client, labels map[string]string) *LokiWriter {
	if labels == nil {
		labels = make(map[string]string)
	}

	return &LokiWriter{
		lokiClient:         lokiClient,
		labels:             labels,
		preformattedLabels: client.LabelMap(labels).Label(),
	}
}

// WithLabels returns a new LokiWriter with the labels added. Keys that already exist will be overwritten. Labels may be
// nil, in which case this is equivalent to calling Clone.
func (writer *LokiWriter) WithLabels(labels map[string]string) *LokiWriter {
	newWriter := writer.Clone()

	maps.Copy(newWriter.labels, labels)
	newWriter.preformattedLabels = newWriter.labels.Label()

	return newWriter
}

// Clone returns a copy of the LokiWriter, sharing only the Loki client. It is safe to call concurrently from multiple
// goroutines.
func (writer *LokiWriter) Clone() *LokiWriter {
	return &LokiWriter{
		lokiClient:         writer.lokiClient,
		labels:             maps.Clone(writer.labels),
		preformattedLabels: writer.preformattedLabels,
	}
}

// Write pushes a new log entry to the Loki instance. It first processes the message to remove any trailing newline
// characters. However, to uphold the requirements of io.Writer, it does not modify the message and returns the original
// length before processing.
//
// It is safe to call Write concurrently from multiple goroutines.
func (writer *LokiWriter) Write(message []byte) (int, error) {
	originalLen := len(message)

	for i := originalLen - 1; i >= 0; i-- {
		if message[i] != '\n' && message[i] != '\r' {
			break
		}

		message = message[:i]
	}

	line := string(message)

	entry := client.Entry{
		Timestamp:          time.Now(),
		Labels:             writer.preformattedLabels,
		Line:               line,
		StructuredMetadata: nil,
	}

	err := writer.lokiClient.Push(context.Background(), entry)
	if err != nil {
		return 0, err
	}

	return originalLen, nil
}
