package claude

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

	"github.com/cenkalti/backoff/v4"
	"github.com/tildaslashalef/mindnest/internal/config"
	"github.com/tildaslashalef/mindnest/internal/loggy"
)

// Client represents an Anthropic Claude API client
// It handles all communication with the Claude API
type Client struct {
	apiKey           string
	baseURL          string
	defaultModel     string
	httpMultiClient  *http.Client
	maxRetries       int
	defaultMaxTokens int
	apiVersion       string
	useAPIBeta       bool
	apiBeta          []string
	topP             *float64
	topK             *int
	temperature      *float64
}

// NewClient creates a new Claude client from config
func NewClient(cfg config.ClaudeConfig) *Client {
	// Ensure baseURL doesn't end with a slash
	baseURL := strings.TrimSuffix(cfg.BaseURL, "/")

	// Set default API version if not provided
	apiVersion := cfg.APIVersion
	if apiVersion == "" {
		apiVersion = "2023-06-01"
	}

	// Set default model if not provided
	defaultModel := cfg.Model
	if defaultModel == "" {
		defaultModel = "claude-3-5-sonnet-20241022"
	}

	// Set default max tokens if not provided
	defaultMaxTokens := cfg.MaxTokens
	if defaultMaxTokens <= 0 {
		defaultMaxTokens = 4096
	}

	// Filter API beta features to ensure no comments or empty values
	var validBeta []string
	if cfg.UseAPIBeta {
		for _, beta := range cfg.APIBeta {
			beta = strings.TrimSpace(beta)
			if beta != "" && !strings.HasPrefix(beta, "#") {
				validBeta = append(validBeta, beta)
			}
		}

		if len(validBeta) > 0 {
			loggy.Info("Claude client initialized with API beta features", "features", validBeta)
		} else if len(cfg.APIBeta) > 0 {
			loggy.Warn("Claude client had invalid API beta features that were filtered out", "original", cfg.APIBeta)
		}
	}

	// Create pointers for optional parameters only if they have valid values
	var tempPtr, topPPtr *float64
	var topKPtr *int

	if cfg.Temperature > 0 {
		tempPtr = &cfg.Temperature
	}
	if cfg.TopP > 0 {
		topPPtr = &cfg.TopP
	}
	if cfg.TopK > 0 {
		topKPtr = &cfg.TopK
	}

	return &Client{
		apiKey:           cfg.APIKey,
		baseURL:          baseURL,
		defaultModel:     defaultModel,
		httpMultiClient:  &http.Client{Timeout: cfg.Timeout},
		maxRetries:       cfg.MaxRetries,
		defaultMaxTokens: defaultMaxTokens,
		apiVersion:       apiVersion,
		useAPIBeta:       cfg.UseAPIBeta && len(validBeta) > 0,
		apiBeta:          validBeta,
		topP:             topPPtr,
		topK:             topKPtr,
		temperature:      tempPtr,
	}
}

// GenerateChat sends a chat completion request to Claude
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
	if err := c.makeRequest(ctx, "POST", "/v1/messages", req, &resp); err != nil {
		return nil, fmt.Errorf("generating chat completion: %w", err)
	}

	return &resp, nil
}

// GenerateChatStream sends a streaming chat completion request to Claude
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
// Note: Claude doesn't natively support embeddings, this is included for interface compatibility
func (c *Client) GenerateEmbedding(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
	// Claude doesn't support embeddings natively, return error
	return nil, errors.New("embedding generation not supported by Claude client")
}

// BatchEmbeddings generates embeddings for multiple texts
// Note: Claude doesn't natively support embeddings, this is included for interface compatibility
func (c *Client) BatchEmbeddings(ctx context.Context, requests []EmbeddingRequest) ([]*EmbeddingResponse, error) {
	// Claude doesn't support embeddings natively, return error
	return nil, errors.New("batch embeddings not supported by Claude client")
}

