package llm

import (
	"context"
	"fmt"

	"golang.org/x/time/rate" // Import rate limiter

	"github.com/tildaslashalef/mindnest/internal/config"
	"github.com/tildaslashalef/mindnest/internal/ollama"
)

// ollamaClientAdapter adapts the Ollama client to the LLM Client interface
type ollamaClientAdapter struct {
	client  *ollama.Client
	config  *config.Config
	limiter *rate.Limiter // Added rate limiter
}

// newOllamaClientAdapter creates a new Ollama client adapter
// Updated to accept limiter
func newOllamaClientAdapter(client *ollama.Client, cfg *config.Config, limiter *rate.Limiter) *ollamaClientAdapter {
	return &ollamaClientAdapter{
		client:  client,
		config:  cfg,
		limiter: limiter, // Store limiter
	}
}

// GenerateChat implements the Client interface for Ollama
func (a *ollamaClientAdapter) GenerateChat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// Wait for rate limiter
	if err := a.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter error: %w", err)
	}

	// Create base request
	ollamaReq := ollama.ChatRequest{
		Model:    req.Model,
		Stream:   req.Stream,
		Messages: convertMessagesToOllama(req.Messages),
	}

	// If stream is not explicitly set, default to false
	if !req.Stream {
		ollamaReq.Stream = false
	}

	// Set options
	options := &ollama.RequestOptions{}
	if req.MaxTokens > 0 {
		numPredict := req.MaxTokens
		options.NumPredict = &numPredict
	}
	if req.Temperature > 0 {
		temp := req.Temperature
		options.Temperature = &temp
	}

	// Add any additional options from the request
	if req.Options != nil {
		for k, v := range req.Options {
			switch k {
			case "top_p":
				if val, ok := v.(float64); ok {
					options.TopP = &val
				}
			case "top_k":
				if val, ok := v.(int); ok {
					options.TopK = &val
				}
			case "seed":
				if val, ok := v.(int); ok {
					options.Seed = &val
				}
			case "num_ctx":
				if val, ok := v.(int); ok {
					options.NumCtx = &val
				}
			case "repeat_penalty":
				if val, ok := v.(float64); ok {
					options.RepeatPenalty = &val
				}
			case "stop":
				if val, ok := v.([]string); ok {
					options.Stop = val
				}
			}
		}
	}

	// Only set options if we have any non-nil values
	if options.Temperature != nil || options.TopP != nil || options.TopK != nil ||
		options.NumPredict != nil || options.Seed != nil || options.NumCtx != nil ||
		options.RepeatPenalty != nil || len(options.Stop) > 0 {
		ollamaReq.Options = options
	}

	// Make the request
	resp, err := a.client.GenerateChat(ctx, ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("ollama chat generation failed: %w", err)
	}

	// Convert to ChatResponse
	return &ChatResponse{
		Content:   resp.Message.Content,
		Model:     resp.Model,
		Completed: resp.Done,
		Error:     resp.Error,
	}, nil
}

// GenerateChatStream implements the Client interface for Ollama
func (a *ollamaClientAdapter) GenerateChatStream(ctx context.Context, req ChatRequest) (<-chan ChatResponse, error) {
	// Wait for rate limiter BEFORE starting the goroutine
	if err := a.limiter.Wait(ctx); err != nil {
		// Create and close an empty channel to fulfill the return type on error
		responseChan := make(chan ChatResponse)
		close(responseChan)
		return responseChan, fmt.Errorf("rate limiter error: %w", err)
	}

	// Convert to Ollama request
	ollamaReq := ollama.ChatRequest{
		Model:    req.Model,
		Stream:   true,
		Messages: convertMessagesToOllama(req.Messages),
	}

	// Set options if provided
	if req.MaxTokens > 0 {
		numPredict := req.MaxTokens
		ollamaReq.Options = &ollama.RequestOptions{
			NumPredict: &numPredict,
		}
	}
	if req.Temperature > 0 {
		temp := req.Temperature
		if ollamaReq.Options == nil {
			ollamaReq.Options = &ollama.RequestOptions{
				Temperature: &temp,
			}
		} else {
			ollamaReq.Options.Temperature = &temp
		}
	}

	// Add any additional options that were provided
	if len(req.Options) > 0 {
		if ollamaReq.Options == nil {
			ollamaReq.Options = &ollama.RequestOptions{}
		}

		for k, v := range req.Options {
			switch k {
			case "top_p":
				if val, ok := v.(float64); ok {
					ollamaReq.Options.TopP = &val
				}
			case "top_k":
				if val, ok := v.(int); ok {
					ollamaReq.Options.TopK = &val
				}
			case "seed":
				if val, ok := v.(int); ok {
					ollamaReq.Options.Seed = &val
				}
			case "num_ctx":
				if val, ok := v.(int); ok {
					ollamaReq.Options.NumCtx = &val
				}
			case "repeat_penalty":
				if val, ok := v.(float64); ok {
					ollamaReq.Options.RepeatPenalty = &val
				}
			case "stop":
				if val, ok := v.([]string); ok {
					ollamaReq.Options.Stop = val
				}
			}
		}
	}

	// Get the stream channel
	ollamaRespChan, err := a.client.GenerateChatStream(ctx, ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("ollama stream generation failed: %w", err)
	}

	// Create output channel
	responseChan := make(chan ChatResponse)

	// Process stream in goroutine
	go func() {
		defer close(responseChan)

		for ollamaResp := range ollamaRespChan {
			responseChan <- ChatResponse{
				Content:   ollamaResp.Message.Content,
				Model:     ollamaResp.Model,
				Completed: ollamaResp.Done,
				Error:     ollamaResp.Error,
			}
		}
	}()

	return responseChan, nil
}

