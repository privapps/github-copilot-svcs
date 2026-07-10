// Package internal provides model-related logic for github-copilot-svcs.
package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/privapps/github-copilot-svcs/pkg/transform"
)

var (
	cachedModels *transform.ModelList
	modelsMutex  sync.RWMutex
	modelsLoaded bool
)

// ModelsDevResponse represents the structure from models.dev API
type ModelsDevResponse map[string]struct {
	ID     string `json:"id"`
	Models map[string]struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		ReleaseDate string `json:"release_date"`
		OwnedBy     string `json:"owned_by,omitempty"`
	} `json:"models"`
}

// FetchFromModelsDev fetches models from models.dev API as fallback
func FetchFromModelsDev(httpClient *http.Client) (*transform.ModelList, error) {
	resp, err := httpClient.Get("https://models.dev/api.json")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			Warn("Error closing response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, NewNetworkError("fetch_models", "https://models.dev/api.json", fmt.Sprintf("API returned HTTP %d", resp.StatusCode), nil)
	}

	var providers ModelsDevResponse
	if err := json.NewDecoder(resp.Body).Decode(&providers); err != nil {
		return nil, err
	}

	// Extract GitHub Copilot models
	copilotProvider, exists := providers["github-copilot"]
	if !exists {
		return nil, NewValidationError("provider", "github-copilot", "provider not found in models.dev response", nil)
	}

	var models []transform.Model
	for modelID, modelInfo := range copilotProvider.Models {
		ownedBy := modelInfo.OwnedBy
		if ownedBy == "" {
			// Determine owner based on model name
			switch {
			case containsAny(modelInfo.Name, []string{"claude", "anthropic"}):
				ownedBy = "anthropic"
			case containsAny(modelInfo.Name, []string{"gpt", "o1", "o3", "o4", "openai"}):
				ownedBy = "openai"
			case containsAny(modelInfo.Name, []string{"gemini", "google"}):
				ownedBy = "google"
			default:
				ownedBy = "github-copilot"
			}
		}

		models = append(models, transform.Model{
			ID:      modelID,
			Object:  "model",
			Created: time.Now().Unix(),
			OwnedBy: ownedBy,
			APIType: apiTypeForModel(modelID),
		})
	}

	return &transform.ModelList{
		Object: "list",
		Data:   models,
	}, nil
}

// apiTypeForModel returns the API endpoint type for a model.
// Models are divided into two categories based on testing:
//   - "chat_completions": Use /v1/chat/completions (OpenAI Chat Completions API)
//   - "responses": Use /v1/responses (OpenAI Responses API)
func apiTypeForModel(modelID string) string {
	responsesModels := map[string]bool{
		"gpt-5.3-codex": true,
		"gpt-5.4-mini":  true,
		"gpt-5.6-luna":  true,
		"gpt-5.6-sol":   true,
		"gpt-5.6-terra": true,
	}
	if responsesModels[modelID] {
		return "responses"
	}
	return "chat_completions"
}

// GetDefault returns a default list of models based on actual GitHub Copilot entries.
func GetDefault() []transform.Model {
	now := time.Now().Unix()
	entries := []struct {
		id      string
		ownedBy string
		apiType string
	}{
		// Chat Completions models
		{"gpt-4o", "openai", "chat_completions"},
		{"gpt-4.1", "openai", "chat_completions"},
		{"claude-haiku-4.5", "anthropic", "chat_completions"},
		{"claude-sonnet-5", "anthropic", "chat_completions"},
		{"claude-opus-4.8", "anthropic", "chat_completions"},
		{"gemini-3.5-flash", "google", "chat_completions"},
		{"gemini-3.1-pro-preview", "google", "chat_completions"},
		// Responses API models
		{"gpt-5.3-codex", "openai", "responses"},
		{"gpt-5.4-mini", "openai", "responses"},
		{"gpt-5.6-luna", "openai", "responses"},
		{"gpt-5.6-sol", "openai", "responses"},
		{"gpt-5.6-terra", "openai", "responses"},
	}
	models := make([]transform.Model, len(entries))
	for i, e := range entries {
		models[i] = transform.Model{
			ID:      e.id,
			Object:  "model",
			Created: now,
			OwnedBy: e.ownedBy,
			APIType: e.apiType,
		}
	}
	return models
}

// containsAny checks if text contains any of the substrings
func containsAny(text string, substrings []string) bool {
	textLower := strings.ToLower(text)
	for _, substr := range substrings {
		if strings.Contains(textLower, strings.ToLower(substr)) {
			return true
		}
	}
	return false
}

// ModelsService provides model operations
type ModelsService struct {
	coalescingCache CoalescingCacheInterface
	httpClient      *http.Client
}

// NewModelsService creates a new models service
func NewModelsService(cache CoalescingCacheInterface, httpClient *http.Client) *ModelsService {
	return &ModelsService{
		coalescingCache: cache,
		httpClient:      httpClient,
	}
}

// CoalescingCacheInterface interface for request coalescing
type CoalescingCacheInterface interface {
	GetRequestKey(method, path string, body interface{}) string
	CoalesceRequest(key string, fn func() interface{}) interface{}
} // Handler returns an HTTP handler for the models endpoint.
// Handler returns an HTTP handler for the models endpoint.
func (s *ModelsService) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		// Use request coalescing for identical concurrent requests
		requestKey := s.coalescingCache.GetRequestKey("GET", "/v1/models", nil)

		result := s.coalescingCache.CoalesceRequest(requestKey, func() interface{} {
			// Check cache first
			modelsMutex.RLock()
			if modelsLoaded && cachedModels != nil {
				modelsMutex.RUnlock()
				return cachedModels
			}
			modelsMutex.RUnlock()

			// Load models if not cached
			modelsMutex.Lock()
			defer modelsMutex.Unlock()

			// Double-check in case another goroutine loaded while we waited
			if modelsLoaded && cachedModels != nil {
				return cachedModels
			}

			Info("Loading models for the first time...")

			// Try models.dev API first (don't hit GitHub Copilot for models list)
			modelList, err := FetchFromModelsDev(s.httpClient)
			if err != nil {
				Warn("Failed to fetch from models.dev, using default models", "error", err)

				// Ultimate fallback to hardcoded models
				modelList = &transform.ModelList{
					Object: "list",
					Data:   GetDefault(),
				}
			}

			// Cache the results
			cachedModels = modelList
			modelsLoaded = true

			Info("Loaded and cached models", "count", len(modelList.Data))
			return modelList
		})

		modelList := result.(*transform.ModelList)
		// Filter if allowed_models is set in config
		cfg, cfgErr := LoadConfig(true)
		filtered := modelList.Data
		filteredMsg := ""
		if cfgErr == nil && cfg.AllowedModels != nil && len(cfg.AllowedModels) > 0 {
			allowedSet := make(map[string]struct{}, len(cfg.AllowedModels))
			for _, name := range cfg.AllowedModels {
				allowedSet[name] = struct{}{}
			}
			var modelsFiltered []transform.Model
			for _, m := range filtered {
				if _, ok := allowedSet[m.ID]; ok {
					modelsFiltered = append(modelsFiltered, m)
				}
			}
			filtered = modelsFiltered
			filteredMsg = "(filtered by allowed_models from config)"
		}
		resp := struct {
			Object   string            `json:"object"`
			Data     []transform.Model `json:"data"`
			Filtered string            `json:"note,omitempty"`
		}{
			Object:   "list",
			Data:     filtered,
			Filtered: filteredMsg,
		}
		Debug("Returning models", "count", len(filtered))
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			Error("Error encoding models response", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
	}
}
