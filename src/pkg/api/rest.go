package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/SpechtLabs/StaticPages/pkg/config"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	ginprometheus "github.com/mcuadros/go-gin-prometheus"
	"github.com/sierrasoftworks/humane-errors-go"
	"github.com/spechtlabs/go-otel-utils/otelzap"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	StatusRequestContextCanceled = 499
)

// RestApi represents a RESTful API server encapsulating an HTTP server, router, and static page configuration.
type RestApi struct {
	srv    *http.Server
	router *gin.Engine
	conf   config.StaticPagesConfig
	tracer trace.Tracer
}

// NewRestApi initializes and returns a new RestApi instance configured with the provided StaticPagesConfig.
func NewRestApi(conf config.StaticPagesConfig) *RestApi {
	r := &RestApi{
		srv:    nil,
		conf:   conf,
		tracer: otel.Tracer("StaticPages-API"),
	}

	// Setup Gin router
	router := gin.New(func(e *gin.Engine) {})

	// Setup otelgin to expose Open Telemetry
	router.Use(otelgin.Middleware("StaticPages-API"))

	// Setup ginzap to log everything correctly to zap
	router.Use(ginzap.GinzapWithConfig(otelzap.L(), &ginzap.Config{
		UTC:        true,
		TimeFormat: time.RFC3339,
		Context: func(c *gin.Context) []zapcore.Field {
			var fields []zapcore.Field
			// log request ID
			if requestID := c.Writer.Header().Get("X-Request-Id"); requestID != "" {
				fields = append(fields, zap.String("request_id", requestID))
			}

			// log trace and span ID
			if trace.SpanFromContext(c.Request.Context()).SpanContext().IsValid() {
				fields = append(fields, zap.String("trace_id", trace.SpanFromContext(c.Request.Context()).SpanContext().TraceID().String()))
				fields = append(fields, zap.String("span_id", trace.SpanFromContext(c.Request.Context()).SpanContext().SpanID().String()))
			}
			return fields
		},
	}))

	// Set-up Prometheus to expose prometheus metrics
	p := ginprometheus.NewPrometheus("conf_room_display")
	p.Use(router)

	router.POST("/upload", r.UploadHandler)
	router.PUT("/activate", r.Activate)

	r.router = router

	return r
}

// ServeAsync starts the REST API server asynchronously on the specified address by calling Serve within a goroutine.
// If the server fails to start, it logs a fatal error with contextual details including error advice and cause.
func (r *RestApi) ServeAsync(addr string) {
	go func() {
		if err := r.Serve(addr); err != nil {
			otelzap.L().Sugar().Fatalw("Unable to start proxy",
				zap.String("error", err.Error()),
				zap.Strings("advice", err.Advice()),
				zap.String("cause", err.Cause().Error()))
		}
	}()
}

// Serve starts the REST API Server on the specified address and returns a humane.Error if any issue occurs during startup.
func (r *RestApi) Serve(addr string) humane.Error {
	otelzap.L().Sugar().Infow("Starting REST API Server", zap.String("address", addr))

	// configure the HTTP Server
	r.srv = &http.Server{
		Addr:    addr,
		Handler: r.router,
	}

	if err := r.srv.ListenAndServe(); err != nil {
		if strings.Contains(err.Error(), http.ErrServerClosed.Error()) {
			otelzap.L().Sugar().Infow("API server stopped",
				zap.String("addr", r.srv.Addr))
			return nil
		}

		return humane.Wrap(err, "Unable to start API Server", "Make sure the api server is not already running and try again.")
	}

	return nil
}

// Shutdown gracefully stops the proxy server if it is running, releasing any resources and handling in-progress requests.
// It returns a humane.Error if the server fails to stop.
func (r *RestApi) Shutdown() humane.Error {
	if r.srv == nil {
		return humane.New("Unable to shutdown API Server. It is not running.", "Start API Server first before attempting to stop it")
	}

	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()

	otelzap.L().Sugar().Info("shutting down proxy")
	if err := r.srv.Shutdown(ctx); err != nil {
		return humane.Wrap(err, "Unable to shutdown api server", "Make sure the api server is running and try again.")
	}

	return nil
}

func (r *RestApi) Activate(ct *gin.Context) {
	panic("yet to implement")
}
