package slog

import (
	"context"
	"log/slog"
	"maps"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/tslnc04/loki-logger/pkg/client"
)

// Handler implements the [slog.Handler] interface and sends logs to a Loki instance. It is best used to create a
// [slog.Logger].
//
// # Labels vs Metadata
//
// An important distinction when logging is that this Handler treats any attributes or groups added to the logger itself
// as labels for the stream in Loki. Attributes or groups included in the Record are treated as structured metadata.
//
// # Options
//
// The Handler uses [slog.HandlerOptions] with the AddSource and Level fields functioning identical to its
// documentation. It uses the ReplaceAttr field in a very similar way to the documentation, but the built-in fields
// attributes are different. Only level and source are supported as time and message are passed directly to loki without
// the ability to be replaced.
type Handler struct {
	client  client.Client
	options slog.HandlerOptions
	labels  map[string]string
	groups  []string
}

var _ slog.Handler = (*Handler)(nil)

// NewLogger creates a new slog.Logger with the Handler attached. It is equivalent to
//
//	slog.New(NewHandler(client, options))
func NewLogger(client client.Client, options *slog.HandlerOptions) *slog.Logger {
	return slog.New(NewHandler(client, options))
}

// NewHandler creates a new Handler with the given client and options. See the documentation of [Handler] for more
// information on how the options are used.
func NewHandler(client client.Client, options *slog.HandlerOptions) *Handler {
	if options == nil {
		options = &slog.HandlerOptions{}
	}

	return &Handler{
		client:  client,
		options: *options,
		labels:  make(map[string]string),
	}
}

// Enabled returns true if the Handler is enabled for the given level.
func (handler *Handler) Enabled(_ context.Context, level slog.Level) bool {
	if handler.options.Level == nil {
		return level >= slog.LevelInfo
	}

	return level >= handler.options.Level.Level()
}

// Handle converts the given Record to a format compatible with Loki and pushes it to the Loki instance via the provided
// client.
func (handler *Handler) Handle(ctx context.Context, record slog.Record) error {
	entry := handler.recordToEntry(record)

	return handler.client.Push(ctx, entry)
}

// WithAttrs returns a new Handler with the given attributes appended to the existing ones. These appear as stream
// labels in Loki.
func (handler *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandler := handler.clone()
	newState := newHandler.newMutatingHandleState()

	newState.appendAttrs(attrs)

	return newHandler
}

// WithGroup returns a new Handler with the given group name appended to the existing ones. These appear in Loki as
// prefixes of stream labels, separated by an underscore (`_`).
func (handler *Handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return handler
	}

	newHandler := handler.clone()
	newHandler.groups = append(newHandler.groups, name)

	return newHandler
}

// clone returns a copy of the Handler only sharing the client, although the client should be safe to use concurrently.
func (handler *Handler) clone() *Handler {
	newHandler := &Handler{
		client:  handler.client,
		options: handler.options,
		labels:  maps.Clone(handler.labels),
		groups:  slices.Clone(handler.groups),
	}

	return newHandler
}

// recordToEntry converts the given Record to the Entry used by the Loki client. This is what adds the built-in
// attributes.
func (handler *Handler) recordToEntry(record slog.Record) client.Entry {
	if record.Time.IsZero() {
		record.Time = time.Now()
	}

	labels := maps.Clone(handler.labels)
	state := handler.newHandleState(labels, nil)

	state.appendAttr(slog.Any(slog.LevelKey, record.Level))

	metadata := map[string]string{}
	state.attrMap = metadata

	if handler.options.AddSource {
		state.appendAttr(slog.Any(slog.SourceKey, newSource(&record)))
	}

	state.groups = slices.Clone(handler.groups)

	record.Attrs(func(attr slog.Attr) bool {
		state.appendAttr(attr)

		return true
	})

	return client.Entry{
		Timestamp:          record.Time,
		Labels:             client.LabelMap(labels),
		Line:               record.Message,
		StructuredMetadata: metadata,
	}
}

