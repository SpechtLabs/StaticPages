package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SpechtLabs/StaticPages/pkg/config"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/spechtlabs/go-otel-utils/otelzap"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func initLogger() *bytes.Buffer {
	// Capture logs for later assertions
	enc := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
	buf := &bytes.Buffer{}
	// Wrap the buffer writer with a mutex to make it thread-safe for concurrent logging
	writer := zapcore.Lock(zapcore.AddSync(buf))
	level := zap.NewAtomicLevelAt(zapcore.DebugLevel)

	otelZapLogger := otelzap.New(zap.New(zapcore.NewCore(enc, writer, level)))
	otelzap.ReplaceGlobals(otelZapLogger)

	return buf
}

// A probe must not follow backend redirects: Backblaze answers some requests
// with a 3xx to a download host (f003.backblazeb2.com), and following it turns
// one probe into a storm of extra HEADs that mostly 404. The probe should
// report the redirect itself and let the downstream GET decide.
func TestProbePathDoesNotFollowRedirects(t *testing.T) {
	initLogger()

	var redirectTargetHits int32
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/source":
			http.Redirect(w, r, "/target", http.StatusFound)
		case "/target":
			atomic.AddInt32(&redirectTargetHits, 1)
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer backend.Close()

	p := NewProxy(config.StaticPagesConfig{})
	backendURL, err := url.Parse(backend.URL)
	assert.NoError(t, err)

	code, probeErr := p.probePath(context.Background(), backendURL, "/source")
	assert.NoError(t, probeErr)
	assert.Equal(t, http.StatusFound, code, "probe should surface the redirect, not follow it")
	assert.Equal(t, int32(0), atomic.LoadInt32(&redirectTargetHits), "probe must not have followed the redirect")
}

