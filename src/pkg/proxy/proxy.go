package proxy

import (
	"context"
	"fmt"
	"github.com/SpechtLabs/StaticPages/pkg/config"
	"github.com/golang/groupcache/singleflight"
	humane "github.com/sierrasoftworks/humane-errors-go"
	"github.com/spf13/viper"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
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
	zapLog   *otelzap.SugaredLogger
	pagesMap map[string]*config.Page
	conf     config.StaticPagesConfig
	proxy    *httputil.ReverseProxy
	group    singleflight.Group
	server   *http.Server
}

// NewProxy initializes and returns a new Proxy instance configured with the provided logger and page definitions.
func NewProxy(zapLog *otelzap.SugaredLogger, conf config.StaticPagesConfig) *Proxy {
	// construct a map for easier lookup in the director
	pagesMap := make(map[string]*config.Page)
	for _, page := range conf.Pages {
		if pagesMap[page.Domain] != nil {
			zapLog.Warnw("duplicate page domain", zap.String("domain", page.Domain))
		}

		pagesMap[page.Domain] = page
	}

	p := &Proxy{
		zapLog:   zapLog,
		pagesMap: pagesMap,
		proxy:    nil,
		conf:     conf,
		server:   nil,
	}

	p.proxy = &httputil.ReverseProxy{
		Director:       p.Director,       // Add a proxy director
		ErrorHandler:   p.ErrorHandler,   // Add error handler to log errors
		ModifyResponse: p.ModifyResponse, // Add response modifier to log response status

		// Allow transport configuration provided by user
		Transport: &http.Transport{
			MaxIdleConns:        viper.GetInt("proxy.maxIdleConns"),
			MaxIdleConnsPerHost: viper.GetInt("proxy.maxIdleConnsPerHost"),
			IdleConnTimeout:     viper.GetDuration("proxy.timeout"),
			DisableCompression:  !viper.GetBool("proxy.compression"),
		},
	}

	return p
}

// Director modifies the incoming HTTP request to route it to the appropriate backend server based on the request host and path.
func (p *Proxy) Director(req *http.Request) {
	if req.Context().Err() != nil {
		p.zapLog.Ctx(req.Context()).Warnw("request context canceled",
			zap.String("url", req.URL.String()),
			zap.String("path", req.URL.Path))
		return
	}

	originalPath := req.URL.Path
	requestUrl := req.Host
	if strings.Contains(requestUrl, ":") {
		requestUrl = strings.Split(requestUrl, ":")[0]
	}

	page, ok := p.pagesMap[requestUrl]
	if !ok {
		p.zapLog.Ctx(req.Context()).Errorw("no page found for requestUrl",
			zap.String("requestUrl", requestUrl))
		return
	}

	backendUrl, err := url.Parse(page.Proxy.URL.String())
	if err != nil {
		p.zapLog.Ctx(req.Context()).Errorw("invalid target URL",
			zap.Error(err),
			zap.String("url", page.Proxy.URL.String()))
		return
	}

	// Find the actual html document we are looking for
	targetPath, err := p.lookupPath(req.Context(), page, requestUrl, backendUrl, path.Clean(fmt.Sprintf("/%s/%s", page.Proxy.Path, originalPath)))
	if err != nil {
		p.zapLog.Ctx(req.Context()).Errorw("no valid path found",
			zap.String("original_path", originalPath),
			zap.String("target_path", targetPath))
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

	// Log the request transformation
	p.zapLog.Ctx(req.Context()).Debugw("transforming request",
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

	p.zapLog.Ctx(r.Context()).Errorw("proxy error",
		zap.String("error", err.Error()),
		zap.String("url", r.URL.String()),
		zap.Int("status", responseCode))

	http.Error(w, err.Error(), responseCode)
}

// ModifyResponse inspects and logs HTTP responses with a status code of 300 or higher, returning nil or an error.
func (p *Proxy) ModifyResponse(r *http.Response) error {
	if r.StatusCode >= 300 {
		if p.zapLog.Desugar().Core().Enabled(zap.DebugLevel) {
			dump, _ := httputil.DumpResponse(r, true)

			p.zapLog.Ctx(r.Request.Context()).Debugw("received response",
				zap.Int("status", r.StatusCode),
				zap.String("url", r.Request.URL.String()),
				zap.ByteString("url", dump))
		} else {
			p.zapLog.Ctx(r.Request.Context()).Infow("received response",
				zap.Int("status", r.StatusCode),
				zap.String("url", r.Request.URL.String()))
		}
	}

	return nil
}

// ServeHTTP handles incoming HTTP requests and proxies them to the configured backend, allowing only GET requests.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		p.zapLog.Ctx(r.Context()).Warnw("received non-GET request",
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
			p.zapLog.Fatalw("Unable to start proxy",
				zap.String("error", err.Error()),
				zap.Strings("advice", err.Advice()),
				zap.String("cause", err.Cause().Error()))
		}
	}()
}

// Serve starts the reverse proxy server on the specified address and logs its startup state.
// It returns a humane.Error if the server fails to start.
func (p *Proxy) Serve(addr string) humane.Error {
	p.zapLog.Infow("starting reverse proxy",
		zap.String("addr", addr))

	p.server = &http.Server{
		Addr:    addr,
		Handler: p,
	}

	if err := p.server.ListenAndServe(); err != nil {
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	p.zapLog.Info("shutting down proxy")
	if err := p.server.Shutdown(ctx); err != nil {
		return humane.Wrap(err, "Unable to shutdown proxy", "Make sure the proxy is running and try again.")
	}

	return nil
}

func (p *Proxy) probePath(ctx context.Context, url *url.URL, location string) (int, error) {
	// create a http client with short timeout for fast failure
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	req, _ := http.NewRequestWithContext(ctx, http.MethodHead, url.String()+location, nil)
	resp, err := client.Do(req)
	if resp != nil {
		return resp.StatusCode, err
	}

	return http.StatusNotFound, err
}

func (p *Proxy) lookupPath(ctx context.Context, page *config.Page, sourceHost string, backendUrl *url.URL, targetPath string) (string, humane.Error) {
	// Find the actual html document we are looking for
	searchPath := append([]string{""}, page.Proxy.SearchPath...)
	var wg sync.WaitGroup
	validPath := make(chan string, len(searchPath))

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
				statusCode, err := p.probePath(ctx, backendUrl, testTarget)
				if err == nil && statusCode < http.StatusBadRequest {
					validPath <- testTarget
					return testTarget, nil
				}

				return nil, humane.Wrap(err, "Unable to probe path", "Make sure the path exists and is accessible.")
			})
		}()
	}

	go func() {
		wg.Wait()
		close(validPath)
	}()

	if validPath, ok := <-validPath; ok {
		return validPath, nil
	}

	return "", humane.New("no valid path found", "Make sure the path exists and is accessible.")
}
