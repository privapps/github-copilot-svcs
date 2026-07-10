package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

// Constants for configuration
const (
	configDirName     = ".local/share/github-copilot-svcs"
	configFileName    = "config.json"
	defaultServerPort = 8081
	dirPerm           = 0o700

	// Default header values
	defaultUserAgent            = "GitHubCopilotChat/0.29.1"
	defaultEditorVersion        = "vscode/1.102.3"
	defaultEditorPluginVersion  = "copilot-chat/0.29.1"
	defaultCopilotIntegrationID = "vscode-chat"
	defaultOpenaiIntent         = "conversation-edits"
	defaultXInitiator           = "user"

	// Timeout defaults
	defaultHTTPClientTimeout     = 300
	defaultServerReadTimeout     = 30
	defaultServerWriteTimeout    = 300
	defaultServerIdleTimeout     = 120
	defaultProxyContextTimeout   = 300
	defaultCircuitBreakerTimeout = 30
	defaultKeepAliveTimeout      = 30
	defaultTLSHandshakeTimeout   = 10
	defaultDialTimeout           = 10
	defaultIdleConnTimeout       = 90

	// Port validation
	minPortNumber = 1
	maxPortNumber = 65535

	// Timeout validation ranges
	minTimeout      = 1
	maxShortTimeout = 300
	maxLongTimeout  = 3600
)

// Config represents the application configuration
type Config struct {
	Port          int      `json:"port"`
	GitHubToken   string   `json:"github_token"`
	CopilotToken  string   `json:"copilot_token"`
	ExpiresAt     int64    `json:"expires_at"`
	RefreshIn     int64    `json:"refresh_in"`
	AllowedModels []string `json:"allowed_models"`

	// HTTP Headers configuration
	Headers struct {
		UserAgent            string `json:"user_agent"`             // Default: "GitHubCopilotChat/0.29.1"
		EditorVersion        string `json:"editor_version"`         // Default: "vscode/1.102.3"
		EditorPluginVersion  string `json:"editor_plugin_version"`  // Default: "copilot-chat/0.29.1"
		CopilotIntegrationID string `json:"copilot_integration_id"` // Default: "vscode-chat"
		OpenaiIntent         string `json:"openai_intent"`          // Default: "conversation-edits"
		XInitiator           string `json:"x_initiator"`            // Default: "user"
	} `json:"headers"`

	// CORS configuration
	CORS struct {
		AllowedOrigins []string `json:"allowed_origins"` // Default: ["*"] (permissive)
		AllowedHeaders []string `json:"allowed_headers"` // Default: ["*"]
	} `json:"cors"`

	// Timeout configurations (in seconds)
	Timeouts struct {
		HTTPClient      int `json:"http_client"`       // Default: 300s for streaming responses
		ServerRead      int `json:"server_read"`       // Default: 30s for request reading
		ServerWrite     int `json:"server_write"`      // Default: 300s for streaming responses
		ServerIdle      int `json:"server_idle"`       // Default: 120s for idle connections
		ProxyContext    int `json:"proxy_context"`     // Default: 300s for proxy request context
		CircuitBreaker  int `json:"circuit_breaker"`   // Default: 30s for circuit breaker recovery
		KeepAlive       int `json:"keep_alive"`        // Default: 30s for connection keep-alive
		TLSHandshake    int `json:"tls_handshake"`     // Default: 10s for TLS handshake
		DialTimeout     int `json:"dial_timeout"`      // Default: 10s for connection dialing
		IdleConnTimeout int `json:"idle_conn_timeout"` // Default: 90s for idle connection timeout
	} `json:"timeouts"`
}

// GetConfigPath returns the path to the config file
func GetConfigPath() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(usr.HomeDir, configDirName)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return "", err
	}
	return filepath.Join(dir, configFileName), nil
}

