// Package slog implements a [slog.Handler] that sends logs to a Loki instance.
//
// The slog handler can be created directly using [NewHandler] or a [slog.Logger] can be created using [NewLogger].
// These functions take a [client.Client] and a [slog.HandlerOptions] as arguments. Note that the [slog.HandlerOptions]
// are used differently than in the slog package. See the documentation of [Handler] for more information.
//
// An important distinction when logging is that this Handler treats any attributes or groups added to the logger itself
// as labels for the stream in Loki. Attributes or groups included in the Record are treated as structured metadata.
//
// # JoinedHandler
//
// The [JoinedHandler] is a [slog.Handler] that wraps multiple other handlers and sends logs to all of them. This can be
// used to send logs both to Loki and to other handlers, although there is no dependency on the Loki client.
//
// If you need something more complex, another library such as [slog-multi] may be a better fit.
//
// [slog-multi]: https://pkg.go.dev/github.com/samber/slog-multi
package slog
