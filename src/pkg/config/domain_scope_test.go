package config_test

import (
	"errors"
	"testing"

	"github.com/SpechtLabs/StaticPages/pkg/config"
)

func TestDomainMatcher_Is(t *testing.T) {
	tests := []struct {
		name           string
		domainMatcher  config.DomainScope
		testDomain     string
		expectedResult bool
	}{
		{
			name:           "exact match",
			domainMatcher:  config.DomainScope("example.com"),
			testDomain:     "example.com",
			expectedResult: true,
		},
		{
			name:           "subdomain match",
			domainMatcher:  config.DomainScope("example.com"),
			testDomain:     "foo.example.com",
			expectedResult: true,
		},
		{
			name:           "nested subdomain match",
			domainMatcher:  config.DomainScope("example.com"),
			testDomain:     "foo.bar.example.com",
			expectedResult: true,
		},
		{
			name:           "no match - different domain",
			domainMatcher:  config.DomainScope("example.com"),
			testDomain:     "example.org",
			expectedResult: false,
		},
		{
			name:           "no match - subdomain of different domain",
			domainMatcher:  config.DomainScope("example.com"),
			testDomain:     "foo.example.org",
			expectedResult: false,
		},
		{
			name:           "no match - partial string match",
			domainMatcher:  config.DomainScope("example.com"),
			testDomain:     "myexample.com",
			expectedResult: false,
		},
		{
			name:           "tld match",
			domainMatcher:  config.DomainScope("com"),
			testDomain:     "example.com",
			expectedResult: true,
		},
		{
			name:           "empty matcher",
			domainMatcher:  config.DomainScope(""),
			testDomain:     "example.com",
			expectedResult: false,
		},
		{
			name:           "empty test domain",
			domainMatcher:  config.DomainScope("example.com"),
			testDomain:     "",
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.domainMatcher.Is(tt.testDomain)
			if result != tt.expectedResult {
				t.Errorf("DomainScope(%q).Is(%q) = %v, want %v",
					string(tt.domainMatcher), tt.testDomain, result, tt.expectedResult)
			}
		})
	}
}

func TestDomainMatcher_Subdomain(t *testing.T) {
	tests := []struct {
		name              string
		domainMatcher     config.DomainScope
		testDomain        string
		expectedSubdomain string
		expectedError     error
	}{
		{
			name:              "exact match - no subdomain",
			domainMatcher:     config.DomainScope("example.com"),
			testDomain:        "example.com",
			expectedSubdomain: "",
			expectedError:     nil,
		},
		{
			name:              "simple subdomain",
			domainMatcher:     config.DomainScope("example.com"),
			testDomain:        "foo.example.com",
			expectedSubdomain: "foo",
			expectedError:     nil,
		},
		{
			name:              "nested subdomain",
			domainMatcher:     config.DomainScope("example.com"),
			testDomain:        "foo.bar.example.com",
			expectedSubdomain: "foo.bar",
			expectedError:     nil,
		},
		{
			name:              "not a subdomain - different domain",
			domainMatcher:     config.DomainScope("example.com"),
			testDomain:        "example.org",
			expectedSubdomain: "",
			expectedError:     errors.New("example.org is not associated with example.com"),
		},
		{
			name:              "not a subdomain - partial string match",
			domainMatcher:     config.DomainScope("example.com"),
			testDomain:        "myexample.com",
			expectedSubdomain: "",
			expectedError:     errors.New("myexample.com is not associated with example.com"),
		},
		{
			name:              "tld as domain matcher",
			domainMatcher:     config.DomainScope("com"),
			testDomain:        "example.com",
			expectedSubdomain: "example",
			expectedError:     nil,
		},
		{
			name:              "multi-part subdomain with tld matcher",
			domainMatcher:     config.DomainScope("com"),
			testDomain:        "foo.bar.example.com",
			expectedSubdomain: "foo.bar.example",
			expectedError:     nil,
		},
		{
			name:              "empty test domain",
			domainMatcher:     config.DomainScope("example.com"),
			testDomain:        "",
			expectedSubdomain: "",
			expectedError:     errors.New(" is not associated with example.com"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subdomain, err := tt.domainMatcher.Subdomain(tt.testDomain)

			// Check error
			if (err == nil && tt.expectedError != nil) ||
				(err != nil && tt.expectedError == nil) ||
				(err != nil && tt.expectedError != nil && err.Error() != tt.expectedError.Error()) {
				t.Errorf("DomainScope(%q).Subdomain(%q) error = %v, want %v",
					string(tt.domainMatcher), tt.testDomain, err, tt.expectedError)
			}

			// Check subdomain
			if subdomain != tt.expectedSubdomain {
				t.Errorf("DomainScope(%q).Subdomain(%q) = %q, want %q",
					string(tt.domainMatcher), tt.testDomain, subdomain, tt.expectedSubdomain)
			}
		})
	}
}

