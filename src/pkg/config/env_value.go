package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/sierrasoftworks/humane-errors-go"
)

// EnvValue represents a value that can be either a literal string
// or an environment variable reference using ENV('VAR_NAME')
type EnvValue string

func (e *EnvValue) Validate() humane.Error {
	if _, err := e.resolveEnvVar(); err != nil {
		return err
	}

	return nil
}

func (e *EnvValue) hasEnvPrefix() bool {
	value := string(*e)
	return strings.HasPrefix(value, "ENV(") && strings.HasSuffix(value, ")")
}

func (e *EnvValue) extractEnvName() string {
	value := string(*e)

	// Extract the environment variable name
	envVarName := strings.TrimPrefix(value, "ENV(")
	envVarName = strings.TrimSuffix(envVarName, ")")

	return envVarName
}

func (e *EnvValue) resolveEnvVar() (string, humane.Error) {
	// Check if the string uses ENV() syntax
	if e.hasEnvPrefix() {
		// Get the environment variable value
		envName := e.extractEnvName()
		if envName == "" {
			return "", humane.New(fmt.Sprintf("environment variable %s is not set", e.extractEnvName()), "Make sure the environment variable is set.")
		}

		envValue := os.Getenv(envName)
		if envValue == "" {
			return "", humane.New(fmt.Sprintf("environment variable %s is not set", e.extractEnvName()), "Make sure the environment variable is set.")
		}

		return envValue, nil
	}

	// If it's not an ENV() reference, use the literal value
	return string(*e), nil
}

// String returns the string representation of the EnvValue.
func (e EnvValue) String() string {
	str := "## UNRESOLVED ENVIRONMENT VARIABLE ##"

	value, err := e.resolveEnvVar()
	if err == nil {
		str = value
	}

	return str
}
