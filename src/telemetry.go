package main

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	sdklog "go.opentelemetry.io/otel/sdk/log"
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
	res, err := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceNameKey.String(cfg.ServiceName),
	))
	if err != nil {
		return nil, err
	}

	logExporter, err := otlploggrpc.New(ctx)
	if err != nil {
		return nil, err
	}

	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
		sdklog.WithResource(res),
	)

	baseHandler := otelslog.NewHandler(
		cfg.ServiceName,
		otelslog.WithLoggerProvider(lp),
	)

	otelHandler := &levelFilterHandler{
		Handler: baseHandler,
		level:   cfg.GetLogLevel(),
	}

	slog.SetDefault(slog.New(otelHandler))

	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, err
	}

	var tp *sdktrace.TracerProvider
	if cfg.IsDevelopment() {
		slog.InfoContext(ctx, "Running in development mode, using synchronous trace exporter for debugging")
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithSyncer(traceExporter), // Synchronous exporting is useful while debugging.
			sdktrace.WithResource(res),
		)
	} else {
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(traceExporter),
			sdktrace.WithResource(res),
		)
	}

	otel.SetTracerProvider(tp)

	shutdown := func(shutCtx context.Context) {
		_ = tp.Shutdown(shutCtx)
		_ = lp.Shutdown(shutCtx)
	}
	return shutdown, nil
}
