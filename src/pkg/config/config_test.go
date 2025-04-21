package config_test

import (
	"testing"

	"github.com/SpechtLabs/StaticPages/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestPageValidate(t *testing.T) {
	tests := []struct {
		name         string
		page         config.Page
		expectError  bool
		errorMessage string
	}{
		{
			name: "valid page config",
			page: config.Page{
				Domain:  "example.com",
				History: 5,
				Bucket: config.BucketConfig{
					URL:           "https://s3.bucket.com",
					Name:          "bucket-name",
					ApplicationID: "app-id",
					Secret:        "app-secret",
				},
				Proxy: config.Proxy{
					URL:        "https://api.example.com",
					Path:       "/proxy",
					SearchPath: []string{"index.html"},
				},
			},
			expectError: false,
		},
		{
			name: "invalid bucket config",
			page: config.Page{
				Domain:  "example.com",
				History: 5,
				Bucket: config.BucketConfig{
					URL:           "", // Invalid (missing URL)
					Name:          "bucket-name",
					ApplicationID: "app-id",
					Secret:        "app-secret",
				},
				Proxy: config.Proxy{
					URL:        "https://api.example.com",
					Path:       "/proxy",
					SearchPath: []string{"index.html"},
				},
			},
			expectError:  true,
			errorMessage: "No S3 Bucket URL is provided",
		},
		{
			name: "invalid proxy config",
			page: config.Page{
				Domain:  "example.com",
				History: 5,
				Bucket: config.BucketConfig{
					URL:           "https://s3.bucket.com",
					Name:          "bucket-name",
					ApplicationID: "app-id",
					Secret:        "app-secret",
				},
				Proxy: config.Proxy{
					URL:        "", // Missing Proxy URL
					Path:       "/proxy",
					SearchPath: []string{"index.html"},
				},
			},
			expectError:  true,
			errorMessage: "No Reverse-Proxy URL is provided",
		},
		{
			name: "negative history",
			page: config.Page{
				Domain:  "example.com",
				History: -1, // Invalid history
				Bucket: config.BucketConfig{
					URL:           "https://s3.bucket.com",
					Name:          "bucket-name",
					ApplicationID: "app-id",
					Secret:        "app-secret",
				},
				Proxy: config.Proxy{
					URL:        "https://api.example.com",
					Path:       "/proxy",
					SearchPath: []string{"index.html"},
				},
			},
			expectError:  true,
			errorMessage: "Invalid number of revisions to keep ", // Should include this message
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.page.Validate()

			if test.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Display(), test.errorMessage)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBucketConfigValidate(t *testing.T) {
	tests := []struct {
		name         string
		bucketConfig config.BucketConfig
		expectError  bool
		errorMessage string
	}{
		{
			name: "valid bucket config",
			bucketConfig: config.BucketConfig{
				URL:           "https://s3.bucket.com",
				Name:          "bucket-name",
				ApplicationID: "app-id",
				Secret:        "app-secret",
			},
			expectError: false,
		},
		{
			name: "missing URL",
			bucketConfig: config.BucketConfig{
				URL:           "",
				Name:          "bucket-name",
				ApplicationID: "app-id",
				Secret:        "app-secret",
			},
			expectError:  true,
			errorMessage: "No S3 Bucket URL is provided",
		},
		{
			name: "invalid URL",
			bucketConfig: config.BucketConfig{
				URL:           "://not-a-valid-url",
				Name:          "bucket-name",
				ApplicationID: "app-id",
				Secret:        "app-secret",
			},
			expectError:  true,
			errorMessage: "Invalid S3 Endpoint",
		},
		{
			name: "missing application ID",
			bucketConfig: config.BucketConfig{
				URL:           "https://s3.bucket.com",
				Name:          "bucket-name",
				ApplicationID: "",
				Secret:        "app-secret",
			},
			expectError:  true,
			errorMessage: "No S3 Application ID is provided",
		},
		{
			name: "missing secret",
			bucketConfig: config.BucketConfig{
				URL:           "https://s3.bucket.com",
				Name:          "bucket-name",
				ApplicationID: "app-id",
				Secret:        "",
			},
			expectError:  true,
			errorMessage: "No S3 Secret is provided",
		},
		{
			name: "missing name",
			bucketConfig: config.BucketConfig{
				URL:           "https://s3.bucket.com",
				Name:          "",
				ApplicationID: "app-id",
				Secret:        "app-secret",
			},
			expectError:  true,
			errorMessage: "No S3 Bucket Name is provided",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.bucketConfig.Validate()

			if test.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), test.errorMessage)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestProxyValidate(t *testing.T) {
	tests := []struct {
		name         string
		proxyConfig  config.Proxy
		expectError  bool
		errorMessage string
	}{
		{
			name: "valid proxy config",
			proxyConfig: config.Proxy{
				URL:        "https://api.example.com",
				Path:       "/proxy",
				SearchPath: []string{"index.html"},
			},
			expectError: false,
		},
		{
			name: "missing URL",
			proxyConfig: config.Proxy{
				URL:        "",
				Path:       "/proxy",
				SearchPath: []string{"index.html"},
			},
			expectError:  true,
			errorMessage: "No Reverse-Proxy URL is provided",
		},
		{
			name: "missing SearchPath",
			proxyConfig: config.Proxy{
				URL:        "https://api.example.com",
				Path:       "/proxy",
				SearchPath: []string{}, // Empty search path
			},
			expectError:  true,
			errorMessage: "No search paths specified",
		},
		{
			name: "missing Path",
			proxyConfig: config.Proxy{
				URL:        "https://api.example.com",
				Path:       "",
				SearchPath: []string{"index.html"},
			},
			expectError:  true,
			errorMessage: "No Reverse-Proxy Path is provided",
		},
		{
			name: "invalid URL",
			proxyConfig: config.Proxy{
				URL:        "://not-a-valid-url",
				Path:       "/proxy",
				SearchPath: []string{"index.html"},
			},
			expectError:  true,
			errorMessage: "Invalid Reverse-Proxy URL",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.proxyConfig.Validate()

			if test.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), test.errorMessage)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPageParse(t *testing.T) {
	tests := []struct {
		name         string
		page         config.Page
		expectError  bool
		errorMessage string
	}{
		{
			name: "valid page with valid bucket and proxy",
			page: config.Page{
				Domain: "example.com",
				Bucket: config.BucketConfig{
					URL:           "https://s3.bucket.com",
					Name:          "bucket-name",
					ApplicationID: "app-id",
					Secret:        "app-secret",
				},
				Proxy: config.Proxy{
					URL:        "https://api.example.com",
					Path:       "/proxy",
					SearchPath: []string{"index.html"},
				},
			},
			expectError: false,
		},
		{
			name: "invalid bucket config in page",
			page: config.Page{
				Domain: "example.com",
				Bucket: config.BucketConfig{
					URL:           "ENV(not-a-valid-url)", // Invalid URL
					Name:          "bucket-name",
					ApplicationID: "app-id",
					Secret:        "app-secret",
				},
				Proxy: config.Proxy{
					URL:        "https://api.example.com",
					Path:       "/proxy",
					SearchPath: []string{"index.html"},
				},
			},
			expectError:  true,
			errorMessage: "Invalid bucket configuration",
		},
		{
			name: "invalid proxy config in page",
			page: config.Page{
				Domain: "example.com",
				Bucket: config.BucketConfig{
					URL:           "https://s3.bucket.com",
					Name:          "bucket-name",
					ApplicationID: "app-id",
					Secret:        "app-secret",
				},
				Proxy: config.Proxy{
					URL:        "ENV(not-a-valid-url)", // Invalid Proxy URL
					Path:       "/proxy",
					SearchPath: []string{"index.html"},
				},
			},
			expectError:  true,
			errorMessage: "Invalid proxy configuration",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.page.Parse()

			if test.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), test.errorMessage)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseBucketConfig(t *testing.T) {
	tests := []struct {
		name         string
		bucketConfig config.BucketConfig
		expectError  bool
		errorMessage string
	}{
		{
			name: "valid bucket config",
			bucketConfig: config.BucketConfig{
				URL:           "https://s3.bucket.com",
				Name:          "bucket-name",
				ApplicationID: "app-id",
				Secret:        "app-secret",
			},
			expectError: false,
		},
		{
			name: "missing URL env",
			bucketConfig: config.BucketConfig{
				URL:           "ENV(URL)",
				Name:          "bucket-name",
				ApplicationID: "app-id",
				Secret:        "app-secret",
			},
			expectError:  true,
			errorMessage: "environment variable URL is not set",
		},
		{
			name: "missing name env",
			bucketConfig: config.BucketConfig{
				URL:           "https://s3.bucket.com",
				Name:          "ENV(bucket-name)",
				ApplicationID: "app-id",
				Secret:        "app-secret",
			},
			expectError:  true,
			errorMessage: "environment variable bucket-name is not set",
		},
		{
			name: "valid appid env",
			bucketConfig: config.BucketConfig{
				URL:           "https://s3.bucket.com",
				Name:          "bucket-name",
				ApplicationID: "ENV(app-id)",
				Secret:        "app-secret",
			},
			expectError:  true,
			errorMessage: "environment variable app-id is not set",
		},
		{
			name: "valid secret env",
			bucketConfig: config.BucketConfig{
				URL:           "https://s3.bucket.com",
				Name:          "bucket-name",
				ApplicationID: "app-id",
				Secret:        "ENV(app-secret)",
			},
			expectError:  true,
			errorMessage: "environment variable app-secret is not set",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.bucketConfig.Parse()
			if test.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), test.errorMessage)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseProxyConfig(t *testing.T) {
	tests := []struct {
		name               string
		proxyConfig        config.Proxy
		expectedSearchPath []string
		expectError        bool
		errorMessage       string
	}{
		{
			name: "valid proxy config",
			proxyConfig: config.Proxy{
				URL:        "https://s3.bucket.com",
				Path:       "foo/bar",
				SearchPath: []string{"/index.html", "/default.html"},
			},
			expectedSearchPath: []string{"/index.html", "/default.html"},
			expectError:        false,
		},
		{
			name: "missing URL env",
			proxyConfig: config.Proxy{
				URL:        "ENV(URL)",
				Path:       "foo/bar",
				SearchPath: []string{"/index.html", "/default.html"},
			},
			expectedSearchPath: []string{"/index.html", "/default.html"},
			expectError:        true,
			errorMessage:       "environment variable URL is not set",
		},
		{
			name: "missing path env",
			proxyConfig: config.Proxy{
				URL:        "https://s3.bucket.com",
				Path:       "ENV(url_path)",
				SearchPath: []string{"/index.html", "/default.html"},
			},
			expectedSearchPath: []string{"/index.html", "/default.html"},
			expectError:        true,
			errorMessage:       "environment variable url_path is not set",
		},
		{
			name: "default search paths",
			proxyConfig: config.Proxy{
				URL:  "https://s3.bucket.com",
				Path: "foo/bar",
			},
			expectedSearchPath: []string{"/index.html", "/index.htm"},
			expectError:        false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.proxyConfig.Parse()
			assert.Equal(t, test.expectedSearchPath, test.proxyConfig.SearchPath)
			if test.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), test.errorMessage)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
