package main

import (
	"context"
	"fmt"
	"os"

	"github.com/SpechtLabs/StaticPages/cmd"
	"github.com/gin-gonic/gin"
	"github.com/sierrasoftworks/humane-errors-go"
	"github.com/spechtlabs/go-otel-utils/otelprovider"
	"github.com/spechtlabs/go-otel-utils/otelzap"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
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
			fmt.Printf("Version: %s\n", Version)
			fmt.Printf("Date:    %s\n", Date)
			fmt.Printf("Commit:  %s\n", Commit)
			fmt.Printf("BuiltBy: %s\n", BuiltBy)
		},
	}
)

func main() {
	traceProvider := otelprovider.NewTracer(
		otelprovider.WithTraceAutomaticEnv(),
	)

	// Initialize Logging
	debug := os.Getenv("OTEL_LOG_LEVEL") == "debug"
	var zapLogger *zap.Logger
	var err error
	if debug {
		zapLogger, err = zap.NewDevelopment()
		gin.SetMode(gin.DebugMode)
	} else {
		zapLogger, err = zap.NewProduction()
		gin.SetMode(gin.ReleaseMode)
	}
	if err != nil {
		fmt.Printf("failed to initialize logger: %v", err)
		os.Exit(1)
	}

	// Replace zap global
	undoZapGlobals := zap.ReplaceGlobals(zapLogger)

	// Redirect stdlib log to zap
	undoStdLogRedirect := zap.RedirectStdLog(zapLogger)

	// Create otelLogger. We deliberately do NOT wire an OTLP log provider:
	// logs are emitted as structured JSON on stdout and scraped into Loki by
	// Alloy. Exporting via OTLP as well would double-ingest every log in two
	// different formats. Traces still go out over OTLP via traceProvider.
	otelZapLogger := otelzap.New(zapLogger,
		otelzap.WithCaller(true),
		otelzap.WithMinLevel(zap.InfoLevel),
		otelzap.WithAnnotateLevel(zap.WarnLevel),
		otelzap.WithErrorStatusLevel(zap.ErrorLevel),
		otelzap.WithStackTrace(false),
	)

	// Replace global otelZap logger
	undoOtelZapGlobals := otelzap.ReplaceGlobals(otelZapLogger)

	defer func() {
		if err := traceProvider.ForceFlush(context.Background()); err != nil {
			otelzap.L().Warn("failed to flush traces")
		}

		if err := traceProvider.Shutdown(context.Background()); err != nil {
			panic(err)
		}

		undoStdLogRedirect()
		undoOtelZapGlobals()
		undoZapGlobals()
	}()

	cmd.RootCmd.AddCommand(versionCmd)
	err = cmd.RootCmd.Execute()
	if err != nil {
		// Render humane errors with their advice; fall back to a plain message
		// for everything else. Either way: a clean message, never a panic.
		if herr, ok := err.(humane.Error); ok {
			fmt.Println(herr.Display())
		} else {
			fmt.Println(err)
		}
		os.Exit(1)
	}
}