func TestDomainMatcher_String(t *testing.T) {
	tests := []struct {
		name           string
		domainMatcher  config.DomainScope
		expectedString string
	}{
		{
			name:           "standard domain",
			domainMatcher:  config.DomainScope("example.com"),
			expectedString: "example.com",
		},
		{
			name:           "subdomain",
			domainMatcher:  config.DomainScope("sub.example.com"),
			expectedString: "sub.example.com",
		},
		{
			name:           "empty domain",
			domainMatcher:  config.DomainScope(""),
			expectedString: "",
		},
		{
			name:           "tld only",
			domainMatcher:  config.DomainScope("com"),
			expectedString: "com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.domainMatcher.String()
			if result != tt.expectedString {
				t.Errorf("DomainScope(%q).String() = %q, want %q",
					string(tt.domainMatcher), result, tt.expectedString)
			}
		})
	}
}

func TestDomainMatcher_Normalize(t *testing.T) {
	tests := []struct {
		name               string
		domainMatcher      config.DomainScope
		expectedNormalized config.DomainScope
	}{
		{
			name:               "standard domain",
			domainMatcher:      config.DomainScope("example.com"),
			expectedNormalized: config.DomainScope("example.com"),
		},
		{
			name:               "uppercase domain",
			domainMatcher:      config.DomainScope("EXAMPLE.COM"),
			expectedNormalized: config.DomainScope("example.com"),
		},
		{
			name:               "mixed case domain",
			domainMatcher:      config.DomainScope("ExAmPlE.CoM"),
			expectedNormalized: config.DomainScope("example.com"),
		},
		{
			name:               "domain with trailing dot",
			domainMatcher:      config.DomainScope("example.com."),
			expectedNormalized: config.DomainScope("example.com"),
		},
		{
			name:               "uppercase with trailing dot",
			domainMatcher:      config.DomainScope("EXAMPLE.COM."),
			expectedNormalized: config.DomainScope("example.com"),
		},
		{
			name:               "empty domain",
			domainMatcher:      config.DomainScope(""),
			expectedNormalized: config.DomainScope(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.domainMatcher.Normalize()
			if result != tt.expectedNormalized {
				t.Errorf("DomainScope(%q).Normalize() = %q, want %q",
					string(tt.domainMatcher), string(result), string(tt.expectedNormalized))
			}
		})
	}
}

func TestFromString(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedResult config.DomainScope
	}{
		{
			name:           "standard domain",
			input:          "example.com",
			expectedResult: config.DomainScope("example.com"),
		},
		{
			name:           "uppercase domain",
			input:          "EXAMPLE.COM",
			expectedResult: config.DomainScope("example.com"),
		},
		{
			name:           "domain with trailing dot",
			input:          "example.com.",
			expectedResult: config.DomainScope("example.com"),
		},
		{
			name:           "empty domain",
			input:          "",
			expectedResult: config.DomainScope(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.FromString(tt.input)
			if result != tt.expectedResult {
				t.Errorf("FromString(%q) = %q, want %q",
					tt.input, string(result), string(tt.expectedResult))
			}
		})
	}
}

func TestDomainMatcher_Parent(t *testing.T) {
	tests := []struct {
		name           string
		domainMatcher  config.DomainScope
		expectedParent config.DomainScope
	}{
		{
			name:           "subdomain to domain",
			domainMatcher:  config.DomainScope("foo.example.com"),
			expectedParent: config.DomainScope("example.com"),
		},
		{
			name:           "domain to tld",
			domainMatcher:  config.DomainScope("example.com"),
			expectedParent: config.DomainScope("com"),
		},
		{
			name:           "tld to empty",
			domainMatcher:  config.DomainScope("com"),
			expectedParent: config.DomainScope(""),
		},
		{
			name:           "multi-level subdomain",
			domainMatcher:  config.DomainScope("sub.foo.example.com"),
			expectedParent: config.DomainScope("foo.example.com"),
		},
		{
			name:           "empty domain",
			domainMatcher:  config.DomainScope(""),
			expectedParent: config.DomainScope(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.domainMatcher.Parent()
			if result != tt.expectedParent {
				t.Errorf("DomainScope(%q).Parent() = %q, want %q",
					string(tt.domainMatcher), string(result), string(tt.expectedParent))
			}
		})
	}
}

