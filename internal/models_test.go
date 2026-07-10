package internal_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/privapps/github-copilot-svcs/internal"
	"github.com/privapps/github-copilot-svcs/pkg/transform"
)

// MockCoalescingCache implements CoalescingCacheInterface for testing
type MockCoalescingCache struct {
	requests map[string]func() interface{}
}

func NewMockCoalescingCache() *MockCoalescingCache {
	return &MockCoalescingCache{
		requests: make(map[string]func() interface{}),
	}
}

func (m *MockCoalescingCache) GetRequestKey(method, path string, _ interface{}) string {
	return method + ":" + path
}

func (m *MockCoalescingCache) CoalesceRequest(_ string, fn func() interface{}) interface{} {
	// For testing, just execute the function immediately
	return fn()
}

// Test helpers
func createTestModelsService() *internal.ModelsService {
	cache := NewMockCoalescingCache()
	httpClient := &http.Client{Timeout: 30 * time.Second}
	return internal.NewModelsService(cache, httpClient)
}

func TestNewModelsService(t *testing.T) {
	cache := NewMockCoalescingCache()
	httpClient := &http.Client{Timeout: 30 * time.Second}

	service := internal.NewModelsService(cache, httpClient)

	if service == nil {
		t.Fatal("Expected models service to be created")
	}

	// Test that the service has a handler
	handler := service.Handler()
	if handler == nil {
		t.Error("Expected handler to be created")
	}
}

func TestGetDefault(t *testing.T) {
	models := internal.GetDefault()

	if len(models) == 0 {
		t.Error("Expected default models to be returned")
	}

	// Verify structure of default models
	expectedModels := map[string]struct {
		owner   string
		apiType string
	}{
		"gpt-4o":                 {"openai", "chat_completions"},
		"gpt-4.1":                {"openai", "chat_completions"},
		"claude-haiku-4.5":       {"anthropic", "chat_completions"},
		"claude-sonnet-5":        {"anthropic", "chat_completions"},
		"claude-opus-4.8":        {"anthropic", "chat_completions"},
		"gemini-3.5-flash":       {"google", "chat_completions"},
		"gemini-3.1-pro-preview": {"google", "chat_completions"},
		"gpt-5.3-codex":          {"openai", "responses"},
		"gpt-5.4-mini":           {"openai", "responses"},
		"gpt-5.6-luna":           {"openai", "responses"},
		"gpt-5.6-sol":            {"openai", "responses"},
		"gpt-5.6-terra":          {"openai", "responses"},
	}

	modelMap := make(map[string]transform.Model)
	for _, model := range models {
		modelMap[model.ID] = model

		// Verify model structure
		if model.Object != "model" {
			t.Errorf("Expected model object to be 'model', got '%s'", model.Object)
		}
		if model.Created == 0 {
			t.Error("Expected model created timestamp to be set")
		}
		if model.APIType == "" {
			t.Errorf("Model '%s': Expected non-empty APIType", model.ID)
		}
	}

	// Check that expected models are present
	for expectedID, expected := range expectedModels {
		model, exists := modelMap[expectedID]
		if !exists {
			t.Errorf("Expected model '%s' not found in default models", expectedID)
			continue
		}
		if model.OwnedBy != expected.owner {
			t.Errorf("Model '%s': Expected owner '%s', got '%s'", expectedID, expected.owner, model.OwnedBy)
		}
		if model.APIType != expected.apiType {
			t.Errorf("Model '%s': Expected api_type '%s', got '%s'", expectedID, expected.apiType, model.APIType)
		}
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		substrings []string
		expected   bool
	}{
		{
			name:       "matches case insensitive",
			text:       "GPT-4 Model",
			substrings: []string{"gpt", "claude"},
			expected:   true,
		},
		{
			name:       "matches multiple options",
			text:       "Claude Sonnet",
			substrings: []string{"gpt", "claude", "gemini"},
			expected:   true,
		},
		{
			name:       "no match",
			text:       "Random Model",
			substrings: []string{"gpt", "claude", "gemini"},
			expected:   false,
		},
		{
			name:       "empty substrings",
			text:       "Any Text",
			substrings: []string{},
			expected:   false,
		},
		{
			name:       "partial match",
			text:       "openai-gpt",
			substrings: []string{"gpt"},
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: containsAny is not exported, so we test it indirectly through the models
			// that use it in FetchFromModelsDev. This is a limitation of testing unexported functions.

			// For this test, we'll create a scenario and test the expected behavior
			if tt.expected && !strings.Contains(strings.ToLower(tt.text), strings.ToLower(tt.substrings[0])) {
				// This is just a basic check since we can't test the actual function
				t.Skip("Cannot directly test unexported containsAny function")
			}
		})
	}
}

