package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

const (
	// ShortFormat prints compact output.
	ShortFormat Format = "short"
	// LongFormat prints verbose output.
	LongFormat Format = "long"
)

// Format selects how the CLI renders its output.
type Format string

// Output configures the CLI's output.
type Output struct {
	Format Format
}

// Server configures the addresses the proxy and the upload API listen on.
type Server struct {
	Host      string
	ProxyPort int
	ApiPort   int
}

// Proxy configures the reverse proxy's connections to the storage backends.
type Proxy struct {
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	Timeout             time.Duration
	Compression         bool

	// ProbeTimeout bounds each per-path HEAD probe issued while resolving a
	// request to a backend object. A probe that exceeds this deadline is
	// treated as inconclusive (the object may still exist) rather than as a
	// definitive "not found".
	ProbeTimeout time.Duration
}

// StaticPagesConfig is the whole configuration file: the listeners, the
// proxy's connection settings, and the pages it serves.
type StaticPagesConfig struct {
	Output Output
	Pages  []*Page
	Server Server
	Proxy  Proxy
}

// SetDefaults registers the default value of every setting that has one, so
// a configuration file only needs to name what it changes.
func SetDefaults(v *viper.Viper) {
	v.SetDefault("server.proxyPort", 8080)
	v.SetDefault("server.apiPort", 8081)
	v.SetDefault("server.host", "")

	v.SetDefault("output.format", ShortFormat)

	v.SetDefault("proxy.maxIdleConns", 1000)
	v.SetDefault("proxy.maxIdleConnsPerHost", 100)
	v.SetDefault("proxy.timeout", "90s")
	v.SetDefault("proxy.compression", true)
	v.SetDefault("proxy.probeTimeout", "2s")
}

// ApiBindAddr returns the address the upload API listens on.
func (s *StaticPagesConfig) ApiBindAddr() string {
	return fmt.Sprintf("%s:%d", s.Server.Host, s.Server.ApiPort)
}

// ProxyBindAddr returns the address the reverse proxy listens on.
func (s *StaticPagesConfig) ProxyBindAddr() string {
	return fmt.Sprintf("%s:%d", s.Server.Host, s.Server.ProxyPort)
}
