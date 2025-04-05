package gemini

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/tildaslashalef/mindnest/internal/loggy"
)

// Client represents a Google Gemini API client
type Client struct {
	apiKey           string
	baseURL          string
	defaultModel     string
	embeddingModel   string
	apiVersion       string
	embeddingVersion string
	httpClient       *http.Client
	maxRetries       int
	defaultMaxTokens int
	topP             *float64
	topK             *int
	temperature      *float64
}

// Config configures the Gemini client
type Config struct {
	APIKey           string        // API key for authentication
	BaseURL          string        // Base URL for Gemini API
	DefaultModel     string        // Default model to use if not specified in request
	EmbeddingModel   string        // Default embedding model to use
	APIVersion       string        // API version for chat models (v1 or v1beta)
	EmbeddingVersion string        // API version for embedding models (v1 or v1beta)
	Timeout          time.Duration // HTTP client timeout
	MaxRetries       int           // Maximum retries on retryable errors
	DefaultMaxTokens int           // Default max tokens for generation
	TopP             *float64      // Default top_p value
	TopK             *int          // Default top_k value
	Temperature      *float64      // Default temperature value
}

// NewClient creates a new Gemini client from config
func NewClient(cfg Config) *Client {
	// Ensure baseURL doesn't end with a slash
	baseURL := strings.TrimSuffix(cfg.BaseURL, "/")

	// Set default model if not provided
	defaultModel := cfg.DefaultModel
	if defaultModel == "" {
		defaultModel = "gemini-2.5-pro-exp-03-25" // Updated to use the latest experimental model
	}

	// Set default embedding model if not provided
	embeddingModel := cfg.EmbeddingModel
	if embeddingModel == "" {
		embeddingModel = "text-embedding-004" // Updated to use the latest embedding model
	}

	// Set default max tokens if not provided
	defaultMaxTokens := cfg.DefaultMaxTokens
	if defaultMaxTokens <= 0 {
		defaultMaxTokens = 4096
	}

	// Set default API versions if not provided
	apiVersion := cfg.APIVersion
	if apiVersion == "" {
		apiVersion = "v1beta" // Default to v1beta for chat models
	}

	embeddingVersion := cfg.EmbeddingVersion
	if embeddingVersion == "" {
		embeddingVersion = "v1beta" // Default to v1beta for embedding models
	}

	return &Client{
		apiKey:           cfg.APIKey,
		baseURL:          baseURL,
		defaultModel:     defaultModel,
		embeddingModel:   embeddingModel,
		apiVersion:       apiVersion,
		embeddingVersion: embeddingVersion,
		httpClient:       &http.Client{Timeout: cfg.Timeout},
		maxRetries:       cfg.MaxRetries,
		defaultMaxTokens: defaultMaxTokens,
		topP:             cfg.TopP,
		topK:             cfg.TopK,
		temperature:      cfg.Temperature,
	}
}

// GenerateChat sends a chat completion request to Gemini
func (c *Client) GenerateChat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// Set default model if none specified
	if req.Model == "" {
		req.Model = c.defaultModel
	}

	// Set default max tokens if none specified
	if req.MaxTokens <= 0 {
		req.MaxTokens = c.defaultMaxTokens
	}

	// Set default temperature if none specified and client has a default
	if req.Temperature == nil && c.temperature != nil {
		req.Temperature = c.temperature
	}

	// Set default top_p if none specified and client has a default
	if req.TopP == nil && c.topP != nil {
		req.TopP = c.topP
	}

	// Set default top_k if none specified and client has a default
	if req.TopK == nil && c.topK != nil {
		req.TopK = c.topK
	}

	// Force stream to false for non-streaming requests
	req.Stream = false

	var resp ChatResponse
	if err := c.makeRequest(ctx, "POST", fmt.Sprintf("/v1/models/%s:generateContent", req.Model), req, &resp); err != nil {
		return nil, fmt.Errorf("generating chat completion: %w", err)
	}

	return &resp, nil
}

// GenerateChatStream sends a streaming chat completion request to Gemini
func (c *Client) GenerateChatStream(ctx context.Context, req ChatRequest) (<-chan ChatResponse, error) {
	responses := make(chan ChatResponse)

	// Set default model if none specified
	if req.Model == "" {
		req.Model = c.defaultModel
	}

	// Set default max tokens if none specified
	if req.MaxTokens <= 0 {
		req.MaxTokens = c.defaultMaxTokens
	}

	// Set default temperature if none specified and client has a default
	if req.Temperature == nil && c.temperature != nil {
		req.Temperature = c.temperature
	}

	// Set default top_p if none specified and client has a default
	if req.TopP == nil && c.topP != nil {
		req.TopP = c.topP
	}

	// Set default top_k if none specified and client has a default
	if req.TopK == nil && c.topK != nil {
		req.TopK = c.topK
	}

	// Force stream to true
	req.Stream = true

	// Create a context with cancel for cleanup
	streamCtx, cancel := context.WithCancel(ctx)

	go func() {
		defer close(responses)
		defer cancel()

		// Use exponential backoff for retries
		operation := func() error {
			return c.handleStreamingRequest(streamCtx, req, responses)
		}

		err := backoff.Retry(operation, backoff.WithMaxRetries(backoff.NewExponentialBackOff(), uint64(c.maxRetries)))
		if err != nil {
			// Try to send error response, but don't block if context is cancelled
			select {
			case responses <- ChatResponse{ErrorMsg: err.Error()}:
			case <-streamCtx.Done():
			}
			return
		}
	}()

	// Return the channel immediately
	return responses, nil
}

