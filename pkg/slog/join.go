package slog

import (
	"context"
	"log/slog"
	"sync"
)

// JoinedHandler is a [slog.Handler] that wraps multiple other handlers and sends logs to all of them. Optionally, it
// can send logs to all handlers concurrently. If concurrency is left disabled, the order that logs are sent to the
// handlers is guaranteed to be the same as the order they are added to the JoinedHandler.
type JoinedHandler struct {
	handlers   []slog.Handler
	concurrent bool
}

// Assert that JoinedHandler implements the [slog.Handler] interface.
var _ slog.Handler = (*JoinedHandler)(nil)

// NewJoinedHandler creates a new JoinedHandler with the given handlers.
func NewJoinedHandler(handlers ...slog.Handler) *JoinedHandler {
	return &JoinedHandler{
		handlers: handlers,
	}
}

// NewJoinedLogger creates a new slog.Logger with the given JoinedHandler constructed with the given handlers. It is
// equivalent to
//
//	slog.New(NewJoinedHandler(handlers...))
func NewJoinedLogger(handlers ...slog.Handler) *slog.Logger {
	return slog.New(NewJoinedHandler(handlers...))
}

// WithConcurrency allows setting whether the JoinedHandler should send logs to all handlers concurrently. By default,
// it is false. Note that if this option is set to true, the order that logs are sent to the handlers is not guaranteed.
func (handler *JoinedHandler) WithConcurrency(concurrent bool) *JoinedHandler {
	handler.concurrent = concurrent

	return handler
}

// WithHandlers allows adding more handlers to the JoinedHandler after it has been created. If concurrency is disabled,
// these handlers are guaranteed to be sent after all existing handlers and in the order they are added.
func (handler *JoinedHandler) WithHandlers(handlers ...slog.Handler) *JoinedHandler {
	handler.handlers = append(handler.handlers, handlers...)

	return handler
}

// Enabled returns true if any of the handlers in the JoinedHandler are enabled for the given level. Since Enabled is
// meant to be run on every log, Enabled functions should be fast and therefore this method is not affected by the
// concurrency option. It is safe to use from multiple goroutines.
func (handler *JoinedHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range handler.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}

	return false
}

// Handle sends the given Record to all of the handlers in the JoinedHandler. For safety, it clones the Record before
// passing it on to all handlers. It is safe to use from multiple goroutines.
//
// If concurrency is disabled, the sending of logs short-circuits after the first error is encountered. If concurrency
// is enabled, the sending of logs continues until all handlers have been sent, although an error will be returned if
// any handler returns an error.
func (handler *JoinedHandler) Handle(ctx context.Context, record slog.Record) error {
	if handler.concurrent {
		return handler.handleConcurrent(ctx, record)
	}

	for _, h := range handler.handlers {
		err := h.Handle(ctx, record)
		if err != nil {
			return err
		}
	}

	return nil
}

// WithAttrs returns a new JoinedHandler with the given attributes appended to the existing ones for all handlers. Since
// ownership of the attrs is passed to each handler, the slice of attrs is cloned for each handler. Attrs will still
// share any state they hold since it is a shallow copy. Be careful.
//
// It is safe to use from multiple goroutines.
func (handler *JoinedHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandlers := make([]slog.Handler, 0, len(handler.handlers))

	for _, h := range handler.handlers {
		newAttrs := make([]slog.Attr, 0, len(attrs))
		newAttrs = append(newAttrs, attrs...)

		newHandlers = append(newHandlers, h.WithAttrs(newAttrs))
	}

	return &JoinedHandler{
		handlers: newHandlers,
	}
}

// WithGroup returns a new JoinedHandler with the given group name appended to the existing ones for all handlers. It is
// safe to use from multiple goroutines.
func (handler *JoinedHandler) WithGroup(name string) slog.Handler {
	newHandlers := make([]slog.Handler, 0, len(handler.handlers))

	for _, h := range handler.handlers {
		newHandlers = append(newHandlers, h.WithGroup(name))
	}

	return &JoinedHandler{
		handlers: newHandlers,
	}
}

func (handler *JoinedHandler) handleConcurrent(ctx context.Context, record slog.Record) error {
	var wg sync.WaitGroup

	wg.Add(len(handler.handlers))

	errChan := make(chan error, len(handler.handlers))

	for _, joinee := range handler.handlers {
		go func(joinee slog.Handler) {
			defer wg.Done()

			err := joinee.Handle(ctx, record.Clone())
			if err != nil {
				errChan <- err
			}
		}(joinee)
	}

	wg.Wait()
	close(errChan)

	return <-errChan
}
