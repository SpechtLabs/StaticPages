package config_test

import (
	"testing"
	"time"

	"github.com/SpechtLabs/StaticPages/pkg/config"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApiBindAddr(t *testing.T) {
	conf := &config.StaticPagesConfig{
		Server: config.Server{
			Host:    "localhost",
			ApiPort: 8081,
		},
	}

	assert.Equal(t, "localhost:8081", conf.ApiBindAddr())
}

func TestProxyBindAddr(t *testing.T) {
	conf := &config.StaticPagesConfig{
		Server: config.Server{
			Host:      "",
			ProxyPort: 8080,
		},
	}

	assert.Equal(t, ":8080", conf.ProxyBindAddr())
}

func TestSetDefaults(t *testing.T) {
	v := viper.New()
	config.SetDefaults(v)

	var conf config.StaticPagesConfig
	require.NoError(t, v.Unmarshal(&conf))

	assert.Equal(t, config.StaticPagesConfig{
		Server: config.Server{ProxyPort: 8080, ApiPort: 8081},
		Output: config.Output{Format: config.ShortFormat},
		Proxy: config.Proxy{
			MaxIdleConns:        1000,
			MaxIdleConnsPerHost: 100,
			Timeout:             90 * time.Second,
			Compression:         true,
			ProbeTimeout:        2 * time.Second,
		},
	}, conf)
}
