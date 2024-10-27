// Package logr provides a logr.LogSink implementation that sends log entries to a Loki instance.
package logr

import (
	"fmt"
	"maps"
	"runtime"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"github.com/tslnc04/loki-logger/pkg/client"
)

const (
	// ErrorKey is the key added to the structured metadata when an error is logged. Its value is the stringified
	// error.
	ErrorKey = "error"
	// LevelKey is the key added to the stream labels for a log line. For errors, it will be -1.
	LevelKey = "level"
	// NameKey is the key added to the stream labels for a log line. Its value is the names of the
	// logger joined by "/".
	NameKey = "name"
	// SourceKey is the prefix for the keys added to the structured metadata when a log line is logged. The actual
	// keys used are SourceKey+"_function", SourceKey+"_file", and SourceKey+"_line".
	SourceKey = "source"
)

// New creates a new [logr.Logger] with the given client and level. Optionally, it can be configured with the given
// level. If multiple levels are provided, the sink will log only messages less than or equal to the first level
// provided. It is safe to call concurrently from multiple goroutines, even if the client is shared.
//
// For more control over the sink, use [NewLokiSink] instead and create a [logr.Logger] with that sink.
//
//	sink := NewLokiSink(lokiClient, level)
//	// ...configure sink...
//	logger := logr.New(sink)
func New(lokiClient client.Client, level ...int) *logr.Logger {
	sink := NewLokiSink(lokiClient, level...)
	logger := logr.New(sink)

	return &logger
}

// LokiSink is a [logr.LogSink] that sends log entries to a Loki instance. Any keys and values added to the
// [logr.Logger] (and thus this sink) will be added as stream labels. Any keys and values set when calling a logging
// function will be added as structured metadata.
type LokiSink struct {
	lokiClient client.Client
	info       logr.RuntimeInfo
	callDepth  int
	level      int
	// labels is a map of labels to add to each log entry. It should never be nil.
	labels map[string]string
}

// NewLokiSink creates a new LokiSink with the given client. Optionally, it can be configured with the given level. If
// multiple levels are provided, the sink will log only messages less than or equal to the first level provided. It is
// safe to call concurrently from multiple goroutines, even if the client is shared.
func NewLokiSink(lokiClient client.Client, level ...int) *LokiSink {
	logLevel := 0
	if len(level) > 0 {
		logLevel = level[0]
	}

	return &LokiSink{
		lokiClient: lokiClient,
		labels:     make(map[string]string),
		level:      logLevel,
	}
}

// WithLevel returns a new LokiSink with the given level. It is safe to call concurrently from multiple goroutines.
func (sink *LokiSink) WithLevel(level int) *LokiSink {
	newSink := sink.Clone()
	newSink.level = level

	return newSink
}

// Clone returns a copy of the sink. Only the client is shared. It is safe to call concurrently from multiple
// goroutines.
func (sink *LokiSink) Clone() *LokiSink {
	newSink := &LokiSink{
		lokiClient: sink.lokiClient,
		info:       sink.info,
		callDepth:  sink.callDepth,
		level:      sink.level,
		labels:     maps.Clone(sink.labels),
	}

	return newSink
}

// Init allows the sink to be initialized with the given [logr.RuntimeInfo]. It modifies the sink in place.
func (sink *LokiSink) Init(info logr.RuntimeInfo) {
	sink.info = info
}

// Enabled reports whether the sink is enabled for the given level, i.e. whether the provided level is less than or
// equal to the sink's level. It is safe to call concurrently from multiple goroutines.
func (sink *LokiSink) Enabled(level int) bool {
	return level <= sink.level
}

// Info logs the message with the provided level. It adds the level to the stream labels and the keys and values to the
// structured metadata. It is safe to call concurrently from multiple goroutines.
func (sink *LokiSink) Info(level int, msg string, keysAndValues ...any) {
	entry := sink.createEntry(level, msg, keysAndValues)
	_ = sink.lokiClient.Push(entry)
}

// Error logs the message with the provided error and level. It adds the level set to -1 to the stream labels and the
// keys and values to the structured metadata. It is safe to call concurrently from multiple goroutines.
func (sink *LokiSink) Error(err error, msg string, keysAndValues ...any) {
	keysAndValues = append(keysAndValues, ErrorKey, err)
	entry := sink.createEntry(-1, msg, keysAndValues)
	_ = sink.lokiClient.Push(entry)
}

