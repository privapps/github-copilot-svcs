// Package testutils provides helpers for testing github-copilot-svcs.
package testutils

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/privapps/github-copilot-svcs/internal"
)

// MockConfig creates a test configuration
const (
	testPort      = 8080
	testExpiresAt = 1000000000
	testRefreshIn = 3600
)

// MockConfig returns a test configuration for use in unit tests.
func MockConfig() *internal.Config {
	cfg := &internal.Config{
		Port:         testPort,
		GitHubToken:  "test-github-token",
		CopilotToken: "test-copilot-token",
		ExpiresAt:    testExpiresAt,
		RefreshIn:    testRefreshIn,
	}

	internal.SetDefaultTimeouts(cfg)
	internal.SetDefaultHeaders(cfg)

	return cfg
}

// LoadFixture loads a test fixture file
func LoadFixture(t *testing.T, path string) []byte {
	t.Helper()

	fixturePath := filepath.Join("..", "fixtures", path)
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to load fixture %s: %v", path, err)
	}
	return data
}

// SetupTestDir creates a temporary directory for tests
func SetupTestDir(t *testing.T) string {
	t.Helper()

	dir, err := os.MkdirTemp("", "copilot-test-")
	if err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}

	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			panic(err)
		}
	})

	return dir
}

// MockGitHubServer creates a mock GitHub API server
func MockGitHubServer() *httptest.Server {
	mux := http.NewServeMux()

	// Mock models endpoint
	mux.HandleFunc("/models", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{
			"object": "list",
			"data": [
				{
					"id": "gpt-4",
					"object": "model",
					"created": 1687882411,
					"owned_by": "openai"
				}
			]
		}`)); err != nil {
			panic(err)
		}
	})

	// Mock auth endpoint
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "Bearer valid-token" {
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte(`{"login": "testuser"}`)); err != nil {
				panic(err)
			}
		} else {
			w.WriteHeader(http.StatusUnauthorized)
		}
	})

	return httptest.NewServer(mux)
}

// SetupValidToken sets up environment for valid token tests
func SetupValidToken() {
	if err := os.Setenv("GITHUB_TOKEN", "valid-token"); err != nil {
		panic(err)
	}
}

// SetupInvalidToken sets up environment for invalid token tests
func SetupInvalidToken() {
	if err := os.Setenv("GITHUB_TOKEN", "invalid-token"); err != nil {
		panic(err)
	}
}

// CleanupEnv cleans up test environment variables
func CleanupEnv() {
	if err := os.Unsetenv("GITHUB_TOKEN"); err != nil {
		panic(err)
	}
	if err := os.Unsetenv("COPILOT_TOKEN"); err != nil {
		panic(err)
	}
	if err := os.Unsetenv("COPILOT_PORT"); err != nil {
		panic(err)
	}
	if err := os.Unsetenv("LOG_LEVEL"); err != nil {
		panic(err)
	}
}

// InitLogger initializes the logger for tests
func InitLogger() {
	internal.Init()
}

// CreateTestServer creates a test server with the given config
func CreateTestServer(cfg *internal.Config) *internal.Server {
	httpClient := internal.CreateHTTPClient(cfg)
	return internal.NewServer(cfg, httpClient)
}
