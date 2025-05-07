package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/SpechtLabs/StaticPages/pkg/api"
	"github.com/SpechtLabs/StaticPages/pkg/config"
	"github.com/SpechtLabs/StaticPages/pkg/s3_client"
	"github.com/golang/groupcache/singleflight"
	"github.com/sierrasoftworks/humane-errors-go"
	"github.com/spechtlabs/go-otel-utils/otelzap"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// Proxy represents a reverse proxy server with logging, page management, and request handling capabilities.
type Proxy struct {
	pagesMap config.DomainMapper
	conf     config.StaticPagesConfig
	proxy    *httputil.ReverseProxy
	group    singleflight.Group
	server   *http.Server
	tracer   trace.Tracer
}

// NewProxy initializes and returns a new Proxy instance configured with the provided logger and page definitions.
func NewProxy(conf config.StaticPagesConfig) *Proxy {
	p := &Proxy{
		pagesMap: config.NewDomainMapperFromPages(conf.Pages),
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
	ctx, span := p.tracer.Start(req.Context(), "proxy.Director", trace.WithAttributes(
		attribute.String("http.method", req.Method),
		attribute.String("http.url", req.Host),
		attribute.String("http.path", req.URL.String()),
	))
	defer span.End()

	if ctx.Err() != nil {
		otelzap.L().Ctx(ctx).Warn("request context canceled",
			zap.String("http.url", req.Host),
			zap.String("http.path", req.URL.String()))
		return
	}

	originalPath := req.URL.Path

	requestUrl := req.Host

	if strings.Contains(req.Host, ":") {
		var err error
		requestUrl, _, err = net.SplitHostPort(req.Host)
		if err != nil {
			otelzap.L().WithError(err).Ctx(ctx).Error("unable to parse request url", zap.String("request_url", req.Host))
			return
		}
	}

	page := p.pagesMap.Lookup(requestUrl)
	if page == nil {
		otelzap.L().Ctx(ctx).Error("no page found", zap.String("request_url", requestUrl))
		return
	}

	backendUrl, err := url.Parse(page.Proxy.URL.String())
	if err != nil {
		otelzap.L().WithError(err).Ctx(ctx).Error("unable to parse proxy.url", zap.String("backend_url", page.Proxy.URL.String()))
		return
	}

	s3Client := s3_client.NewS3PageClient(page)
	metadata, err := s3Client.DownloadPageIndex(ctx)
	if err != nil {
		otelzap.L().WithError(err).Ctx(ctx).Error("unable to get metadata", zap.String("domain", page.Domain.String()))
	}

	// Find the actual html document we are looking for
	lookupPath := path.Join(path.Clean(page.Proxy.Path.String()), path.Clean(page.Git.Repository))

	sub, err := page.Domain.Subdomain(requestUrl)
	if err != nil {
		otelzap.L().WithError(err).Ctx(ctx).Error("unable to parse subdomain", zap.String("request_url", requestUrl))
		return
	}

	if !page.Preview.Enabled || sub == "" {
		sub = page.Git.MainBranch

		sha, _, err := metadata.GetLatestForBranch(sub)
		if err != nil {
			otelzap.L().WithError(err).Ctx(ctx).Error("could not find a commit to serve page for",
				zap.String("request_url", requestUrl),
				zap.String("domain", page.Domain.String()),
				zap.String("branch", sub),
			)
			return
		}

		lookupPath = path.Join(lookupPath, path.Clean(sha))
	} else {
		if sha, _, err := metadata.GetLatestForBranch(sub); err == nil {
			lookupPath = path.Join(lookupPath, path.Clean(sha))
		} else if _, err := metadata.GetBySHA(sub); err == nil {
			lookupPath = path.Join(lookupPath, path.Clean(sub))
		} else {
			otelzap.L().Ctx(ctx).Error("could not find a commit to serve page for",
				zap.String("request_url", requestUrl),
				zap.String("domain", page.Domain.String()),
			)
			return
		}
	}

	lookupPath = path.Join(lookupPath, path.Clean(originalPath))

	targetPath, err := p.lookupPath(ctx, page, requestUrl, backendUrl, lookupPath)
	if err != nil {
		var err404 humane.Error
		targetPath, err404 = p.lookupPath(ctx, page, requestUrl, backendUrl, path.Clean(fmt.Sprintf("/%s/%s", page.Proxy.Path, page.Proxy.NotFound)))

		if err404 == nil {
			otelzap.L().Ctx(ctx).Warn("no path found", zap.String("request_path", originalPath))
		} else {
			otelzap.L().WithError(err).Ctx(ctx).Error("no path found", zap.String("request_path", originalPath))
		}
	}

	req.URL.Scheme = backendUrl.Scheme
	req.URL.Host = backendUrl.Host
	req.URL.Path = targetPath

	// Clear the RequestURI as it's required for s3_client requests
	req.RequestURI = ""

	// Set or update headers
	if _, ok := req.Header["User-Agent"]; !ok {
		req.Header.Set("User-Agent", "StaticPages-Proxy")
	}

	req.Header.Set("X-Forwarded-Host", req.Host)
	req.Header.Set("X-Origin-Host", backendUrl.Host)

	// Inject trace context headers for the backend call
	req = req.WithContext(ctx)
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	// Log the request transformation
	otelzap.L().Ctx(ctx).Debug("transformed request",
		zap.String("request_path", originalPath),
		zap.String("backend_path", targetPath),
		zap.String("backend_url", backendUrl.String()),
		zap.String("backend_path", req.URL.String()))
}

// ErrorHandler handles errors during request processing by logging the error and responding with the appropriate HTTP status code.
func (p *Proxy) ErrorHandler(w http.ResponseWriter, req *http.Request, err error) {
	ctx, span := p.tracer.Start(req.Context(), "proxy.ErrorHandler", trace.WithAttributes(
		attribute.String("http.method", req.Method),
		attribute.String("http.url", req.Host),
		attribute.String("http.path", req.URL.String()),
	))
	defer span.End()

	responseCode := http.StatusBadGateway

	switch err.Error() {
	case "context canceled":
		responseCode = api.StatusRequestContextCanceled // Nginx non-standard code for when a s3_client closes the connection
	}

	otelzap.L().WithError(err).Ctx(ctx).Error("proxy error",
		zap.String("request_url", req.Host),
		zap.Int("http.code", responseCode))

	http.Error(w, err.Error(), responseCode)
}

// ModifyResponse inspects and logs HTTP responses with a status code of 300 or higher, returning nil or an error.
func (p *Proxy) ModifyResponse(r *http.Response) error {
	ctx, span := p.tracer.Start(r.Request.Context(), "proxy.ModifyResponse", trace.WithAttributes(
		attribute.String("http.method", r.Request.Method),
		attribute.String("http.url", r.Request.Host),
		attribute.String("proxy_url", r.Request.URL.String()),
	))
	defer span.End()

	if r.StatusCode >= 300 {
		if otelzap.L().Core().Enabled(zap.DebugLevel) {
			dump, _ := httputil.DumpResponse(r, true)

			otelzap.L().Ctx(ctx).Debug("received response",
				zap.Int("http.code", r.StatusCode),
				zap.String("request_url", r.Request.URL.String()),
				zap.ByteString("body", dump))
		} else {
			otelzap.L().Ctx(ctx).Info("received response",
				zap.Int("http.code", r.StatusCode),
				zap.String("request_url", r.Request.URL.String()))
		}
	}

	return nil
}

// ServeHTTP handles incoming HTTP requests and proxies them to the configured backend, allowing only GET requests.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ctx, span := p.tracer.Start(req.Context(), "proxy.ServeHTTP", trace.WithAttributes(
		attribute.String("http.method", req.Method),
		attribute.String("http.url", req.Host),
		attribute.String("http.path", req.URL.String()),
		attribute.String("http.user_agent", req.UserAgent()),
	))
	defer span.End()

	// Only allow GET requests
	switch req.Method {
	case http.MethodGet:
		// Inject trace context headers for the backend call
		req = req.WithContext(ctx)
		otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

		p.proxy.ServeHTTP(w, req)

	default:
		otelzap.L().Ctx(ctx).Warn("received invalid request",
			zap.String("http.method", req.Method),
			zap.String("http.url", req.Host),
			zap.String("http.path", req.URL.String()),
		)

		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
}

