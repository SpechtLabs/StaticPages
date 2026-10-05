package proxy

import (
	"context"
	"errors"
	"fmt"
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
	"github.com/sierrasoftworks/humane-errors-go"
	"github.com/spechtlabs/go-otel-utils/otelzap"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// readHeaderTimeout bounds how long a client may take to send its request
// headers, so slow clients can't hold connections open (Slowloris).
const readHeaderTimeout = 10 * time.Second

// Proxy represents a reverse proxy server with logging, page management, and request handling capabilities.
type Proxy struct {
	tracer      trace.Tracer
	pagesMap    config.DomainMapper
	proxy       *httputil.ReverseProxy
	server      *http.Server
	dnsResolver *net.Resolver // Custom DNS resolver using external DNS servers
	originCache sync.Map      // Cache of hostname -> resolved IP (thread-safe map)
	conf        config.StaticPagesConfig
}

// NewProxy initializes and returns a new Proxy instance configured with the provided logger and page definitions.
func NewProxy(conf config.StaticPagesConfig) *Proxy {
	// Create custom DNS resolver using external DNS servers (Google DNS and Cloudflare DNS)
	// This helps bypass local DNS that might return CDN IPs
	dnsResolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: 5 * time.Second,
			}
			// Try Google DNS first (8.8.8.8), fallback to Cloudflare DNS (1.1.1.1)
			conn, err := d.DialContext(ctx, "udp", "8.8.8.8:53")
			if err != nil {
				conn, err = d.DialContext(ctx, "udp", "1.1.1.1:53")
			}
			return conn, err
		},
	}

	p := &Proxy{
		pagesMap:    config.NewDomainMapperFromPages(conf.Pages),
		proxy:       nil,
		conf:        conf,
		server:      nil,
		tracer:      otel.Tracer("StaticPages-Proxy"),
		dnsResolver: dnsResolver,
	}

	// Create custom dialer for origin IP support
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	p.proxy = &httputil.ReverseProxy{
		Rewrite:        p.Rewrite,        // Point the outgoing request at the resolved target
		ErrorHandler:   p.ErrorHandler,   // Add error handler to log errors
		ModifyResponse: p.ModifyResponse, // Add response modifier to log response status

		// Allow transport configuration provided by user
		Transport: &http.Transport{
			DialContext:         p.createDialContext(dialer),
			MaxIdleConns:        conf.Proxy.MaxIdleConns,
			MaxIdleConnsPerHost: conf.Proxy.MaxIdleConnsPerHost,
			IdleConnTimeout:     conf.Proxy.Timeout,
			DisableCompression:  !conf.Proxy.Compression,
		},
	}

	return p
}

// createDialContext creates a custom DialContext function that bypasses local DNS
// by using external DNS servers (Google DNS, Cloudflare DNS) to avoid CDN loops
func (p *Proxy) createDialContext(dialer *net.Dialer) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		ctx, span := p.tracer.Start(ctx, "proxy.DialContext", trace.WithAttributes(
			attribute.String("network", network),
			attribute.String("addr", addr),
		))
		defer span.End()

		// Parse the host and port from addr
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			otelzap.L().WithError(err).Ctx(ctx).Error("failed to parse host:port", zap.String("addr", addr))
			return nil, err
		}

		// Always use external DNS resolution to avoid CloudFlare loops
		// This resolves the hostname using external DNS servers instead of local DNS
		originIP, err := p.resolveOriginIP(ctx, host)
		if err != nil {
			otelzap.L().WithError(err).Ctx(ctx).Warn("failed to resolve origin IP via external DNS, using default DNS",
				zap.String("host", host))
			// Fall back to default DNS resolution
			return dialer.DialContext(ctx, network, addr)
		}

		// Use the resolved IP
		resolvedAddr := net.JoinHostPort(originIP, port)
		span.SetAttributes(
			attribute.String("origin_ip", originIP),
			attribute.String("resolved_addr", resolvedAddr),
		)
		otelzap.L().Ctx(ctx).Debug("resolved origin IP via external DNS",
			zap.String("host", host),
			zap.String("origin_ip", originIP))

		return dialer.DialContext(ctx, network, resolvedAddr)
	}
}

