package config

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvValueParse(t *testing.T) {
	// Save and restore the original environment
	originalEnv := os.Environ()
	defer func() {
		os.Clearenv()
		for _, env := range originalEnv {
			parts := strings.SplitN(env, "=", 2)
			_ = os.Setenv(parts[0], parts[1])
		}
	}()

	// Define test cases
	tests := []struct {
		name          string
		envVariable   string // Key of the environment variable to set (if any)
		envValue      string // Value of the environment variable to set (if any)
		inputValue    string // Input value to EnvValue
		expectedValue string // Expected parsed EnvValue
		expectError   bool   // Whether an error is expected
		errorMessage  string // Expected error message
	}{
		{
			name:          "literal value",
			envVariable:   "",
			envValue:      "",
			inputValue:    "literal_value",
			expectedValue: "literal_value",
			expectError:   false,
		},
		{
			name:          "ENV() reference with variable set",
			envVariable:   "MY_ENV",
			envValue:      "env_value",
			inputValue:    "ENV(MY_ENV)",
			expectedValue: "env_value",
			expectError:   false,
		},
		{
			name:          "ENV() reference with variable not set",
			envVariable:   "", // No variable set
			envValue:      "",
			inputValue:    "ENV(MISSING_ENV)",
			expectedValue: "",
			expectError:   true,
			errorMessage:  "environment variable MISSING_ENV is not set",
		},
		{
			name:          "invalid ENV() format (no closing parenthesis)",
			envVariable:   "",
			envValue:      "",
			inputValue:    "ENV(MY_ENV",
			expectedValue: "ENV(MY_ENV",
			expectError:   false,
		},
		{
			name:          "empty ENV() reference",
			envVariable:   "",
			envValue:      "",
			inputValue:    "ENV()",
			expectedValue: "",
			expectError:   true,
			errorMessage:  "environment variable  is not set",
		},
		{
			name:          "ENV() with spaces around key",
			envVariable:   "MY_ENV",
			envValue:      "spaced_value",
			inputValue:    "ENV( MY_ENV  )",
			expectedValue: "",
			expectError:   true,
			errorMessage:  "environment variable  MY_ENV   is not set", // Spaces in variable names are invalid in `os.Getenv`
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Set the environment variable if defined
			if test.envVariable != "" {
				_ = os.Setenv(test.envVariable, test.envValue)
			}

			// Create an EnvValue and parse it
			e := EnvValue(test.inputValue)
			err := e.Validate()

			// Check expected results
			if test.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), test.errorMessage)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expectedValue, e.String())
			}

			// Cleanup environment variable
			if test.envVariable != "" {
				_ = os.Unsetenv(test.envVariable)
			}
		})
	}
}