// ServeAsync starts the reverse proxy server on the specified address and logs the startup message.
// It runs the server in a separate goroutine and handles failure to start by logging a fatal error.
// It Panics when the Proxy Server could not start
func (p *Proxy) ServeAsync(addr string) {
	go func() {
		if err := p.Serve(addr); err != nil {
			otelzap.L().WithError(err).Fatal("Unable to start proxy")
		}
	}()
}

// Serve starts the reverse proxy server on the specified address and logs its startup state.
// It returns a humane.Error if the server fails to start.
func (p *Proxy) Serve(addr string) humane.Error {
	otelzap.L().Info("starting reverse proxy", zap.String("addr", addr))

	p.server = &http.Server{
		Addr:    addr,
		Handler: p,
	}

	if err := p.server.ListenAndServe(); err != nil {
		if strings.Contains(err.Error(), http.ErrServerClosed.Error()) {
			otelzap.L().Info("proxy server stopped", zap.String("addr", addr))
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

	otelzap.L().Info("shutting down proxy")
	if err := p.server.Shutdown(ctx); err != nil {
		return humane.Wrap(err, "Unable to shutdown proxy", "Make sure the proxy is running and try again.")
	}

	return nil
}

func (p *Proxy) probePath(ctx context.Context, url *url.URL, location string) (int, error) {
	// Start a span for the probePath method
	ctx, span := p.tracer.Start(ctx, "proxy.probePath", trace.WithAttributes(
		attribute.String("proxy_host", url.String()),
		attribute.String("target_path", location),
	))
	defer span.End()

	// create a http s3_client with short timeout for fast failure
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url.String()+location, nil)
	if err != nil {
		otelzap.L().WithError(err).Ctx(ctx).Error("failed to create request", zap.String("url", url.String()+location), zap.String("http.method", http.MethodHead))
		return http.StatusInternalServerError, err
	}

	// Inject trace context headers for the backend call
	req = req.WithContext(ctx)
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	resp, err := client.Do(req)
	if err != nil {
		if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
			otelzap.L().WithError(err).Ctx(ctx).Error("failed to probe path",
				zap.String("proxy_host", url.String()),
				zap.String("target_path", location),
			)
		}
		return http.StatusNotFound, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			otelzap.L().WithError(err).Ctx(ctx).Error("failed to close response body")
		}
	}(resp.Body)

	span.SetAttributes(attribute.Int("code", resp.StatusCode))
	span.SetStatus(codes.Ok, "")
	return resp.StatusCode, nil
}