// resolveOriginIP resolves the origin IP for a hostname using external DNS servers
// Results are cached to avoid repeated lookups
func (p *Proxy) resolveOriginIP(ctx context.Context, hostname string) (string, error) {
	ctx, span := p.tracer.Start(ctx, "proxy.resolveOriginIP", trace.WithAttributes(
		attribute.String("hostname", hostname),
	))
	defer span.End()

	// Check cache first (fast path)
	if cachedIP, ok := p.originCache.Load(hostname); ok {
		if ip, ok := cachedIP.(string); ok {
			span.SetAttributes(attribute.String("cached_ip", ip))
			return ip, nil
		}
	}

	// Resolve using external DNS
	ips, err := p.dnsResolver.LookupIP(ctx, "ip4", hostname)
	if err != nil {
		span.SetStatus(codes.Error, "DNS resolution failed")
		return "", humane.Wrap(err, fmt.Sprintf("failed to resolve %s", hostname),
			"Make sure the backend host exists and public DNS (8.8.8.8, 1.1.1.1) is reachable.")
	}

	if len(ips) == 0 {
		span.SetStatus(codes.Error, "No IPs found")
		return "", humane.New(fmt.Sprintf("no IPs found for %s", hostname),
			"Make sure the backend host has an A record.")
	}

	// Use the first IP
	resolvedIP := ips[0].String()

	// Cache the result using LoadOrStore to avoid race condition
	// If another goroutine already cached it, use that value instead
	if actualIP, loaded := p.originCache.LoadOrStore(hostname, resolvedIP); loaded {
		if ip, ok := actualIP.(string); ok {
			span.SetAttributes(attribute.String("cached_ip", ip))
			return ip, nil
		}
	}

	span.SetAttributes(attribute.String("resolved_ip", resolvedIP))
	otelzap.L().Ctx(ctx).Debug("resolved origin IP",
		zap.String("hostname", hostname),
		zap.String("ip", resolvedIP))

	return resolvedIP, nil
}

// ctxResolvedTarget is the context key under which ServeHTTP stashes the
// resolved backend target for Rewrite and ModifyResponse to consume.
type ctxResolvedTarget struct{}

// resolvedTarget is the outcome of mapping an inbound request to a concrete
// object on the storage backend.
type resolvedTarget struct {
	backendURL *url.URL
	path       string
	// isNotFound is true when we fell back to the page's configured not-found
	// document rather than the requested object. The response status is then
	// rewritten to 404 so the fallback is not mistaken for a valid page.
	isNotFound bool
}

// resolveTarget maps an inbound request to a concrete backend object: it finds
// the page for the host, resolves the commit to serve, and probes for the
// requested path (falling back to the page's not-found document). It returns a
// humane.Error when the request cannot be resolved, so the caller can serve a
// clean 404 instead of letting the reverse proxy fail on a half-built request.
func (p *Proxy) resolveTarget(ctx context.Context, req *http.Request) (*resolvedTarget, humane.Error) {
	ctx, span := p.tracer.Start(ctx, "proxy.resolveTarget", trace.WithAttributes(
		attribute.String("http.method", req.Method),
		attribute.String("http.url", req.Host),
		attribute.String("http.path", req.URL.String()),
	))
	defer span.End()

	if ctx.Err() != nil {
		return nil, humane.Wrap(ctx.Err(), "request context canceled", "The client closed the connection before the request could be served.")
	}

	originalPath := req.URL.Path

	requestUrl := req.Host
	if strings.Contains(req.Host, ":") {
		var err error
		requestUrl, _, err = net.SplitHostPort(req.Host)
		if err != nil {
			return nil, humane.Wrap(err, "unable to parse request url", "Make sure the request targets a valid host.")
		}
	}

	page := p.pagesMap.Lookup(requestUrl)
	if page == nil {
		return nil, humane.New("no page configured for host", "Make sure a page is configured for this domain.")
	}

	backendUrl, err := url.Parse(page.Proxy.URL.String())
	if err != nil {
		otelzap.L().WithError(err).Ctx(ctx).Error("unable to parse proxy.url", zap.String("backend_url", page.Proxy.URL.String()))
		return nil, humane.Wrap(err, "unable to parse configured proxy url", "Make sure pages[].proxy.url is a valid URL.")
	}

	metadata, mErr := s3_client.GetPageMetadata(ctx, page)
	if mErr != nil {
		otelzap.L().WithError(mErr).Ctx(ctx).Error("unable to get metadata", zap.String("domain", page.Domain.String()))
	}

	// Find the actual html document we are looking for
	lookupPath := path.Join(path.Clean(page.Proxy.Path.String()), path.Clean(page.Git.Repository))

	sub, err := page.Domain.Subdomain(requestUrl)
	if err != nil {
		return nil, humane.Wrap(err, "unable to parse subdomain", "Make sure the request host belongs to the configured domain.")
	}

	sub, resolvedSHA, herr := resolveCommit(page, metadata, sub)
	if herr != nil {
		return nil, herr
	}
	lookupPath = path.Join(lookupPath, path.Clean(resolvedSHA))

	span.SetAttributes(
		attribute.String("proxy.domain", page.Domain.String()),
		attribute.String("proxy.subdomain", sub),
		attribute.String("proxy.resolved_sha", resolvedSHA),
		attribute.String("proxy.repository", page.Git.Repository),
	)
	otelzap.L().Ctx(ctx).Debug("resolved commit for request",
		zap.String("request_url", requestUrl),
		zap.String("subdomain", sub),
		zap.String("sha", resolvedSHA),
		zap.String("base_lookup_path", lookupPath))

	return p.resolvePath(ctx, page, requestUrl, backendUrl, lookupPath, originalPath)
}

