# loki-logger

loki-logger is a simple library for sending logs to a Loki instance from Go. It currently intgrates with [log],
[log/slog], and [logr].

[log]: https://pkg.go.dev/log
[log/slog]: https://pkg.go.dev/log/slog
[logr]: https://pkg.go.dev/github.com/go-logr/logr

## Usage

```go
package main

import (
	"context"
	"log/slog"

	"github.com/tslnc04/loki-logger/pkg/client"
	lokislog "github.com/tslnc04/loki-logger/pkg/slog"
)

func main() {
	lokiClient := client.NewLokiClient("http://localhost:3100/loki/api/v1/push")
	logger := lokislog.NewLogger(lokiClient, &slog.HandlerOptions{AddSource: true})
	logger.LogAttrs(context.Background(), slog.LevelInfo, "Hello, world!", slog.Bool("test", true))
}
```

## Copyright

This repo is licensed under the MIT license. Copyright 2024 Kirsten Laskoski.

Some of the code is based heavily on Promtail, in particular the [labelsMapToString function]. The original code is licensed under the Apache 2.0 license.

Code has also been taken from the [slog package], which is licensed under the BSD 3-clause license. See the [LICENSE-go] file for details.

[labelsMapToString function]: https://github.com/grafana/loki/blob/main/clients/pkg/promtail/client/batch.go#L76
[slog package]: https://pkg.go.dev/log/slog
[LICENSE-go]: ./LICENSE-go
