// Package internal provides proxy service logic for github-copilot-svcs.
package internal

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

var copilotAPIBase = "https://api.githubcopilot.com"
var completionsPath = "/completions"
var chatCompletionsPath = "/chat/completions"

const (
	maxChatRetries     = 3
	baseChatRetryDelay = 1 // seconds

	// Circuit breaker configuration
	circuitBreakerFailureThreshold = 5

	// Request configuration
	maxRequestBodySize  = 5 * 1024 * 1024 // 5MB
	streamingBufferSize = 1024

	// Status code ranges
	statusCodeServerError     = 500
	statusCodeTooManyRequests = 429
	statusCodeRequestTimeout  = 408
)

const (
	// ProxyCBStateClosed indicates the circuit breaker is closed.
	ProxyCBStateClosed = 0
	// ProxyCBStateOpen indicates the circuit breaker is open.
	ProxyCBStateOpen = 1
	// ProxyCBStateHalfOpen indicates the circuit breaker is half-open.
	ProxyCBStateHalfOpen = 2
)

// CircuitBreakerState represents the state of the circuit breaker
type CircuitBreakerState int

const (
	// CircuitClosed allows all requests through
	CircuitClosed CircuitBreakerState = iota
	// CircuitOpen rejects all requests
	CircuitOpen
	// CircuitHalfOpen allows limited requests through
	CircuitHalfOpen
)

// CircuitBreaker implements circuit breaker pattern for upstream API calls
type CircuitBreaker struct {
	failureCount    int64
	lastFailureTime time.Time
	state           CircuitBreakerState
	timeout         time.Duration
	mutex           sync.RWMutex
}

// CoalescingCache handles request coalescing for identical requests
type CoalescingCache struct {
	requests map[string]chan interface{}
	mutex    sync.RWMutex
}

// ProxyService provides proxy functionality
type ProxyService struct {
	config         *Config
	httpClient     *http.Client
	authService    *AuthService
	workerPool     WorkerPoolInterface
	circuitBreaker *CircuitBreaker
	bufferPool     *sync.Pool
}

// WorkerPoolInterface interface for background processing
type WorkerPoolInterface interface {
	Submit(job func())
}

// responseWrapper tracks if headers have been sent
type responseWrapper struct {
	http.ResponseWriter
	headersSent bool
}

// NewCoalescingCache creates a new coalescing cache
func NewCoalescingCache() *CoalescingCache {
	return &CoalescingCache{
		requests: make(map[string]chan interface{}),
	}
}