// WithValues returns a new LokiSink with the given keys and values added to the stream labels. If there are an odd
// number of keys and values, the last value is ignored. It is safe to call concurrently from multiple goroutines.
//
//nolint:ireturn
func (sink *LokiSink) WithValues(keysAndValues ...any) logr.LogSink {
	newSink := sink.Clone()

	if len(keysAndValues)%2 != 0 {
		keysAndValues = keysAndValues[:len(keysAndValues)-1]
	}

	for i := 0; i < len(keysAndValues); i += 2 {
		key := fmt.Sprint(keysAndValues[i])
		value := fmt.Sprint(keysAndValues[i+1])
		newSink.labels[key] = value
	}

	return newSink
}

// WithName returns a new LokiSink with the given name joined to the existing names in the stream labels by a `/`. It is
// safe to call concurrently from multiple goroutines.
//
//nolint:ireturn
func (sink *LokiSink) WithName(name string) logr.LogSink {
	newSink := sink.Clone()

	if names, ok := newSink.labels[NameKey]; ok {
		newSink.labels[NameKey] = fmt.Sprintf("%s/%s", names, name)
	} else {
		newSink.labels[NameKey] = name
	}

	return newSink
}

// WithCallDepth returns a new LokiSink with the given call depth offset as specified by the [logr.CallDepthLogSink]
// interface. It is safe to call concurrently from multiple goroutines.
//
//nolint:ireturn
func (sink *LokiSink) WithCallDepth(depth int) logr.LogSink {
	newSink := sink.Clone()

	if depth > 0 {
		newSink.callDepth += depth
	}

	return newSink
}

// createEntry creates a new [client.Entry] with the given level, message, and keys and values. It adds the level to the
// stream labels and the keys and values to the structured metadata. It also adds the source keys to the structured
// metadata. It is safe to call concurrently from multiple goroutines.
func (sink *LokiSink) createEntry(level int, msg string, keysAndValues []any) client.Entry {
	labels := maps.Clone(sink.labels)
	labels[LevelKey] = strconv.Itoa(level)

	var metadata map[string]string

	if len(keysAndValues) > 1 {
		metadata = make(map[string]string)
		addValuesToLabels(metadata, keysAndValues)
	}

	callDepth := sink.callDepth
	if callDepth == 0 {
		callDepth = sink.info.CallDepth
	}

	// account for this function being called from the actual log function
	callDepth++

	source := newSource(callDepth)
	if source != nil {
		source.addToLabels(metadata)
	}

	entry := client.Entry{
		Timestamp:          time.Now(),
		Labels:             client.LabelMap(sink.labels).Label(),
		Line:               msg,
		StructuredMetadata: metadata,
	}

	return entry
}

// source is a helper struct for getting the source code position of a log call and adding it to the structured
// metadata.
type source struct {
	function string
	file     string
	line     int
}

// newSource returns a new source from the given call depth. It already accounts for being called from within It is safe
// to call concurrently from multiple goroutines.
func newSource(skip int) *source {
	pc, _, _, ok := runtime.Caller(skip + 1)
	if !ok {
		return nil
	}

	frames := runtime.CallersFrames([]uintptr{pc})
	frame, _ := frames.Next()

	return &source{
		function: frame.Function,
		file:     frame.File,
		line:     frame.Line,
	}
}

// addToLabels adds the source to the labels, ignoring any values that are empty. It will modify the labels in place.
func (source *source) addToLabels(labels map[string]string) {
	if source.function != "" {
		labels[SourceKey+"_function"] = source.function
	}

	if source.file != "" {
		labels[SourceKey+"_file"] = source.file
	}

	if source.line != 0 {
		labels[SourceKey+"_line"] = strconv.Itoa(source.line)
	}
}

// addValuesToLabels modifies the labels in place by adding the keys and values. If there are an odd number of keys and
// values, the last value is ignored. It uses fmt.Sprint to convert the keys and values to strings.
func addValuesToLabels(labels map[string]string, keysAndValues []any) {
	if len(keysAndValues)%2 != 0 {
		keysAndValues = keysAndValues[:len(keysAndValues)-1]
	}

	for i := 0; i < len(keysAndValues); i += 2 {
		key := fmt.Sprint(keysAndValues[i])
		value := fmt.Sprint(keysAndValues[i+1])
		labels[key] = value
	}
}
