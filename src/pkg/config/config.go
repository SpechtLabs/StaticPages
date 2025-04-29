package config

import (
	"fmt"
	"github.com/sierrasoftworks/humane-errors-go"
	"github.com/spf13/viper"
	"time"
)

type Format string

func init() {
	viper.SetDefault("server.proxyPort", 8080)
	viper.SetDefault("server.apiPort", 8081)
	viper.SetDefault("server.host", "")

	viper.SetDefault("output.format", ShortFormat)

	viper.SetDefault("proxy.maxIdleConns", 1000)
	viper.SetDefault("proxy.maxIdleConnsPerHost", 100)
	viper.SetDefault("proxy.timeout", "90s")
	viper.SetDefault("proxy.compression", true)
}

const (
	ShortFormat Format = "short"
	LongFormat  Format = "long"
)

type Output struct {
	Format Format
}

type Server struct {
	Host      string
	ProxyPort int
	ApiPort   int
}

type Proxy struct {
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	Timeout             time.Duration
	Compression         bool
}

type StaticPagesConfig struct {
	Server Server
	Proxy  Proxy
	Output Output
	Pages  []*Page
}

func (s *StaticPagesConfig) Parse() humane.Error {
	// Parse each page configuration
	for _, page := range s.Pages {
		if err := page.Parse(); err != nil {
			return humane.Wrap(err, fmt.Sprintf("Invalid page configuration for %s", page.Domain), err.Advice()...)
		}

		// Validate page config
		if err := page.Validate(); err != nil {
			return humane.Wrap(err, fmt.Sprintf("Validation failed for %s", page.Domain), err.Advice()...)
		}
	}

	return nil
}

func (s *StaticPagesConfig) ApiBindAddr() string {
	return fmt.Sprintf("%s:%d", s.Server.Host, s.Server.ApiPort)
}

func (s *StaticPagesConfig) ProxyBindAddr() string {
	return fmt.Sprintf("%s:%d", s.Server.Host, s.Server.ProxyPort)
}
