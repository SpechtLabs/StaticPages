package proxy

import (
	"bytes"
	"fmt"
	"github.com/SpechtLabs/StaticPages/pkg/config"
	"github.com/spechtlabs/go-otel-utils/otelzap"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func initLogger() *bytes.Buffer {
	// Capture logs for later assertions
	enc := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
	buf := &bytes.Buffer{}
	writer := zapcore.AddSync(buf) //zap.CombineWriteSyncers(zaptest.NewTestingWriter(t), )
	level := zap.NewAtomicLevelAt(zapcore.DebugLevel)

	otelZapLogger := otelzap.New(zap.New(zapcore.NewCore(enc, writer, level)))
	otelzap.ReplaceGlobals(otelZapLogger)

	return buf
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
				page, exists := proxy.pagesMap[domain]
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
	name                string
	domain              string
	method              string
	requestPath         string
	searchPaths         []string
	requestPathResponse int
	searchPathResponses map[string]int
	expectedStatus      int
	expectedBody        string
	expectErrorLog      bool
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
			expectErrorLog: true,
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

			conf := config.StaticPagesConfig{
				Pages: []*config.Page{
					{
						Domain: test.domain,
						Proxy: config.PageProxy{
							URL:        config.EnvValue(backend.URL),
							SearchPath: test.searchPaths,
						},
					},
				},
			}

			// Create the proxy
			proxy := NewProxy(conf)

			// Simulate the client request
			req := httptest.NewRequest(test.method, fmt.Sprintf("http://%s%s", test.domain, test.requestPath), nil)
			rr := httptest.NewRecorder()

			proxy.ServeHTTP(rr, req)

			// Assert response status and body
			assert.Equal(t, test.expectedStatus, rr.Code, "Unexpected HTTP status")
			assert.Equal(t, test.expectedBody, rr.Body.String(), "Unexpected response body")

			// Assert whether logs contain errors
			if test.expectErrorLog {
				if !strings.Contains(strings.ToLower(buf.String()), "warn") {
					assert.Contains(t, strings.ToLower(buf.String()), "error", "Expected an error to be logged")
				}
			} else {
				assert.NotContains(t, strings.ToLower(buf.String()), "error", "Did not expect an error to be logged")
			}
		})
	}
}

func setupMockServer(test *testProxyServer) *httptest.Server {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate backend behavior for search path resolution
		if r.Method == http.MethodHead {
			status := http.StatusNotFound

			if r.URL.Path == test.requestPath {
				status = test.requestPathResponse
			} else {
				if s, ok := test.searchPathResponses[r.URL.Path]; ok {
					status = s
				}
			}
			w.WriteHeader(status)
			return
		}

		status := http.StatusNotFound
		if r.URL.Path == test.requestPath {
			status = test.requestPathResponse
		} else {
			if s, ok := test.searchPathResponses[r.URL.Path]; ok {
				status = s
			}
		}

		w.WriteHeader(status)

		if status == http.StatusOK {
			// Normal GET request should return success
			_, _ = w.Write([]byte("Hello from backend"))
		}
	}))
	return backend
}
