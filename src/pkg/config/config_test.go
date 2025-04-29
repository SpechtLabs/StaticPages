package config_test

import (
	"github.com/SpechtLabs/StaticPages/pkg/config"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestApiBindAddr(t *testing.T) {
	config := &config.StaticPagesConfig{
		Server: config.Server{
			Host:    "localhost",
			ApiPort: 8081,
		},
	}

	assert.Equal(t, "localhost:8081", config.ApiBindAddr())
}

func TestProxyBindAddr(t *testing.T) {
	config := &config.StaticPagesConfig{
		Server: config.Server{
			Host:      "",
			ProxyPort: 8080,
		},
	}

	assert.Equal(t, ":8080", config.ProxyBindAddr())
}

func TestParse(t *testing.T) {
	tests := []struct {
		name         string
		config       config.StaticPagesConfig
		expectError  bool
		errorMessage string
	}{
		{
			name: "valid proxy config",
			config: config.StaticPagesConfig{
				Pages: []*config.Page{
					{
						Domain: "example.com",
						Bucket: config.BucketConfig{
							URL:           "https://s3.bucket.com",
							Name:          "foo",
							ApplicationID: "bar",
							Secret:        "foobar2342",
						},
						Proxy: config.PageProxy{
							URL:        "https://s3.bucket.com",
							Path:       "foo/bar",
							SearchPath: []string{"/index.html", "/default.html"},
						},
						History:    0,
						Repository: "",
						SubDomains: nil,
					},
				},
			},
			expectError: false,
		},
		{
			name: "parse fail",
			config: config.StaticPagesConfig{
				Pages: []*config.Page{
					{
						Domain: "example.com",
						Bucket: config.BucketConfig{
							URL:           "ENV(URL)",
							Name:          "foo",
							ApplicationID: "bar",
							Secret:        "foobar2342",
						},
						Proxy: config.PageProxy{
							URL:        "https://s3.bucket.com",
							Path:       "foo/bar",
							SearchPath: []string{"/index.html", "/default.html"},
						},
						History:    0,
						Repository: "",
						SubDomains: nil,
					},
				},
			},
			expectError:  true,
			errorMessage: "Invalid page configuration for",
		},
		{
			name: "validation fail",
			config: config.StaticPagesConfig{
				Pages: []*config.Page{
					{
						Domain: "example.com",
						Bucket: config.BucketConfig{
							URL:           "://not-a-valid-url",
							Name:          "foo",
							ApplicationID: "bar",
							Secret:        "foobar2342",
						},
						Proxy: config.PageProxy{
							URL:        "https://s3.bucket.com",
							Path:       "foo/bar",
							SearchPath: []string{"/index.html", "/default.html"},
						},
						History:    0,
						Repository: "",
						SubDomains: nil,
					},
				},
			},
			expectError:  true,
			errorMessage: "Validation failed for",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.config.Parse()
			if test.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), test.errorMessage)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
