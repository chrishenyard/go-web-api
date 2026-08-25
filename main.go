package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/chrishenyard/go-web-api/config"
	httpHandler "github.com/chrishenyard/go-web-api/handlers"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatal(err.Error())
	}

	if err := run(cfg); err != nil {
		log.Fatal(err.Error())
	}
}

func run(cfg *config.Config) error {
	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	startupCtx, cancel := context.WithTimeout(sigCtx, 10*time.Second)
	defer cancel()

	shutdown, err := initTelemetry(startupCtx, cfg)
	if err != nil {
		log.Fatalf("Failed to init telemetry: %v", err)
		os.Exit(1)
	}
	defer func() {
		shutdown(startupCtx)
	}()

	exePath, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to get executable path: %v", err)
	}

	cwd := filepath.Dir(exePath)
	if err != nil {
		log.Fatalf("Failed to get current working directory: %v", err)
	}

	certFilePath := filepath.Join(cwd, cfg.CertFilePath)
	keyFilePath := filepath.Join(cwd, cfg.KeyFilePath)

	handler, err := httpHandler.NewHttpHandler(startupCtx, cfg)
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
		log.Println("Running HTTP server...")
		srvErr <- server.ListenAndServeTLS(certFilePath, keyFilePath)
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
