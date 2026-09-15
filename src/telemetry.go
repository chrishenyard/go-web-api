package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"google.golang.org/grpc/grpclog"
)

const (
	otelExportTimeout = 2 * time.Second
	metricInterval    = 5 * time.Second
)

type multiHandler struct {
	handlers []slog.Handler
}

func (h *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *multiHandler) Handle(ctx context.Context, record slog.Record) error {
	for _, handler := range h.handlers {
		if !handler.Enabled(ctx, record.Level) {
			continue
		}

		if err := handler.Handle(ctx, record.Clone()); err != nil {
			return err
		}
	}
	return nil
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, 0, len(h.handlers))
	for _, handler := range h.handlers {
		handlers = append(handlers, handler.WithAttrs(attrs))
	}
	return &multiHandler{handlers: handlers}
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, 0, len(h.handlers))
	for _, handler := range h.handlers {
		handlers = append(handlers, handler.WithGroup(name))
	}
	return &multiHandler{handlers: handlers}
}

// initConsoleLogger sets up local diagnostics independent from OpenTelemetry.
// If the collector is unavailable, export errors are still visible locally and
// do not get sent back through a failing OTLP logging pipeline.
func initConsoleLogger() *slog.Logger {
	consoleHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.GetLogLevel(),
	})
	consoleLogger := slog.New(consoleHandler)

	// grpc-go can emit noisy connection warnings directly to stderr while the
	// collector is unavailable. Suppress informational/debug output here; OTEL
	// exporter failures are reported through the error handler below.
	grpclog.SetLoggerV2(grpclog.NewLoggerV2(io.Discard, io.Discard, os.Stderr))
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		consoleLogger.Warn("OpenTelemetry export failed", "error", err)
	}))

	return consoleLogger
}

// initLogs wires up the OTLP log pipeline and installs a slog default logger
// that fans out to both the console and OTLP. Local logging remains functional
// when the collector is stopped or unreachable.
func initLogs(ctx context.Context, res *resource.Resource, consoleLogger *slog.Logger) (*sdklog.LoggerProvider, error) {
	// Exporters use short timeouts so telemetry remains a best-effort dependency.
	// A missing collector must not interfere with serving HTTP requests.
	logExporter, err := otlploggrpc.New(ctx, otlploggrpc.WithTimeout(otelExportTimeout))
	if err != nil {
		return nil, fmt.Errorf("create OTLP log exporter: %w", err)
	}

	loggerProvider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
		sdklog.WithResource(res),
	)

	otelLogHandler := otelslog.NewHandler(cfg.ServiceName, otelslog.WithLoggerProvider(loggerProvider))

	slog.SetDefault(slog.New(&multiHandler{
		handlers: []slog.Handler{
			consoleLogger.Handler(),
			otelLogHandler,
		},
	}))

	return loggerProvider, nil
}

// initTraces wires up the OTLP trace pipeline and registers it as the global
// tracer provider.
func initTraces(ctx context.Context, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithTimeout(otelExportTimeout))
	if err != nil {
		return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
	}

	// Always batch traces. A synchronous exporter runs on the request path and
	// can delay HTTP completion when the collector is unavailable.
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(
			traceExporter,
			sdktrace.WithBatchTimeout(otelExportTimeout),
			sdktrace.WithExportTimeout(otelExportTimeout),
		),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tracerProvider)

	return tracerProvider, nil
}

// initMetrics wires up the OTLP metric pipeline and registers it as the global
// meter provider.
func initMetrics(ctx context.Context, res *resource.Resource) (*sdkmetric.MeterProvider, error) {
	metricExporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithTimeout(otelExportTimeout))
	if err != nil {
		return nil, fmt.Errorf("create OTLP metric exporter: %w", err)
	}

	metricReader := sdkmetric.NewPeriodicReader(
		metricExporter,
		sdkmetric.WithInterval(metricInterval),
		sdkmetric.WithTimeout(otelExportTimeout),
	)
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(metricReader),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(meterProvider)

	return meterProvider, nil
}

func initTelemetry(ctx context.Context) (func(context.Context), error) {
	consoleLogger := initConsoleLogger()

	res, err := resource.New(
		ctx,
		resource.WithAttributes(semconv.ServiceNameKey.String(cfg.ServiceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("create OpenTelemetry resource: %w", err)
	}

	loggerProvider, err := initLogs(ctx, res, consoleLogger)
	if err != nil {
		return nil, err
	}

	tracerProvider, err := initTraces(ctx, res)
	if err != nil {
		_ = loggerProvider.Shutdown(ctx)
		return nil, err
	}

	meterProvider, err := initMetrics(ctx, res)
	if err != nil {
		_ = tracerProvider.Shutdown(ctx)
		_ = loggerProvider.Shutdown(ctx)
		return nil, err
	}

	shutdown := func(shutdownCtx context.Context) {
		// Shutdown in reverse dependency order. Failures are logged locally and do
		// not prevent the remaining providers from being shut down.
		if err := meterProvider.Shutdown(shutdownCtx); err != nil {
			consoleLogger.Error("Failed to shut down meter provider", "error", err)
		}
		if err := tracerProvider.Shutdown(shutdownCtx); err != nil {
			consoleLogger.Error("Failed to shut down tracer provider", "error", err)
		}
		if err := loggerProvider.Shutdown(shutdownCtx); err != nil {
			consoleLogger.Error("Failed to shut down logger provider", "error", err)
		}
	}

	return shutdown, nil
}