// handleState is used to hold the state necessary for adding attributes and groups to a handler. The handler is not
// mutated and only its options are read.
type handleState struct {
	handler *Handler
	attrMap map[string]string
	groups  []string
}

// newHandleState returns a new handleState with the given attributes and groups. These may be modified by the state
// when adding attributes and groups.
func (handler *Handler) newHandleState(attrMap map[string]string, groups []string) *handleState {
	return &handleState{
		handler: handler,
		attrMap: attrMap,
		groups:  groups,
	}
}

// newMutatingHandleState returns a new handleState with the same attributes and groups as the handler. Note that this
// cannot be used concurrently with a shared handler without additional synchronization.
func (handler *Handler) newMutatingHandleState() *handleState {
	return handler.newHandleState(handler.labels, handler.groups)
}

// appendAttrs appends the given attributes to the state. It is equivalent to looping over the attributes and calling
// appendAttr.
func (state *handleState) appendAttrs(attrs []slog.Attr) {
	for _, attr := range attrs {
		state.appendAttr(attr)
	}
}

// appendAttr appends the given attribute to the state. It is responsible for handling the ReplaceAttr option.
func (state *handleState) appendAttr(attr slog.Attr) {
	attr.Value = attr.Value.Resolve()

	if state.handler.options.ReplaceAttr != nil && attr.Value.Kind() != slog.KindGroup {
		attr = state.handler.options.ReplaceAttr(state.groups, attr)
		attr.Value = attr.Value.Resolve()
	}

	if attr.Equal(slog.Attr{}) {
		return
	}

	if attr.Value.Kind() == slog.KindAny {
		if source, ok := attr.Value.Any().(*slog.Source); ok {
			attr.Value = (*groupableSource)(source).group()
		}
	}

	if attr.Value.Kind() != slog.KindGroup {
		state.insertAttr(attr)

		return
	}

	if attr.Key != "" {
		state.groups = append(state.groups, attr.Key)
	}

	state.appendAttrs(attr.Value.Group())

	if attr.Key != "" {
		state.groups = state.groups[:len(state.groups)-1]
	}
}

// insertAttr appends the given attribute to the state. It assumes the attribute is not a group and has already been
// resolved. All it does is add the attribute to the map and formats the key.
func (state *handleState) insertAttr(attr slog.Attr) {
	var fullKey strings.Builder

	for _, group := range state.groups {
		fullKey.WriteString(group)
		fullKey.WriteByte('_')
	}

	fullKey.WriteString(attr.Key)

	state.attrMap[fullKey.String()] = attr.Value.String()
}

// groupableSource is a slog.Source that copies the private group method from the slog package. This allows converting
// the Source into a group for adding as an attribute.
type groupableSource slog.Source

// newSource returns a new Source from the given Record. It uses the runtime package to get the function, file, and line
// number of the log call.
func newSource(r *slog.Record) *slog.Source {
	// Function taken from log/slog/record.go. Copyright The Go Authors. License BSD-3-Clause.
	frames := runtime.CallersFrames([]uintptr{r.PC})
	frame, _ := frames.Next()

	return &slog.Source{
		Function: frame.Function,
		File:     frame.File,
		Line:     frame.Line,
	}
}

// group takes the source and returns a group of attributes that can be added to the handler. It includes the function,
// file, and line, if available.
func (source *groupableSource) group() slog.Value {
	// Function taken from log/slog/record.go. Copyright The Go Authors. License BSD-3-Clause.
	var attrs []slog.Attr

	if source.Function != "" {
		attrs = append(attrs, slog.String("function", source.Function))
	}

	if source.File != "" {
		attrs = append(attrs, slog.String("file", source.File))
	}

	if source.Line != 0 {
		attrs = append(attrs, slog.Int("line", source.Line))
	}

	return slog.GroupValue(attrs...)
}
