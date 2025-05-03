package config_test

import (
	"testing"

	"github.com/SpechtLabs/StaticPages/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestDomainMapper_Lookup(t *testing.T) {
	// Create some test pages
	page1 := &config.Page{Domain: "specht.av0.de"}
	page2 := &config.Page{Domain: "cedi.av0.de"}
	page3 := &config.Page{Domain: "av0.de"}
	page4 := &config.Page{Domain: "very-long-domain.com"}
	page5 := &config.Page{Domain: "a.b.c.d.short.com"}

	// Create a DomainMapper
	domainMapper := config.DomainMapper{
		config.DomainScope("specht.av0.de"):        page1,
		config.DomainScope("cedi.av0.de"):          page2,
		config.DomainScope("av0.de"):               page3,
		config.DomainScope("very-long-domain.com"): page4,
		config.DomainScope("a.b.c.d.short.com"):    page5,
	}

	tests := []struct {
		name         string
		domain       string
		expectedPage *config.Page
	}{
		{
			name:         "exact match",
			domain:       "specht.av0.de",
			expectedPage: page1,
		},
		{
			name:         "subdomain match",
			domain:       "dev.specht.av0.de",
			expectedPage: page1,
		},
		{
			name:         "level preference over length",
			domain:       "x.a.b.c.d.short.com",
			expectedPage: page5,
		},
		{
			name:         "same level but different length",
			domain:       "test.very-long-domain.com",
			expectedPage: page4,
		},
		{
			name:         "no match",
			domain:       "example.com",
			expectedPage: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := domainMapper.Lookup(tt.domain)
			assert.Equal(t, tt.expectedPage, result)
		})
	}
}

func TestDomainMapper_GetMatchingDomain(t *testing.T) {
	// Create some test pages
	// Create some test pages
	page1 := &config.Page{Domain: "specht.av0.de"}
	page2 := &config.Page{Domain: "cedi.av0.de"}
	page3 := &config.Page{Domain: "av0.de"}

	// Create a DomainMapper
	domainMapper := config.DomainMapper{
		config.DomainScope("specht.av0.de"): page1,
		config.DomainScope("cedi.av0.de"):   page2,
		config.DomainScope("av0.de"):        page3,
	}

	tests := []struct {
		name            string
		domain          string
		expectedMatcher config.DomainScope
	}{
		{
			name:            "exact match",
			domain:          "specht.av0.de",
			expectedMatcher: config.DomainScope("specht.av0.de"),
		},
		{
			name:            "subdomain match",
			domain:          "dev.specht.av0.de",
			expectedMatcher: config.DomainScope("specht.av0.de"),
		},
		{
			name:            "parent domain match",
			domain:          "something.else.av0.de",
			expectedMatcher: config.DomainScope("av0.de"),
		},
		{
			name:            "no match",
			domain:          "example.com",
			expectedMatcher: config.DomainScope(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := domainMapper.GetMatchingDomain(tt.domain)

			if result != tt.expectedMatcher {
				t.Errorf("DomainMapper.GetMatchingDomain(%q) = %q, want %q",
					tt.domain, string(result), string(tt.expectedMatcher))
			}
		})
	}
}