// GetRequestKey generates a cache key for request coalescing
func (cc *CoalescingCache) GetRequestKey(method, url string, body interface{}) string {
	h := sha256.New()
	h.Write([]byte(method))
	h.Write([]byte(url))
	if body != nil {
		if bodyBytes, ok := body.([]byte); ok {
			h.Write(bodyBytes)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// CoalesceRequest executes a function only once for identical concurrent requests
func (cc *CoalescingCache) CoalesceRequest(key string, fn func() interface{}) interface{} {
	cc.mutex.Lock()

	// Check if request is already in progress
	if ch, exists := cc.requests[key]; exists {
		cc.mutex.Unlock()
		// Wait for the existing request to complete
		return <-ch
	}

	// Create new channel for this request
	ch := make(chan interface{}, 1)
	cc.requests[key] = ch
	cc.mutex.Unlock()

	// Execute the request
	result := fn()

	// Broadcast result to all waiting goroutines
	ch <- result
	close(ch)

	// Clean up
	cc.mutex.Lock()
	delete(cc.requests, key)
	cc.mutex.Unlock()

	return result
}

// NewProxyService creates a new proxy service
func NewProxyService(cfg *Config, httpClient *http.Client, authService *AuthService, workerPool WorkerPoolInterface) *ProxyService {
	circuitBreaker := &CircuitBreaker{
		state:   CircuitClosed,
		timeout: time.Duration(cfg.Timeouts.CircuitBreaker) * time.Second,
	}

	bufferPool := &sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}

	return &ProxyService{
		config:         cfg,
		httpClient:     httpClient,
		authService:    authService,
		workerPool:     workerPool,
		circuitBreaker: circuitBreaker,
		bufferPool:     bufferPool,
	}
}

// Handler returns an HTTP handler for the proxy endpoint
func (s *ProxyService) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Create context with extended timeout for long-lived streaming responses
		ctx, cancel := context.WithTimeout(r.Context(), time.Duration(s.config.Timeouts.ProxyContext)*time.Second)
		defer cancel()

		// Check circuit breaker
		if !s.circuitBreaker.canExecute() {
			Warn("Circuit breaker is open, rejecting request")
			http.Error(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
			return
		}

		// Limit request body size
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)

		// Use a response wrapper to track if headers have been sent
		respWrapper := &responseWrapper{ResponseWriter: w, headersSent: false}

		// Create a done channel to track completion
		done := make(chan error, 1)

		// Submit request to worker pool
		s.workerPool.Submit(func() {
			defer func() {
				if recovery := recover(); recovery != nil {
					Error("Worker panic recovered", "panic", recovery)
					done <- NewProxyError("request_processing", "worker panic during request processing", fmt.Errorf("panic: %v", recovery))
				}
			}()

			err := s.processProxyRequest(ctx, respWrapper, r)
			done <- err
		})

		// Wait for worker to complete or context timeout
		select {
		case err := <-done:
			if err != nil {
				Error("Worker error", "error", err)
				// Only write error if headers haven't been sent
				if !respWrapper.headersSent {
					switch {
					case strings.Contains(err.Error(), "authentication error"):
						http.Error(w, err.Error(), http.StatusUnauthorized)
					case strings.Contains(err.Error(), "token validation failed"):
						http.Error(w, err.Error(), http.StatusUnauthorized)
					case strings.Contains(err.Error(), "bad request"):
						http.Error(w, err.Error(), http.StatusBadRequest)
					case strings.Contains(err.Error(), "method not allowed"):
						http.Error(w, err.Error(), http.StatusMethodNotAllowed)
					default:
						http.Error(w, err.Error(), http.StatusInternalServerError)
					}
				}
			}
		case <-ctx.Done():
			Warn("Request timeout in worker pool")
			// Only write timeout error if headers haven't been sent
			if !respWrapper.headersSent {
				http.Error(w, "Request timeout", http.StatusRequestTimeout)
			}
		}
	}
}

func (rw *responseWrapper) WriteHeader(statusCode int) {
	if !rw.headersSent {
		rw.headersSent = true
		rw.ResponseWriter.WriteHeader(statusCode)
	}
}

func (rw *responseWrapper) Write(data []byte) (int, error) {
	if !rw.headersSent {
		rw.headersSent = true
	}
	return rw.ResponseWriter.Write(data)
}

func (cb *CircuitBreaker) canExecute() bool {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	// No metrics to update for circuit breaker state changes

	if cb.state == CircuitClosed {
		return true
	}

	if cb.state == CircuitOpen {
		if time.Since(cb.lastFailureTime) > cb.timeout {
			cb.mutex.RUnlock()
			cb.mutex.Lock()
			cb.state = CircuitHalfOpen
			cb.mutex.Unlock()
			cb.mutex.RLock()
			return true
		}
		return false
	}

	// CircuitHalfOpen
	return true
}

func (cb *CircuitBreaker) onSuccess() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.failureCount = 0
	cb.state = CircuitClosed
}

func (cb *CircuitBreaker) onFailure() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.failureCount++
	cb.lastFailureTime = time.Now()

	if cb.failureCount >= circuitBreakerFailureThreshold {
		cb.state = CircuitOpen
	}
}