// resolvePath finds the requested path, or failing that the page's not-found
// document, under the commit's lookupPath on the backend.
func (p *Proxy) resolvePath(ctx context.Context, page *config.Page, requestUrl string, backendUrl *url.URL, lookupPath, originalPath string) (*resolvedTarget, humane.Error) {
	span := trace.SpanFromContext(ctx)
	lookupRequestPath := joinPagePath(page, lookupPath, originalPath)

	otelzap.L().Ctx(ctx).Debug("constructed lookup path",
		zap.String("original_path", originalPath),
		zap.String("lookup_request_path", lookupRequestPath),
		zap.String("proxy_path", page.Proxy.Path.String()),
		zap.Strings("search_paths", page.Proxy.SearchPath))

	if targetPath, lErr := p.lookupPath(ctx, page, requestUrl, backendUrl, lookupRequestPath); lErr == nil {
		span.SetAttributes(
			attribute.String("proxy.resolved_path", targetPath),
			attribute.Bool("proxy.not_found_fallback", false),
		)
		otelzap.L().Ctx(ctx).Debug("successfully resolved path",
			zap.String("request_path", originalPath),
			zap.String("target_path", targetPath))
		return &resolvedTarget{backendURL: backendUrl, path: targetPath}, nil
	}

	// Requested path not found — fall back to the page's configured 404 document.
	lookup404Path := joinPagePath(page, lookupPath, page.Proxy.NotFound)

	otelzap.L().Ctx(ctx).Warn("original path not found, attempting 404 fallback",
		zap.String("request_path", originalPath),
		zap.String("lookup_path", lookupRequestPath),
		zap.String("not_found_page", page.Proxy.NotFound),
		zap.String("lookup_404_path", lookup404Path))
	targetPath, err404 := p.lookupPath(ctx, page, requestUrl, backendUrl, lookup404Path)
	if err404 != nil {
		return nil, humane.New("no path found and 404 page not available",
			"Configure a valid pages[].proxy.notFound document to serve for missing paths.")
	}

	span.SetAttributes(
		attribute.String("proxy.resolved_path", targetPath),
		attribute.Bool("proxy.not_found_fallback", true),
	)
	otelzap.L().Ctx(ctx).Info("serving 404 page",
		zap.String("request_path", originalPath),
		zap.String("404_path", targetPath))
	return &resolvedTarget{backendURL: backendUrl, path: targetPath, isNotFound: true}, nil
}

