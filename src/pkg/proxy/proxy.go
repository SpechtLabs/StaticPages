package proxy

import (
	"context"
	"fmt"
	"github.com/SpechtLabs/StaticPages/pkg/config"
	"github.com/golang/groupcache/singleflight"
	humane "github.com/sierrasoftworks/humane-errors-go"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"
)

// Proxy represents a reverse proxy server with logging, page management, and request handling capabilities.
type Proxy struct {
	pagesMap map[string]*config.Page
	conf     config.StaticPagesConfig
	proxy    *httputil.ReverseProxy
	group    singleflight.Group
	server   *http.Server
	tracer   trace.Tracer
}

// NewProxy initializes and returns a new Proxy instance configured with the provided logger and page definitions.
func NewProxy(conf config.StaticPagesConfig) *Proxy {
	// construct a map for easier lookup in the director
	pagesMap := make(map[string]*config.Page)
	for _, page := range conf.Pages {
		if pagesMap[page.Domain] != nil {
			otelzap.L().Sugar().Warnw("duplicate page domain", zap.String("domain", page.Domain))
		}

		pagesMap[page.Domain] = page
	}

	p := &Proxy{
		pagesMap: pagesMap,
		proxy:    nil,
		conf:     conf,
		server:   nil,
		tracer:   otel.Tracer("StaticPages-Proxy"),
	}

	p.proxy = &httputil.ReverseProxy{
		Director:       p.Director,       // Add a proxy director
		ErrorHandler:   p.ErrorHandler,   // Add error handler to log errors
		ModifyResponse: p.ModifyResponse, // Add response modifier to log response status

		// Allow transport configuration provided by user
		Transport: &http.Transport{
			MaxIdleConns:        conf.Proxy.MaxIdleConns,
			MaxIdleConnsPerHost: conf.Proxy.MaxIdleConnsPerHost,
			IdleConnTimeout:     conf.Proxy.Timeout,
			DisableCompression:  !conf.Proxy.Compression,
		},
	}

	return p
}

// Director modifies the incoming HTTP request to route it to the appropriate backend server based on the request host and path.
func (p *Proxy) Director(req *http.Request) {
	ctx := req.Context()

	// Start a new Span
	ctx, span := p.tracer.Start(ctx, "proxy.Director", trace.WithAttributes(
		attribute.String("http.method", req.Method),
		attribute.String("http.host", req.Host),
		attribute.String("http.url", req.URL.String()),
	))
	defer span.End()

	if ctx.Err() != nil {
		otelzap.L().Sugar().Ctx(ctx).Warnw("request context canceled",
			zap.String("url", req.URL.String()),
			zap.String("path", req.URL.Path))

		span.RecordError(ctx.Err())
		span.SetStatus(codes.Error, "context canceled")
		return
	}

	originalPath := req.URL.Path
	requestUrl := req.Host
	if strings.Contains(requestUrl, ":") {
		requestUrl = strings.Split(requestUrl, ":")[0]
	}

	page, ok := p.pagesMap[requestUrl]
	if !ok {
		errMessage := "no page found"

		otelzap.L().Sugar().Ctx(ctx).Errorw(errMessage, zap.String("requestUrl", requestUrl))
		span.RecordError(fmt.Errorf(errMessage), trace.WithAttributes(attribute.String("requestUrl", requestUrl)))
		span.SetStatus(codes.Error, errMessage)
		return
	}

	backendUrl, err := url.Parse(page.Proxy.URL.String())
	if err != nil {
		errMessage := "invalid target URL"

		otelzap.L().Sugar().Ctx(ctx).Errorw(errMessage, zap.String("url", page.Proxy.URL.String()), zap.Error(err))

		span.RecordError(fmt.Errorf(errMessage), trace.WithAttributes(attribute.String("url", page.Proxy.URL.String()), attribute.String("error", err.Error())))
		span.SetStatus(codes.Error, errMessage)
		return
	}

	// Find the actual html document we are looking for
	targetPath, err := p.lookupPath(ctx, page, requestUrl, backendUrl, path.Clean(fmt.Sprintf("/%s/%s", page.Proxy.Path, originalPath)))
	if err != nil {
		otelzap.L().Sugar().Ctx(ctx).Errorw("no valid path found",
			zap.String("original_path", originalPath),
			zap.String("target_path", targetPath),
			zap.Error(err),
		)

		span.RecordError(err)
		span.SetStatus(codes.Error, "path lookup failed")
		//return
	}

	req.URL.Scheme = backendUrl.Scheme
	req.URL.Host = backendUrl.Host
	req.URL.Path = targetPath

	// Clear the RequestURI as it's required for client requests
	req.RequestURI = ""

	// Set or update headers
	if _, ok := req.Header["User-Agent"]; !ok {
		req.Header.Set("User-Agent", "StaticPages-Proxy")
	}

	req.Header.Set("X-Forwarded-Host", req.Host)
	req.Header.Set("X-Origin-Host", backendUrl.Host)

	// Inject trace context headers for the backend call
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	// Log the request transformation
	otelzap.L().Sugar().DebugwContext(ctx, "transformed request",
		zap.String("original_path", originalPath),
		zap.String("target_path", targetPath),
		zap.String("backend_host", backendUrl.String()),
		zap.String("backend_url", req.URL.String()))
}

