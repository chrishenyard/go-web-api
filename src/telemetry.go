package main

import (
	"context"

	"github.com/chrishenyard/go-web-api/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

func initTelemetry(ctx context.Context, cfg *config.Config) (*sdklog.LoggerProvider, *sdktrace.TracerProvider, error) {
	// Shared resource configuration
	res, err := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceNameKey.String(cfg.ServiceName),
	))
	if err != nil {
		return nil, nil, err
	}

	logExporter, err := otlploggrpc.New(ctx)
	if err != nil {
		return nil, nil, err
	}
	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
		sdklog.WithResource(res),
	)
	global.SetLoggerProvider(lp)

	// 2. Setup Trace Exporter & Provider
	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, nil, err
	}
	tp := sdktrace.NewTracerProvider(
		// sdktrace.WithBatcher(traceExporter),
		sdktrace.WithSyncer(traceExporter), // Synchronous exporting is useful while debugging.
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	return lp, tp, nil
}
