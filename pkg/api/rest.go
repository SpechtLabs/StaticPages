package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/SpechtLabs/StaticPages/pkg/config"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/sierrasoftworks/humane-errors-go"
	"github.com/spechtlabs/go-otel-utils/otelzap"
	ginprometheus "github.com/zsais/go-gin-prometheus"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	// StatusRequestContextCanceled is the status the API answers with when the
	// client went away before the request was handled (nginx's 499).
	StatusRequestContextCanceled = 499

	// readHeaderTimeout bounds how long a client may take to send its request
	// headers, so slow clients can't hold connections open (Slowloris).
	readHeaderTimeout = 10 * time.Second
)

// RestApi represents a RESTful API server encapsulating an HTTP server, router, and static page configuration.
type RestApi struct {
	tracer trace.Tracer
	srv    *http.Server
	router *gin.Engine
	// now dates the commits uploads publish.
	now  func() time.Time
	conf config.StaticPagesConfig
}

// NewRestApi initializes and returns a new RestApi instance configured with the provided StaticPagesConfig.
func NewRestApi(conf config.StaticPagesConfig) *RestApi {
	r := &RestApi{
		srv:    nil,
		conf:   conf,
		tracer: otel.Tracer("StaticPages-API"),
		now:    time.Now,
	}

	// Setup Gin router
	r.router = gin.New(func(e *gin.Engine) {})

	// Setup Routes
	r.router.POST("/api/upload", r.UploadHandler)

	// Setup otelgin to expose Open Telemetry
	r.router.Use(otelgin.Middleware("StaticPages-API"))

	// Setup ginzap to log everything correctly to zap
	r.router.Use(ginzap.GinzapWithConfig(otelzap.L(), &ginzap.Config{
		UTC:        true,
		TimeFormat: time.RFC3339,
		Context: func(c *gin.Context) []zapcore.Field {
			var fields []zapcore.Field
			// log request ID
			if requestID := c.Writer.Header().Get("X-Request-Id"); requestID != "" {
				fields = append(fields, zap.String("request_id", requestID))
			}

			// log trace and span ID
			if spanContext := trace.SpanFromContext(c.Request.Context()).SpanContext(); spanContext.IsValid() {
				fields = append(fields, zap.String("trace_id", spanContext.TraceID().String()))
				fields = append(fields, zap.String("span_id", spanContext.SpanID().String()))
			}
			return fields
		},
	}))

	// Set-up Prometheus to expose prometheus metrics
	p := ginprometheus.NewPrometheus("staticpages")
	p.Use(r.router)

	return r
}

// ServeAsync starts the REST API server on addr in a goroutine and returns at
// once; Shutdown stops it. If the server fails to start, it logs a fatal error.
func (r *RestApi) ServeAsync(addr string) {
	r.srv = r.newServer(addr)
	go func(srv *http.Server) {
		if err := ListenAndServe(srv, "API server"); err != nil {
			otelzap.L().WithError(err).Fatal("Unable to start API server", zap.String("addr", srv.Addr))
		}
	}(r.srv)
}

// Serve starts the REST API Server on the specified address and returns a humane.Error if any issue occurs during startup.
func (r *RestApi) Serve(addr string) humane.Error {
	r.srv = r.newServer(addr)
	return ListenAndServe(r.srv, "API server")
}

// Shutdown gracefully stops the proxy server if it is running, releasing any resources and handling in-progress requests.
// It returns a humane.Error if the server fails to stop.
func (r *RestApi) Shutdown() humane.Error {
	if r.srv == nil {
		return humane.New("Unable to shutdown API Server. It is not running.", "Start API Server first before attempting to stop it")
	}

	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()

	otelzap.L().Info("shutting down API server", zap.String("addr", r.srv.Addr))
	if err := r.srv.Shutdown(ctx); err != nil {
		return humane.Wrap(err, "Unable to shutdown api server", "Make sure the api server is running and try again.")
	}

	return nil
}

// ListenAndServe serves srv, logging under name, until it's shut down, which
// isn't an error. The API and the proxy both run their servers through it.
func ListenAndServe(srv *http.Server, name string) humane.Error {
	otelzap.L().Info("starting "+name, zap.String("addr", srv.Addr))

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return humane.Wrap(err, "Unable to start "+name, "Make sure nothing else listens on "+srv.Addr+" and try again.")
	}

	otelzap.L().Info(name+" stopped", zap.String("addr", srv.Addr))
	return nil
}

func (r *RestApi) newServer(addr string) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           r.router,
		ReadHeaderTimeout: readHeaderTimeout,
	}
}
