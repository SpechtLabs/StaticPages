package proxy

import (
	"context"
	"fmt"
	"github.com/SpechtLabs/StaticPages/pkg/config"
	"github.com/golang/groupcache/singleflight"
	"github.com/sierrasoftworks/humane-errors-go"
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

type Proxy struct {
	zapLog *otelzap.Logger
	pages  map[string]*config.Page
	proxy  *httputil.ReverseProxy
	group  singleflight.Group
}

func NewProxy(zapLog *otelzap.Logger, pages []*config.Page) *Proxy {
	// construct a map for easier lookup in the director
	pagesMap := make(map[string]*config.Page)
	for _, page := range pages {
		if pagesMap[page.Domain] != nil {
			zapLog.Warn("duplicate page domain", zap.String("domain", page.Domain))
		}

		pagesMap[page.Domain] = page
	}

	p := &Proxy{
		zapLog: zapLog,
		pages:  pagesMap,
		proxy:  nil,
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

func (p *Proxy) Director(req *http.Request) {
	if req.Context().Err() != nil {
		p.zapLog.Ctx(req.Context()).Warn("request context canceled", zap.String("url", req.URL.String()), zap.String("path", req.URL.Path))
		return
	}

	originalPath := req.URL.Path
	requestUrl := req.Host
	if strings.Contains(requestUrl, ":") {
		requestUrl = strings.Split(requestUrl, ":")[0]
	}

	page, ok := p.pages[requestUrl]
	if !ok {
		p.zapLog.Ctx(req.Context()).Error("no page found for requestUrl", zap.String("requestUrl", requestUrl))
		return
	}

	backendUrl, err := url.Parse(page.Proxy.URL.String())
	if err != nil {
		p.zapLog.Ctx(req.Context()).Error("invalid target URL", zap.Error(err), zap.String("url", page.Proxy.URL.String()))
		return
	}

	// Find the actual html document we are looking for
	targetPath, err := p.lookupPath(req.Context(), page, requestUrl, backendUrl, path.Clean(fmt.Sprintf("/%s/%s", page.Proxy.Path, originalPath)))
	if err != nil {
		p.zapLog.Ctx(req.Context()).Error("no valid path found", zap.String("original_path", originalPath), zap.String("target_path", targetPath))
		return
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
	p.zapLog.Ctx(req.Context()).Debug("transforming request",
		zap.String("original_path", originalPath),
		zap.String("target_path", targetPath),
		zap.String("target_server", backendUrl.String()),
		zap.String("target_url", req.URL.String()),
	)
}

func (p *Proxy) ErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	responseCode := http.StatusBadGateway

	switch err.Error() {
	case "context canceled":
		responseCode = 499 // Nginx' non-standard code for when a client closes the connection
	}

	p.zapLog.Ctx(r.Context()).Error("proxy error",
		zap.String("error", err.Error()),
		zap.String("url", r.URL.String()),
		zap.Int("status", responseCode),
	)

	http.Error(w, err.Error(), responseCode)
}

func (p *Proxy) ModifyResponse(r *http.Response) error {
	if r.StatusCode >= 300 {
		if p.zapLog.Core().Enabled(zap.DebugLevel) {
			dump, _ := httputil.DumpResponse(r, true)

			p.zapLog.Ctx(r.Request.Context()).Debug("received response",
				zap.Int("status", r.StatusCode),
				zap.String("url", r.Request.URL.String()),
				zap.ByteString("url", dump),
			)
		} else {
			p.zapLog.Ctx(r.Request.Context()).Info("received response",
				zap.Int("status", r.StatusCode),
				zap.String("url", r.Request.URL.String()),
			)
		}
	}

	return nil
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		p.zapLog.Warn("received non-GET request",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
		)

		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	p.proxy.ServeHTTP(w, r)
}

func (p *Proxy) Serve(addr string) humane.Error {
	p.zapLog.Info("starting reverse proxy",
		zap.String("addr", addr),
	)

	server := &http.Server{
		Addr:    addr,
		Handler: p,
	}

	if err := server.ListenAndServe(); err != nil {
		return humane.Wrap(err, "failed to start proxy")
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
	if err != nil {
		return http.StatusNotFound, err
	}

	return resp.StatusCode, err
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