func (s *ProxyService) processProxyRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	Debug("Starting proxy request", "method", r.Method, "path", r.URL.Path)

	// Validate method
	if r.Method != http.MethodPost {
		return fmt.Errorf("method not allowed: %s", r.Method)
	}

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		Error("Error reading request body", "error", err)
		// Check for "http: request body too large" error and return 413
		if strings.Contains(err.Error(), "http: request body too large") {
			return fmt.Errorf("payload too large: %w", err)
		}
		return fmt.Errorf("bad request: failed to read request body: %w", err)
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			Warn("Error closing request body", "error", err)
		}
	}()

	// Basic body validation (for demonstration: consider empty body an error)
	if len(body) == 0 {
		return fmt.Errorf("bad request: empty request body")
	}

	var input struct {
		Model string `json:"model"`
	}
	if jsonErr := json.Unmarshal(body, &input); jsonErr != nil {
		return fmt.Errorf("bad request: invalid JSON: %w", jsonErr)
	}

	// AllowedModels validation
	if len(s.config.AllowedModels) > 0 {
		allowed := false
		for _, m := range s.config.AllowedModels {
			if input.Model == m {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("bad request: model '%s' is not allowed by allowed_models in config", input.Model)
		}
	}

	// Ensure we have a valid token before making the request
	if tokenErr := s.authService.EnsureValidToken(s.config); tokenErr != nil {
		Error("Failed to ensure valid token", "error", tokenErr)
		return NewAuthError("token validation failed", tokenErr)
	}

	// Create new request to GitHub Copilot
	var targetURL string
	base := copilotAPIBase
	switch r.URL.Path {
	case "/v1/completions":
		targetURL = base + completionsPath
	case "/v1/chat/completions":
		targetURL = base + chatCompletionsPath
	default:
		return fmt.Errorf("unsupported proxy path: %s", r.URL.Path)
	}
	Debug("Sending request to target", "url", targetURL, "body_length", len(body))

	req, err := http.NewRequestWithContext(ctx, r.Method, targetURL, bytes.NewBuffer(body))
	if err != nil {
		Error("Error creating request", "error", err)
		return NewProxyError("create_request", "failed to create proxy request", err)
	}

	// Set headers
	// Forward content/negotiation headers from client if present; use defaults if missing
	headersToProxy := []string{"Content-Type", "Accept", "Accept-Encoding", "TE"}
	defaults := map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json",
	}
	for _, h := range headersToProxy {
		if v := r.Header.Get(h); v != "" {
			req.Header.Set(h, v)
		} else if def, ok := defaults[h]; ok {
			req.Header.Set(h, def)
		}
	}

	req.Header.Set("Authorization", "Bearer "+s.config.CopilotToken)
	req.Header.Set("User-Agent", s.config.Headers.UserAgent)
	req.Header.Set("Editor-Version", s.config.Headers.EditorVersion)
	req.Header.Set("Editor-Plugin-Version", s.config.Headers.EditorPluginVersion)
	req.Header.Set("Copilot-Integration-Id", s.config.Headers.CopilotIntegrationID)
	req.Header.Set("Openai-Intent", s.config.Headers.OpenaiIntent)
	req.Header.Set("X-Initiator", s.config.Headers.XInitiator)

	resp, err := s.makeRequestWithRetry(req, body)
	if err != nil {
		s.circuitBreaker.onFailure()
		Error("Error making request after retries", "error", err)
		return NewNetworkError("proxy_request", targetURL, "failed to complete request after retries", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			Warn("Error closing response body", "error", err)
		}
	}()

	// Update circuit breaker based on response
	if resp.StatusCode < statusCodeServerError {
		s.circuitBreaker.onSuccess()
	} else {
		s.circuitBreaker.onFailure()
	}

	Debug("Received response", "status", resp.StatusCode, "content_type", resp.Header.Get("Content-Type"))

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Add configurable CORS headers
	if len(s.config.CORS.AllowedOrigins) > 0 {
		w.Header().Set("Access-Control-Allow-Origin", strings.Join(s.config.CORS.AllowedOrigins, ", "))
	}
	if len(s.config.CORS.AllowedHeaders) > 0 {
		w.Header().Set("Access-Control-Allow-Headers", strings.Join(s.config.CORS.AllowedHeaders, ", "))
	}

	// Copy status code
	w.WriteHeader(resp.StatusCode)

	// Handle streaming vs regular responses
	if resp.Header.Get("Content-Type") == "text/event-stream" {
		return s.handleStreamingResponse(w, resp)
	}
	return s.handleRegularResponse(w, resp)
}

