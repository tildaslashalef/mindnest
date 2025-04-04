package ollama

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

// setupTestServer creates a test HTTP server that simulates the Ollama API
func setupTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	server := httptest.NewServer(handler)

	cfg := config.OllamaConfig{
		Endpoint:            server.URL,
		Timeout:             5 * time.Second,
		MaxRetries:          1,
		DefaultModel:        "test-model",
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     30 * time.Second,
	}

	client := NewClient(cfg)
	return server, client
}

// assertModelsEqual compares ModelInfo slices ignoring the exact time values
func assertModelsEqual(t *testing.T, expected, actual []ModelInfo) {
	assert.Equal(t, len(expected), len(actual), "Models length should match")
	for i, expectedModel := range expected {
		actualModel := actual[i]
		assert.Equal(t, expectedModel.Name, actualModel.Name, "Model name should match")
		assert.Equal(t, expectedModel.Size, actualModel.Size, "Model size should match")
		assert.Equal(t, expectedModel.Digest, actualModel.Digest, "Model digest should match")
		assert.Equal(t, expectedModel.Details.Family, actualModel.Details.Family, "Model family should match")
		assert.Equal(t, expectedModel.Details.ParameterSize, actualModel.Details.ParameterSize, "Model parameter size should match")
		// Not comparing ModifiedAt as it's a time.Time value
	}
}

// assertRunningModelsEqual compares RunningModelInfo slices ignoring the exact time values
func assertRunningModelsEqual(t *testing.T, expected, actual []RunningModelInfo) {
	assert.Equal(t, len(expected), len(actual), "Running models length should match")
	for i, expectedModel := range expected {
		actualModel := actual[i]
		assert.Equal(t, expectedModel.Name, actualModel.Name, "Model name should match")
		assert.Equal(t, expectedModel.Model, actualModel.Model, "Model field should match")
		assert.Equal(t, expectedModel.Size, actualModel.Size, "Model size should match")
		assert.Equal(t, expectedModel.Details.Family, actualModel.Details.Family, "Model family should match")
		assert.Equal(t, expectedModel.Details.ParameterSize, actualModel.Details.ParameterSize, "Model parameter size should match")
		// Not comparing ExpiresAt as it's a time.Time value
	}
}

// assertChatResponseEqual compares ChatResponse objects ignoring the exact time values
func assertChatResponseEqual(t *testing.T, expected, actual *ChatResponse) {
	assert.Equal(t, expected.Model, actual.Model, "Model name should match")
	assert.Equal(t, expected.Message, actual.Message, "Message should match")
	assert.Equal(t, expected.Done, actual.Done, "Done flag should match")
	assert.Equal(t, expected.TotalDuration, actual.TotalDuration, "TotalDuration should match")
	assert.Equal(t, expected.LoadDuration, actual.LoadDuration, "LoadDuration should match")
	assert.Equal(t, expected.PromptEvalCount, actual.PromptEvalCount, "PromptEvalCount should match")
	assert.Equal(t, expected.PromptEvalDuration, actual.PromptEvalDuration, "PromptEvalDuration should match")
	assert.Equal(t, expected.EvalCount, actual.EvalCount, "EvalCount should match")
	assert.Equal(t, expected.EvalDuration, actual.EvalDuration, "EvalDuration should match")
	assert.Equal(t, expected.Error, actual.Error, "Error should match")
	// Not comparing CreatedAt as it's a time.Time value
}

func TestNewClient(t *testing.T) {
	// Test with default config
	cfg := config.OllamaConfig{
		Endpoint:            "http://localhost:11434",
		Timeout:             5 * time.Minute,
		MaxRetries:          3,
		DefaultModel:        "test-model",
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
	}

	client := NewClient(cfg)

	assert.NotNil(t, client, "Client should not be nil")
	assert.Equal(t, cfg, client.config, "Client config should match provided config")
	assert.NotNil(t, client.httpClient, "HTTP client should not be nil")
	assert.Equal(t, cfg.Timeout, client.httpClient.Timeout, "HTTP client timeout should match config")

	// Check that transport config is properly set
	transport, ok := client.httpClient.Transport.(*http.Transport)
	require.True(t, ok, "HTTP client transport should be *http.Transport")
	assert.Equal(t, cfg.MaxIdleConns, transport.MaxIdleConns, "MaxIdleConns should match config")
	assert.Equal(t, cfg.MaxIdleConnsPerHost, transport.MaxIdleConnsPerHost, "MaxIdleConnsPerHost should match config")
	assert.Equal(t, cfg.IdleConnTimeout, transport.IdleConnTimeout, "IdleConnTimeout should match config")
}