// ErrorHandler handles errors during request processing by logging the error and responding with the appropriate HTTP status code.
func (p *Proxy) ErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	responseCode := http.StatusBadGateway

	switch err.Error() {
	case "context canceled":
		responseCode = 499 // Nginx non-standard code for when a client closes the connection
	}

	otelzap.L().Sugar().Ctx(r.Context()).Errorw("proxy error",
		zap.Error(err),
		zap.String("url", r.URL.String()),
		zap.Int("code", responseCode))

	http.Error(w, err.Error(), responseCode)
}

// ModifyResponse inspects and logs HTTP responses with a status code of 300 or higher, returning nil or an error.
func (p *Proxy) ModifyResponse(r *http.Response) error {
	if r.StatusCode >= 300 {
		if otelzap.L().Sugar().Desugar().Core().Enabled(zap.DebugLevel) {
			dump, _ := httputil.DumpResponse(r, true)

			otelzap.L().Sugar().Ctx(r.Request.Context()).Debugw("received response",
				zap.Int("code", r.StatusCode),
				zap.String("url", r.Request.URL.String()),
				zap.ByteString("url", dump))
		} else {
			otelzap.L().Sugar().Ctx(r.Request.Context()).Infow("received response",
				zap.Int("code", r.StatusCode),
				zap.String("url", r.Request.URL.String()))
		}
	}

	return nil
}

// ServeHTTP handles incoming HTTP requests and proxies them to the configured backend, allowing only GET requests.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		otelzap.L().Sugar().Ctx(r.Context()).Warnw("received non-GET request",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path))

		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	p.proxy.ServeHTTP(w, r)
}

// ServeAsync starts the reverse proxy server on the specified address and logs the startup message.
// It runs the server in a separate goroutine and handles failure to start by logging a fatal error.
// It Panics when the Proxy Server could not start
func (p *Proxy) ServeAsync(addr string) {
	go func() {
		if err := p.Serve(addr); err != nil {
			otelzap.L().Sugar().Fatalw("Unable to start proxy",
				zap.String("error", err.Error()),
				zap.Strings("advice", err.Advice()),
				zap.String("cause", err.Cause().Error()))
		}
	}()
}

// Serve starts the reverse proxy server on the specified address and logs its startup state.
// It returns a humane.Error if the server fails to start.
func (p *Proxy) Serve(addr string) humane.Error {
	otelzap.L().Sugar().Infow("starting reverse proxy",
		zap.String("addr", addr))

	p.server = &http.Server{
		Addr:    addr,
		Handler: p,
	}

	if err := p.server.ListenAndServe(); err != nil {
		if err == http.ErrServerClosed {
			otelzap.L().Sugar().Infow("proxy server stopped",
				zap.String("addr", addr))
			return nil
		}
		return humane.Wrap(err, "Unable to start proxy", "Make sure the proxy is not already running and try again.")
	}

	return nil
}

// Shutdown gracefully stops the proxy server if it is running, releasing any resources and handling in-progress requests.
// It returns a humane.Error if the server fails to stop.
func (p *Proxy) Shutdown() humane.Error {
	if p.server == nil {
		return humane.New("Unable to shutdown proxy. It is not running.", "Start Proxy first before attempting to stop it")
	}

	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()

	otelzap.L().Sugar().Info("shutting down proxy")
	if err := p.server.Shutdown(ctx); err != nil {
		return humane.Wrap(err, "Unable to shutdown proxy", "Make sure the proxy is running and try again.")
	}

	return nil
}