// Two requests for the same URL that overlap (e.g. a prefetch racing the real
// navigation) must both resolve. A shared singleflight that signals success via
// a per-request channel drops the result for the deduped caller, 404'ing it.
func TestProxyConcurrentSamePathBothResolve(t *testing.T) {
	initLogger()

	const commit = mockCommit
	var headHits int32
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqPath, _ := strings.CutPrefix(r.URL.Path, "/"+commit)
		if reqPath == "/page.html" {
			if r.Method == http.MethodHead {
				atomic.AddInt32(&headHits, 1)
				// Hold the winning probe open so a second request overlaps it.
				time.Sleep(200 * time.Millisecond)
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("Hello from backend"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer backend.Close()

	test := testProxyServer{domain: "example.com"}
	s3Backend := setupMockS3(&test)
	defer s3Backend.Close()

	proxy := NewProxy(config.StaticPagesConfig{
		Pages: []*config.Page{{
			Domain: config.FromString("example.com"),
			Proxy: config.PageProxy{
				URL:        config.EnvValue(backend.URL),
				SearchPath: []string{".html"},
			},
			Bucket: config.BucketConfig{
				URL: config.EnvValue(s3Backend.URL), Name: "test",
				ApplicationID: "test", Secret: "test", Region: "test",
			},
		}},
	})

	const n = 2
	var wg sync.WaitGroup
	codes := make([]int, n)
	bodies := make([]string, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "http://example.com/page", nil)
			rr := httptest.NewRecorder()
			proxy.ServeHTTP(rr, req)
			codes[i] = rr.Code
			bodies[i] = rr.Body.String()
		}(i)
	}
	wg.Wait()

	for i := 0; i < n; i++ {
		assert.Equal(t, http.StatusOK, codes[i], "request %d should resolve the page", i)
		assert.Equal(t, "Hello from backend", bodies[i], "request %d body", i)
	}
}

// Cloudflare injects a `Speculation-Rules: "/cdn-cgi/speculation"` response
// header. Forwarding it makes the browser fetch /cdn-cgi/speculation from the
// proxy origin, which 404s (and the rules drive prefetch that races
// navigation). The proxy must strip it.
func TestProxyStripsSpeculationRulesHeader(t *testing.T) {
	initLogger()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqPath, _ := strings.CutPrefix(r.URL.Path, "/"+mockCommit)
		if reqPath == "/page.html" {
			if r.Method == http.MethodHead {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.Header().Set("Speculation-Rules", `"/cdn-cgi/speculation"`)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("Hello from backend"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer backend.Close()

	test := testProxyServer{domain: "example.com"}
	s3Backend := setupMockS3(&test)
	defer s3Backend.Close()

	proxy := NewProxy(config.StaticPagesConfig{
		Pages: []*config.Page{{
			Domain: config.FromString("example.com"),
			Proxy: config.PageProxy{
				URL:        config.EnvValue(backend.URL),
				SearchPath: []string{".html"},
			},
			Bucket: config.BucketConfig{
				URL: config.EnvValue(s3Backend.URL), Name: "test",
				ApplicationID: "test", Secret: "test", Region: "test",
			},
		}},
	})

	req := httptest.NewRequest(http.MethodGet, "http://example.com/page", nil)
	rr := httptest.NewRecorder()
	proxy.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "Hello from backend", rr.Body.String())
	assert.Empty(t, rr.Header().Get("Speculation-Rules"), "Cloudflare speculation-rules header must not be forwarded")
}

func TestBuildProbePath(t *testing.T) {
	const target = "repo/sha/guides/shell-integration"

	tests := []struct {
		name           string
		proxyPathEmpty bool
		lookup         string
		want           string
	}{
		{"empty lookup, no proxy path", true, "", target},
		{"empty lookup, with proxy path", false, "", "/" + target},
		{"html suffix resolves clean URL", true, ".html", target + ".html"},
		{"htm suffix", true, ".htm", target + ".htm"},
		{"html suffix with proxy path", false, ".html", "/" + target + ".html"},
		{"directory index sub-path", true, "/index.html", target + "/index.html"},
		{"directory index with proxy path", false, "/index.html", "/" + target + "/index.html"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, buildProbePath(tt.proxyPathEmpty, target, tt.lookup))
		})
	}
}

func TestNewProxy(t *testing.T) {
	tests := []struct {
		name          string
		pages         []*config.Page
		expectedPages map[string]string // Expected map for `proxy.pages` [domain -> backend URL]
		expectWarning bool              // Whether we expect a duplicate domain warning in the logs
	}{
		{
			name: "no duplicates",
			pages: []*config.Page{
				{Domain: "example.com", Proxy: config.PageProxy{URL: config.EnvValue("https://example-backend1.com")}},
				{Domain: "test.com", Proxy: config.PageProxy{URL: config.EnvValue("https://test-backend.com")}},
			},
			expectedPages: map[string]string{
				"example.com": "https://example-backend1.com",
				"test.com":    "https://test-backend.com",
			},
			expectWarning: false,
		},
		{
			name: "with duplicates",
			pages: []*config.Page{
				{Domain: "example.com", Proxy: config.PageProxy{URL: config.EnvValue("https://example-backend1.com")}},
				{Domain: "example.com", Proxy: config.PageProxy{URL: config.EnvValue("https://example-backend2.com")}}, // Duplicate
				{Domain: "test.com", Proxy: config.PageProxy{URL: config.EnvValue("https://test-backend.com")}},
			},
			expectedPages: map[string]string{
				"example.com": "https://example-backend2.com", // Expect last duplicate to overwrite
				"test.com":    "https://test-backend.com",
			},
			expectWarning: true,
		},
	}

	// Capture logs for later assertions
	buf := initLogger()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Clear the log buffer for each test run.
			buf.Reset()

			proxy := NewProxy(config.StaticPagesConfig{
				Pages: test.pages,
			})
			assert.Len(t, proxy.pagesMap, len(test.expectedPages))

			// Validate the resulting `proxy.pages` map
			for domain, expectedURL := range test.expectedPages {
				page, exists := proxy.pagesMap[config.FromString(domain)]
				assert.True(t, exists, "Expected domain %s to exist", domain)
				if exists {
					assert.Equal(t, expectedURL, page.Proxy.URL.String(), "Expected backend URL for domain %s to be %s", domain, expectedURL)
				}
			}

			// Check if the logs contain the expected warnings
			if test.expectWarning {
				assert.Contains(t, buf.String(), "duplicate page domain", "Expected a warning about duplicate domains")
			} else {
				assert.NotContains(t, buf.String(), "duplicate page domain", "Did not expect a warning about duplicate domains")
			}
		})
	}
}

type testProxyServer struct {
	name                 string
	domain               string
	requestHost          string // host used for the request (defaults to domain)
	notFound             string // page.Proxy.NotFound (configured 404 document)
	method               string
	requestPath          string
	searchPaths          []string
	requestPathResponse  int
	searchPathResponses  map[string]int
	requestPathHeadDelay time.Duration // delay applied to the HEAD probe of the bare request path
	probeTimeout         time.Duration // per-probe HEAD timeout (0 => default)
	expectedStatus       int
	expectedBody         string
	expectErrorLog       bool
}