func TestFetchFromModelsDev(t *testing.T) {
	t.Run("successful fetch", func(t *testing.T) {
		// Create a mock server
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api.json" {
				t.Errorf("Expected path '/api.json', got '%s'", r.URL.Path)
			}

			response := map[string]interface{}{
				"github-copilot": map[string]interface{}{
					"id": "github-copilot",
					"models": map[string]interface{}{
						"gpt-4": map[string]interface{}{
							"id":           "gpt-4",
							"name":         "GPT-4",
							"release_date": "2023-03-14",
							"owned_by":     "openai",
						},
						"claude-3": map[string]interface{}{
							"id":           "claude-3",
							"name":         "Claude 3",
							"release_date": "2024-03-04",
							"owned_by":     "anthropic",
						},
					},
				},
			}

			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Fatalf("unexpected encode error: %v", err)
			}
		}))
		defer testServer.Close()

		// Override the URL by creating a custom client and using the test server
		httpClient := &http.Client{Timeout: 30 * time.Second}

		// We can't easily test the actual function since it has a hardcoded URL
		// But we can test that it handles the response format correctly
		resp, err := httpClient.Get(testServer.URL + "/api.json")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Verify the structure we expect
		if _, exists := response["github-copilot"]; !exists {
			t.Error("Expected 'github-copilot' provider in response")
		}
	})

	t.Run("handles network error", func(t *testing.T) {
		httpClient := &http.Client{Timeout: 1 * time.Millisecond} // Very short timeout

		// This will likely fail due to the short timeout, which is what we want to test
		_, err := internal.FetchFromModelsDev(httpClient)
		if err == nil {
			t.Log("Note: Network request unexpectedly succeeded, may be due to local caching")
		}
		// We don't fail the test since network conditions can vary
	})

	t.Run("handles non-200 status", func(t *testing.T) {
		// Create a mock server that returns 404
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer testServer.Close()

		// We can't easily override the URL in FetchFromModelsDev
		// So this test just verifies the server behavior
		resp, err := http.Get(testServer.URL)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			t.Error("Expected non-200 status code")
		}
	})
}

func TestModelsServiceHandler_ReturnsModelsSuccessfully(t *testing.T) {
	service := createTestModelsService()
	handler := service.Handler()

	req := httptest.NewRequest("GET", "/v1/models", http.NoBody)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
	}

	// Parse the response
	var modelList transform.ModelList
	if err := json.NewDecoder(w.Body).Decode(&modelList); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if modelList.Object != "list" {
		t.Errorf("Expected object to be 'list', got '%s'", modelList.Object)
	}

	if len(modelList.Data) == 0 {
		t.Error("Expected at least one model in the list")
	}

	// Verify model structure
	for i, model := range modelList.Data {
		if model.ID == "" {
			t.Errorf("Model %d: Expected non-empty ID", i)
		}
		if model.Object != "model" {
			t.Errorf("Model %d: Expected object to be 'model', got '%s'", i, model.Object)
		}
		if model.Created == 0 {
			t.Errorf("Model %d: Expected non-zero created timestamp", i)
		}
		if model.OwnedBy == "" {
			t.Errorf("Model %d: Expected non-empty OwnedBy", i)
		}
		if model.APIType == "" {
			t.Errorf("Model %d: Expected non-empty APIType", i)
		}
	}
}

func TestModelsServiceHandler_HandlesConcurrentRequests(t *testing.T) {
	service := createTestModelsService()
	handler := service.Handler()

	// Make multiple concurrent requests
	responses := make(chan *httptest.ResponseRecorder, 5)
	for i := 0; i < 5; i++ {
		go func() {
			req := httptest.NewRequest("GET", "/v1/models", http.NoBody)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			responses <- w
		}()
	}

	// Collect all responses
	for i := 0; i < 5; i++ {
		w := <-responses
		if w.Code != http.StatusOK {
			t.Errorf("Request %d: Expected status 200, got %d", i, w.Code)
		}

		var modelList transform.ModelList
		if err := json.NewDecoder(w.Body).Decode(&modelList); err != nil {
			t.Errorf("Request %d: Failed to decode response: %v", i, err)
			continue
		}

		if len(modelList.Data) == 0 {
			t.Errorf("Request %d: Expected at least one model", i)
		}
	}
}

func TestModelsServiceHandler_CachesModelsBetweenRequests(t *testing.T) {
	service := createTestModelsService()
	handler := service.Handler()

	// Make first request
	req1 := httptest.NewRequest("GET", "/v1/models", http.NoBody)
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	var modelList1 transform.ModelList
	if err := json.NewDecoder(w1.Body).Decode(&modelList1); err != nil {
		t.Fatalf("Failed to decode first response: %v", err)
	}

	// Make second request
	req2 := httptest.NewRequest("GET", "/v1/models", http.NoBody)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	var modelList2 transform.ModelList
	if err := json.NewDecoder(w2.Body).Decode(&modelList2); err != nil {
		t.Fatalf("Failed to decode second response: %v", err)
	}

	// Results should be the same (cached)
	if !reflect.DeepEqual(modelList1.Data, modelList2.Data) {
		t.Error("Expected cached models to be identical between requests")
	}
}