func (s *ProxyService) handleStreamingResponse(w http.ResponseWriter, resp *http.Response) error {
	Debug("Starting streaming response copy")

	if flusher, ok := w.(http.Flusher); ok {
		// Copy in chunks and flush periodically for better streaming
		buf := make([]byte, streamingBufferSize)
		for {
			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				_, writeErr := w.Write(buf[:n])
				if writeErr != nil {
					Error("Error writing streaming chunk", "error", writeErr)
					return writeErr
				}
				flusher.Flush()
			}
			if readErr == io.EOF {
				Debug("Streaming response completed successfully")
				break
			}
			if readErr != nil {
				Error("Error reading streaming response", "error", readErr)
				return readErr
			}
		}
	} else {
		// Fallback to direct copy if no flusher available
		_, err := io.Copy(w, resp.Body)
		if err != nil {
			Error("Error copying streaming response", "error", err)
			return err
		}
	}
	return nil
}

func (s *ProxyService) handleRegularResponse(w http.ResponseWriter, resp *http.Response) error {
	Debug("Starting regular response copy")

	// Use buffer pool for regular responses
	buf := s.bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer s.bufferPool.Put(buf)

	_, err := io.CopyBuffer(w, resp.Body, buf.Bytes()[:0])
	if err != nil {
		Error("Error copying response", "error", err)
		return err
	}

	Debug("Regular response completed successfully")
	return nil
}

func (s *ProxyService) makeRequestWithRetry(req *http.Request, body []byte) (*http.Response, error) {
	var lastResp *http.Response
	var lastErr error

	for attempt := 1; attempt <= maxChatRetries; attempt++ {
		// Create a new request for each attempt with the original context
		retryReq, err := http.NewRequestWithContext(req.Context(), req.Method, req.URL.String(), bytes.NewBuffer(body))
		if err != nil {
			return nil, err
		}

		// Copy all headers
		for key, values := range req.Header {
			for _, value := range values {
				retryReq.Header.Add(key, value)
			}
		}

		Debug("Making request attempt", "attempt", attempt, "max_attempts", maxChatRetries)

		resp, err := s.httpClient.Do(retryReq)
		if err != nil {
			lastErr = err
			if attempt == maxChatRetries {
				Error("Request failed after max attempts", "attempts", maxChatRetries, "error", err)
				return nil, err
			}

			// Context-aware waiting instead of blocking sleep
			waitTime := time.Duration(baseChatRetryDelay*attempt*attempt) * time.Second
			Warn("Request failed, retrying", "attempt", attempt, "wait_time", waitTime, "error", err)

			timer := time.NewTimer(waitTime)
			select {
			case <-timer.C:
				// Continue with retry
			case <-req.Context().Done():
				timer.Stop()
				return nil, req.Context().Err()
			}
			continue
		}

		lastResp = resp

		// Check if we should retry based on status code
		if !s.isRetriableError(resp.StatusCode, nil) {
			Debug("Request successful", "attempt", attempt, "status", resp.StatusCode)
			return resp, nil
		}

		// Close the response body before retrying
		if closeErr := resp.Body.Close(); closeErr != nil {
			Warn("Failed to close response body during retry", "error", closeErr)
		}

		if attempt == maxChatRetries {
			Warn("Request failed after max attempts", "attempts", maxChatRetries, "status", resp.StatusCode)
			return resp, nil // Return the last response even if it failed
		}

		// Context-aware waiting for status code retries
		waitTime := time.Duration(baseChatRetryDelay*attempt*attempt) * time.Second
		Warn("Request failed, retrying", "status", resp.StatusCode, "attempt", attempt, "wait_time", waitTime)

		timer := time.NewTimer(waitTime)
		select {
		case <-timer.C:
			// Continue with retry
		case <-req.Context().Done():
			timer.Stop()
			return nil, req.Context().Err()
		}
	}

	return lastResp, lastErr
}

func (s *ProxyService) isRetriableError(statusCode int, err error) bool {
	if err != nil {
		return true // Network errors are retriable
	}

	// Retry on server errors and rate limiting
	return statusCode >= statusCodeServerError || statusCode == statusCodeTooManyRequests || statusCode == statusCodeRequestTimeout
}
