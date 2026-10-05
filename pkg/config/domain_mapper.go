package config

import (
	"github.com/spechtlabs/go-otel-utils/otelzap"
	"go.uber.org/zap"
)

// DomainMapper is a wrapper around a map of DomainMatcher to Page pointers
// that provides methods for longest-prefix matching.
type DomainMapper map[DomainScope]*Page

// NewDomainMapperFromPages indexes the pages by their domain.
func NewDomainMapperFromPages(pages []*Page) DomainMapper {
	// construct a map for easier lookup in the proxy
	pagesMap := make(DomainMapper)
	var duplicates, nested []string
	for _, page := range pages {
		if pagesMap[page.Domain] != nil {
			duplicates = append(duplicates, page.Domain.String())
		}

		// Nested domains are expected (e.g. an apex page plus its
		// subdomains); longest-prefix matching routes each to the most
		// specific page. They are logged at debug for routing diagnostics only.
		if p := pagesMap.Lookup(page.Domain.String()); p != nil {
			nested = append(nested, page.Domain.String()+" under "+p.Domain.String())
		}

		pagesMap[page.Domain] = page
	}

	if len(duplicates) > 0 {
		otelzap.L().Warn("duplicate page domains; the last page configured for each wins", zap.Strings("domains", duplicates))
	}

	if len(nested) > 0 {
		otelzap.L().Debug("nested page domains; longest-prefix match will route them", zap.Strings("domains", nested))
	}

	return pagesMap
}

// Lookup finds the longest matching DomainMatcher for a given domain
// and returns the corresponding *Page.
// Returns nil if no match is found.
//
// For example, with a map containing "specht.av0.de" and "cedi.av0.de" as keys,
// a lookup for "dev.specht.av0.de" would return the page for "specht.av0.de".
func (dm DomainMapper) Lookup(domain string) *Page {
	var (
		longestMatchLevel int
		matchedPage       *Page
		foundValidMatch   bool
	)

	// Iterate through all domain matchers
	for matcher, page := range dm {
		// Skip if this matcher doesn't match the domain
		if !matcher.Is(domain) {
			continue
		}

		// Get the domain level of the matcher - represents its specificity
		currentMatchLevel := matcher.Level()

		// If this is our first match or it has more specific level than our previous best match
		if !foundValidMatch || currentMatchLevel > longestMatchLevel {
			longestMatchLevel = currentMatchLevel
			matchedPage = page
			foundValidMatch = true
		}
	}

	// If we found a match, return the corresponding page
	if foundValidMatch {
		return matchedPage
	}

	// No match found
	return nil
}

// GetMatchingDomain returns the DomainMatcher that was used for the longest match
// This can be useful for informational purposes or debugging
func (dm DomainMapper) GetMatchingDomain(domain string) DomainScope {
	var (
		longestMatch    DomainScope
		longestMatchLen int
		foundValidMatch bool
	)

	for matcher := range dm {
		if !matcher.Is(domain) {
			continue
		}

		currentMatchLen := len(matcher)

		if !foundValidMatch || currentMatchLen > longestMatchLen {
			longestMatch = matcher
			longestMatchLen = currentMatchLen
			foundValidMatch = true
		}
	}

	if foundValidMatch {
		return longestMatch
	}

	return ""
}
