package config_test

import (
	"testing"

	"github.com/SpechtLabs/StaticPages/pkg/config"
	"github.com/stretchr/testify/assert"
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
