package main

import (
	"apphost/modules/aspire"
	"fmt"
	"log"
	"os"
	"strconv"
)

func main() {
	builder, err := aspire.CreateBuilder()
	if err != nil {
		log.Fatal(aspire.FormatError(err))
	}

	targetPort, err := strconv.ParseFloat(os.Getenv("PORT"), 64)
	if err != nil {
		fmt.Println("Error parsing float:", err)
		return
	}

	protocol := aspire.OtlpProtocolGrpc
	webApp := builder.AddExecutable("go-web-api", "go", "./src", []string{"run", "."}).
		WithHttpsEndpoint(&aspire.WithHttpsEndpointOptions{TargetPort: &targetPort}).
		WithHttpsDeveloperCertificate().
		WithExternalHttpEndpoints().
		WithOtlpExporter(&aspire.WithOtlpExporterOptions{Protocol: &protocol}).
		WithDeveloperCertificateTrust(true).
		WithEnvironment("OTEL_EXPORTER_OTLP_CERTIFICATE", os.Getenv("OTEL_EXPORTER_OTLP_CERTIFICATE"))

	if err := webApp.Err(); err != nil {
		log.Fatal(aspire.FormatError(err))
	}

	app, err := builder.Build()
	if err != nil {
		log.Fatal(aspire.FormatError(err))
	}

	if err := app.Run(); err != nil {
		log.Fatal(aspire.FormatError(err))
	}
}
