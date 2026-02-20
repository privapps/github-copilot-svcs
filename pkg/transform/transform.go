// Package transform provides OpenAI-compatible request/response structures for github-copilot-svcs.
package transform

import "encoding/json"

// ChatCompletionRequest ...
type ChatCompletionRequest struct {
	Model       string                  `json:"model"`
	Messages    []ChatCompletionMessage `json:"messages"`
	Temperature *float64                `json:"temperature,omitempty"`
	MaxTokens   *int                    `json:"max_tokens,omitempty"`
	Stream      bool                    `json:"stream,omitempty"`
}

// ChatCompletionMessage supports both text-only content (string) and multi-part content (array)
// for vision/image requests. Content can be either:
//   - A string for simple text messages
//   - An array of ContentPart objects for messages with images
type ChatCompletionMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"` // Can be string or []ContentPart
}

// ContentPart represents a part of a multi-part message (text or image)
type ContentPart struct {
	Type     string    `json:"type"`               // "text" or "image_url"
	Text     string    `json:"text,omitempty"`     // For type="text"
	ImageURL *ImageURL `json:"image_url,omitempty"` // For type="image_url"
}

// ImageURL contains the image URL (can be http(s):// or data: URI with base64)
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"` // "auto", "low", or "high"
}

// ChatCompletionResponse ...
type ChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
	Usage   ChatCompletionUsage    `json:"usage"`
}

// ChatCompletionChoice ...
type ChatCompletionChoice struct {
	Index        int                   `json:"index"`
	Message      ChatCompletionMessage `json:"message"`
	FinishReason string                `json:"finish_reason"`
}

// ChatCompletionUsage ...
type ChatCompletionUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ModelList ...
type ModelList struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

// Model ...
type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}