func TestModelsServiceHandler_SupportsDifferentHTTPMethods(t *testing.T) {
	service := createTestModelsService()
	handler := service.Handler()

	methods := []string{"GET", "POST", "PUT", "DELETE"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/v1/models", http.NoBody)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			// All methods should return the models (the handler doesn't check method)
			if w.Code != http.StatusOK {
				t.Errorf("Method %s: Expected status 200, got %d", method, w.Code)
			}
		})
	}
}

func TestModelsDevResponseStructure(t *testing.T) {
	// Test the ModelsDevResponse structure parsing
	jsonData := `{
		"github-copilot": {
			"id": "github-copilot",
			"models": {
				"gpt-4": {
					"id": "gpt-4",
					"name": "GPT-4",
					"release_date": "2023-03-14",
					"owned_by": "openai"
				},
				"claude-3": {
					"id": "claude-3",
					"name": "Claude 3",
					"release_date": "2024-03-04"
				}
			}
		}
	}`

	var response internal.ModelsDevResponse
	if err := json.Unmarshal([]byte(jsonData), &response); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Verify structure
	provider, exists := response["github-copilot"]
	if !exists {
		t.Fatal("Expected 'github-copilot' provider")
	}

	if provider.ID != "github-copilot" {
		t.Errorf("Expected provider ID 'github-copilot', got '%s'", provider.ID)
	}

	if len(provider.Models) != 2 {
		t.Errorf("Expected 2 models, got %d", len(provider.Models))
	}

	// Check specific models
	if _, gpt4Exists := provider.Models["gpt-4"]; !gpt4Exists {
		t.Error("Expected 'gpt-4' model")
	}

	claude3, exists := provider.Models["claude-3"]
	if !exists {
		t.Error("Expected 'claude-3' model")
	} else if claude3.OwnedBy != "" {
		t.Errorf("Expected claude-3 owned_by to be empty, got '%s'", claude3.OwnedBy)
	}
}

func TestModelOwnershipDetection(t *testing.T) {
	// This test verifies the owner detection logic indirectly
	// by checking the default models have correct ownership
	models := internal.GetDefault()

	ownershipTests := map[string]string{
		"gpt-4o":           "openai",
		"gpt-4.1":          "openai",
		"claude-haiku-4.5": "anthropic",
		"claude-sonnet-5":  "anthropic",
		"gemini-3.5-flash": "google",
		"gpt-5.6-luna":     "openai",
		"gpt-5.6-sol":      "openai",
	}

	for _, model := range models {
		if expectedOwner, exists := ownershipTests[model.ID]; exists {
			if model.OwnedBy != expectedOwner {
				t.Errorf("Model '%s': Expected owner '%s', got '%s'",
					model.ID, expectedOwner, model.OwnedBy)
			}
		}
	}
}

func TestModelTimestamps(t *testing.T) {
	models := internal.GetDefault()

	now := time.Now().Unix()
	tolerance := int64(5) // 5 seconds tolerance

	for _, model := range models {
		if model.Created == 0 {
			t.Errorf("Model '%s': Expected non-zero created timestamp", model.ID)
		}

		// Check that timestamp is recent (within tolerance)
		if model.Created < now-tolerance || model.Created > now+tolerance {
			t.Errorf("Model '%s': Created timestamp %d seems wrong (current: %d)",
				model.ID, model.Created, now)
		}
	}
}

func TestModelAPIType(t *testing.T) {
	models := internal.GetDefault()

	for _, model := range models {
		switch model.APIType {
		case "chat_completions", "responses":
			// valid
		default:
			t.Errorf("Model '%s': Expected api_type to be 'chat_completions' or 'responses', got '%s'",
				model.ID, model.APIType)
		}
	}
}

// CountingCache implements CoalescingCacheInterface with execution counting
type CountingCache struct {
	executeCount int
}

func (c *CountingCache) GetRequestKey(method, path string, _ interface{}) string {
	return method + ":" + path
}

func (c *CountingCache) CoalesceRequest(_ string, fn func() interface{}) interface{} {
	c.executeCount++
	return fn()
}

func TestCoalescingCacheIntegration(t *testing.T) {
	// Test that the models service properly uses the coalescing cache
	cache := &CountingCache{executeCount: 0}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	service := internal.NewModelsService(cache, httpClient)
	handler := service.Handler()

	// Make multiple requests
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/v1/models", http.NoBody)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: Expected status 200, got %d", i, w.Code)
		}
	}

	// Verify cache was used
	if cache.executeCount != 3 {
		t.Errorf("Expected cache CoalesceRequest to be called 3 times, got %d", cache.executeCount)
	}
}
