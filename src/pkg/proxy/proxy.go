package proxy

import (
	"fmt"
	"github.com/SpechtLabs/StaticPages/pkg/config"
	"github.com/sierrasoftworks/humane-errors-go"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.uber.org/zap"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"
)

type Proxy struct {
	zapLog *otelzap.Logger
	pages  map[string]*config.Page
	proxy  *httputil.ReverseProxy
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
		// Add a proxy director
		Director: p.Director,

		// Add error handler to log errors
		ErrorHandler: p.ErrorHandler,

		// Add response modifier to log response status
		ModifyResponse: p.ModifyResponse,
	}

	return p
}

func (p *Proxy) Director(req *http.Request) {
	host := req.Host

	if strings.Contains(host, ":") {
		host = strings.Split(host, ":")[0]
	}

	page, ok := p.pages[host]
	if !ok {
		p.zapLog.Error("no page found for host", zap.String("host", host))
		return
	}

	targetURL, err := url.Parse(page.Proxy.URL.String())
	if err != nil {
		p.zapLog.Error("invalid target URL", zap.Error(err), zap.String("url", page.Proxy.URL.String()))
		return
	}

	originalPath := req.URL.Path

	// Create a clean path without double slashes
	targetPath := path.Clean(fmt.Sprintf("/%s/%s",
		page.Proxy.Path,
		originalPath,
	))

	searchPath := append([]string{""}, page.Proxy.SearchPath...)

	for _, lookupPath := range searchPath {
		testTarget := path.Clean(fmt.Sprintf("/%s/%s",
			targetPath,
			lookupPath,
		))

		if resp, err := http.Head(targetURL.String() + testTarget); err == nil && resp.StatusCode < http.StatusBadRequest {
			targetPath = testTarget
			break
		}
	}

	req.URL.Scheme = targetURL.Scheme
	req.URL.Host = targetURL.Host
	req.URL.Path = targetPath

	// Clear the RequestURI as it's required for client requests
	req.RequestURI = ""

	// Set or update headers
	if _, ok := req.Header["User-Agent"]; !ok {
		req.Header.Set("User-Agent", "StaticPages-Proxy")
	}

	req.Header.Set("X-Forwarded-Host", req.Host)
	req.Header.Set("X-Origin-Host", targetURL.Host)

	// Log the request transformation
	p.zapLog.Debug("transforming request",
		zap.String("original_path", originalPath),
		zap.String("target_path", targetPath),
		zap.String("target_server", targetURL.String()),
		zap.String("target_url", req.URL.String()),
	)
}

func (p *Proxy) ErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	responseCode := http.StatusBadGateway

	p.zapLog.Error("proxy error",
		zap.String("error", err.Error()),
		zap.String("url", r.URL.String()),
	)

	switch err.Error() {
	case "context canceled":
		responseCode = 499 // Nginx' non-standard code for when a client closes the connection
	}

	http.Error(w, err.Error(), responseCode)
}

func (p *Proxy) ModifyResponse(r *http.Response) error {
	if r.StatusCode >= 300 {
		dump, _ := httputil.DumpResponse(r, true)

		p.zapLog.Debug("received response",
			zap.Int("status", r.StatusCode),
			zap.String("url", r.Request.URL.String()),
			zap.ByteString("url", dump),
		)
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