func (p *Proxy) probePath(ctx context.Context, url *url.URL, location string) (int, error) {
	// Start a span for the probePath method
	ctx, span := p.tracer.Start(ctx, "proxy.probePath", trace.WithAttributes(
		attribute.String("url", url.String()),
		attribute.String("probe_location", location),
	))
	defer span.End()

	// create a http client with short timeout for fast failure
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	req, _ := http.NewRequestWithContext(ctx, http.MethodHead, url.String()+location, nil)

	// Inject trace context headers for the backend call
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	resp, err := client.Do(req)
	if resp != nil {
		if err != nil {
			span.SetAttributes(attribute.String("error", err.Error()))
			span.SetStatus(codes.Error, "Unable to probe path")
		}

		span.SetAttributes(attribute.Int("code", resp.StatusCode))
		if resp.StatusCode < http.StatusBadRequest {
			span.SetStatus(codes.Ok, "Path exists")
		} else {
			span.SetStatus(codes.Ok, "Path does not exists")
		}

		return resp.StatusCode, err
	}

	code := codes.Error
	statusDesc := "Unable to probe path"

	if err != nil {

		if strings.Contains(err.Error(), context.DeadlineExceeded.Error()) ||
			strings.Contains(err.Error(), context.Canceled.Error()) {
			code = codes.Ok
		}

		statusDesc = err.Error()
	}

	span.SetStatus(code, statusDesc)
	return http.StatusNotFound, err
}

func (p *Proxy) lookupPath(ctx context.Context, page *config.Page, sourceHost string, backendUrl *url.URL, targetPath string) (string, humane.Error) {
	// Start a span for the lookupPath method
	ctx, span := p.tracer.Start(ctx, "proxy.lookupPath", trace.WithAttributes(
		attribute.String("page.domain", page.Domain),
		attribute.String("backend_url", backendUrl.String()),
		attribute.String("target_path", targetPath),
		attribute.String("source_host", sourceHost),
	))
	defer span.End()

	// Find the actual html document we are looking for
	searchPath := append([]string{""}, page.Proxy.SearchPath...)

	// find the validPath fast and not block
	var wg sync.WaitGroup
	validPath := make(chan string, 1)
	cancelableCtx, cancelCtx := context.WithCancel(ctx)

	for _, lookupPath := range searchPath {
		// Probe each searchPath asynchronously
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Cache search path computation
			cacheKey := fmt.Sprintf("%s-%s-%s", sourceHost, targetPath, lookupPath)
			_, _ = p.group.Do(cacheKey, func() (interface{}, error) {
				// Probe the path
				testTarget := path.Clean(fmt.Sprintf("/%s/%s", targetPath, lookupPath))

				statusCode, err := p.probePath(cancelableCtx, backendUrl, testTarget)
				if err != nil {
					return nil, humane.Wrap(err, "Unable to probe path", "Make sure the path exists and is accessible.")
				}

				if statusCode < http.StatusBadRequest {
					span.SetStatus(codes.Ok, "Path found")
					span.SetAttributes(attribute.String("found_path", testTarget))

					select {
					case validPath <- testTarget: // if a valid path is found, send it
						cancelCtx() // cancel all others
						return nil, nil
					case <-cancelableCtx.Done(): // if context is canceled, stop sending
						return nil, nil
					}
				}
				return nil, nil
			})
		}()
	}

	// Wait for all go-routines to finish, then close path
	go func() {
		wg.Wait()
		close(validPath)
	}()

	select {
	case validPath, ok := <-validPath:
		if ok {
			cancelCtx()
			return validPath, nil
		} else {
			cancelCtx()
			return "", humane.New("no valid path found", "Make sure the path exists and is accessible.")
		}
	case <-cancelableCtx.Done():
		cancelCtx()
		return "", humane.New("no valid path found", "Make sure the path exists and is accessible.")

	case <-time.After(5 * time.Second):
		cancelCtx()
		return "", humane.New("Timeout waiting for valid path", "Make sure the path exists and is accessible.")
	}
}
