package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/chrishenyard/go-web-api/config"
	httpHandler "github.com/chrishenyard/go-web-api/handlers"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	cfg, err := config.NewConfig()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	if err := run(cfg); err != nil {
		slog.Error("Failed to run application", "error", err)
		os.Exit(1)
	}
}

func run(cfg *config.Config) error {
	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	ctx, cancel := context.WithTimeout(sigCtx, 180*time.Second)
	defer cancel()

	lp, tp, err := initTelemetry(ctx, cfg)
	if err != nil {
		slog.Error("Failed to init telemetry", "error", err)
		os.Exit(1)
	}
	defer func() {
		_ = lp.Shutdown(ctx)
		_ = tp.Shutdown(ctx)
	}()

	logger := slog.New(otelslog.NewHandler(cfg.ServiceName, otelslog.WithLoggerProvider(lp)))
	slog.SetDefault(logger)

	handler, err := httpHandler.NewHttpHandler(ctx, cfg)
	if err != nil {
		return fmt.Errorf("initialize HTTP handler: %w", err)
	}

	otelHandler := otelhttp.NewHandler(handler, cfg.ServiceName)

	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		Handler:           otelHandler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	srvErr := make(chan error, 1)
	go func() {
		slog.InfoContext(ctx, "Starting server on %s:%s", cfg.Host, cfg.Port)
		srvErr <- server.ListenAndServeTLS(cfg.CertFilePath, cfg.KeyFilePath)
	}()

	select {
	case err := <-srvErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-sigCtx.Done():
		stop()
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}

	return nil
}