// resolveCommit picks the commit to serve. Without previews, or without a
// subdomain, that's the latest commit on the page's main branch; otherwise the
// subdomain names a branch, whose latest commit it is, or a commit SHA. It
// returns the branch or SHA it looked up and the commit.
func resolveCommit(page *config.Page, metadata s3_client.PageIndex, sub string) (string, string, humane.Error) {
	if !page.Preview.Enabled || sub == "" {
		sha, _, err := metadata.GetLatestForBranch(page.Git.MainBranch)
		if err != nil {
			return "", "", humane.Wrap(err, "could not find a commit to serve page for",
				"Make sure the page has been published for its main branch.")
		}
		return page.Git.MainBranch, sha, nil
	}

	if sha, _, err := metadata.GetLatestForBranch(sub); err == nil {
		return sub, sha, nil
	}

	if _, err := metadata.GetBySHA(sub); err == nil {
		return sub, sub, nil
	}

	return "", "", humane.New("could not find a commit to serve page for",
		"Make sure the requested branch or commit has been published.")
}

// joinPagePath joins the request or not-found path p onto base. Without a
// proxy.path the joined path stays relative, so p's leading / is dropped.
func joinPagePath(page *config.Page, base, p string) string {
	if page.Proxy.Path.String() == "" {
		return path.Join(base, strings.TrimPrefix(path.Clean(p), "/"))
	}

	return path.Join(base, path.Clean(p))
}

