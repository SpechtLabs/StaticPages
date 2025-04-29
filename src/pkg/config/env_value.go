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
