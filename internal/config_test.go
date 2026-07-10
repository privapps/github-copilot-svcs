package internal_test

import (
	"os"
	"testing"

	"github.com/privapps/github-copilot-svcs/internal"
)

func TestConfigValidation(t *testing.T) {
	t.Run("valid config passes validation", func(t *testing.T) {
		cfg := &internal.Config{
			Port:        8081,
			GitHubToken: "test-token",
		}
		internal.SetDefaultHeaders(cfg)
		internal.SetDefaultCORS(cfg)
		internal.SetDefaultTimeouts(cfg)

		err := cfg.Validate()
		if err != nil {
			t.Errorf("Expected valid config to pass validation, got error: %v", err)
		}
	})

	t.Run("invalid port fails validation", func(t *testing.T) {
		cfg := &internal.Config{
			Port:        99999, // Invalid port
			GitHubToken: "test-token",
		}
		internal.SetDefaultHeaders(cfg)
		internal.SetDefaultCORS(cfg)
		internal.SetDefaultTimeouts(cfg)

		err := cfg.Validate()
		if err == nil {
			t.Error("Expected invalid port to fail validation")
		}
	})

	t.Run("negative port fails validation", func(t *testing.T) {
		cfg := &internal.Config{
			Port:        -1, // Invalid port
			GitHubToken: "test-token",
		}
		internal.SetDefaultHeaders(cfg)
		internal.SetDefaultCORS(cfg)
		internal.SetDefaultTimeouts(cfg)

		err := cfg.Validate()
		if err == nil {
			t.Error("Expected negative port to fail validation")
		}
	})

	t.Run("missing tokens fails validation", func(t *testing.T) {
		cfg := &internal.Config{
			Port: 8081,
			// No tokens provided
		}
		internal.SetDefaultHeaders(cfg)
		internal.SetDefaultCORS(cfg)
		internal.SetDefaultTimeouts(cfg)

		err := cfg.Validate()
		if err == nil {
			t.Error("Expected missing tokens to fail validation")
		}
		if !internalerrorsIs(err, internal.ErrMissingTokens) {
			t.Errorf("Expected ErrMissingTokens, got %v", err)
		}
	})

	t.Run("valid with copilot token only", func(t *testing.T) {
		cfg := &internal.Config{
			Port:         8081,
			CopilotToken: "test-copilot-token",
		}
		internal.SetDefaultHeaders(cfg)
		internal.SetDefaultCORS(cfg)
		internal.SetDefaultTimeouts(cfg)

		err := cfg.Validate()
		if err != nil {
			t.Errorf("Expected valid config with copilot token to pass validation, got error: %v", err)
		}
	})

	t.Run("invalid timeout values fail validation", func(t *testing.T) {
		cfg := &internal.Config{
			Port:        8081,
			GitHubToken: "test-token",
		}
		internal.SetDefaultHeaders(cfg)
		internal.SetDefaultCORS(cfg)
		internal.SetDefaultTimeouts(cfg)

		// Test invalid HTTP client timeout
		original := cfg.Timeouts.HTTPClient
		cfg.Timeouts.HTTPClient = -1
		err := cfg.Validate()
		if err == nil {
			t.Error("Expected negative HTTP client timeout to fail validation")
		}
		cfg.Timeouts.HTTPClient = original

		// Test invalid server read timeout
		cfg.Timeouts.ServerRead = 1000000 // Too large
		err = cfg.Validate()
		if err == nil {
			t.Error("Expected too large server read timeout to fail validation")
		}
	})

	t.Run("empty headers fail validation", func(t *testing.T) {
		cfg := &internal.Config{
			Port:        8081,
			GitHubToken: "test-token",
		}
		internal.SetDefaultHeaders(cfg)
		internal.SetDefaultCORS(cfg)
		internal.SetDefaultTimeouts(cfg)

		// Test empty user agent
		original := cfg.Headers.UserAgent
		cfg.Headers.UserAgent = ""
		err := cfg.Validate()
		if err == nil {
			t.Error("Expected empty user agent to fail validation")
		}
		cfg.Headers.UserAgent = original
	})

	t.Run("empty CORS configuration fails validation", func(t *testing.T) {
		cfg := &internal.Config{
			Port:        8081,
			GitHubToken: "test-token",
		}
		internal.SetDefaultHeaders(cfg)
		internal.SetDefaultCORS(cfg)
		internal.SetDefaultTimeouts(cfg)

		// Test empty allowed origins
		original := cfg.CORS.AllowedOrigins
		cfg.CORS.AllowedOrigins = []string{}
		err := cfg.Validate()
		if err == nil {
			t.Error("Expected empty CORS allowed origins to fail validation")
		}
		cfg.CORS.AllowedOrigins = original
	})
}