func TestListModels(t *testing.T) {
	expectedResponse := ListModelsResponse{
		Models: []ModelInfo{
			{
				Name:       "test-model",
				Size:       1234567890,
				Digest:     "sha256:1234567890",
				ModifiedAt: time.Now(),
				Details: ModelDetails{
					Family:        "test-family",
					ParameterSize: "1B",
				},
			},
			{
				Name:       "another-model",
				Size:       9876543210,
				Digest:     "sha256:0987654321",
				ModifiedAt: time.Now(),
				Details: ModelDetails{
					Family:        "another-family",
					ParameterSize: "7B",
				},
			},
		},
	}

	server, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/tags", r.URL.Path, "Unexpected request path")
		assert.Equal(t, http.MethodGet, r.Method, "Unexpected HTTP method")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedResponse)
	})
	defer server.Close()

	models, err := client.ListModels(context.Background())

	assert.NoError(t, err, "ListModels should not return an error")
	assertModelsEqual(t, expectedResponse.Models, models)
}

func TestListRunningModels(t *testing.T) {
	expectedResponse := ListRunningModelsResponse{
		Models: []RunningModelInfo{
			{
				Name:      "test-model",
				Model:     "test-model",
				Size:      1234567890,
				ExpiresAt: time.Now().Add(time.Hour),
				Details: ModelDetails{
					Family:        "test-family",
					ParameterSize: "1B",
				},
			},
		},
	}

	server, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/ps", r.URL.Path, "Unexpected request path")
		assert.Equal(t, http.MethodGet, r.Method, "Unexpected HTTP method")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedResponse)
	})
	defer server.Close()

	models, err := client.ListRunningModels(context.Background())

	assert.NoError(t, err, "ListRunningModels should not return an error")
	assertRunningModelsEqual(t, expectedResponse.Models, models)
}

func TestGetVersion(t *testing.T) {
	expectedResponse := VersionResponse{
		Version: "0.1.14",
	}

	server, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/version", r.URL.Path, "Unexpected request path")
		assert.Equal(t, http.MethodGet, r.Method, "Unexpected HTTP method")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedResponse)
	})
	defer server.Close()

	version, err := client.GetVersion(context.Background())

	assert.NoError(t, err, "GetVersion should not return an error")
	assert.Equal(t, expectedResponse.Version, version, "Version should match expected response")
}

func TestGenerateChat(t *testing.T) {
	expectedRequest := ChatRequest{
		Model: "test-model",
		Messages: []Message{
			{
				Role:    "user",
				Content: "Hello, how are you?",
			},
		},
		Stream: false, // Should be explicitly set to false
	}

	created := time.Now()
	expectedResponse := ChatResponse{
		Model:              "test-model",
		CreatedAt:          created,
		Message:            Message{Role: "assistant", Content: "I'm doing well, thank you for asking!"},
		Done:               true,
		TotalDuration:      1234567890,
		LoadDuration:       123456789,
		PromptEvalCount:    10,
		PromptEvalDuration: 123456,
		EvalCount:          20,
		EvalDuration:       234567,
	}

	server, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/chat", r.URL.Path, "Unexpected request path")
		assert.Equal(t, http.MethodPost, r.Method, "Unexpected HTTP method")

		var req ChatRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err, "Should decode request body without error")
		assert.Equal(t, expectedRequest, req, "Request should match expected request")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedResponse)
	})
	defer server.Close()

	resp, err := client.GenerateChat(context.Background(), ChatRequest{
		Model: "test-model",
		Messages: []Message{
			{
				Role:    "user",
				Content: "Hello, how are you?",
			},
		},
	})

	assert.NoError(t, err, "GenerateChat should not return an error")
	assertChatResponseEqual(t, &expectedResponse, resp)
}