func (p *Proxy) lookupPath(ctx context.Context, page *config.Page, sourceHost string, backendURL *url.URL, targetPath string) (string, humane.Error) {
	ctx, span := p.tracer.Start(ctx, "proxy.lookupPath", trace.WithAttributes(
		attribute.String("proxy_host", backendURL.String()),
		attribute.String("target_path", targetPath),
		attribute.String("source_host", sourceHost),
	))
	defer span.End()

	searchPaths := append([]string{""}, page.Proxy.SearchPath...)
	foundPath := make(chan string, 1)

	timeoutCtx, cancelTimeout := context.WithTimeout(ctx, 5*time.Second)
	defer cancelTimeout()

	probeCtx, cancelProbes := context.WithCancel(timeoutCtx)
	defer cancelProbes()

	var wg sync.WaitGroup

	for _, lookup := range searchPaths {
		wg.Add(1)

		go func(lookup string) {
			defer wg.Done()

			cacheKey := fmt.Sprintf("%s-%s-%s", sourceHost, targetPath, lookup)
			_, _ = p.group.Do(cacheKey, func() (interface{}, error) {
				testPath := path.Clean(fmt.Sprintf("/%s/%s", targetPath, lookup))
				statusCode, err := p.probePath(probeCtx, backendURL, testPath)
				if err != nil {
					return nil, humane.Wrap(err, "Unable to probe path", "Make sure the path exists and is accessible.")
				}

				if statusCode < http.StatusBadRequest {
					select {
					case foundPath <- testPath:
					case <-probeCtx.Done():
					}
				}
				return nil, nil
			})
		}(lookup)
	}

	go func() {
		wg.Wait()
		close(foundPath)
	}()

	select {
	case p, ok := <-foundPath:
		if ok {
			cancelProbes()
			return p, nil
		}
		return "", humane.New("No valid path found", "Make sure the path exists and is accessible.")
	case <-probeCtx.Done():
		return "", humane.New("Context cancelled", "Make sure the path exists and is accessible.")
	}
}