// Rewrite applies the target resolved by resolveTarget to the outgoing
// request. ServeHTTP only proxies requests it has already resolved, so the
// target is always present in the request context.
//
// ReverseProxy strips the inbound X-Forwarded-* headers from the outgoing
// request before calling Rewrite. Carrying X-Forwarded-For over and calling
// SetXForwarded appends the client address to the chain, as Director-based
// proxies did on their own, and sets X-Forwarded-Host to the original host.
func (p *Proxy) Rewrite(pr *httputil.ProxyRequest) {
	ctx := pr.In.Context()

	target, ok := ctx.Value(ctxResolvedTarget{}).(*resolvedTarget)
	if !ok || target == nil {
		otelzap.L().Ctx(ctx).Error("rewrite invoked without a resolved target", zap.String("host", pr.In.Host))
		return
	}

	// Save original host for logging and forwarding headers
	originalHost := pr.In.Host

	pr.Out.Header["X-Forwarded-For"] = pr.In.Header["X-Forwarded-For"]
	pr.SetXForwarded()

	req := pr.Out
	req.URL.Scheme = target.backendURL.Scheme
	req.URL.Host = target.backendURL.Host
	req.URL.Path = target.path

	// Clear the RequestURI as it's required for s3_client requests
	req.RequestURI = ""

	// Set Host header to backend host for virtual hosting (critical for CDNs)
	req.Host = target.backendURL.Host

	// Set or update headers
	if _, ok := req.Header["User-Agent"]; !ok {
		req.Header.Set("User-Agent", "StaticPages-Proxy")
	}

	req.Header.Set("X-Origin-Host", target.backendURL.Host)

	// Inject trace context headers for the backend call
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	otelzap.L().Ctx(ctx).Debug("proxying request",
		zap.String("original_host", originalHost),
		zap.String("proxy_url", req.URL.String()),
		zap.String("backend_host", target.backendURL.Host),
		zap.String("backend_path", target.path),
		zap.Bool("not_found_fallback", target.isNotFound))
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

	if errors.Is(err, context.Canceled) {
		responseCode = api.StatusRequestContextCanceled // Nginx non-standard code for when the client closes the connection
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
		attribute.Int64("content_length", r.ContentLength),
		attribute.Int("status_code", r.StatusCode),
	))
	defer span.End()

	// Strip the storage CDN's Speculation-Rules header. It points at
	// /cdn-cgi/speculation, a Cloudflare-internal endpoint that does not exist
	// through this proxy: forwarding it makes the browser fetch a URL that
	// 404s and drives prefetching that races real navigation.
	r.Header.Del("Speculation-Rules")

	switch {
	case r.StatusCode >= 400:
		// Client/Server error responses
		if otelzap.L().Core().Enabled(zap.DebugLevel) {
			dump, _ := httputil.DumpResponse(r, true)

			otelzap.L().Ctx(ctx).Warn("received unsuccessful response from backend",
				zap.Int("status_code", r.StatusCode),
				zap.String("status", r.Status),
				zap.String("request_url", r.Request.URL.String()),
				zap.String("content_type", r.Header.Get("Content-Type")),
				zap.Int64("content_length", r.ContentLength),
				zap.ByteString("response_dump", dump))
		} else {
			otelzap.L().Ctx(ctx).Warn("received unsuccessful response from backend",
				zap.Int("status_code", r.StatusCode),
				zap.String("status", r.Status),
				zap.String("request_url", r.Request.URL.String()),
				zap.String("content_type", r.Header.Get("Content-Type")),
				zap.Int64("content_length", r.ContentLength))
		}
	case r.StatusCode >= 300:
		// Redirect responses
		otelzap.L().Ctx(ctx).Debug("received redirect response",
			zap.Int("status_code", r.StatusCode),
			zap.String("status", r.Status),
			zap.String("location", r.Header.Get("Location")),
			zap.String("request_url", r.Request.URL.String()))
	default:
		// Success responses
		otelzap.L().Ctx(ctx).Debug("received successful response",
			zap.Int("status_code", r.StatusCode),
			zap.String("request_url", r.Request.URL.String()),
			zap.String("content_type", r.Header.Get("Content-Type")),
			zap.Int64("content_length", r.ContentLength))
	}

	// When we served the page's configured not-found document, report it
	// honestly as a 404 instead of passing through the storage backend's 200.
	// A soft-404 (200 body for a missing page) poisons CDN/browser caches and
	// makes SPAs flicker through the 404 page before self-correcting.
	if target, ok := r.Request.Context().Value(ctxResolvedTarget{}).(*resolvedTarget); ok && target != nil && target.isNotFound {
		otelzap.L().Ctx(ctx).Debug("rewriting backend status to 404 for not-found fallback",
			zap.Int("backend_status", r.StatusCode))
		r.StatusCode = http.StatusNotFound
		r.Status = http.StatusText(http.StatusNotFound)
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
		// Resolve the request to a concrete backend object before proxying.
		// If it cannot be resolved (unknown host, unpublished branch/commit,
		// missing path with no 404 document) serve a clean 404 rather than
		// letting the reverse proxy fail on a half-built request with a 502.
		target, herr := p.resolveTarget(ctx, req)
		if herr != nil {
			otelzap.L().WithError(herr).Ctx(ctx).Warn("unable to resolve request; serving 404",
				zap.String("http.url", req.Host),
				zap.String("http.path", req.URL.String()))
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}

		req = req.WithContext(context.WithValue(ctx, ctxResolvedTarget{}, target))
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

// ServeAsync starts the reverse proxy on addr in a goroutine and returns at
// once; Shutdown stops it. If the proxy fails to start, it logs a fatal error.
func (p *Proxy) ServeAsync(addr string) {
	p.server = p.newServer(addr)
	go func(srv *http.Server) {
		if err := api.ListenAndServe(srv, "reverse proxy"); err != nil {
			otelzap.L().WithError(err).Fatal("Unable to start proxy", zap.String("addr", srv.Addr))
		}
	}(p.server)
}

// Serve starts the reverse proxy server on the specified address and logs its startup state.
// It returns a humane.Error if the server fails to start.
func (p *Proxy) Serve(addr string) humane.Error {
	p.server = p.newServer(addr)
	return api.ListenAndServe(p.server, "reverse proxy")
}

func (p *Proxy) newServer(addr string) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           p,
		ReadHeaderTimeout: readHeaderTimeout,
	}
}

// Shutdown gracefully stops the proxy server if it is running, releasing any resources and handling in-progress requests.
// It returns a humane.Error if the server fails to stop.
func (p *Proxy) Shutdown() humane.Error {
	if p.server == nil {
		return humane.New("Unable to shutdown proxy. It is not running.", "Start Proxy first before attempting to stop it")
	}

	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()

	otelzap.L().Info("shutting down proxy", zap.String("addr", p.server.Addr))
	if err := p.server.Shutdown(ctx); err != nil {
		return humane.Wrap(err, "Unable to shutdown proxy", "Make sure the proxy is running and try again.")
	}

	return nil
}

// statusProbeInconclusive is returned by probePath when the origin did not
// answer within the probe budget. It is not a real HTTP status: it signals
// that existence could not be determined, as opposed to a definitive >= 400.
const statusProbeInconclusive = 0

