// Package internal provides core authentication, proxy, and service logic for github-copilot-svcs.
package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	copilotDeviceCodeURL = "https://github.com/login/device/code"
	copilotTokenURL      = "https://github.com/login/oauth/access_token"
	copilotAPIKeyURL     = "https://api.github.com/copilot_internal/v2/token"
	copilotClientID      = "Iv1.b507a08c87ecfe98"
	copilotScope         = "read:user"

	// Retry configuration
	maxRefreshRetries = 3
	baseRetryDelay    = 2 // seconds
)

type deviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	Error       string `json:"error,omitempty"`
	ErrorDesc   string `json:"error_description,omitempty"`
}

type copilotTokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
	RefreshIn int64  `json:"refresh_in"`
	Endpoints struct {
		API string `json:"api"`
	} `json:"endpoints"`
}

// AuthService provides authentication operations for GitHub Copilot.
type AuthService struct {
	httpClient *http.Client

	// For testability: override config save path
	configPath string

	// For testability: optional custom token refresh function
	refreshFunc func(cfg *Config) error
}

// NewAuthService creates a new auth service
func NewAuthService(httpClient *http.Client, opts ...func(*AuthService)) *AuthService {
	svc := &AuthService{
		httpClient: httpClient,
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

// WithConfigPath sets the config path for AuthService.
// WithConfigPath is used for tests.
func WithConfigPath(path string) func(*AuthService) {
	return func(s *AuthService) {
		s.configPath = path
	}
}

// WithRefreshFunc sets a custom refresh function for AuthService.
func WithRefreshFunc(f func(cfg *Config) error) func(*AuthService) {
	return func(s *AuthService) {
		s.refreshFunc = f
	}
}

// Authenticate performs the full GitHub Copilot authentication flow
func (s *AuthService) Authenticate(cfg *Config) error {
	now := time.Now().Unix()
	if cfg.CopilotToken != "" && cfg.ExpiresAt > now+60 {
		Info("Token still valid", "expires_in", cfg.ExpiresAt-now)
		return nil // Already authenticated
	}

	if cfg.CopilotToken != "" {
		Info("Token expired or expiring soon, triggering re-auth", "expires_in", cfg.ExpiresAt-now)
	} else {
		Info("No token found, starting authentication flow")
	}

	// Step 1: Get device code
	dc, err := s.getDeviceCode(cfg)
	if err != nil {
		return fmt.Errorf("failed to get device code: %w", err)
	}

	fmt.Printf("\nTo authenticate, visit: %s\nEnter code: %s\n", dc.VerificationURI, dc.UserCode)

	// Step 2: Poll for GitHub token
	githubToken, err := s.pollForGitHubToken(cfg, dc.DeviceCode, dc.Interval)
	if err != nil {
		return fmt.Errorf("failed to get GitHub token: %w", err)
	}
	cfg.GitHubToken = githubToken

	// Step 3: Exchange GitHub token for Copilot token
	copilotToken, expiresAt, refreshIn, err := s.getCopilotToken(cfg, githubToken)
	if err != nil {
		return fmt.Errorf("failed to get Copilot token: %w", err)
	}

	cfg.CopilotToken = copilotToken
	cfg.ExpiresAt = expiresAt
	cfg.RefreshIn = refreshIn

	var saveErr error
	if s.configPath != "" {
		saveErr = cfg.SaveConfig(s.configPath)
	} else {
		saveErr = cfg.SaveConfig()
	}
	if saveErr != nil {
		return fmt.Errorf("failed to save config: %w", saveErr)
	}

	fmt.Println("Authentication successful!")
	return nil
}

// RefreshToken refreshes the Copilot token using the stored GitHub token
func (s *AuthService) RefreshToken(cfg *Config) error {
	return s.RefreshTokenWithContext(context.Background(), cfg)
}

// RefreshTokenWithContext refreshes the Copilot token using the provided context and config.
func (s *AuthService) RefreshTokenWithContext(ctx context.Context, cfg *Config) error {
	if s.refreshFunc != nil {
		// Use injected refresh function for tests
		err := s.refreshFunc(cfg)
		if err != nil {
			return err
		}
		// Save config to injected path if set
		if s.configPath != "" {
			return cfg.SaveConfig(s.configPath)
		}
		return cfg.SaveConfig()
	}

	if cfg.GitHubToken == "" {
		Warn("Cannot refresh token: no GitHub token available")
		return NewAuthError("no GitHub token available for refresh", nil)
	}

	// Retry with exponential backoff
	for attempt := 1; attempt <= maxRefreshRetries; attempt++ {
		Info("Attempting to refresh Copilot token", "attempt", attempt, "max_attempts", maxRefreshRetries)

		copilotToken, expiresAt, refreshIn, err := s.getCopilotToken(cfg, cfg.GitHubToken)
		if err != nil {
			if attempt == maxRefreshRetries {
				Error("Token refresh failed after max attempts", "attempts", maxRefreshRetries, "error", err)
				return err
			}

			// Wait before retry with exponential backoff
			waitTime := time.Duration(baseRetryDelay*attempt*attempt) * time.Second
			Warn("Token refresh failed, retrying", "attempt", attempt, "wait_time", waitTime, "error", err)

			// Use context-aware sleep
			select {
			case <-time.After(waitTime):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		Info("Token refresh successful", "expires_in", expiresAt-time.Now().Unix())
		cfg.CopilotToken = copilotToken
		cfg.ExpiresAt = expiresAt
		cfg.RefreshIn = refreshIn

		return cfg.SaveConfig()
	}

	return NewAuthError("maximum retry attempts exceeded", nil)
}

// EnsureValidToken ensures we have a valid token, refreshing if necessary
func (s *AuthService) EnsureValidToken(cfg *Config) error {
	now := time.Now().Unix()
	if cfg.CopilotToken == "" {
		return NewAuthError("no token available - authentication required", nil)
	}

	// Check if token needs refresh (within 5 minutes of expiry or already expired)
	if cfg.ExpiresAt <= now+300 {
		return s.RefreshToken(cfg)
	}

	return nil
}

func (s *AuthService) getDeviceCode(cfg *Config) (*deviceCodeResponse, error) {
	body := fmt.Sprintf(`{"client_id":%q,"scope":%q}`, copilotClientID, copilotScope)
	req, err := http.NewRequest("POST", copilotDeviceCodeURL, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", cfg.Headers.UserAgent)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			Warn("Error closing response body", "error", err)
		}
	}()

	var dc deviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&dc); err != nil {
		return nil, err
	}

	return &dc, nil
}

func (s *AuthService) pollForGitHubToken(cfg *Config, deviceCode string, interval int) (string, error) {
	return s.pollForGitHubTokenWithContext(context.Background(), cfg, deviceCode, interval)
}

func (s *AuthService) pollForGitHubTokenWithContext(ctx context.Context, cfg *Config, deviceCode string, interval int) (string, error) {
	for i := 0; i < 120; i++ { // Poll for 2 minutes max
		// Use context-aware sleep
		select {
		case <-time.After(time.Duration(interval) * time.Second):
			// Continue with polling
		case <-ctx.Done():
			return "", ctx.Err()
		}

		body := fmt.Sprintf(`{"client_id":%q,"device_code":%q,"grant_type":"urn:ietf:params:oauth:grant-type:device_code"}`,
			copilotClientID, deviceCode)
		req, err := http.NewRequest("POST", copilotTokenURL, strings.NewReader(body))
		if err != nil {
			return "", err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", cfg.Headers.UserAgent)

		resp, err := s.httpClient.Do(req)
		if err != nil {
			continue
		}

		var tr tokenResponse
		if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
			if err := resp.Body.Close(); err != nil {
				Warn("Error closing response body", "error", err)
			}
			continue
		}
		if err := resp.Body.Close(); err != nil {
			Warn("Error closing response body", "error", err)
		}

		if tr.Error != "" {
			if tr.Error == "authorization_pending" {
				continue
			}
			return "", NewAuthError(fmt.Sprintf("authorization failed: %s - %s", tr.Error, tr.ErrorDesc), nil)
		}

		if tr.AccessToken != "" {
			return tr.AccessToken, nil
		}
	}

	return "", NewAuthError("authentication timed out", nil)
}

func (s *AuthService) getCopilotToken(cfg *Config, githubToken string) (token string, expiresAt, refreshIn int64, err error) {
	req, err := http.NewRequest("GET", copilotAPIKeyURL, http.NoBody)
	if err != nil {
		return "", 0, 0, err
	}
	req.Header.Set("Authorization", "token "+githubToken)
	req.Header.Set("User-Agent", cfg.Headers.UserAgent)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", 0, 0, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			Warn("Error closing response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", 0, 0, NewNetworkError("getCopilotToken", copilotAPIKeyURL, fmt.Sprintf("HTTP %d response", resp.StatusCode), nil)
	}

	var ctr copilotTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&ctr); err != nil {
		return "", 0, 0, err
	}

	return ctr.Token, ctr.ExpiresAt, ctr.RefreshIn, nil
}
