package prefab

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// ConfigMustString returns the string value for the given key.
// It panics if the key doesn't exist or the value is empty.
//
// Example:
//
//	apiKey := prefab.ConfigMustString("myapp.apiKey", "Set PF__MYAPP__API_KEY environment variable")
func ConfigMustString(key, helpMsg string) string {
	if !Config.Exists(key) {
		panic(fmt.Sprintf("required config '%s' not set: %s", key, helpMsg))
	}
	value := Config.String(key)
	if value == "" {
		panic(fmt.Sprintf("required config '%s' is empty: %s", key, helpMsg))
	}
	return value
}

// ConfigMustInt returns the int value for the given key with range validation.
// It panics if the key doesn't exist or the value is outside the given range.
//
// Example:
//
//	port := prefab.ConfigMustInt("server.port", 1, 65535)
func ConfigMustInt(key string, minVal, maxVal int) int {
	if !Config.Exists(key) {
		panic(fmt.Sprintf("required config '%s' not set (expected %d-%d)", key, minVal, maxVal))
	}
	value := Config.Int(key)
	if err := ValidateIntRange(value, minVal, maxVal); err != nil {
		panic(fmt.Sprintf("config '%s': %v", key, err))
	}
	return value
}

// ConfigMustDurationRange returns the duration value for the given key with range validation.
// It panics if the key doesn't exist or the value is outside the given range.
//
// Example:
//
//	timeout := prefab.ConfigMustDurationRange("myapp.timeout", time.Second, time.Minute)
func ConfigMustDurationRange(key string, minVal, maxVal time.Duration) time.Duration {
	if !Config.Exists(key) {
		panic(fmt.Sprintf("required config '%s' not set (expected %s-%s)", key, minVal, maxVal))
	}
	value := Config.Duration(key)
	if err := ValidateDurationRange(value, minVal, maxVal); err != nil {
		panic(fmt.Sprintf("config '%s': %v", key, err))
	}
	return value
}

// ValidateIntRange validates that a value is within the given range (inclusive).
func ValidateIntRange(value, minVal, maxVal int) error {
	if value < minVal || value > maxVal {
		return fmt.Errorf("must be between %d and %d, got: %d", minVal, maxVal, value)
	}
	return nil
}

// ValidateDurationRange validates that a duration is within the given range (inclusive).
func ValidateDurationRange(value, minVal, maxVal time.Duration) error {
	if value < minVal || value > maxVal {
		return fmt.Errorf("must be between %s and %s, got: %s", minVal, maxVal, value)
	}
	return nil
}

// ValidatePort validates that a port number is valid (1-65535).
func ValidatePort(port int) error {
	return ValidateIntRange(port, 1, 65535)
}

// ValidatePositiveInt validates that an integer is positive (> 0).
func ValidatePositiveInt(value int) error {
	if value <= 0 {
		return fmt.Errorf("must be positive, got: %d", value)
	}
	return nil
}

// ValidatePositiveDuration validates that a duration is positive (> 0).
func ValidatePositiveDuration(value time.Duration) error {
	if value <= 0 {
		return fmt.Errorf("must be positive, got: %s", value)
	}
	return nil
}

// ValidateNonNegativeDuration validates that a duration is non-negative (>= 0).
func ValidateNonNegativeDuration(value time.Duration) error {
	if value < 0 {
		return fmt.Errorf("must be non-negative, got: %s", value)
	}
	return nil
}

// ValidateURL validates that a string is a valid URL.
func ValidateURL(urlStr string) error {
	if urlStr == "" {
		return errors.New("URL cannot be empty")
	}
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme == "" {
		return errors.New("URL must have a scheme (http:// or https://)")
	}
	if parsed.Host == "" {
		return errors.New("URL must have a host")
	}
	return nil
}

// ValidateNonEmpty validates that a string is not empty.
func ValidateNonEmpty(value string) error {
	if value == "" {
		return errors.New("cannot be empty")
	}
	return nil
}

// ValidationError represents a configuration validation error.
type ValidationError struct {
	Key     string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Key, e.Message)
}

// ValidateConfig performs comprehensive validation of critical configuration values.
// Returns all validation errors found, or nil if configuration is valid.
//
// This should be called early in server initialization to fail fast on misconfigurations.
func ValidateConfig() []ValidationError {
	var errors []ValidationError

	// Validate server.port if set
	if Config.Exists("server.port") {
		port := Config.Int("server.port")
		if err := ValidatePort(port); err != nil {
			errors = append(errors, ValidationError{
				Key:     "server.port",
				Message: err.Error(),
			})
		}
	}

	// Validate server.host is not empty if set
	if Config.Exists("server.host") {
		host := Config.String("server.host")
		if err := ValidateNonEmpty(host); err != nil {
			errors = append(errors, ValidationError{
				Key:     "server.host",
				Message: err.Error(),
			})
		}
	}

	// Validate server.maxMsgSizeBytes if set
	if Config.Exists("server.maxMsgSizeBytes") {
		size := Config.Int("server.maxMsgSizeBytes")
		if err := ValidatePositiveInt(size); err != nil {
			errors = append(errors, ValidationError{
				Key:     "server.maxMsgSizeBytes",
				Message: err.Error(),
			})
		}
	}

	// Validate server.security.hstsExpiration if set
	if Config.Exists("server.security.hstsExpiration") {
		duration := Config.Duration("server.security.hstsExpiration")
		if duration > 0 { // 0 means disabled, which is valid
			if err := ValidatePositiveDuration(duration); err != nil {
				errors = append(errors, ValidationError{
					Key:     "server.security.hstsExpiration",
					Message: err.Error(),
				})
			}
		}
	}

	// Validate server.security.corsMaxAge if set
	if Config.Exists("server.security.corsMaxAge") {
		duration := Config.Duration("server.security.corsMaxAge")
		if err := ValidateNonNegativeDuration(duration); err != nil {
			errors = append(errors, ValidationError{
				Key:     "server.security.corsMaxAge",
				Message: err.Error(),
			})
		}
	}

	// Validate auth.expiration if set
	if Config.Exists("auth.expiration") {
		duration := Config.Duration("auth.expiration")
		if err := ValidatePositiveDuration(duration); err != nil {
			errors = append(errors, ValidationError{
				Key:     "auth.expiration",
				Message: err.Error(),
			})
		}
	}

	return errors
}

// FormatValidationErrors formats a slice of validation errors into a readable error message.
func FormatValidationErrors(errors []ValidationError) string {
	if len(errors) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("Configuration validation failed:\n")
	for _, err := range errors {
		sb.WriteString(fmt.Sprintf("  - %s\n", err.Error()))
	}
	sb.WriteString("\nFix these errors in prefab.yaml or environment variables and try again.")
	return sb.String()
}
