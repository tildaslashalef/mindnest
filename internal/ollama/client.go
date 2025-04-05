package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/tildaslashalef/mindnest/internal/config"
	"github.com/tildaslashalef/mindnest/internal/loggy"
)

// Client is the Ollama API client
type Client struct {
	// Config for the client
	config config.OllamaConfig

	// HTTP client for API requests
	httpClient *http.Client
}

// NewClient creates a new Ollama client with the provided configuration
func NewClient(cfg config.OllamaConfig) *Client {
	// Create HTTP client with timeout from config
	httpClient := &http.Client{
		Timeout: cfg.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:        cfg.MaxIdleConns,
			MaxIdleConnsPerHost: cfg.MaxIdleConnsPerHost,
			IdleConnTimeout:     cfg.IdleConnTimeout,
		},
	}

	return &Client{
		config:     cfg,
		httpClient: httpClient,
	}
}

// ListModels lists all available models from Ollama
func (c *Client) ListModels(ctx context.Context) ([]ModelInfo, error) {
	var resp ListModelsResponse
	if err := c.makeRequest(ctx, http.MethodGet, "/api/tags", nil, &resp); err != nil {
		return nil, fmt.Errorf("listing models: %w", err)
	}
	return resp.Models, nil
}

// ListRunningModels lists all models currently loaded in memory
func (c *Client) ListRunningModels(ctx context.Context) ([]RunningModelInfo, error) {
	var resp ListRunningModelsResponse
	if err := c.makeRequest(ctx, http.MethodGet, "/api/ps", nil, &resp); err != nil {
		return nil, fmt.Errorf("listing running models: %w", err)
	}
	return resp.Models, nil
}

// GetVersion returns the Ollama server version
func (c *Client) GetVersion(ctx context.Context) (string, error) {
	var resp VersionResponse
	if err := c.makeRequest(ctx, http.MethodGet, "/api/version", nil, &resp); err != nil {
		return "", fmt.Errorf("getting version: %w", err)
	}
	return resp.Version, nil
}

// GenerateChat sends a chat completion request
func (c *Client) GenerateChat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// Use default model if none specified
	if req.Model == "" {
		req.Model = c.config.Model
	}

	// Explicitly set streaming to false for non-streaming requests
	req.Stream = false

	var resp ChatResponse
	if err := c.makeRequest(ctx, http.MethodPost, "/api/chat", req, &resp); err != nil {
		return nil, fmt.Errorf("generating chat completion: %w", err)
	}

	// Check for errors in the response
	if resp.Error != "" {
		return &resp, fmt.Errorf("model error: %s", resp.Error)
	}

	return &resp, nil
}

// GenerateChatStream sends a streaming chat completion request
func (c *Client) GenerateChatStream(ctx context.Context, req ChatRequest) (<-chan ChatResponse, error) {
	// Use default model if none specified
	if req.Model == "" {
		req.Model = c.config.Model
	}

	// Force streaming to true
	req.Stream = true

	// Create response channel
	responseChan := make(chan ChatResponse)

	// Build the request
	url := fmt.Sprintf("%s/api/chat", c.config.Endpoint)
	body, err := json.Marshal(req)
	if err != nil {
		close(responseChan)
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		close(responseChan)
		return nil, fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Start the streaming in a goroutine
	go func() {
		defer close(responseChan)

		// Make the request
		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			responseChan <- ChatResponse{Error: fmt.Sprintf("HTTP request failed: %v", err)}
			return
		}
		defer resp.Body.Close()

		// Check for non-200 status codes
		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			errMsg := fmt.Sprintf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
			responseChan <- ChatResponse{Error: errMsg}
			return
		}

		// Use a decoder to read the streaming JSON objects
		decoder := json.NewDecoder(resp.Body)
		for {
			var chatResp ChatResponse
			if err := decoder.Decode(&chatResp); err != nil {
				if err == io.EOF {
					// Normal end of stream
					break
				}
				// Only send error if it's not due to context cancellation
				select {
				case <-ctx.Done():
					// Context was cancelled, exit silently
					return
				default:
					// Send the error
					responseChan <- ChatResponse{Error: fmt.Sprintf("decoding response: %v", err)}
				}
				break
			}

			// Send the response on the channel
			select {
			case <-ctx.Done():
				// Context was cancelled, exit
				return
			case responseChan <- chatResp:
				// Response sent successfully
				// If this is the last message (Done=true), exit
				if chatResp.Done {
					return
				}
			}
		}
	}()

	return responseChan, nil
}

// GenerateCompletion sends a completion request
func (c *Client) GenerateCompletion(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	// Use default model if none specified
	if req.Model == "" {
		req.Model = c.config.Model
	}

	// Explicitly set streaming to false
	req.Stream = false

	var resp GenerateResponse
	if err := c.makeRequest(ctx, http.MethodPost, "/api/generate", req, &resp); err != nil {
		return nil, fmt.Errorf("generating completion: %w", err)
	}

	// Check for errors in the response
	if resp.Error != "" {
		return &resp, fmt.Errorf("model error: %s", resp.Error)
	}

	return &resp, nil
}