// LoadConfig loads the configuration from file and environment variables
func LoadConfig(skipTokenValidation ...bool) (*Config, error) {
	path, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	// Start with default config
	cfg := &Config{Port: defaultServerPort}
	SetDefaultTimeouts(cfg)
	SetDefaultHeaders(cfg)
	SetDefaultCORS(cfg)

	// Load from file if it exists
	file, err := os.Open(path)
	if err == nil {
		defer func() {
			if closeErr := file.Close(); closeErr != nil {
				Error("Failed to close config file", "error", closeErr)
			}
		}()
		if err := json.NewDecoder(file).Decode(cfg); err != nil {
			return nil, err
		}
	}

	// Override with environment variables if present
	if port := os.Getenv("COPILOT_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.Port = p
		}
	}
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		cfg.GitHubToken = token
	}
	if token := os.Getenv("COPILOT_TOKEN"); token != "" {
		cfg.CopilotToken = token
	}

	// Set default port if still not specified
	if cfg.Port == 0 {
		cfg.Port = defaultServerPort
	}

	// Validate configuration
	skip := len(skipTokenValidation) > 0 && skipTokenValidation[0]
	if skip {
		if err := cfg.validateCore(); err != nil {
			return nil, fmt.Errorf("configuration validation failed: %w", err)
		}
	} else {
		if err := cfg.Validate(); err != nil {
			return nil, fmt.Errorf("configuration validation failed: %w", err)
		}
	}

	return cfg, nil
}

// SetDefaultTimeouts sets default timeout values if they are zero
func SetDefaultTimeouts(cfg *Config) {
	if cfg.Timeouts.HTTPClient == 0 {
		cfg.Timeouts.HTTPClient = defaultHTTPClientTimeout
	}
	if cfg.Timeouts.ServerRead == 0 {
		cfg.Timeouts.ServerRead = defaultServerReadTimeout
	}
	if cfg.Timeouts.ServerWrite == 0 {
		cfg.Timeouts.ServerWrite = defaultServerWriteTimeout
	}
	if cfg.Timeouts.ServerIdle == 0 {
		cfg.Timeouts.ServerIdle = defaultServerIdleTimeout
	}
	if cfg.Timeouts.ProxyContext == 0 {
		cfg.Timeouts.ProxyContext = defaultProxyContextTimeout
	}
	if cfg.Timeouts.CircuitBreaker == 0 {
		cfg.Timeouts.CircuitBreaker = defaultCircuitBreakerTimeout
	}
	if cfg.Timeouts.KeepAlive == 0 {
		cfg.Timeouts.KeepAlive = defaultKeepAliveTimeout
	}
	if cfg.Timeouts.TLSHandshake == 0 {
		cfg.Timeouts.TLSHandshake = defaultTLSHandshakeTimeout
	}
	if cfg.Timeouts.DialTimeout == 0 {
		cfg.Timeouts.DialTimeout = defaultDialTimeout
	}
	if cfg.Timeouts.IdleConnTimeout == 0 {
		cfg.Timeouts.IdleConnTimeout = defaultIdleConnTimeout
	}
}

// SetDefaultHeaders sets default header values if they are empty
func SetDefaultHeaders(cfg *Config) {
	if cfg.Headers.UserAgent == "" {
		cfg.Headers.UserAgent = defaultUserAgent
	}
	if cfg.Headers.EditorVersion == "" {
		cfg.Headers.EditorVersion = defaultEditorVersion
	}
	if cfg.Headers.EditorPluginVersion == "" {
		cfg.Headers.EditorPluginVersion = defaultEditorPluginVersion
	}
	if cfg.Headers.CopilotIntegrationID == "" {
		cfg.Headers.CopilotIntegrationID = defaultCopilotIntegrationID
	}
	if cfg.Headers.OpenaiIntent == "" {
		cfg.Headers.OpenaiIntent = defaultOpenaiIntent
	}
	if cfg.Headers.XInitiator == "" {
		cfg.Headers.XInitiator = defaultXInitiator
	}
}

// SetDefaultCORS sets default CORS values if they are empty
func SetDefaultCORS(cfg *Config) {
	if len(cfg.CORS.AllowedOrigins) == 0 {
		cfg.CORS.AllowedOrigins = []string{"*"}
	}
	if len(cfg.CORS.AllowedHeaders) == 0 {
		cfg.CORS.AllowedHeaders = []string{"*"}
	}
}

// Validate checks the configuration for correctness.
func (c *Config) Validate() error {
	if err := c.validatePort(); err != nil {
		return err
	}
	if err := c.validateTokens(); err != nil {
		return err
	}
	if err := c.validateTimeouts(); err != nil {
		return err
	}
	if err := c.validateHeaders(); err != nil {
		return err
	}
	if err := c.validateCORS(); err != nil {
		return err
	}
	return nil
}

