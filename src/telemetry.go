package main

import (
	"context"
	"fmt"
	"log/slog"
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
)

type levelFilterHandler struct {
	slog.Handler
	level slog.Level
}

func (h *levelFilterHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level
}

func initTelemetry(ctx context.Context) (func(context.Context), error) {
	res, err := resource.New(
		ctx,
		resource.WithAttributes(semconv.ServiceNameKey.String(cfg.ServiceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("create OpenTelemetry resource: %w", err)
	}

	// The OTLP gRPC exporters read the standard OTEL_EXPORTER_OTLP_* environment
	// variables. docker-compose.yml supplies the CA, client certificate, and
	// client private key required for mTLS.
	logExporter, err := otlploggrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("create OTLP log exporter: %w", err)
	}

	loggerProvider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
		sdklog.WithResource(res),
	)

	baseHandler := otelslog.NewHandler(
		cfg.ServiceName,
		otelslog.WithLoggerProvider(loggerProvider),
	)

	slog.SetDefault(slog.New(&levelFilterHandler{
		Handler: baseHandler,
		level:   cfg.GetLogLevel(),
	}))

	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		_ = loggerProvider.Shutdown(ctx)
		return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
	}

	var tracerProvider *sdktrace.TracerProvider
	if cfg.IsDevelopment() {
		slog.Info("Running in development mode, using synchronous trace exporter")
		tracerProvider = sdktrace.NewTracerProvider(
			sdktrace.WithSyncer(traceExporter),
			sdktrace.WithResource(res),
		)
	} else {
		tracerProvider = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(traceExporter),
			sdktrace.WithResource(res),
		)
	}
	otel.SetTracerProvider(tracerProvider)

	metricExporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		_ = tracerProvider.Shutdown(ctx)
		_ = loggerProvider.Shutdown(ctx)
		return nil, fmt.Errorf("create OTLP metric exporter: %w", err)
	}

	metricReader := sdkmetric.NewPeriodicReader(
		metricExporter,
		sdkmetric.WithInterval(5*time.Second),
	)
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(metricReader),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(meterProvider)

	shutdown := func(shutdownCtx context.Context) {
		if err := meterProvider.Shutdown(shutdownCtx); err != nil {
			slog.Error("Failed to shut down meter provider", "error", err)
		}
		if err := tracerProvider.Shutdown(shutdownCtx); err != nil {
			slog.Error("Failed to shut down tracer provider", "error", err)
		}
		if err := loggerProvider.Shutdown(shutdownCtx); err != nil {
			slog.Error("Failed to shut down logger provider", "error", err)
		}
	}

	return shutdown, nil
}