func TestDomainMatcher_HasParent(t *testing.T) {
	tests := []struct {
		name          string
		domainMatcher config.DomainScope
		parent        config.DomainScope
		expected      bool
	}{
		{
			name:          "direct parent",
			domainMatcher: config.DomainScope("foo.example.com"),
			parent:        config.DomainScope("example.com"),
			expected:      true,
		},
		{
			name:          "ancestor parent",
			domainMatcher: config.DomainScope("sub.foo.example.com"),
			parent:        config.DomainScope("example.com"),
			expected:      true,
		},
		{
			name:          "tld parent",
			domainMatcher: config.DomainScope("example.com"),
			parent:        config.DomainScope("com"),
			expected:      true,
		},
		{
			name:          "same domain - not parent",
			domainMatcher: config.DomainScope("example.com"),
			parent:        config.DomainScope("example.com"),
			expected:      false,
		},
		{
			name:          "unrelated domain",
			domainMatcher: config.DomainScope("example.com"),
			parent:        config.DomainScope("example.org"),
			expected:      false,
		},
		{
			name:          "subdomain of different domain",
			domainMatcher: config.DomainScope("foo.example.com"),
			parent:        config.DomainScope("example.org"),
			expected:      false,
		},
		{
			name:          "empty parent",
			domainMatcher: config.DomainScope("example.com"),
			parent:        config.DomainScope(""),
			expected:      false,
		},
		{
			name:          "empty domain",
			domainMatcher: config.DomainScope(""),
			parent:        config.DomainScope("com"),
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.domainMatcher.HasParent(tt.parent)
			if result != tt.expected {
				t.Errorf("DomainScope(%q).HasParent(%q) = %v, want %v",
					string(tt.domainMatcher), string(tt.parent), result, tt.expected)
			}
		})
	}
}

func TestDomainMatcher_Level(t *testing.T) {
	tests := []struct {
		name          string
		domainMatcher config.DomainScope
		expectedLevel int
	}{
		{
			name:          "top-level domain",
			domainMatcher: config.DomainScope("com"),
			expectedLevel: 1,
		},
		{
			name:          "second-level domain",
			domainMatcher: config.DomainScope("example.com"),
			expectedLevel: 2,
		},
		{
			name:          "subdomain",
			domainMatcher: config.DomainScope("foo.example.com"),
			expectedLevel: 3,
		},
		{
			name:          "multi-level subdomain",
			domainMatcher: config.DomainScope("sub.foo.example.com"),
			expectedLevel: 4,
		},
		{
			name:          "empty domain",
			domainMatcher: config.DomainScope(""),
			expectedLevel: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.domainMatcher.Level()
			if result != tt.expectedLevel {
				t.Errorf("DomainScope(%q).Level() = %d, want %d",
					string(tt.domainMatcher), result, tt.expectedLevel)
			}
		})
	}
}

// TestDomainMatcherIntegration performs more complex tests involving multiple methods
func TestDomainMatcherIntegration(t *testing.T) {
	tests := []struct {
		name            string
		domainMatcher   config.DomainScope
		testDomain      string
		expectIs        bool
		expectSubdomain string
		expectParent    config.DomainScope
		expectLevel     int
	}{
		{
			name:            "standard domain",
			domainMatcher:   config.DomainScope("example.com"),
			testDomain:      "foo.example.com",
			expectIs:        true,
			expectSubdomain: "foo",
			expectParent:    config.DomainScope("com"),
			expectLevel:     2,
		},
		{
			name:            "multi-level domain",
			domainMatcher:   config.DomainScope("foo.example.com"),
			testDomain:      "bar.foo.example.com",
			expectIs:        true,
			expectSubdomain: "bar",
			expectParent:    config.DomainScope("example.com"),
			expectLevel:     3,
		},
		{
			name:            "edge case - tld",
			domainMatcher:   config.DomainScope("com"),
			testDomain:      "example.com",
			expectIs:        true,
			expectSubdomain: "example",
			expectParent:    config.DomainScope(""),
			expectLevel:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Is method
			isResult := tt.domainMatcher.Is(tt.testDomain)
			if isResult != tt.expectIs {
				t.Errorf("DomainScope(%q).Is(%q) = %v, want %v",
					string(tt.domainMatcher), tt.testDomain, isResult, tt.expectIs)
			}

			// Test Subdomain method
			subdomain, _ := tt.domainMatcher.Subdomain(tt.testDomain)
			if subdomain != tt.expectSubdomain {
				t.Errorf("DomainScope(%q).Subdomain(%q) = %q, want %q",
					string(tt.domainMatcher), tt.testDomain, subdomain, tt.expectSubdomain)
			}

			// Test Parent method
			parent := tt.domainMatcher.Parent()
			if parent != tt.expectParent {
				t.Errorf("DomainScope(%q).Parent() = %q, want %q",
					string(tt.domainMatcher), string(parent), string(tt.expectParent))
			}

			// Test Level method
			level := tt.domainMatcher.Level()
			if level != tt.expectLevel {
				t.Errorf("DomainScope(%q).Level() = %d, want %d",
					string(tt.domainMatcher), level, tt.expectLevel)
			}
		})
	}
}