func (c *Config) validatePort() error {
	if c.Port < minPortNumber || c.Port > maxPortNumber {
		return NewValidationError("port", c.Port, fmt.Sprintf("must be between %d and %d", minPortNumber, maxPortNumber), nil)
	}
	return nil
}

func (c *Config) validateTokens() error {
	if c.GitHubToken == "" && c.CopilotToken == "" {
		return ErrMissingTokens
	}
	return nil
}

func (c *Config) validateTimeouts() error {
	if err := c.validateHTTPClientTimeout(); err != nil {
		return err
	}
	if err := c.validateServerReadTimeout(); err != nil {
		return err
	}
	if err := c.validateServerWriteTimeout(); err != nil {
		return err
	}
	if err := c.validateServerIdleTimeout(); err != nil {
		return err
	}
	if err := c.validateProxyContextTimeout(); err != nil {
		return err
	}
	if err := c.validateCircuitBreakerTimeout(); err != nil {
		return err
	}
	if err := c.validateKeepAliveTimeout(); err != nil {
		return err
	}
	if err := c.validateTLSHandshakeTimeout(); err != nil {
		return err
	}
	if err := c.validateDialTimeout(); err != nil {
		return err
	}
	if err := c.validateIdleConnTimeout(); err != nil {
		return err
	}
	return nil
}

func (c *Config) validateHTTPClientTimeout() error {
	if c.Timeouts.HTTPClient < minTimeout || c.Timeouts.HTTPClient > maxLongTimeout {
		return NewValidationError("timeouts.http_client", c.Timeouts.HTTPClient,
			fmt.Sprintf("must be between %d and %d seconds", minTimeout, maxLongTimeout), nil)
	}
	return nil
}

func (c *Config) validateServerReadTimeout() error {
	if c.Timeouts.ServerRead < minTimeout || c.Timeouts.ServerRead > maxShortTimeout {
		return NewValidationError("timeouts.server_read", c.Timeouts.ServerRead,
			fmt.Sprintf("must be between %d and %d seconds", minTimeout, maxShortTimeout), nil)
	}
	return nil
}

func (c *Config) validateServerWriteTimeout() error {
	if c.Timeouts.ServerWrite < minTimeout || c.Timeouts.ServerWrite > maxLongTimeout {
		return NewValidationError("timeouts.server_write", c.Timeouts.ServerWrite,
			fmt.Sprintf("must be between %d and %d seconds", minTimeout, maxLongTimeout), nil)
	}
	return nil
}

func (c *Config) validateServerIdleTimeout() error {
	if c.Timeouts.ServerIdle < minTimeout || c.Timeouts.ServerIdle > maxLongTimeout {
		return NewValidationError("timeouts.server_idle", c.Timeouts.ServerIdle,
			fmt.Sprintf("must be between %d and %d seconds", minTimeout, maxLongTimeout), nil)
	}
	return nil
}

func (c *Config) validateProxyContextTimeout() error {
	if c.Timeouts.ProxyContext < minTimeout || c.Timeouts.ProxyContext > maxLongTimeout {
		return NewValidationError("timeouts.proxy_context", c.Timeouts.ProxyContext,
			fmt.Sprintf("must be between %d and %d seconds", minTimeout, maxLongTimeout), nil)
	}
	return nil
}

func (c *Config) validateCircuitBreakerTimeout() error {
	if c.Timeouts.CircuitBreaker < minTimeout || c.Timeouts.CircuitBreaker > maxShortTimeout {
		return NewValidationError("timeouts.circuit_breaker", c.Timeouts.CircuitBreaker,
			fmt.Sprintf("must be between %d and %d seconds", minTimeout, maxShortTimeout), nil)
	}
	return nil
}

func (c *Config) validateKeepAliveTimeout() error {
	if c.Timeouts.KeepAlive < minTimeout || c.Timeouts.KeepAlive > maxShortTimeout {
		return NewValidationError("timeouts.keep_alive", c.Timeouts.KeepAlive,
			fmt.Sprintf("must be between %d and %d seconds", minTimeout, maxShortTimeout), nil)
	}
	return nil
}

