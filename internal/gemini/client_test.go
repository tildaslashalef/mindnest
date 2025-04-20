package gemini

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tildaslashalef/mindnest/internal/config"
)

func setupTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	server := httptest.NewServer(handler)

	geminiCfg := config.GeminiConfig{
		APIKey:           "test-key",
		BaseURL:          server.URL,
		Model:            "test-model",
		EmbeddingModel:   "test-embedding-model",
		APIVersion:       "v1",
		EmbeddingVersion: "v1",
		Timeout:          5 * time.Second,
		MaxRetries:       1,
		MaxTokens:        2048,
		Temperature:      0.7,
	}

	client := NewClient(geminiCfg)
	require.NotNil(t, client, "Client should not be nil")
	require.NotNil(t, client.httpMultiClient, "HTTP client should not be nil")
	return server, client
}

func ptr[T any](v T) *T {
	return &v
}

func TestNewClient(t *testing.T) {
	geminiCfg := config.GeminiConfig{
		APIKey:           "test-key",
		BaseURL:          "https://generativelanguage.googleapis.com",
		Model:            "gemini-2.5-pro",
		EmbeddingModel:   "text-embedding-004",
		APIVersion:       "v1beta",
		EmbeddingVersion: "v1",
		Timeout:          10 * time.Second,
		MaxRetries:       3,
		MaxTokens:        4096,
		Temperature:      0.8,
		TopP:             0.95,
		TopK:             40,
	}

	client := NewClient(geminiCfg)

	assert.NotNil(t, client, "Client should not be nil")
	assert.Equal(t, "test-key", client.apiKey, "API key should match")
	assert.Equal(t, "https://generativelanguage.googleapis.com", client.baseURL, "Base URL should match")
	assert.Equal(t, "gemini-2.5-pro", client.defaultModel, "Default model should match")
	assert.Equal(t, "text-embedding-004", client.embeddingModel, "Embedding model should match")
	assert.Equal(t, "v1beta", client.apiVersion, "API version should match")
	assert.Equal(t, "v1", client.embeddingVersion, "Embedding version should match")
	assert.Equal(t, 3, client.maxRetries, "Max retries should match")
	assert.Equal(t, 4096, client.defaultMaxTokens, "Default max tokens should match")
	assert.NotNil(t, client.temperature, "Temperature pointer should not be nil")
	assert.Equal(t, 0.8, *client.temperature, "Temperature should match")
	assert.NotNil(t, client.topP, "TopP pointer should not be nil")
	assert.Equal(t, 0.95, *client.topP, "Top P should match")
	assert.NotNil(t, client.topK, "TopK pointer should not be nil")
	assert.Equal(t, 40, *client.topK, "Top K should match")
	assert.NotNil(t, client.httpMultiClient, "HTTP Client should not be nil")
	assert.Equal(t, 10*time.Second, client.httpMultiClient.Timeout, "Timeout should match")
}

func TestGenerateChat(t *testing.T) {
	expectedRequest := ChatRequest{
		Model:    "test-model",
		Contents: []Content{{Role: "user", Parts: []Part{{Text: "Hello, world!"}}}},
		GenerationConfig: &GenerationConfig{
			MaxOutputTokens: 2048,
			Temperature:     ptr(0.7),
		},
		Stream: false, // Should be explicitly set to false
	}

	expectedResponse := ChatResponse{
		Candidates: []Candidate{
			{
				Content: Content{
					Role:  "model",
					Parts: []Part{{Text: "Hello! How can I help you today?"}},
				},
				FinishReason: "STOP",
			},
		},
	}

	server, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/models/test-model:generateContent", r.URL.Path, "Unexpected request path")
		assert.Equal(t, "POST", r.Method, "Unexpected HTTP method")
		assert.Contains(t, r.URL.RawQuery, "key=test-key", "API key should be in query params")

		var req ChatRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err, "Should decode request body without error")
		assert.Equal(t, expectedRequest.Model, req.Model, "Model should match")
		assert.Equal(t, expectedRequest.Contents, req.Contents, "Contents should match")
		assert.Equal(t, expectedRequest.GenerationConfig.MaxOutputTokens, req.GenerationConfig.MaxOutputTokens, "MaxOutputTokens should match")
		assert.Equal(t, *expectedRequest.GenerationConfig.Temperature, *req.GenerationConfig.Temperature, "Temperature should match")
		assert.Equal(t, expectedRequest.Stream, req.Stream, "Stream should match")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedResponse)
	})
	defer server.Close()

	resp, err := client.GenerateChat(context.Background(), ChatRequest{
		Model:    "test-model",
		Contents: []Content{{Role: "user", Parts: []Part{{Text: "Hello, world!"}}}},
	})

	assert.NoError(t, err, "GenerateChat should not return an error")
	assert.NotNil(t, resp, "Response should not be nil")
	assert.Equal(t, expectedResponse.Candidates, resp.Candidates, "Response candidates should match")
	assert.Equal(t, "Hello! How can I help you today?", resp.Candidates[0].Content.Parts[0].Text, "Response text should match")
}