func TestLoadConfig(t *testing.T) {
	t.Run("loads config with validation", func(t *testing.T) {
		// Save original environment
		originalPort := os.Getenv("COPILOT_PORT")
		originalToken := os.Getenv("GITHUB_TOKEN")

		// Set test environment
		os.Setenv("COPILOT_PORT", "8081")
		os.Setenv("GITHUB_TOKEN", "test-token")

		// Restore environment
		defer func() {
			if originalPort != "" {
				os.Setenv("COPILOT_PORT", originalPort)
			} else {
				os.Unsetenv("COPILOT_PORT")
			}

			if originalToken != "" {
				os.Setenv("GITHUB_TOKEN", originalToken)
			} else {
				os.Unsetenv("GITHUB_TOKEN")
			}
		}()

		cfg, err := internal.LoadConfig()
		if err != nil {
			t.Errorf("Expected successful config load, got error: %v", err)
		}

		if cfg.Port != 8081 {
			t.Errorf("Expected port 8081, got %d", cfg.Port)
		}

		if cfg.GitHubToken != "test-token" {
			t.Errorf("Expected GitHub token 'test-token', got '%s'", cfg.GitHubToken)
		}
	})

	t.Run("fails with invalid port in environment", func(t *testing.T) {
		// Save original environment
		originalPort := os.Getenv("COPILOT_PORT")
		originalToken := os.Getenv("GITHUB_TOKEN")

		// Set test environment with invalid port
		os.Setenv("COPILOT_PORT", "99999")
		os.Setenv("GITHUB_TOKEN", "test-token")

		// Restore environment
		defer func() {
			if originalPort != "" {
				os.Setenv("COPILOT_PORT", originalPort)
			} else {
				os.Unsetenv("COPILOT_PORT")
			}

			if originalToken != "" {
				os.Setenv("GITHUB_TOKEN", originalToken)
			} else {
				os.Unsetenv("GITHUB_TOKEN")
			}
		}()

		_, err := internal.LoadConfig()
		if err == nil {
			t.Error("Expected config load to fail with invalid port")
		}
	})
}

func TestSetDefaultValues(t *testing.T) {
	t.Run("sets default timeouts correctly", func(t *testing.T) {
		cfg := &internal.Config{}
		internal.SetDefaultTimeouts(cfg)

		// Check that all timeouts have reasonable default values
		if cfg.Timeouts.HTTPClient == 0 {
			t.Error("Expected HTTPClient timeout to have default value")
		}
		if cfg.Timeouts.ServerRead == 0 {
			t.Error("Expected ServerRead timeout to have default value")
		}
		if cfg.Timeouts.ServerWrite == 0 {
			t.Error("Expected ServerWrite timeout to have default value")
		}
	})

	t.Run("sets default headers correctly", func(t *testing.T) {
		cfg := &internal.Config{}
		internal.SetDefaultHeaders(cfg)

		// Check that all headers have default values
		if cfg.Headers.UserAgent == "" {
			t.Error("Expected UserAgent to have default value")
		}
		if cfg.Headers.EditorVersion == "" {
			t.Error("Expected EditorVersion to have default value")
		}
		if cfg.Headers.EditorPluginVersion == "" {
			t.Error("Expected EditorPluginVersion to have default value")
		}
	})

	t.Run("sets default CORS correctly", func(t *testing.T) {
		cfg := &internal.Config{}
		internal.SetDefaultCORS(cfg)

		// Check that CORS has default values
		if len(cfg.CORS.AllowedOrigins) == 0 {
			t.Error("Expected AllowedOrigins to have default value")
		}
		if len(cfg.CORS.AllowedHeaders) == 0 {
			t.Error("Expected AllowedHeaders to have default value")
		}
	})
}
func TestAllowedModelsConfig(t *testing.T) {
	t.Run("loads allowed_models and respects null behavior", func(t *testing.T) {
		cfg := &internal.Config{
			Port: 8081,
		}
		// Should default (nil) when not set
		if cfg.AllowedModels != nil {
			t.Errorf("Expected AllowedModels nil, got %v", cfg.AllowedModels)
		}
		cfg.AllowedModels = []string{"gpt-4o", "claude-3.7-sonnet"}
		// Simulate allowed
		allowed := func(model string) bool {
			for _, m := range cfg.AllowedModels {
				if m == model {
					return true
				}
			}
			return false
		}
		if !allowed("gpt-4o") || !allowed("claude-3.7-sonnet") {
			t.Errorf("Known allowed models not accepted")
		}
		if allowed("bad-model") {
			t.Errorf("Unexpected model allowed")
		}
	})
	t.Run("config JSON parsing includes allowed_models", func(t *testing.T) {
		jsonCfg := []byte(`{"port":8081, "allowed_models": ["foo", "bar"]}`)
		var cfg internal.Config
		if err := internal.UnmarshalConfig(jsonCfg, &cfg); err != nil {
			t.Fatalf("Failed to decode allowed_models config: %v", err)
		}
		if len(cfg.AllowedModels) != 2 || cfg.AllowedModels[0] != "foo" || cfg.AllowedModels[1] != "bar" {
			t.Errorf("Config parsing error for allowed_models: %#v", cfg.AllowedModels)
		}
	})
}
func internalerrorsIs(err, target error) bool {
	// Handle errors.Is for wrapped errors in Go 1.13+, separate helper avoids import cycle
	if err == nil {
		return false
	}
	if err == target {
		return true
	}
	if unwrapper, ok := err.(interface{ Unwrap() error }); ok {
		return internalerrorsIs(unwrapper.Unwrap(), target)
	}
	return false
}