// handleStreamingRequest handles streaming responses from Claude
// It processes the SSE stream from Claude and sends responses to the channel
func (c *Client) handleStreamingRequest(ctx context.Context, req ChatRequest, responseChan chan<- ChatResponse) error {
	jsonBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/messages", strings.NewReader(string(jsonBody)))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", c.apiVersion)

	// Set API beta headers if specified and we have valid features
	if c.useAPIBeta && len(c.apiBeta) > 0 {
		betaHeader, err := json.Marshal(c.apiBeta)
		if err == nil {
			httpReq.Header.Set("anthropic-beta", string(betaHeader))
			loggy.Debug("Setting anthropic-beta header", "beta_features", c.apiBeta)
		} else {
			loggy.Warn("Failed to marshal anthropic-beta header", "error", err)
		}
	}

	resp, err := c.httpMultiClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read the response body for the error
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("reading error response body: %w", err)
		}
		return c.handleErrorResponse(resp, respBody)
	}

	scanner := bufio.NewScanner(resp.Body)
	var model string

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			line := scanner.Text()
			if len(line) == 0 {
				continue
			}

			var streamResp MessageStreamResponse
			if err := json.Unmarshal([]byte(line), &streamResp); err != nil {
				loggy.Error("error decoding streaming response", "error", err, "line", line)
				continue
			}

			// Set the model if it's not set yet
			if model == "" && streamResp.Message.Model != "" {
				model = streamResp.Message.Model
			}

			switch streamResp.Type {
			case "content_block_start", "content_block_delta":
				responseChan <- ChatResponse{
					Model:   model,
					Message: Message{Role: "assistant", Content: streamResp.Delta.Text},
					Done:    false,
				}
			case "message_delta":
				if streamResp.Delta.StopReason != "" {
					responseChan <- ChatResponse{
						Model:   model,
						Message: Message{Role: "assistant", Content: ""},
						Done:    true,
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading stream: %w", err)
	}

	return nil
}

// makeRequest is a helper function to make HTTP requests with retries
// It uses exponential backoff for retrying failed requests
func (c *Client) makeRequest(ctx context.Context, method, path string, body interface{}, response interface{}) error {
	var bodyReader io.Reader
	var bodyBytes []byte
	var err error

	if body != nil {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling request body: %w", err)
		}
		bodyReader = strings.NewReader(string(bodyBytes))

		loggy.Debug("Sending CLAUDE LLM request",
			"method", method,
			"url", c.baseURL+path,
			"body", string(bodyBytes))
	}

	url := fmt.Sprintf("%s%s", c.baseURL, path)
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", c.apiVersion)

	// Log request headers for debugging (excluding sensitive info)
	headers := make(map[string]string)
	for k, v := range req.Header {
		if k != "x-api-key" { // Don't log the API key
			headers[k] = strings.Join(v, ", ")
		} else {
			headers[k] = "[REDACTED]"
		}
	}
	loggy.Debug("Claude request headers", "headers", headers)

	// Set API beta headers if specified and we have valid features
	if c.useAPIBeta && len(c.apiBeta) > 0 {
		betaHeader, err := json.Marshal(c.apiBeta)
		if err == nil {
			req.Header.Set("anthropic-beta", string(betaHeader))
			loggy.Debug("Setting anthropic-beta header", "beta_features", c.apiBeta)
		} else {
			loggy.Warn("Failed to marshal anthropic-beta header", "error", err)
		}
	}

	var lastErr error
	operation := func() error {
		resp, err := c.httpMultiClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("sending request: %w", err)
			return lastErr
		}
		defer resp.Body.Close()

		// Read the full response body for debugging
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("reading response body: %w", err)
			return lastErr
		}

		// Log response details
		loggy.Debug("Claude API response",
			"status", resp.Status,
			"status_code", resp.StatusCode,
			"content_length", len(respBody),
			"response_body", string(respBody))

		// Create a new reader with the body for json decoding
		bodyReader = bytes.NewReader(respBody)

		if resp.StatusCode != http.StatusOK {
			// Log additional response headers for error diagnosis
			respHeaders := make(map[string]string)
			for k, v := range resp.Header {
				respHeaders[k] = strings.Join(v, ", ")
			}
			loggy.Error("Claude API error response",
				"status", resp.Status,
				"headers", respHeaders,
				"body", string(respBody))

			lastErr = c.handleErrorResponse(resp, respBody)
			return lastErr
		}

		if err := json.NewDecoder(bodyReader).Decode(response); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}

		return nil
	}

	err = backoff.Retry(operation, backoff.WithMaxRetries(backoff.NewExponentialBackOff(), uint64(c.maxRetries)))
	if err != nil {
		return lastErr
	}

	return nil
}

// handleErrorResponse processes error responses from the API
// It attempts to parse the error JSON and return a structured error
func (c *Client) handleErrorResponse(resp *http.Response, body []byte) error {
	// If body is already read, use it directly
	var apiErr APIError
	if err := json.Unmarshal(body, &apiErr); err != nil {
		// Try a more generic approach to see if there's any JSON in the response
		var genericErr map[string]interface{}
		if jsonErr := json.Unmarshal(body, &genericErr); jsonErr == nil {
			loggy.Debug("Parsed generic error response", "error", genericErr)
		}

		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	return &apiErr
}
