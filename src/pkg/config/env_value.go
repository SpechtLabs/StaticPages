package config

import (
	"fmt"
	"os"
	"strings"

	humane "github.com/sierrasoftworks/humane-errors-go"
)

// EnvValue represents a value that can be either a literal string
// or an environment variable reference using ENV('VAR_NAME')
type EnvValue string

// UnmarshalYAML unmarshals a YAML value into an EnvValue, resolving environment variables if the ENV() syntax is used.
func (e *EnvValue) UnmarshalYAML(value string) humane.Error {
	*e = EnvValue(value)
	return e.Parse()
}

// UnmarshalText unmarshals a text byte slice into an EnvValue by delegating to its YAML unmarshaling implementation.
func (e *EnvValue) UnmarshalText(text []byte) humane.Error {
	return e.UnmarshalYAML(string(text))
}

func (e *EnvValue) Parse() humane.Error {
	value := string(*e)

	// Check if the string uses ENV() syntax
	if strings.HasPrefix(value, "ENV(") && strings.HasSuffix(value, ")") {
		// Extract the environment variable name
		envVarName := strings.TrimPrefix(value, "ENV(")
		envVarName = strings.TrimSuffix(envVarName, ")")

		// Get the environment variable value
		envValue := os.Getenv(envVarName)
		if envValue == "" {
			return humane.New(fmt.Sprintf("environment variable %s is not set", envVarName), "Make sure the environment variable is set.")
		}

		*e = EnvValue(envValue)
	} else {
		// If it's not an ENV() reference, use the literal value
		*e = EnvValue(value)
	}

	return nil
}

// String returns the string representation of the EnvValue.
func (e EnvValue) String() string {
	return string(e)
}
