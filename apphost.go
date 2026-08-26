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
	webApp := builder.AddExecutable("go-web-api", "go", "/home/chenyard/source/repos/go-web-api/src", []string{"run", "."}).
		// webApp := builder.AddExecutable("go-web-api", "dlv", "/home/chenyard/source/repos/go-web-api/src", []string{
		// 	"dap",
		// 	"--listen=0.0.0.0:4000",
		// }).
		WithHttpsEndpoint(&aspire.WithHttpsEndpointOptions{TargetPort: &targetPort}).
		WithHttpsDeveloperCertificate().
		WithExternalHttpEndpoints().
		WithOtlpExporter(&aspire.WithOtlpExporterOptions{Protocol: &protocol}).
		WithDeveloperCertificateTrust(true)

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