func TestProxyServeHTTP(t *testing.T) {
	tests := []testProxyServer{
		{
			name:                "valid target path resolved",
			domain:              "example.com",
			method:              http.MethodGet,
			requestPath:         "/some/path",
			requestPathResponse: http.StatusNotFound,
			searchPaths:         []string{"index.html", "home.html"},
			searchPathResponses: map[string]int{
				"/some/path/index.html": http.StatusOK,
				"/some/path/home.html":  http.StatusNotFound,
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "Hello from backend",
			expectErrorLog: false,
		},
		{
			name:                "valid target path no resolving",
			domain:              "example.com",
			method:              http.MethodGet,
			requestPath:         "/some/path",
			requestPathResponse: http.StatusOK,
			expectedStatus:      http.StatusOK,
			expectedBody:        "Hello from backend",
			expectErrorLog:      false,
		},
		{
			name:                "no valid target path resolved",
			domain:              "example.com",
			method:              http.MethodGet,
			requestPath:         "/some/path",
			requestPathResponse: http.StatusNotFound,
			searchPaths:         []string{"index.html", "home.html"},
			searchPathResponses: map[string]int{
				"/some/path/index.html": http.StatusNotFound,
				"/some/path/home.html":  http.StatusNotFound,
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   "Not Found\n",
			expectErrorLog: false,
		},
		{
			// Regression: a slow/cold origin made the HEAD probe of the real
			// object time out, so the proxy served the 404 fallback for a file
			// that actually exists. A probe timeout must be treated as
			// inconclusive and the object proxied anyway.
			name:                 "probe timeout on existing file still serves the file",
			domain:               "example.com",
			method:               http.MethodGet,
			requestPath:          "/some/path",
			requestPathResponse:  http.StatusOK, // GET resolves fine
			requestPathHeadDelay: 1 * time.Second,
			probeTimeout:         200 * time.Millisecond, // HEAD probe gives up before the origin answers
			searchPaths:          []string{"index.html", "home.html"},
			searchPathResponses: map[string]int{
				"/some/path/index.html": http.StatusNotFound,
				"/some/path/home.html":  http.StatusNotFound,
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "Hello from backend",
			expectErrorLog: false,
		},
		{
			// A request whose host/branch/commit cannot be resolved (e.g. a
			// scanner hitting an unconfigured subdomain) must get a clean 404,
			// not a 502 "unsupported protocol scheme" from a half-built request.
			name:           "unresolvable host returns clean 404 not 502",
			domain:         "example.com",
			requestHost:    "scanner.evil.example",
			method:         http.MethodGet,
			requestPath:    "/wp-admin/",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "Not Found\n",
			expectErrorLog: false,
		},
		{
			// When the path is missing and the configured 404 page is served,
			// the response status must be 404 — not the storage backend's 200
			// (a soft-404 that poisons caches and makes SPAs flicker).
			name:                "configured 404 page served with 404 status",
			domain:              "example.com",
			method:              http.MethodGet,
			requestPath:         "/missing",
			requestPathResponse: http.StatusNotFound,
			notFound:            "404.html",
			searchPaths:         []string{"index.html"},
			searchPathResponses: map[string]int{
				"/missing/index.html": http.StatusNotFound,
				"/404.html":           http.StatusOK,
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   "Hello from backend",
			expectErrorLog: false,
		},
		{
			// VitePress (and most static generators with clean URLs) deploy
			// /guides/x as the file guides/x.html. The proxy must resolve a
			// ".html" suffix, otherwise the real page is served as the 404
			// fallback and SPA hydration papers over it as a "404 flash".
			name:                "clean URL resolves to .html file",
			domain:              "example.com",
			method:              http.MethodGet,
			requestPath:         "/guides/shell-integration",
			requestPathResponse: http.StatusNotFound,
			searchPaths:         []string{".html", "/index.html"},
			searchPathResponses: map[string]int{
				"/guides/shell-integration.html":       http.StatusOK,
				"/guides/shell-integration/index.html": http.StatusNotFound,
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "Hello from backend",
			expectErrorLog: false,
		},
		{
			name:                "invalid method",
			domain:              "example.com",
			method:              http.MethodPost,
			requestPath:         "/some/path",
			requestPathResponse: http.StatusNotFound,
			searchPaths:         []string{"index.html", "home.html"},
			searchPathResponses: map[string]int{
				"/some/path/index.html": http.StatusOK,
				"/some/path/home.html":  http.StatusNotFound,
			},
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   "Method not allowed\n",
			expectErrorLog: true,
		},
	}

	// Capture logs for later assertions
	buf := initLogger()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Clear the log buffer for each test run.
			buf.Reset()

			// Setup test server with simulated backend for HEAD requests
			backend := setupMockServer(&test)
			defer backend.Close()

			s3Backend := setupMockS3(&test)
			defer s3Backend.Close()

			conf := config.StaticPagesConfig{
				Proxy: config.Proxy{
					ProbeTimeout: test.probeTimeout,
				},
				Pages: []*config.Page{
					{
						Domain: config.FromString(test.domain),
						Proxy: config.PageProxy{
							URL:        config.EnvValue(backend.URL),
							SearchPath: test.searchPaths,
							NotFound:   test.notFound,
						},
						Bucket: config.BucketConfig{
							URL:           config.EnvValue(s3Backend.URL),
							Name:          "test",
							ApplicationID: "test",
							Secret:        "test",
							Region:        "test",
						},
					},
				},
			}

			// Create the proxy
			proxy := NewProxy(conf)

			// Simulate the s3_client request
			requestHost := test.domain
			if test.requestHost != "" {
				requestHost = test.requestHost
			}
			req := httptest.NewRequest(test.method, fmt.Sprintf("http://%s%s", requestHost, test.requestPath), nil)
			rr := httptest.NewRecorder()

			proxy.ServeHTTP(rr, req)

			// Assert response status and body
			assert.Equal(t, test.expectedStatus, rr.Code, "Unexpected HTTP status")
			assert.Equal(t, test.expectedBody, rr.Body.String(), "Unexpected response body")

			// Assert whether logs contain errors. Match on the log *level*
			// (the encoder writes it as a tab-delimited "ERROR"/"WARN" token),
			// not on the substring "error" — a Warn line may legitimately carry
			// an "error" field for an expected, handled condition (e.g. a 404).
			logOutput := buf.String()
			hasErrorLevel := strings.Contains(logOutput, "\tERROR\t")
			if test.expectErrorLog {
				assert.True(t, hasErrorLevel || strings.Contains(logOutput, "\tWARN\t"), "Expected a warning or error to be logged")
			} else {
				assert.False(t, hasErrorLevel, "Did not expect an error to be logged")
			}
		})
	}
}

const mockCommit = "6af8739ec3559ae35088b7d84748d15d4d440776"

func setupMockServer(test *testProxyServer) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqPath, _ := strings.CutPrefix(r.URL.Path, fmt.Sprintf("/%s", mockCommit))

		// Simulate backend behavior for search path resolution
		if r.Method == http.MethodHead {
			status := http.StatusNotFound

			if reqPath == test.requestPath {
				// Simulate a slow/cold origin for the exact requested object so
				// the HEAD probe exceeds its timeout while the GET still works.
				if test.requestPathHeadDelay > 0 {
					time.Sleep(test.requestPathHeadDelay)
				}
				status = test.requestPathResponse
			} else {
				if s, ok := test.searchPathResponses[reqPath]; ok {
					status = s
				}
			}
			w.WriteHeader(status)
			return
		}

		status := http.StatusNotFound
		if reqPath == test.requestPath {
			status = test.requestPathResponse
		} else {
			if s, ok := test.searchPathResponses[reqPath]; ok {
				status = s
			}
		}

		w.WriteHeader(status)

		if status == http.StatusOK {
			// Normal GET request should return success
			_, _ = w.Write([]byte("Hello from backend"))
		}
	}))
}

