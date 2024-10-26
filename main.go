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