// GenerateEmbedding generates an embedding for a single text input
func (c *Client) GenerateEmbedding(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
	// Create embedding request
	embedReq := map[string]interface{}{
		"model": c.embeddingModel,
		"content": map[string]interface{}{
			"parts": []map[string]interface{}{
				{
					"text": req.Text,
				},
			},
		},
	}

	// Log embedding request with model and API version
	loggy.Debug("Generating embedding",
		"model", c.embeddingModel,
		"api_version", c.embeddingVersion,
		"text_length", len(req.Text))

	// Make the embedding request using the configured embedding API version
	var resp struct {
		Embedding struct {
			Values []float32 `json:"values"`
		} `json:"embedding"`
	}

	if err := c.makeRequest(ctx, "POST", fmt.Sprintf("models/%s:embedContent", c.embeddingModel), embedReq, &resp); err != nil {
		return nil, fmt.Errorf("generating embedding: %w", err)
	}

	return &EmbeddingResponse{
		Embedding: resp.Embedding.Values,
	}, nil
}

// BatchEmbeddings generates embeddings for multiple texts
func (c *Client) BatchEmbeddings(ctx context.Context, requests []EmbeddingRequest) ([]*EmbeddingResponse, error) {
	// Gemini API doesn't support batch embedding natively, so we'll do them sequentially
	responses := make([]*EmbeddingResponse, len(requests))

	for i, req := range requests {
		resp, err := c.GenerateEmbedding(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("batch embedding failed at index %d: %w", i, err)
		}
		responses[i] = resp
	}

	return responses, nil
}

// makeRequest makes a request to the Gemini API
func (c *Client) makeRequest(ctx context.Context, method, path string, requestBody interface{}, responseBody interface{}) error {
	// Extract path components to determine if this is an embedding request
	isEmbedding := strings.Contains(path, ":embedContent")

	// Choose the appropriate API version
	apiVersion := c.apiVersion
	if isEmbedding {
		apiVersion = c.embeddingVersion
	}

	// Normalize the path to ensure consistent formatting
	normalizedPath := path
	if strings.HasPrefix(normalizedPath, "/v1/") {
		normalizedPath = strings.TrimPrefix(normalizedPath, "/v1/")
	} else if strings.HasPrefix(normalizedPath, "/v1beta/") {
		normalizedPath = strings.TrimPrefix(normalizedPath, "/v1beta/")
	} else if strings.HasPrefix(normalizedPath, "/") {
		normalizedPath = strings.TrimPrefix(normalizedPath, "/")
	}

	// Create full URL with the appropriate API version
	url := fmt.Sprintf("%s/%s/%s", c.baseURL, apiVersion, normalizedPath)

	// Convert request to JSON
	var reqBody io.Reader
	var requestBytes []byte
	if requestBody != nil {
		var err error
		requestBytes, err = json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("marshalling request: %w", err)
		}
		reqBody = bytes.NewBuffer(requestBytes)

		// Log request details without sensitive information
		requestCopy := make(map[string]interface{})
		if err := json.Unmarshal(requestBytes, &requestCopy); err == nil {
			// Sanitize any sensitive information if needed
			loggy.Debug("Sending GEMINI LLM request",
				"method", method,
				"url", url,
				"api_version", apiVersion,
				"body", string(requestBytes))
		}
	}

	// Create the HTTP request
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Add API key as query parameter
	q := req.URL.Query()
	q.Add("key", c.apiKey)
	req.URL.RawQuery = q.Encode()

	// Log request headers for debugging (excluding API key)
	headers := make(map[string]string)
	for k, v := range req.Header {
		headers[k] = strings.Join(v, ", ")
	}
	loggy.Debug("Gemini request headers", "headers", headers, "url_path", req.URL.Path)

	// Use exponential backoff for retries
	var lastErr error
	operation := func() error {
		// Send the request
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("sending request: %w", err)
			return lastErr
		}
		defer resp.Body.Close()

		// Read the response body
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("reading response body: %w", err)
			return lastErr
		}

		// Log response details
		loggy.Debug("Gemini API response",
			"status", resp.Status,
			"status_code", resp.StatusCode,
			"content_length", len(bodyBytes),
			"response_body", string(bodyBytes))

		// Check for error status
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			// Log response headers for error diagnosis
			respHeaders := make(map[string]string)
			for k, v := range resp.Header {
				respHeaders[k] = strings.Join(v, ", ")
			}
			loggy.Error("Gemini API error response",
				"status", resp.Status,
				"headers", respHeaders,
				"body", string(bodyBytes))

			// Try to parse as API error
			var apiErr APIError
			if err := json.Unmarshal(bodyBytes, &apiErr); err == nil {
				lastErr = &apiErr
				// Check if error is retryable
				if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError {
					return lastErr // retryable
				}
				return backoff.Permanent(lastErr) // not retryable
			}

			// If not API error, return generic HTTP error
			lastErr = fmt.Errorf("HTTP error: %s, body: %s", resp.Status, string(bodyBytes))
			return lastErr
		}

		// Parse response
		if responseBody != nil {
			if err := json.Unmarshal(bodyBytes, responseBody); err != nil {
				lastErr = fmt.Errorf("unmarshalling response: %w, body: %s", err, string(bodyBytes))
				return lastErr
			}
		}

		return nil
	}

	// Execute with retry
	if err := backoff.Retry(operation, backoff.WithMaxRetries(backoff.NewExponentialBackOff(), uint64(c.maxRetries))); err != nil {
		return errors.Join(err, lastErr)
	}

	return nil
}