// probeTimeout returns the per-probe HEAD timeout, falling back to a sane
// default when the configuration leaves it unset.
func (p *Proxy) probeTimeout() time.Duration {
	if p.conf.Proxy.ProbeTimeout > 0 {
		return p.conf.Proxy.ProbeTimeout
	}
	return 2 * time.Second
}

// probeClient returns an HTTP client for path probes, with the short probe
// timeout for fast failure and the same origin DNS bypass as the proxy.
func (p *Proxy) probeClient(probeTimeout time.Duration) *http.Client {
	// Create custom dialer for origin IP support
	dialer := &net.Dialer{
		Timeout:   probeTimeout,
		KeepAlive: 30 * time.Second,
	}

	return &http.Client{
		Timeout: probeTimeout,
		Transport: &http.Transport{
			DialContext: p.createDialContext(dialer),
		},
		// Don't follow backend redirects while probing. Backblaze answers some
		// requests with a 3xx to a download host; following it turns a single
		// probe into a cascade of extra HEADs that mostly 404 and inflate the
		// outbound error rate. Surface the redirect and let the proxied GET
		// pass it through to the client instead.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// isProbeTimeout reports whether a probe error is a timeout (the per-probe
// client deadline or the overall lookup deadline) rather than a definitive
// failure such as a refused connection. A probe context that was canceled
// because a sibling probe already succeeded is not a timeout.
func isProbeTimeout(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var nerr net.Error
	return errors.As(err, &nerr) && nerr.Timeout()
}

func (p *Proxy) probePath(ctx context.Context, backendURL *url.URL, location string) (int, humane.Error) {
	// Start a span for the probePath method
	ctx, span := p.tracer.Start(ctx, "proxy.probePath", trace.WithAttributes(
		attribute.String("proxy_host", backendURL.String()),
		attribute.String("target_path", location),
	))
	defer span.End()

	probeTimeout := p.probeTimeout()
	client := p.probeClient(probeTimeout)

	// Construct URL properly: ensure path starts with / for valid HTTP URL
	// When Proxy.Path is empty, location might not start with /, so we need to add it
	pathToUse := location
	if !strings.HasPrefix(pathToUse, "/") {
		pathToUse = "/" + pathToUse
	}

	// Use url.URL methods to properly construct the full URL
	fullURL := *backendURL
	fullURL.Path = pathToUse
	fullURLString := fullURL.String()

	otelzap.L().Ctx(ctx).Debug("probing path",
		zap.String("full_url", fullURLString),
		zap.String("base_url", backendURL.String()),
		zap.String("path", pathToUse),
		zap.String("hostname", backendURL.Hostname()))

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, fullURLString, nil)
	if err != nil {
		otelzap.L().WithError(err).Ctx(ctx).Error("failed to create request", zap.String("url", fullURLString), zap.String("http.method", http.MethodHead))
		return http.StatusInternalServerError, humane.Wrap(err, "failed to create probe request", "Check the page's proxy.url and proxy.path.")
	}

	// Ensure Host header is set correctly for virtual hosting (important for CDNs)
	req.Host = backendURL.Host

	// Inject trace context headers for the backend call
	req = req.WithContext(ctx)
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	otelzap.L().Ctx(ctx).Debug("sending probe request",
		zap.String("url", fullURLString),
		zap.String("host_header", req.Host),
		zap.String("method", http.MethodHead))

	resp, err := client.Do(req)
	if err != nil {
		return probeFailure(ctx, err, fullURLString, probeTimeout)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			otelzap.L().WithError(err).Ctx(ctx).Error("failed to close response body", zap.String("full_url", fullURLString))
		}
	}()

	probeOutcome := "hit"
	if resp.StatusCode >= http.StatusBadRequest {
		probeOutcome = "miss"
	}
	span.SetAttributes(
		attribute.Int("http.status_code", resp.StatusCode),
		attribute.String("proxy.probe.outcome", probeOutcome),
	)
	otelzap.L().Ctx(ctx).Debug("path probe answered",
		zap.String("full_url", fullURLString),
		zap.Int("status_code", resp.StatusCode),
		zap.String("outcome", probeOutcome))

	span.SetStatus(codes.Ok, "")
	return resp.StatusCode, nil
}