func TestGenerateEmbedding(t *testing.T) {
	expectedRequest := map[string]interface{}{
		"model": "test-embedding-model",
		"content": map[string]interface{}{
			"parts": []map[string]interface{}{
				{
					"text": "Hello, world!",
				},
			},
		},
	}

	expectedResponse := struct {
		Embedding struct {
			Values []float32 `json:"values"`
		} `json:"embedding"`
	}{
		Embedding: struct {
			Values []float32 `json:"values"`
		}{
			Values: []float32{0.1, 0.2, 0.3, 0.4, 0.5},
		},
	}

	server, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/models/test-embedding-model:embedContent", r.URL.Path, "Unexpected request path")
		assert.Equal(t, "POST", r.Method, "Unexpected HTTP method")
		assert.Contains(t, r.URL.RawQuery, "key=test-key", "API key should be in query params")

		var req map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err, "Should decode request body without error")
		assert.Equal(t, expectedRequest["model"], req["model"], "Model should match")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedResponse)
	})
	defer server.Close()

	resp, err := client.GenerateEmbedding(context.Background(), EmbeddingRequest{
		Text: "Hello, world!",
	})

	assert.NoError(t, err, "GenerateEmbedding should not return an error")
	assert.NotNil(t, resp, "Response should not be nil")
	assert.Equal(t, expectedResponse.Embedding.Values, resp.Embedding, "Embedding should match")
}

func TestBatchEmbeddings(t *testing.T) {
	texts := []string{"Hello", "World"}

	expectedResponses := []struct {
		Embedding struct {
			Values []float32 `json:"values"`
		} `json:"embedding"`
	}{
		{
			Embedding: struct {
				Values []float32 `json:"values"`
			}{
				Values: []float32{0.1, 0.2, 0.3},
			},
		},
		{
			Embedding: struct {
				Values []float32 `json:"values"`
			}{
				Values: []float32{0.4, 0.5, 0.6},
			},
		},
	}

	callCount := 0
	server, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/models/test-embedding-model:embedContent", r.URL.Path, "Unexpected request path")
		assert.Equal(t, "POST", r.Method, "Unexpected HTTP method")

		var req map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err, "Should decode request body without error")

		content := req["content"].(map[string]interface{})
		parts := content["parts"].([]interface{})
		text := parts[0].(map[string]interface{})["text"].(string)

		// Check which text we're processing
		responseIndex := -1
		for i, t := range texts {
			if t == text {
				responseIndex = i
				break
			}
		}

		assert.GreaterOrEqual(t, responseIndex, 0, "Should find matching text")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedResponses[responseIndex])
		callCount++
	})
	defer server.Close()

	requests := []EmbeddingRequest{
		{Text: "Hello"},
		{Text: "World"},
	}

	responses, err := client.BatchEmbeddings(context.Background(), requests)

	assert.NoError(t, err, "BatchEmbeddings should not return an error")
	assert.Equal(t, 2, callCount, "Should make two API calls")
	assert.Len(t, responses, 2, "Should return two responses")
	assert.Equal(t, expectedResponses[0].Embedding.Values, responses[0].Embedding, "First embedding should match")
	assert.Equal(t, expectedResponses[1].Embedding.Values, responses[1].Embedding, "Second embedding should match")
}

func TestErrorHandling(t *testing.T) {
	// Test server error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIError{
			ErrorDetail: &ErrorDetails{
				Code:    400,
				Message: "Invalid request",
				Status:  "INVALID_ARGUMENT",
			},
		})
	}))
	defer server.Close()

	geminiCfg := config.GeminiConfig{
		APIKey:         "test-key",
		BaseURL:        server.URL,
		Model:          "test-model",
		EmbeddingModel: "test-embedding-model",
		Timeout:        5 * time.Second,
		MaxRetries:     1,
	}

	client := NewClient(geminiCfg)

	_, err := client.GenerateChat(context.Background(), ChatRequest{
		Model:    "test-model",
		Contents: []Content{{Role: "user", Parts: []Part{{Text: "Hello"}}}},
	})

	assert.Error(t, err, "Should return an error on 400 response")
	assert.Contains(t, err.Error(), "Invalid request", "Error should contain the message from the API")
}