// handleStreamingRequest handles the streaming API response for Gemini
func (c *Client) handleStreamingRequest(ctx context.Context, req ChatRequest, responseChan chan<- ChatResponse) error {
	// Convert request body to JSON
	reqData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshalling request: %w", err)
	}

	// Log streaming request
	loggy.Debug("Sending GEMINI streaming request",
		"model", req.Model,
		"body", string(reqData))

	// Create URL with API key and appropriate API version
	url := fmt.Sprintf("%s/%s/models/%s:streamGenerateContent?key=%s",
		c.baseURL, c.apiVersion, req.Model, c.apiKey)

	loggy.Debug("Gemini streaming URL", "url_path", url, "api_version", c.apiVersion)

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqData))
	if err != nil {
		return fmt.Errorf("creating HTTP request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")

	// Log request headers (excluding API key)
	headers := make(map[string]string)
	for k, v := range httpReq.Header {
		headers[k] = strings.Join(v, ", ")
	}
	loggy.Debug("Gemini streaming request headers", "headers", headers, "url_path", httpReq.URL.Path)

	// Send request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	// Log response status and headers
	respHeaders := make(map[string]string)
	for k, v := range resp.Header {
		respHeaders[k] = strings.Join(v, ", ")
	}
	loggy.Debug("Gemini streaming response headers",
		"status", resp.Status,
		"status_code", resp.StatusCode,
		"headers", respHeaders)

	// Check for error status
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		// Read error response
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("reading error response: %w", err)
		}

		loggy.Error("Gemini streaming API error",
			"status", resp.Status,
			"body", string(bodyBytes))

		// Try to parse as API error
		var apiErr APIError
		if err := json.Unmarshal(bodyBytes, &apiErr); err == nil {
			return &apiErr
		}

		// If not API error, return generic HTTP error
		return fmt.Errorf("HTTP error: %s, body: %s", resp.Status, string(bodyBytes))
	}

	// Create scanner for reading the SSE stream
	scanner := bufio.NewScanner(resp.Body)

	// Use a buffer that can handle larger SSE messages
	const maxScanTokenSize = 1024 * 1024 // 1MB
	scanBuf := make([]byte, maxScanTokenSize)
	scanner.Buffer(scanBuf, maxScanTokenSize)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			// Check for data prefix (SSE format)
			if !bytes.HasPrefix(line, []byte("data: ")) {
				continue
			}

			// Extract the JSON data
			data := bytes.TrimPrefix(line, []byte("data: "))
			dataStr := string(data)

			if dataStr == "[DONE]" {
				loggy.Debug("Gemini streaming received DONE marker")
				break
			}

			// Log raw chunk data at trace level
			loggy.Debug("Gemini streaming chunk received", "data", dataStr)

			// Parse the chunk
			var chunk StreamResponseChunk
			if err := json.Unmarshal(data, &chunk); err != nil {
				loggy.Error("Failed to parse stream chunk", "error", err, "data", dataStr)
				continue
			}

			// Check for error in the chunk
			if chunk.ErrorDetail != nil {
				loggy.Error("Gemini streaming chunk error",
					"error", chunk.ErrorDetail.Message,
					"code", chunk.ErrorDetail.Code)
				return fmt.Errorf("stream error: %s", chunk.ErrorDetail.Message)
			}

			// Send the response chunk to the channel
			if len(chunk.Candidates) > 0 {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case responseChan <- ChatResponse{
					Candidates: chunk.Candidates,
					ErrorMsg:   "",
				}:
					// Log successful delivery of chunk
					loggy.Debug("Gemini streaming sent chunk to channel")
				}
			}
		}
	}

	// Check for scanner errors
	if err := scanner.Err(); err != nil {
		loggy.Error("Gemini stream scanner error", "error", err)
		return fmt.Errorf("error reading stream: %w", err)
	}

	return nil
}