// probeFailure classifies a probe that got no HTTP response. A probe that
// times out tells us nothing about whether the object exists — the origin was
// simply too slow to confirm within the probe budget (e.g. a cold CDN cache
// miss). It is reported as inconclusive so the caller can still proxy the
// object rather than treating it as a hard 404. A definitive negative only
// comes from an actual HTTP response.
func probeFailure(ctx context.Context, err error, fullURL string, probeTimeout time.Duration) (int, humane.Error) {
	span := trace.SpanFromContext(ctx)

	if isProbeTimeout(err) {
		span.SetAttributes(attribute.String("proxy.probe.outcome", "inconclusive"))
		otelzap.L().Ctx(ctx).Debug("path probe timed out (inconclusive)",
			zap.String("full_url", fullURL),
			zap.Duration("probe_timeout", probeTimeout))
		return statusProbeInconclusive, humane.Wrap(err, "path probe timed out", "The backend may be slow; the object is proxied unconfirmed.")
	}

	span.SetAttributes(attribute.String("proxy.probe.outcome", "error"))
	if !errors.Is(err, context.Canceled) {
		otelzap.L().WithError(err).Ctx(ctx).Warn("failed to probe path", zap.String("full_url", fullURL))
	}
	return http.StatusNotFound, humane.Wrap(err, "failed to probe path", "Make sure the backend is reachable.")
}

// buildProbePath constructs a candidate backend path for a single search-path
// entry. An empty lookup is the requested path itself. A lookup that begins
// with "." (e.g. ".html") is a filename *suffix* appended to the requested
// path — this resolves clean URLs (/guides/x -> /guides/x.html) the way static
// generators like VitePress deploy them. Any other lookup is a sub-path joined
// onto the requested path (e.g. "/index.html" -> /guides/x/index.html).
func buildProbePath(proxyPathEmpty bool, targetPath, lookup string) string {
	base := targetPath
	if !proxyPathEmpty {
		base = path.Clean("/" + targetPath)
	}

	switch {
	case lookup == "":
		return base
	case strings.HasPrefix(lookup, "."):
		// Filename suffix (clean URL). Append directly; do not path.Join, which
		// would turn ".html" into a "/.html" directory entry.
		return base + lookup
	case proxyPathEmpty:
		return path.Join(targetPath, lookup)
	default:
		return path.Clean(fmt.Sprintf("/%s/%s", targetPath, lookup))
	}
}

// lookupPath probes the requested path and each search-path variant of it at
// once, and returns the first the origin confirms. Every probe's result fits in
// the results channel, so none of them blocks; on return the canceled context
// stops the probes still in flight before it waits for them.
func (p *Proxy) lookupPath(ctx context.Context, page *config.Page, sourceHost string, backendURL *url.URL, targetPath string) (string, humane.Error) {
	ctx, span := p.tracer.Start(ctx, "proxy.lookupPath", trace.WithAttributes(
		attribute.String("proxy_host", backendURL.String()),
		attribute.String("target_path", targetPath),
		attribute.String("source_host", sourceHost),
	))
	defer span.End()

	testedPaths := candidatePaths(page, targetPath)

	otelzap.L().Ctx(ctx).Debug("starting path lookup",
		zap.String("target_path", targetPath),
		zap.Strings("search_paths", page.Proxy.SearchPath),
		zap.String("backend_url", backendURL.String()))

	var wg sync.WaitGroup
	defer wg.Wait()

	probeCtx, cancelProbes := context.WithTimeout(ctx, 5*time.Second)
	defer cancelProbes()

	results := make(chan probeResult, len(testedPaths))
	for i, testPath := range testedPaths {
		wg.Go(func() {
			// The first candidate is the exact requested path.
			results <- p.probeCandidate(probeCtx, backendURL, testPath, i == 0)
		})
	}

	// inconclusivePrimary holds the exact requested path when its probe could
	// not be confirmed (origin too slow). It is used as a last resort so a
	// slow-but-existing object is still proxied rather than 404'd.
	var inconclusivePrimary string

	timedOut := false
probes:
	for range len(testedPaths) {
		select {
		case result := <-results:
			switch {
			case result.statusCode >= http.StatusOK && result.statusCode < http.StatusBadRequest:
				span.SetAttributes(
					attribute.String("proxy.lookup.outcome", "found"),
					attribute.String("proxy.lookup.resolved_path", result.path),
				)
				return result.path, nil

			case result.statusCode == statusProbeInconclusive && result.primary:
				inconclusivePrimary = result.path
			}

		case <-probeCtx.Done():
			timedOut = true
			break probes
		}
	}

	if timedOut {
		span.SetAttributes(
			attribute.String("proxy.lookup.outcome", "timeout"),
			attribute.StringSlice("proxy.lookup.tested_paths", testedPaths),
		)
		otelzap.L().Ctx(ctx).Warn("path lookup timed out",
			zap.String("target_path", targetPath),
			zap.Strings("tested_paths", testedPaths))

		return "", humane.New("Path lookup timed out", "Make sure the path exists and the backend answers within 5 seconds.")
	}

	return noPathConfirmed(ctx, targetPath, testedPaths, backendURL, inconclusivePrimary)
}

