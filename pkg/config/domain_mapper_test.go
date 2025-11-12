package config_test

import (
	"testing"

	"github.com/SpechtLabs/StaticPages/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestNewDomainMapperFromPages_Initialization(t *testing.T) {
	// Create the sample domain mapper
	mapper := createTestDomainMapper()

	// Test basic structure
	assert.Len(t, mapper, 3, "Mapper should contain 3 domain entries")
}

func TestNewDomainMapperFromPages_DirectAccess(t *testing.T) {
	// Create the sample domain mapper
	mapper := createTestDomainMapper()

	// Test direct map access
	directAccessTests := []struct {
		name     string
		domain   config.DomainScope
		expected bool
	}{
		{
			name:     "existing domain - example.com",
			domain:   config.DomainScope("example.com"),
			expected: true,
		},
		{
			name:     "existing domain - sub.example.com",
			domain:   config.DomainScope("sub.example.com"),
			expected: true,
		},
		{
			name:     "existing domain - another.com",
			domain:   config.DomainScope("another.com"),
			expected: true,
		},
		{
			name:     "non-existent domain",
			domain:   config.DomainScope("nonexistent.com"),
			expected: false,
		},
	}

	for _, tc := range directAccessTests {
		t.Run(tc.name, func(t *testing.T) {
			page := mapper[tc.domain]
			if tc.expected {
				assert.NotNil(t, page, "Should have page for %s", tc.domain)
			} else {
				assert.Nil(t, page, "Should not have page for %s", tc.domain)
			}
		})
	}
}

func TestNewDomainMapperFromPages_Lookup(t *testing.T) {
	// Create the sample domain mapper
	mapper := createTestDomainMapper()

	// Test lookup functionality
	lookupTests := []struct {
		name           string
		lookupDomain   string
		expectFound    bool
		expectedBucket string
	}{
		{
			name:           "exact match - example.com",
			lookupDomain:   "example.com",
			expectFound:    true,
			expectedBucket: "example-bucket",
		},
		{
			name:           "exact match - sub.example.com",
			lookupDomain:   "sub.example.com",
			expectFound:    true,
			expectedBucket: "sub-example-bucket",
		},
		{
			name:           "subdomain match - deep.sub.example.com",
			lookupDomain:   "deep.sub.example.com",
			expectFound:    true,
			expectedBucket: "sub-example-bucket",
		},
		{
			name:           "no match - nonexistent.com",
			lookupDomain:   "nonexistent.com",
			expectFound:    false,
			expectedBucket: "",
		},
	}

	for _, tc := range lookupTests {
		t.Run(tc.name, func(t *testing.T) {
			page := mapper.Lookup(tc.lookupDomain)
			if tc.expectFound {
				assert.NotNil(t, page, "Should find page for %s", tc.lookupDomain)
				assert.Equal(t, tc.expectedBucket, page.Bucket.Name.String(),
					"Should get correct bucket for %s", tc.lookupDomain)
			} else {
				assert.Nil(t, page, "Should not find page for %s", tc.lookupDomain)
			}
		})
	}
}

func TestNewDomainMapperFromPages_GetMatchingDomain(t *testing.T) {
	// Create the sample domain mapper
	mapper := createTestDomainMapper()

	// Test GetMatchingDomain
	matchingDomainTests := []struct {
		name           string
		inputDomain    string
		expectedDomain config.DomainScope
	}{
		{
			name:           "exact match - example.com",
			inputDomain:    "example.com",
			expectedDomain: config.DomainScope("example.com"),
		},
		{
			name:           "exact match - sub.example.com",
			inputDomain:    "sub.example.com",
			expectedDomain: config.DomainScope("sub.example.com"),
		},
		{
			name:           "subdomain match - deep.sub.example.com",
			inputDomain:    "deep.sub.example.com",
			expectedDomain: config.DomainScope("sub.example.com"),
		},
		{
			name:           "no match - nonexistent.com",
			inputDomain:    "nonexistent.com",
			expectedDomain: config.DomainScope(""),
		},
	}

	for _, tc := range matchingDomainTests {
		t.Run(tc.name, func(t *testing.T) {
			matchingDomain := mapper.GetMatchingDomain(tc.inputDomain)
			assert.Equal(t, tc.expectedDomain, matchingDomain,
				"Should return the correct matching domain for %s", tc.inputDomain)
		})
	}
}

// Helper function to create a test domain mapper
func createTestDomainMapper() config.DomainMapper {
	// Create test pages
	pages := []*config.Page{
		{
			Domain: config.DomainScope("example.com"),
			Bucket: config.BucketConfig{
				Name: "example-bucket",
			},
		},
		{
			Domain: config.DomainScope("sub.example.com"),
			Bucket: config.BucketConfig{
				Name: "sub-example-bucket",
			},
		},
		{
			Domain: config.DomainScope("another.com"),
			Bucket: config.BucketConfig{
				Name: "another-bucket",
			},
		},
	}

	// Create and return the domain mapper
	return config.NewDomainMapperFromPages(pages)
}

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