func TestGenerateChatStream(t *testing.T) {
	expectedRequest := ChatRequest{
		Model: "test-model",
		Messages: []Message{
			{
				Role:    "user",
				Content: "Hello, how are you?",
			},
		},
		Stream: true, // Should be explicitly set to true
	}

	created := time.Now()
	responseChunks := []ChatResponse{
		{
			Model:     "test-model",
			CreatedAt: created,
			Message:   Message{Role: "assistant", Content: "I'm"},
			Done:      false,
		},
		{
			Model:     "test-model",
			CreatedAt: created,
			Message:   Message{Role: "assistant", Content: " doing"},
			Done:      false,
		},
		{
			Model:     "test-model",
			CreatedAt: created,
			Message:   Message{Role: "assistant", Content: " well"},
			Done:      false,
		},
		{
			Model:              "test-model",
			CreatedAt:          created,
			Message:            Message{Role: "assistant", Content: "!"},
			Done:               true,
			TotalDuration:      1234567890,
			LoadDuration:       123456789,
			PromptEvalCount:    10,
			PromptEvalDuration: 123456,
			EvalCount:          20,
			EvalDuration:       234567,
		},
	}

	server, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/chat", r.URL.Path, "Unexpected request path")
		assert.Equal(t, http.MethodPost, r.Method, "Unexpected HTTP method")

		var req ChatRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err, "Should decode request body without error")
		assert.Equal(t, expectedRequest, req, "Request should match expected request")

		w.Header().Set("Content-Type", "application/json")

		// Stream the response chunks
		flusher, ok := w.(http.Flusher)
		assert.True(t, ok, "ResponseWriter should support flushing")

		for _, chunk := range responseChunks {
			json.NewEncoder(w).Encode(chunk)
			flusher.Flush()
		}
	})
	defer server.Close()

	respChan, err := client.GenerateChatStream(context.Background(), ChatRequest{
		Model: "test-model",
		Messages: []Message{
			{
				Role:    "user",
				Content: "Hello, how are you?",
			},
		},
	})

	assert.NoError(t, err, "GenerateChatStream should not return an error")

	var receivedChunks []ChatResponse
	for resp := range respChan {
		receivedChunks = append(receivedChunks, resp)
	}

	assert.Equal(t, len(responseChunks), len(receivedChunks), "Should receive expected number of chunks")
	for i, expected := range responseChunks {
		// Compare fields individually to skip time comparison
		assert.Equal(t, expected.Model, receivedChunks[i].Model, "Model should match")
		assert.Equal(t, expected.Message, receivedChunks[i].Message, "Message should match")
		assert.Equal(t, expected.Done, receivedChunks[i].Done, "Done flag should match")
		if expected.Done {
			assert.Equal(t, expected.TotalDuration, receivedChunks[i].TotalDuration, "TotalDuration should match")
			assert.Equal(t, expected.LoadDuration, receivedChunks[i].LoadDuration, "LoadDuration should match")
			assert.Equal(t, expected.PromptEvalCount, receivedChunks[i].PromptEvalCount, "PromptEvalCount should match")
			assert.Equal(t, expected.PromptEvalDuration, receivedChunks[i].PromptEvalDuration, "PromptEvalDuration should match")
			assert.Equal(t, expected.EvalCount, receivedChunks[i].EvalCount, "EvalCount should match")
			assert.Equal(t, expected.EvalDuration, receivedChunks[i].EvalDuration, "EvalDuration should match")
		}
	}
}

func TestGenerateCompletion(t *testing.T) {
	expectedRequest := GenerateRequest{
		Model:  "test-model",
		Prompt: "Once upon a time",
		Stream: false, // Should be explicitly set to false
	}

	expectedResponse := GenerateResponse{
		Model:              "test-model",
		Response:           ", there was a kingdom far, far away.",
		Done:               true,
		Context:            []int{1, 2, 3, 4, 5},
		TotalDuration:      1234567890,
		LoadDuration:       123456789,
		PromptEvalCount:    10,
		PromptEvalDuration: 123456,
		EvalCount:          20,
		EvalDuration:       234567,
	}

	server, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/generate", r.URL.Path, "Unexpected request path")
		assert.Equal(t, http.MethodPost, r.Method, "Unexpected HTTP method")

		var req GenerateRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err, "Should decode request body without error")
		assert.Equal(t, expectedRequest, req, "Request should match expected request")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedResponse)
	})
	defer server.Close()

	resp, err := client.GenerateCompletion(context.Background(), GenerateRequest{
		Model:  "test-model",
		Prompt: "Once upon a time",
	})

	assert.NoError(t, err, "GenerateCompletion should not return an error")
	assert.Equal(t, &expectedResponse, resp, "Response should match expected response")
}

