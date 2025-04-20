package proxy

import (
	"fmt"
	"github.com/SpechtLabs/StaticPages/pkg/config"
	humane "github.com/sierrasoftworks/humane-errors-go"
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
	page   *config.Page
	proxy  *httputil.ReverseProxy
}

func NewProxy(zapLog *otelzap.Logger, page *config.Page) *Proxy {
	targetURL, err := url.Parse(page.Proxy.URL.String())
	if err != nil {
		return nil
	}

	rproxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			originalPath := req.URL.Path

			// Create a clean path without double slashes
			targetPath := path.Clean(fmt.Sprintf("/%s/%s/",
				page.Proxy.Path,
				originalPath,
			))

			if strings.HasSuffix(originalPath, "/") {
				for _, fallbackPath := range page.Proxy.SearchPath {
					testTarget := targetPath + fallbackPath
					if _, err := http.Head(targetURL.String() + testTarget); err == nil {
						targetPath = testTarget
						break
					}
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
			zapLog.Debug("transforming request",
				zap.String("original_path", originalPath),
				zap.String("target_path", targetPath),
				zap.String("target_server", targetURL.String()),
				zap.String("target_url", req.URL.String()),
			)
		},

		// Add error handler to log errors
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			zapLog.Error("proxy error",
				zap.Error(err),
				zap.String("url", r.URL.String()),
			)
			http.Error(w, "Bad Gateway", http.StatusBadGateway)
		},

		// Add response modifier to log response status
		ModifyResponse: func(r *http.Response) error {
			if r.StatusCode >= 400 {
				dump, _ := httputil.DumpResponse(r, true)

				zapLog.Debug("received response",
					zap.Int("status", r.StatusCode),
					zap.String("url", r.Request.URL.String()),
					zap.ByteString("url", dump),
				)
			} else {
				zapLog.Debug("received response",
					zap.Int("status", r.StatusCode),
					zap.String("url", r.Request.URL.String()),
				)
			}

			return nil
		},
	}

	proxy := &Proxy{
		zapLog: zapLog,
		page:   page,
		proxy:  rproxy,
	}

	return proxy
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

	p.zapLog.Info("forwarding request",
		zap.String("target", p.page.Domain),
		zap.String("path", r.URL.Path),
	)

	p.proxy.ServeHTTP(w, r)
}

func (p *Proxy) Serve(addr string) humane.Error {
	p.zapLog.Info("starting reverse proxy",
		zap.String("addr", addr),
		zap.String("domain", p.page.Domain),
		zap.Int("history", p.page.History),
	)

	server := &http.Server{
		Addr:    addr,
		Handler: p,
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return humane.Wrap(err, "failed to start server")
	}

	return nil
}
