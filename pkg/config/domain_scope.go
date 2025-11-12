package config

import (
	"fmt"
	"strings"
)

// DomainScope is a string that represents a Domain Name.
// The speciality of DomainScope is the ability to perform a longest-prefix-match of a given domain against the
// DomainScope Domain.
// For Example:
// domain := DomainScope("example.com")
// domain.Is("example.com") == true
// domain.Is("foo.example.com") == true
// domain.Is("foo.bar.com") == false
// domain.Subdomain("foo.bar.example.com") = "foo.bar", error(nil)
// domain.Subdomain("foo.bar.com") = "", error(foo.bar.com is not associated with example.com)
type DomainScope string

// Is checks if the given domain matches or is a subdomain of the DomainScope.
// Returns true if the domain is either exactly the same as the DomainScope
// or is a subdomain of it (ends with .DomainScope).
func (d DomainScope) Is(domain string) bool {
	// If domains are exactly the same, they match
	if string(d) == domain {
		return true
	}

	// Check if domain is a subdomain of the matcher
	suffix := "." + string(d)
	return strings.HasSuffix(domain, suffix)
}

// Subdomain extracts the subdomain part from a given domain.
// For example, if DomainScope is "example.com" and domain is "foo.bar.example.com",
// it returns "foo.bar".
// Returns an error if the given domain is not associated with the DomainScope.
func (d DomainScope) Subdomain(domain string) (string, error) {
	if !d.Is(domain) {
		return "", fmt.Errorf("%s is not associated with %s", domain, string(d))
	}

	// If domains are exactly the same, there's no subdomain
	if string(d) == domain {
		return "", nil
	}

	// Remove the domain part and the dot separator
	suffixLength := len(string(d)) + 1 // +1 for the dot
	return domain[:len(domain)-suffixLength], nil
}

// String returns the string representation of the DomainScope
func (d DomainScope) String() string {
	return string(d)
}

// Normalize ensures the domain is in a consistent format
// (currently just returns the domain as-is, but could be extended
// to handle normalization like stripping trailing dots, lowercasing, etc.)
func (d DomainScope) Normalize() DomainScope {
	return DomainScope(strings.ToLower(strings.TrimSuffix(string(d), ".")))
}

// FromString creates a new DomainScope from a string
func FromString(domain string) DomainScope {
	return DomainScope(domain).Normalize()
}

// Parent returns the parent domain of the current domain, or empty string if it's a top-level domain
// For example: "foo.example.com" -> "example.com", "example.com" -> "com", "com" -> ""
func (d DomainScope) Parent() DomainScope {
	parts := strings.Split(string(d), ".")
	if len(parts) <= 1 {
		return ""
	}
	return DomainScope(strings.Join(parts[1:], "."))
}

// HasParent checks if this domain is a subdomain of the provided parent
func (d DomainScope) HasParent(parent DomainScope) bool {
	return parent.Is(string(d)) && string(d) != string(parent)
}

// Level returns the domain level (number of dot-separated parts)
// For example: "foo.example.com" -> 3, "example.com" -> 2, "com" -> 1
func (d DomainScope) Level() int {
	if d == "" {
		return 0
	}
	return len(strings.Split(string(d), "."))
}
