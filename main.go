package main

import (
	"context"
	"errors"
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

// Build information, set by GoReleaser through -ldflags.
var (
	// Version is the release version.
	Version string
	// Commit is the commit the binary was built from.
	Commit string
	// Date is the commit's timestamp.
	Date string
	// BuiltBy names what built the binary.
	BuiltBy string
)

func main() {
	os.Exit(run(os.Args[1:]))
}

// run sets up tracing and logging, runs the command line with args, and
// returns the process exit code. Keeping it apart from main lets the deferred
// cleanup run before the process exits.
func run(args []string) int {
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
		return 1
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
		if flushErr := traceProvider.ForceFlush(context.Background()); flushErr != nil {
			otelzap.L().Warn("failed to flush traces", zap.Error(flushErr))
		}

		if shutdownErr := traceProvider.Shutdown(context.Background()); shutdownErr != nil {
			panic(shutdownErr)
		}

		undoStdLogRedirect()
		undoOtelZapGlobals()
		undoZapGlobals()
	}()

	rootCmd, herr := cmd.NewRootCmd()
	if herr != nil {
		fmt.Println(herr.Display())
		return 1
	}

	rootCmd.AddCommand(newVersionCmd())
	rootCmd.SetArgs(args)
	if err := rootCmd.Execute(); err != nil {
		// Render humane errors with their advice; fall back to a plain message
		// for everything else. Either way: a clean message, never a panic.
		if herr, ok := errors.AsType[humane.Error](err); ok {
			fmt.Println(herr.Display())
		} else {
			fmt.Println(err)
		}
		return 1
	}

	return 0
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Shows version information",
		Args:  cobra.ExactArgs(0),
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Printf("Version: %s\n", Version)
			fmt.Printf("Date:    %s\n", Date)
			fmt.Printf("Commit:  %s\n", Commit)
			fmt.Printf("BuiltBy: %s\n", BuiltBy)
		},
	}
}