// GenerateCompletionStream sends a streaming completion request
func (c *Client) GenerateCompletionStream(ctx context.Context, req GenerateRequest) (<-chan GenerateResponse, error) {
	// Use default model if none specified
	if req.Model == "" {
		req.Model = c.config.Model
	}

	// Force streaming to true
	req.Stream = true

	// Create response channel
	responseChan := make(chan GenerateResponse)

	// Build request URL and body
	url := fmt.Sprintf("%s/api/generate", c.config.Endpoint)
	body, err := json.Marshal(req)
	if err != nil {
		close(responseChan)
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		close(responseChan)
		return nil, fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Start the streaming in a goroutine
	go func() {
		defer close(responseChan)

		// Make the request
		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			responseChan <- GenerateResponse{Error: fmt.Sprintf("HTTP request failed: %v", err)}
			return
		}
		defer resp.Body.Close()

		// Check for non-200 status codes
		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			errMsg := fmt.Sprintf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
			responseChan <- GenerateResponse{Error: errMsg}
			return
		}

		// Use a decoder to read the streaming JSON objects
		decoder := json.NewDecoder(resp.Body)
		for {
			var genResp GenerateResponse
			if err := decoder.Decode(&genResp); err != nil {
				if err == io.EOF {
					// Normal end of stream
					break
				}
				// Only send error if it's not due to context cancellation
				select {
				case <-ctx.Done():
					// Context was cancelled, exit silently
					return
				default:
					// Send the error
					responseChan <- GenerateResponse{Error: fmt.Sprintf("decoding response: %v", err)}
				}
				break
			}

			// Send the response on the channel
			select {
			case <-ctx.Done():
				// Context was cancelled, exit
				return
			case responseChan <- genResp:
				// Response sent successfully
				// If this is the last message (Done=true), exit
				if genResp.Done {
					return
				}
			}
		}
	}()

	return responseChan, nil
}

// GenerateEmbedding generates embeddings for text(s)
func (c *Client) GenerateEmbedding(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
	// Always use the embedding model for embedding operations
	req.Model = c.config.EmbeddingModel

	var resp EmbeddingResponse
	if err := c.makeRequest(ctx, http.MethodPost, "/api/embed", req, &resp); err != nil {
		return nil, fmt.Errorf("generating embedding: %w", err)
	}

	// Check for errors in the response
	if resp.Error != "" {
		return &resp, fmt.Errorf("model error: %s", resp.Error)
	}

	return &resp, nil
}

// GenerateSingleEmbedding uses the legacy /api/embeddings endpoint for compatibility
func (c *Client) GenerateSingleEmbedding(ctx context.Context, req SingleEmbeddingRequest) (*SingleEmbeddingResponse, error) {
	// Always use the embedding model for embedding operations
	req.Model = c.config.EmbeddingModel

	var resp SingleEmbeddingResponse
	if err := c.makeRequest(ctx, http.MethodPost, "/api/embeddings", req, &resp); err != nil {
		return nil, fmt.Errorf("generating single embedding: %w", err)
	}

	// Check for errors in the response
	if resp.Error != "" {
		return &resp, fmt.Errorf("model error: %s", resp.Error)
	}

	return &resp, nil
}

// BatchEmbeddings is a convenience method to generate embeddings for multiple texts
func (c *Client) BatchEmbeddings(ctx context.Context, reqs []EmbeddingRequest) ([]*EmbeddingResponse, error) {
	responses := make([]*EmbeddingResponse, len(reqs))

	// Process requests sequentially (Ollama doesn't support batching natively)
	for i, req := range reqs {
		resp, err := c.GenerateEmbedding(ctx, req)
		if err != nil {
			return responses, fmt.Errorf("batch embedding %d: %w", i, err)
		}
		responses[i] = resp
	}

	return responses, nil
}

// makeRequest is a helper method to make HTTP requests to the Ollama API
func (c *Client) makeRequest(ctx context.Context, method, path string, reqBody interface{}, respBody interface{}) error {
	url := fmt.Sprintf("%s%s", c.config.Endpoint, path)

	var bodyReader io.Reader
	if reqBody != nil {
		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshaling request body: %w", err)
		}
		bodyReader = bytes.NewBuffer(bodyBytes)

		loggy.Debug("Sending OLLAMA LLM request",
			"method", method,
			"url", url,
			"body", string(bodyBytes))
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	// Set headers
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	// Check for non-200 status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Check for empty response
	if len(bodyBytes) == 0 {
		return fmt.Errorf("empty response body")
	}

	// Unmarshal response
	if err := json.Unmarshal(bodyBytes, respBody); err != nil {
		// Try to extract JSON if the response might contain additional text
		if extractedJSON, extractErr := extractJSON(string(bodyBytes)); extractErr == nil {
			if unmarshalErr := json.Unmarshal([]byte(extractedJSON), respBody); unmarshalErr == nil {
				// Successfully extracted and unmarshaled JSON
				return nil
			}
		}

		// Return original error if extraction failed
		return fmt.Errorf("unmarshaling response body: %w", err)
	}

	return nil
}

// extractJSON attempts to extract a JSON object from a string that might contain additional text
func extractJSON(input string) (string, error) {
	// Find the first occurrence of '{'
	start := strings.Index(input, "{")
	if start == -1 {
		return "", fmt.Errorf("no JSON object found")
	}

	// Find the matching closing brace
	braceCount := 1
	for i := start + 1; i < len(input); i++ {
		if input[i] == '{' {
			braceCount++
		} else if input[i] == '}' {
			braceCount--
			if braceCount == 0 {
				// Found the end of the JSON object
				return input[start : i+1], nil
			}
		}
	}

	return "", fmt.Errorf("incomplete JSON object")
}