func (c *Config) validateTLSHandshakeTimeout() error {
	if c.Timeouts.TLSHandshake < minTimeout || c.Timeouts.TLSHandshake > maxShortTimeout {
		return NewValidationError("timeouts.tls_handshake", c.Timeouts.TLSHandshake,
			fmt.Sprintf("must be between %d and %d seconds", minTimeout, maxShortTimeout), nil)
	}
	return nil
}

func (c *Config) validateDialTimeout() error {
	if c.Timeouts.DialTimeout < minTimeout || c.Timeouts.DialTimeout > maxShortTimeout {
		return NewValidationError("timeouts.dial_timeout", c.Timeouts.DialTimeout,
			fmt.Sprintf("must be between %d and %d seconds", minTimeout, maxShortTimeout), nil)
	}
	return nil
}

func (c *Config) validateIdleConnTimeout() error {
	if c.Timeouts.IdleConnTimeout < minTimeout || c.Timeouts.IdleConnTimeout > maxLongTimeout {
		return NewValidationError("timeouts.idle_conn_timeout", c.Timeouts.IdleConnTimeout,
			fmt.Sprintf("must be between %d and %d seconds", minTimeout, maxLongTimeout), nil)
	}
	return nil
}

func (c *Config) validateHeaders() error {
	if c.Headers.UserAgent == "" {
		return NewValidationError("headers.user_agent", "", "user_agent cannot be empty", nil)
	}
	if c.Headers.EditorVersion == "" {
		return NewValidationError("headers.editor_version", "", "editor_version cannot be empty", nil)
	}
	if c.Headers.EditorPluginVersion == "" {
		return NewValidationError("headers.editor_plugin_version", "", "editor_plugin_version cannot be empty", nil)
	}
	if c.Headers.CopilotIntegrationID == "" {
		return NewValidationError("headers.copilot_integration_id", "", "copilot_integration_id cannot be empty", nil)
	}
	if c.Headers.OpenaiIntent == "" {
		return NewValidationError("headers.openai_intent", "", "openai_intent cannot be empty", nil)
	}
	if c.Headers.XInitiator == "" {
		return NewValidationError("headers.x_initiator", "", "x_initiator cannot be empty", nil)
	}
	return nil
}

func (c *Config) validateCORS() error {
	if len(c.CORS.AllowedOrigins) == 0 {
		return NewValidationError("cors.allowed_origins", "", "allowed_origins cannot be empty", nil)
	}
	if len(c.CORS.AllowedHeaders) == 0 {
		return NewValidationError("cors.allowed_headers", "", "allowed_headers cannot be empty", nil)
	}
	for _, origin := range c.CORS.AllowedOrigins {
		if origin != "*" && origin != "" {
			if !strings.HasPrefix(origin, "http://") && !strings.HasPrefix(origin, "https://") {
				if !strings.HasPrefix(origin, "localhost") && !strings.HasPrefix(origin, "127.0.0.1") {
					Warn("CORS origin may not be valid URL format", "origin", origin)
				}
			}
		}
	}
	return nil
}

// SaveConfig saves the configuration to file
func (c *Config) SaveConfig(pathOverride ...string) error {
	var path string
	var err error
	if len(pathOverride) > 0 && pathOverride[0] != "" {
		path = pathOverride[0]
	} else {
		path, err = GetConfigPath()
		if err != nil {
			return err
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			Error("Failed to close config file", "error", closeErr)
		}
	}()
	return json.NewEncoder(f).Encode(c)
}

// UnmarshalConfig is a helper for direct config JSON parsing in tests
func UnmarshalConfig(data []byte, cfg *Config) error {
	return json.Unmarshal(data, cfg)
}

// ErrMissingTokens is returned when neither github_token nor copilot_token are present in configuration.
var ErrMissingTokens = errors.New("missing github_token or copilot_token")

// validateCore validates config without token validation
func (c *Config) validateCore() error {
	if err := c.validatePort(); err != nil {
		return err
	}
	if err := c.validateTimeouts(); err != nil {
		return err
	}
	if err := c.validateHeaders(); err != nil {
		return err
	}
	if err := c.validateCORS(); err != nil {
		return err
	}
	return nil
}