func TestGenerateEmbedding(t *testing.T) {
	expectedRequest := EmbeddingRequest{
		Model: "test-model",
		Input: "This is a test sentence.",
	}

	expectedResponse := EmbeddingResponse{
		Model:      "test-model",
		Embeddings: [][]float32{{0.1, 0.2, 0.3, 0.4, 0.5}},
	}

	server, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/embed", r.URL.Path, "Unexpected request path")
		assert.Equal(t, http.MethodPost, r.Method, "Unexpected HTTP method")

		var req EmbeddingRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err, "Should decode request body without error")
		assert.Equal(t, expectedRequest, req, "Request should match expected request")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedResponse)
	})
	defer server.Close()

	resp, err := client.GenerateEmbedding(context.Background(), EmbeddingRequest{
		Model: "test-model",
		Input: "This is a test sentence.",
	})

	assert.NoError(t, err, "GenerateEmbedding should not return an error")
	assert.Equal(t, &expectedResponse, resp, "Response should match expected response")
}

func TestErrorHandling(t *testing.T) {
	// Test server error
	serverErrorTests := []struct {
		name       string
		statusCode int
		handler    func(t *testing.T, client *Client)
	}{
		{
			name:       "ListModels server error",
			statusCode: http.StatusInternalServerError,
			handler: func(t *testing.T, client *Client) {
				_, err := client.ListModels(context.Background())
				assert.Error(t, err, "ListModels should return an error for server error")
				assert.Contains(t, err.Error(), "500", "Error should contain the status code")
			},
		},
		{
			name:       "ListRunningModels server error",
			statusCode: http.StatusBadRequest,
			handler: func(t *testing.T, client *Client) {
				_, err := client.ListRunningModels(context.Background())
				assert.Error(t, err, "ListRunningModels should return an error for server error")
				assert.Contains(t, err.Error(), "400", "Error should contain the status code")
			},
		},
		{
			name:       "GetVersion server error",
			statusCode: http.StatusInternalServerError,
			handler: func(t *testing.T, client *Client) {
				_, err := client.GetVersion(context.Background())
				assert.Error(t, err, "GetVersion should return an error for server error")
				assert.Contains(t, err.Error(), "500", "Error should contain the status code")
			},
		},
		{
			name:       "GenerateChat server error",
			statusCode: http.StatusBadRequest,
			handler: func(t *testing.T, client *Client) {
				_, err := client.GenerateChat(context.Background(), ChatRequest{Model: "test-model"})
				assert.Error(t, err, "GenerateChat should return an error for server error")
				assert.Contains(t, err.Error(), "400", "Error should contain the status code")
			},
		},
		{
			name:       "GenerateCompletion server error",
			statusCode: http.StatusInternalServerError,
			handler: func(t *testing.T, client *Client) {
				_, err := client.GenerateCompletion(context.Background(), GenerateRequest{Model: "test-model"})
				assert.Error(t, err, "GenerateCompletion should return an error for server error")
				assert.Contains(t, err.Error(), "500", "Error should contain the status code")
			},
		},
		{
			name:       "GenerateEmbedding server error",
			statusCode: http.StatusBadRequest,
			handler: func(t *testing.T, client *Client) {
				_, err := client.GenerateEmbedding(context.Background(), EmbeddingRequest{Model: "test-model"})
				assert.Error(t, err, "GenerateEmbedding should return an error for server error")
				assert.Contains(t, err.Error(), "400", "Error should contain the status code")
			},
		},
	}

	for _, tt := range serverErrorTests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(`{"error": "test error"}`))
			}))
			defer server.Close()

			cfg := config.OllamaConfig{
				Endpoint:            server.URL,
				Timeout:             2 * time.Second,
				MaxRetries:          1,
				DefaultModel:        "test-model",
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     30 * time.Second,
			}

			client := NewClient(cfg)
			tt.handler(t, client)
		})
	}

	// Test context cancellation
	t.Run("Context cancellation", func(t *testing.T) {
		server, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			// Simulate a slow response
			time.Sleep(2 * time.Second)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{}`))
		})
		defer server.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err := client.ListModels(ctx)
		assert.Error(t, err, "ListModels should return an error when context is cancelled")
		assert.Contains(t, err.Error(), "context", "Error should mention context")
	})

	// Test model error in response
	t.Run("Model error in response", func(t *testing.T) {
		server, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"error": "model not found"}`))
		})
		defer server.Close()

		resp, err := client.GenerateChat(context.Background(), ChatRequest{Model: "non-existent-model"})
		assert.Error(t, err, "GenerateChat should return an error for model error in response")
		assert.NotNil(t, resp, "Response should not be nil even with error")
		assert.Contains(t, err.Error(), "model error", "Error should mention model error")
	})
}