// noPathConfirmed settles a lookup whose probes all finished without a
// definitive hit. If the exact requested object probe was merely inconclusive
// (origin too slow), it is proxied anyway: the downstream GET uses the longer
// proxy timeout and will return the real content — or a real error — instead
// of us inventing a 404 for a file that may exist.
func noPathConfirmed(ctx context.Context, targetPath string, testedPaths []string, backendURL *url.URL, inconclusivePrimary string) (string, humane.Error) {
	span := trace.SpanFromContext(ctx)

	if inconclusivePrimary != "" {
		span.SetAttributes(
			attribute.String("proxy.lookup.outcome", "inconclusive_proxied"),
			attribute.String("proxy.lookup.resolved_path", inconclusivePrimary),
		)
		otelzap.L().Ctx(ctx).Info("primary path probe inconclusive; proxying object without confirmation",
			zap.String("target_path", targetPath),
			zap.String("path_to_return", inconclusivePrimary))
		return inconclusivePrimary, nil
	}

	span.SetAttributes(
		attribute.String("proxy.lookup.outcome", "not_found"),
		attribute.StringSlice("proxy.lookup.tested_paths", testedPaths),
	)
	otelzap.L().Ctx(ctx).Warn("no valid path found after testing all options",
		zap.String("target_path", targetPath),
		zap.Strings("tested_paths", testedPaths),
		zap.String("backend_url", backendURL.String()))

	return "", humane.New("No valid path found", "Make sure the path exists and is accessible.")
}

// candidatePaths lists the paths lookupPath probes: the requested path first,
// then one per search-path entry.
func candidatePaths(page *config.Page, targetPath string) []string {
	searchPaths := append([]string{""}, page.Proxy.SearchPath...)
	paths := make([]string, 0, len(searchPaths))
	for _, lookup := range searchPaths {
		paths = append(paths, buildProbePath(page.Proxy.Path.String() == "", targetPath, lookup))
	}

	return paths
}

// probeResult is what the origin said about one candidate path.
type probeResult struct {
	// path is the candidate with a leading /, as lookupPath hands it back.
	path       string
	statusCode int
	// primary marks the exact requested path, as opposed to a search-path
	// variant of it.
	primary bool
}

// probeCandidate probes one candidate path for lookupPath.
func (p *Proxy) probeCandidate(ctx context.Context, backendURL *url.URL, testPath string, primary bool) probeResult {
	statusCode, err := p.probePath(ctx, backendURL, testPath)

	// Ensure any path we hand back has a leading / for a valid HTTP URL.
	pathToReturn := testPath
	if !strings.HasPrefix(pathToReturn, "/") {
		pathToReturn = "/" + pathToReturn
	}

	switch {
	case statusCode >= http.StatusOK && statusCode < http.StatusBadRequest:
		// Definitive success: the origin confirmed this path exists.
		otelzap.L().Ctx(ctx).Debug("found valid path",
			zap.String("test_path", testPath),
			zap.String("path_to_return", pathToReturn),
			zap.Int("status_code", statusCode))

	case err != nil:
		// A timeout, or a definitive probe failure (e.g. connection
		// refused). lookupPath only falls back on a timed-out primary path.
		otelzap.L().Ctx(ctx).Debug("probe did not resolve",
			zap.String("test_path", testPath),
			zap.Int("status_code", statusCode))
	}

	return probeResult{path: pathToReturn, statusCode: statusCode, primary: primary}
}
