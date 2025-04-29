package main

import (
	"context"
	"fmt"
	"github.com/SpechtLabs/StaticPages/cmd"
	"github.com/spf13/cobra"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log/global"
	sdklogs "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.uber.org/zap"
	"os"
	"time"
)

var (
	Version    string
	Commit     string
	Date       string
	BuiltBy    string
	versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Shows version information",
		Args:  cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			printVersion()
		},
	}
)

func main() {
	loggerShutdown := initLogging()
	tracingShutdown := initTracing()

	defer func() {
		tracingShutdown()
		loggerShutdown()
	}()

	cmd.RootCmd.AddCommand(versionCmd)
	err := cmd.RootCmd.Execute()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func printVersion() {
	fmt.Printf("Version: %s\n", Version)
	fmt.Printf("Date:    %s\n", Date)
	fmt.Printf("Commit:  %s\n", Commit)
	fmt.Printf("BuiltBy: %s\n", BuiltBy)
}

func initTracing() func() {
	var err error

	// OTLP gRPC Endpoint from Environment (Default: "localhost:4317")
	otelGrpcEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_GRPC_ENDPOINT")
	if otelGrpcEndpoint == "" {
		otelGrpcEndpoint = "localhost:4317"
	}

	// OTLP HTTP Endpoint from Environment (Default: "localhost:4318")
	otelHttpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_HTTP_ENDPOINT")
	if otelHttpEndpoint == "" {
		otelHttpEndpoint = "localhost:4318"
	}

	// OTLP Insecure from Environment (Default: "true")
	otelInsecure := os.Getenv("OTEL_EXPORTER_OTLP_INSECURE") == "true"

	// Initialize Tracing
	ctx := context.Background()

	grpcExporterOptions := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(otelGrpcEndpoint),
	}
	httpExporterOptions := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(otelHttpEndpoint),
	}

	if otelInsecure {
		grpcExporterOptions = append(grpcExporterOptions, otlptracegrpc.WithInsecure())
		httpExporterOptions = append(httpExporterOptions, otlptracehttp.WithInsecure())
	}

	grpcExporter, err := otlptrace.New(ctx, otlptracegrpc.NewClient(grpcExporterOptions...))
	if err != nil {
		otelzap.L().Sugar().Fatalw("Failed to create OTLP gRPC trace exporter", zap.Error(err))
	}

	httpExporter, err := otlptrace.New(ctx, otlptracehttp.NewClient(httpExporterOptions...))
	if err != nil {
		otelzap.L().Sugar().Fatalw("Failed to create OTLP HTTP trace exporter", zap.Error(err))
	}

	// 3. Combine the gRPC and HTTP Exporters
	multiExporter := sdktrace.NewBatchSpanProcessor(grpcExporter)
	multiExporterHTTP := sdktrace.NewBatchSpanProcessor(httpExporter)

	// Define the Tracer Provider with both exporters
	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(multiExporter),     // Send via gRPC
		sdktrace.WithSpanProcessor(multiExporterHTTP), // Send via HTTP
		sdktrace.WithResource(newOtelResources()),
	)

	// Register the Provider globally
	otel.SetTracerProvider(traceProvider)

	// Return a unified shutdown function
	return func() {
		// Shutdown Tracer Provider
		if err := traceProvider.Shutdown(ctx); err != nil {
			otelzap.L().Sugar().Errorw("Failed to shutdown trace provider", zap.Error(err))
		}
	}
}

