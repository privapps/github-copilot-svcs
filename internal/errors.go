// Package internal provides error types and helpers for github-copilot-svcs.
package internal

import (
	"fmt"
	"net/http"
)

type (
	// AuthenticationError ...
	AuthenticationError struct {
		Message string
		Err     error
	}

	// ConfigurationError ...
	ConfigurationError struct {
		Field   string
		Value   interface{}
		Message string
		Err     error
	}

	// NetworkError ...
	NetworkError struct {
		Operation string
		URL       string
		Message   string
		Err       error
	}

	// ValidationError ...
	ValidationError struct {
		Field   string
		Value   interface{}
		Message string
		Err     error
	}

	// ProxyError ...
	ProxyError struct {
		Operation string
		Message   string
		Err       error
	}
)

func (e *AuthenticationError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("authentication error: %s: %v", e.Message, e.Err)
	}
	return fmt.Sprintf("authentication error: %s", e.Message)
}

func (e *AuthenticationError) Unwrap() error {
	return e.Err
}

func (e *ConfigurationError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("configuration error for %s=%v: %s: %v", e.Field, e.Value, e.Message, e.Err)
	}
	return fmt.Sprintf("configuration error for %s=%v: %s", e.Field, e.Value, e.Message)
}

func (e *ConfigurationError) Unwrap() error {
	return e.Err
}

func (e *NetworkError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("network error during %s to %s: %s: %v", e.Operation, e.URL, e.Message, e.Err)
	}
	return fmt.Sprintf("network error during %s to %s: %s", e.Operation, e.URL, e.Message)
}

func (e *NetworkError) Unwrap() error {
	return e.Err
}

func (e *ValidationError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("validation error for %s=%v: %s: %v", e.Field, e.Value, e.Message, e.Err)
	}
	return fmt.Sprintf("validation error for %s=%v: %s", e.Field, e.Value, e.Message)
}

func (e *ValidationError) Unwrap() error {
	return e.Err
}

func (e *ProxyError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("proxy error during %s: %s: %v", e.Operation, e.Message, e.Err)
	}
	return fmt.Sprintf("proxy error during %s: %s", e.Operation, e.Message)
}

func (e *ProxyError) Unwrap() error {
	return e.Err
}

// NewAuthError ...
func NewAuthError(message string, err error) *AuthenticationError {
	return &AuthenticationError{Message: message, Err: err}
}

// NewConfigError ...
func NewConfigError(field string, value interface{}, message string, err error) *ConfigurationError {
	return &ConfigurationError{Field: field, Value: value, Message: message, Err: err}
}

// NewNetworkError ...
func NewNetworkError(operation, url, message string, err error) *NetworkError {
	return &NetworkError{Operation: operation, URL: url, Message: message, Err: err}
}

// NewValidationError ...
func NewValidationError(field string, value interface{}, message string, err error) *ValidationError {
	return &ValidationError{Field: field, Value: value, Message: message, Err: err}
}

// NewProxyError ...
func NewProxyError(operation, message string, err error) *ProxyError {
	return &ProxyError{Operation: operation, Message: message, Err: err}
}

// WriteHTTPError ...
func WriteHTTPError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = fmt.Fprintf(w, `{"error": {"message": "%s", "type": "error", "code": %d}}`, message, statusCode)
}

// WriteHTTPErrorWithDetails ...
func WriteHTTPErrorWithDetails(w http.ResponseWriter, statusCode int, errorType, message, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = fmt.Fprintf(w, `{"error": {"message": "%s", "type": "%s", "code": %d, "details": "%s"}}`,
		message, errorType, statusCode, details)
}

// WriteAuthenticationError ...
func WriteAuthenticationError(w http.ResponseWriter) {
	WriteHTTPError(w, http.StatusUnauthorized, "Authentication required")
}

// WriteAuthorizationError ...
func WriteAuthorizationError(w http.ResponseWriter) {
	WriteHTTPError(w, http.StatusForbidden, "Insufficient permissions")
}

// WriteValidationError ...
func WriteValidationError(w http.ResponseWriter, message string) {
	WriteHTTPError(w, http.StatusBadRequest, message)
}

// WriteInternalError ...
func WriteInternalError(w http.ResponseWriter) {
	WriteHTTPError(w, http.StatusInternalServerError, "Internal server error")
}

// WriteServiceUnavailableError ...
func WriteServiceUnavailableError(w http.ResponseWriter) {
	WriteHTTPError(w, http.StatusServiceUnavailable, "Service temporarily unavailable")
}

// WriteRateLimitError ...
func WriteRateLimitError(w http.ResponseWriter) {
	WriteHTTPError(w, http.StatusTooManyRequests, "Rate limit exceeded")
}

// IsAuthenticationError ...
func IsAuthenticationError(err error) bool {
	_, ok := err.(*AuthenticationError)
	return ok
}

// IsConfigurationError ...
func IsConfigurationError(err error) bool {
	_, ok := err.(*ConfigurationError)
	return ok
}

// IsNetworkError ...
func IsNetworkError(err error) bool {
	_, ok := err.(*NetworkError)
	return ok
}

// IsValidationError ...
func IsValidationError(err error) bool {
	_, ok := err.(*ValidationError)
	return ok
}

// IsProxyError ...
func IsProxyError(err error) bool {
	_, ok := err.(*ProxyError)
	return ok
}