// GenerateEmbedding implements the Client interface for Ollama
func (a *ollamaClientAdapter) GenerateEmbedding(ctx context.Context, req EmbeddingRequest) ([]float32, error) {
	// Wait for rate limiter
	if err := a.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter error: %w", err)
	}

	ollamaReq := ollama.EmbeddingRequest{
		// Always use the embedding model from config
		Model: a.config.Ollama.EmbeddingModel,
		Input: req.Text,
	}

	resp, err := a.client.GenerateEmbedding(ctx, ollamaReq)
	if err != nil {
		// Try legacy endpoint as fallback
		legacyReq := ollama.SingleEmbeddingRequest{
			Model:  a.config.Ollama.EmbeddingModel,
			Prompt: req.Text,
		}

		legacyResp, legacyErr := a.client.GenerateSingleEmbedding(ctx, legacyReq)
		if legacyErr != nil {
			return nil, fmt.Errorf("ollama embedding generation failed: %w", err)
		}

		return legacyResp.Embedding, nil
	}

	// For the new /api/embed endpoint, we expect an array of embeddings
	if len(resp.Embeddings) > 0 {
		return resp.Embeddings[0], nil
	}

	return nil, fmt.Errorf("empty embedding response")
}

// BatchEmbeddings implements the Client interface for Ollama
func (a *ollamaClientAdapter) BatchEmbeddings(ctx context.Context, reqs []EmbeddingRequest) ([][]float32, error) {
	// Important: Apply rate limit *before* calling the underlying BatchEmbeddings,
	// which might make multiple internal calls or one large call.
	// If the underlying client makes sequential calls, we might want to limit *inside* the loop instead.
	// Assuming a.client.BatchEmbeddings makes one logical API call (even if implemented sequentially client-side).
	if err := a.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter error: %w", err)
	}

	ollamaReqs := make([]ollama.EmbeddingRequest, len(reqs))
	for i, req := range reqs {
		ollamaReqs[i] = ollama.EmbeddingRequest{
			// Always use the embedding model from config
			Model: a.config.Ollama.EmbeddingModel,
			Input: req.Text,
		}
	}

	resps, err := a.client.BatchEmbeddings(ctx, ollamaReqs)
	if err != nil {
		return nil, fmt.Errorf("ollama batch embedding generation failed: %w", err)
	}

	embeddings := make([][]float32, len(resps))
	for i, resp := range resps {
		if len(resp.Embeddings) > 0 {
			embeddings[i] = resp.Embeddings[0]
		} else {
			embeddings[i] = []float32{}
		}
	}

	return embeddings, nil
}

// GenerateCompletion implements the Client interface for Ollama
func (a *ollamaClientAdapter) GenerateCompletion(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	// Wait for rate limiter
	if err := a.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter error: %w", err)
	}

	// Convert to Ollama request
	ollamaReq := ollama.GenerateRequest{
		Model:  req.Model,
		Prompt: req.Prompt,
		System: req.System,
		Stream: false, // Explicitly set to false for non-streaming requests
	}

	// Set options if provided
	if req.MaxTokens > 0 {
		numPredict := req.MaxTokens
		ollamaReq.Options = &ollama.RequestOptions{
			NumPredict: &numPredict,
		}
	}
	if req.Temperature > 0 {
		temp := req.Temperature
		if ollamaReq.Options == nil {
			ollamaReq.Options = &ollama.RequestOptions{
				Temperature: &temp,
			}
		} else {
			ollamaReq.Options.Temperature = &temp
		}
	}

	// Add any additional options from the request
	if req.Options != nil {
		if ollamaReq.Options == nil {
			ollamaReq.Options = &ollama.RequestOptions{}
		}

		for k, v := range req.Options {
			switch k {
			case "top_p":
				if val, ok := v.(float64); ok {
					ollamaReq.Options.TopP = &val
				}
			case "top_k":
				if val, ok := v.(int); ok {
					ollamaReq.Options.TopK = &val
				}
			case "seed":
				if val, ok := v.(int); ok {
					ollamaReq.Options.Seed = &val
				}
			case "num_ctx":
				if val, ok := v.(int); ok {
					ollamaReq.Options.NumCtx = &val
				}
			case "repeat_penalty":
				if val, ok := v.(float64); ok {
					ollamaReq.Options.RepeatPenalty = &val
				}
			case "stop":
				if val, ok := v.([]string); ok {
					ollamaReq.Options.Stop = val
				}
			}
		}
	}

	// Make the request
	resp, err := a.client.GenerateCompletion(ctx, ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("ollama completion generation failed: %w", err)
	}

	// Convert to GenerateResponse
	return &GenerateResponse{
		Content:   resp.Response,
		Model:     resp.Model,
		Completed: resp.Done,
		Error:     resp.Error,
	}, nil
}

// Helper to convert message format
func convertMessagesToOllama(messages []Message) []ollama.Message {
	ollamaMessages := make([]ollama.Message, len(messages))
	for i, msg := range messages {
		ollamaMessages[i] = ollama.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}
	return ollamaMessages
}