func TestExtractJSON(t *testing.T) {
	testCases := []struct {
		name         string
		input        string
		expectedJSON string
		expectedErr  bool
	}{
		{
			name:         "Valid JSON object",
			input:        `{"key": "value"}`,
			expectedJSON: `{"key": "value"}`,
			expectedErr:  false,
		},
		{
			name:         "JSON object with surrounding text",
			input:        `Some text before {"key": "value"} and after`,
			expectedJSON: `{"key": "value"}`,
			expectedErr:  false,
		},
		{
			name:         "Nested JSON objects",
			input:        `{"outer": {"inner": "value"}}`,
			expectedJSON: `{"outer": {"inner": "value"}}`,
			expectedErr:  false,
		},
		{
			name:         "No JSON object",
			input:        `This is just plain text`,
			expectedJSON: "",
			expectedErr:  true,
		},
		{
			name:         "Incomplete JSON object",
			input:        `{"key": "value"`,
			expectedJSON: "",
			expectedErr:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			json, err := extractJSON(tc.input)

			if tc.expectedErr {
				assert.Error(t, err, "extractJSON should return an error for invalid input")
			} else {
				assert.NoError(t, err, "extractJSON should not return an error for valid input")
				assert.Equal(t, tc.expectedJSON, json, "Extracted JSON should match expected JSON")
			}
		})
	}
}

func TestBatchEmbeddings(t *testing.T) {
	testCases := []struct {
		name     string
		requests []EmbeddingRequest
		expected []*EmbeddingResponse
	}{
		{
			name: "Multiple embedding requests",
			requests: []EmbeddingRequest{
				{Model: "test-model", Input: "This is the first test."},
				{Model: "test-model", Input: "This is the second test."},
			},
			expected: []*EmbeddingResponse{
				{Model: "test-model", Embeddings: [][]float32{{0.1, 0.2, 0.3}}},
				{Model: "test-model", Embeddings: [][]float32{{0.4, 0.5, 0.6}}},
			},
		},
		{
			name:     "Empty requests",
			requests: []EmbeddingRequest{},
			expected: []*EmbeddingResponse{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/embed", r.URL.Path, "Unexpected request path")
				assert.Equal(t, http.MethodPost, r.Method, "Unexpected HTTP method")

				var req EmbeddingRequest
				err := json.NewDecoder(r.Body).Decode(&req)
				assert.NoError(t, err, "Should decode request body without error")

				// Find the corresponding response for this request
				for i, request := range tc.requests {
					if request.Input == req.Input {
						w.Header().Set("Content-Type", "application/json")
						json.NewEncoder(w).Encode(tc.expected[i])
						return
					}
				}

				// If we got here, no matching request was found
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error": "Unknown request"}`))
			})
			defer server.Close()

			responses, err := client.BatchEmbeddings(context.Background(), tc.requests)

			if len(tc.requests) == 0 {
				assert.NoError(t, err, "BatchEmbeddings should not return an error for empty requests")
				assert.Empty(t, responses, "Responses should be empty for empty requests")
			} else {
				assert.NoError(t, err, "BatchEmbeddings should not return an error")
				assert.Equal(t, len(tc.expected), len(responses), "Should receive expected number of responses")

				for i, expected := range tc.expected {
					assert.Equal(t, expected.Embeddings, responses[i].Embeddings, "Embeddings should match expected")
				}
			}
		})
	}
}