func setupMockS3(test *testProxyServer) *httptest.Server {

	testIndex := fmt.Sprintf(`%s:
    environment: main
    branch: ""
    date: 2025-05-04T18:13:45.715404+02:00
`, mockCommit)

	reader, size, err := writeAndOpenTempFile(testIndex)
	if err != nil {
		panic(err)
	}
	defer func() { _ = reader.(io.Closer).Close() }()

	s3Backend := s3mem.New()
	err = s3Backend.CreateBucket("test")
	if err != nil {
		panic(err)
	}

	_, err = s3Backend.PutObject("test", "index.yaml", nil, reader, size, nil)
	if err != nil {
		panic(err)
	}

	faker := gofakes3.New(s3Backend, gofakes3.WithHostBucket(false))
	return httptest.NewServer(faker.Server())
}

// writeAndOpenTempFile writes the string to a temp file and returns an io.Reader to read it back.
func writeAndOpenTempFile(content string) (io.Reader, int64, error) {
	tmpFile, err := os.CreateTemp("", "example-*.txt")
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = tmpFile.Close() }()

	if _, err := tmpFile.WriteString(content); err != nil {
		return nil, 0, err
	}

	// Reopen for reading
	fileReader, err := os.Open(tmpFile.Name())
	if err != nil {
		return nil, 0, err
	}

	stat, err := os.Stat(tmpFile.Name())
	if err != nil {
		return nil, 0, err
	}

	return fileReader, stat.Size(), nil
}
