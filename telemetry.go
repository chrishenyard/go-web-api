package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/chrishenyard/go-web-api/config"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

type levelFilterHandler struct {
	slog.Handler
	level slog.Level
}

func (h *levelFilterHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level
}

func initTelemetry(ctx context.Context, cfg *config.Config) (func(context.Context), error) {
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(cfg.ServiceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("creating OTLP resource: %w", err)
	}

	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating OTLP trace exporter: %w", err)
	}
	tp := sdktrace.NewTracerProvider(
		// sdktrace.WithBatcher(traceExporter),
		sdktrace.WithSyncer(traceExporter), // Synchronous exporting is useful while debugging.
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	logExporter, err := otlploggrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf(
			"creating OTLP log exporter: %w", err)
	}

	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
		sdklog.WithResource(res),
	)

	baseHandler := otelslog.NewHandler(
		cfg.ServiceName,
		otelslog.WithLoggerProvider(lp),
	)

	level := config.ParseLogLevel(cfg.LogLevel)
	otelHandler := &levelFilterHandler{
		Handler: baseHandler,
		level:   level,
	}
	slog.SetDefault(slog.New(otelHandler))

	shutdown := func(shutCtx context.Context) {
		_ = tp.Shutdown(shutCtx)
		_ = lp.Shutdown(shutCtx)
	}

	return shutdown, nil
}