func initLogging() func() {
	var err error

	// OTLP gRPC Endpoint from Environment (Default: "localhost:4317")
	otelGrpcEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_GRPC_ENDPOINT")
	if otelGrpcEndpoint == "" {
		otelGrpcEndpoint = "localhost:4317"
	}

	// OTLP HTTP Endpoint from Environment (Default: "localhost:4318")
	otelHttpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_HTTP_ENDPOINT")
	if otelHttpEndpoint == "" {
		otelHttpEndpoint = "localhost:4318"
	}

	// OTLP Insecure from Environment (Default: "true")
	otelInsecure := os.Getenv("OTEL_EXPORTER_OTLP_INSECURE") == "true"

	debug := os.Getenv("DEBUG") == "true"

	// Initialize Logging
	var zapLogger *zap.Logger
	if debug {
		zapLogger, err = zap.NewDevelopment()
	} else {
		zapLogger, err = zap.NewProduction()
	}
	if err != nil {
		fmt.Printf("failed to initialize logger: %v", err)
		os.Exit(1)
	}

	undoZapGlobals := zap.ReplaceGlobals(zapLogger)

	// Redirect stdlib log to zap
	undoStdLogRedirect := zap.RedirectStdLog(zapLogger)

	// Initialize Tracing
	ctx := context.Background()

	grpcExporterOptions := []otlploggrpc.Option{
		otlploggrpc.WithEndpoint(otelGrpcEndpoint),
	}
	httpExporterOptions := []otlploghttp.Option{
		otlploghttp.WithEndpoint(otelHttpEndpoint),
	}

	if otelInsecure {
		grpcExporterOptions = append(grpcExporterOptions, otlploggrpc.WithInsecure())
		httpExporterOptions = append(httpExporterOptions, otlploghttp.WithInsecure())
	}

	grpcExporter, err := otlploggrpc.New(ctx, grpcExporterOptions...)
	if err != nil {
		zapLogger.Sugar().Fatalw("Failed to create OTLP gRPC log exporter", zap.Error(err))
	}

	httpExporter, err := otlploghttp.New(ctx, httpExporterOptions...)
	if err != nil {
		zapLogger.Sugar().Fatalw("Failed to create OTLP HTTP log exporter", zap.Error(err))
	}

	multiExporter := sdklogs.NewBatchProcessor(grpcExporter,
		sdklogs.WithMaxQueueSize(10_000),
		sdklogs.WithExportMaxBatchSize(10_000),
		sdklogs.WithExportInterval(10*time.Second),
		sdklogs.WithExportTimeout(10*time.Second),
	)
	multiExporterHTTP := sdklogs.NewBatchProcessor(httpExporter,
		sdklogs.WithMaxQueueSize(10_000),
		sdklogs.WithExportMaxBatchSize(10_000),
		sdklogs.WithExportInterval(10*time.Second),
		sdklogs.WithExportTimeout(10*time.Second),
	)

	// Define the Log Provider with both exporters
	logProvider := sdklogs.NewLoggerProvider(
		sdklogs.WithProcessor(multiExporter),
		sdklogs.WithProcessor(multiExporterHTTP),
		sdklogs.WithResource(newOtelResources()),
	)

	// Register the Provider globally
	global.SetLoggerProvider(logProvider)

	// Create otelLogger
	otelLogger := otelzap.New(zapLogger,
		otelzap.WithCaller(true),
		otelzap.WithMinLevel(zap.InfoLevel),
		otelzap.WithErrorStatusLevel(zap.ErrorLevel),
		otelzap.WithStackTrace(false),
		otelzap.WithLoggerProvider(logProvider),
	)

	undoOtelZapGlobals := otelzap.ReplaceGlobals(otelLogger)

	// Return a unified shutdown function
	shutdown := func() {
		// Shutdown Logger Provider
		if err := logProvider.Shutdown(ctx); err != nil {
			otelLogger.Sugar().Errorw("Failed to shutdown log provider", zap.Error(err))
		}

		undoStdLogRedirect()

		// Undo otelzap logger replacement
		undoOtelZapGlobals()
		undoZapGlobals()
	}
	return shutdown
}

func newOtelResources() *resource.Resource {
	// Service Name from Environment (Default: "static-pages-proxy")
	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "static-pages-proxy"
	}

	res, err := resource.Merge(resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(Version),
		))

	if err != nil {
		panic(err)
	}

	return res
}
