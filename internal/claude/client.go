package claude

import (
	"bufio"
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

// Client represents an Anthropic Claude API client
// It handles all communication with the Claude API
type Client struct {
	apiKey           string
	baseURL          string
	defaultModel     string
	httpClient       *http.Client
	maxRetries       int
	defaultMaxTokens int
	apiVersion       string
	apiBeta          []string
	topP             *float64
	topK             *int
	temperature      *float64
}

// Config configures the Claude client
type Config struct {
	APIKey           string        // API key for authentication
	BaseURL          string        // Base URL for Claude API (e.g., "https://api.anthropic.com")
	DefaultModel     string        // Default model to use if not specified in request
	Timeout          time.Duration // HTTP client timeout
	MaxRetries       int           // Maximum retries on retryable errors
	DefaultMaxTokens int           // Default max tokens for generation
	APIVersion       string        // API version to use (default: "2023-06-01")
	APIBeta          []string      // API beta features to enable
	TopP             *float64      // Default top_p value
	TopK             *int          // Default top_k value
	Temperature      *float64      // Default temperature value
}

// NewClient creates a new Claude client from config
func NewClient(cfg Config) *Client {
	// Ensure baseURL doesn't end with a slash
	baseURL := strings.TrimSuffix(cfg.BaseURL, "/")

	// Set default API version if not provided
	apiVersion := cfg.APIVersion
	if apiVersion == "" {
		apiVersion = "2023-06-01"
	}

	// Set default model if not provided
	defaultModel := cfg.DefaultModel
	if defaultModel == "" {
		defaultModel = "claude-3-7-sonnet-20250219"
	}

	// Set default max tokens if not provided
	defaultMaxTokens := cfg.DefaultMaxTokens
	if defaultMaxTokens <= 0 {
		defaultMaxTokens = 4096
	}

	return &Client{
		apiKey:           cfg.APIKey,
		baseURL:          baseURL,
		defaultModel:     defaultModel,
		httpClient:       &http.Client{Timeout: cfg.Timeout},
		maxRetries:       cfg.MaxRetries,
		defaultMaxTokens: defaultMaxTokens,
		apiVersion:       apiVersion,
		apiBeta:          cfg.APIBeta,
		topP:             cfg.TopP,
		topK:             cfg.TopK,
		temperature:      cfg.Temperature,
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

	// Set API beta headers if specified
	if len(c.apiBeta) > 0 {
		betaHeader, err := json.Marshal(c.apiBeta)
		if err == nil {
			httpReq.Header.Set("anthropic-beta", string(betaHeader))
		}
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.handleErrorResponse(resp)
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
	if body != nil {
		bodyBytes, err := json.Marshal(body)
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

	// Set API beta headers if specified
	if len(c.apiBeta) > 0 {
		betaHeader, err := json.Marshal(c.apiBeta)
		if err == nil {
			req.Header.Set("anthropic-beta", string(betaHeader))
		}
	}

	var lastErr error
	operation := func() error {
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("sending request: %w", err)
			return lastErr
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			lastErr = c.handleErrorResponse(resp)
			return lastErr
		}

		if err := json.NewDecoder(resp.Body).Decode(response); err != nil {
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
func (c *Client) handleErrorResponse(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading error response body: %w", err)
	}

	var apiErr APIError
	if err := json.Unmarshal(body, &apiErr); err != nil {
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	return &apiErr
}
