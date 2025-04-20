package review

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/tildaslashalef/mindnest/internal/config"
	"github.com/tildaslashalef/mindnest/internal/extractor"
	"github.com/tildaslashalef/mindnest/internal/llm"
	"github.com/tildaslashalef/mindnest/internal/loggy"
	"github.com/tildaslashalef/mindnest/internal/rag"
	"github.com/tildaslashalef/mindnest/internal/ulid"
	"github.com/tildaslashalef/mindnest/internal/workspace"
)

// Service provides code review functionality
type Service struct {
	repo             Repository
	workspaceService *workspace.Service
	ragService       *rag.Service
	llmClient        llm.Client
	config           *config.Config
	logger           *loggy.Logger
}

// NewService creates a new review service
func NewService(
	db *sql.DB,
	workspaceService *workspace.Service,
	ragService *rag.Service,
	llmClient llm.Client,
	config *config.Config,
	logger *loggy.Logger,
) *Service {
	repo := NewSQLRepository(db, logger)

	return &Service{
		repo:             repo,
		workspaceService: workspaceService,
		ragService:       ragService,
		llmClient:        llmClient,
		config:           config,
		logger:           logger,
	}
}

// CreateReview starts a new code review
func (s *Service) CreateReview(ctx context.Context, workspaceID string, reviewType ReviewType, commitHash, branchFrom, branchTo string) (*Review, error) {
	// Create a new review
	review := NewReview(workspaceID, reviewType)
	review.ID = ulid.ReviewID()

	// Set commit hash and branch information if provided
	review.CommitHash = commitHash
	review.BranchFrom = branchFrom
	review.BranchTo = branchTo

	// Save the review to the database
	if err := s.repo.CreateReview(ctx, review); err != nil {
		return nil, fmt.Errorf("creating review: %w", err)
	}

	return review, nil
}

// GetReview retrieves a review by ID
func (s *Service) GetReview(ctx context.Context, id string) (*Review, error) {
	return s.repo.GetReview(ctx, id)
}

// GetReviewFilesByReview retrieves all review files by review ID
func (s *Service) GetReviewFilesByReview(ctx context.Context, reviewID string) ([]*ReviewFile, error) {
	return s.repo.GetReviewFilesByReview(ctx, reviewID)
}

// GetReviewFile retrieves a review file by ID
func (s *Service) GetReviewFile(ctx context.Context, id string) (*ReviewFile, error) {
	return s.repo.GetReviewFile(ctx, id)
}

// GetIssue retrieves an issue by ID
func (s *Service) GetIssue(ctx context.Context, id string) (*Issue, error) {
	return s.repo.GetIssue(ctx, id)
}

// GetIssuesByReviewFile retrieves all issues by review file ID
func (s *Service) GetIssuesByReviewFile(ctx context.Context, reviewFileID string) ([]*Issue, error) {
	return s.repo.GetIssuesByReviewFile(ctx, reviewFileID)
}

// MarkIssueAsValid marks an issue as valid
func (s *Service) MarkIssueAsValid(ctx context.Context, issueID string) error {
	return s.repo.MarkIssueAsValid(ctx, issueID, true)
}

// MarkIssueAsInvalid marks an issue as invalid
func (s *Service) MarkIssueAsInvalid(ctx context.Context, issueID string) error {
	return s.repo.MarkIssueAsValid(ctx, issueID, false)
}

// ReviewFile performs a code review on a single file
func (s *Service) ReviewFile(ctx context.Context, reviewID string, file *workspace.File, content string, diffInfo string) (*ReviewFile, error) {
	// Create a new review file
	reviewFile := NewReviewFile(reviewID, file.ID)
	reviewFile.ID = ulid.ReviewID()

	// Save the review file to the database
	if err := s.repo.CreateReviewFile(ctx, reviewFile); err != nil {
		return nil, fmt.Errorf("creating review file: %w", err)
	}

	// Get similar chunks for context
	similarChunks, err := s.findSimilarChunks(ctx, file, content)
	if err != nil {
		s.logger.Warn("Error finding similar chunks", "error", err)
		// Continue anyway, as we can still perform the review without similar chunks
	}

	fileChunks, err := s.workspaceService.GetChunksByFile(ctx, file.ID)
	if err != nil {
		s.logger.Warn("Error getting file chunks", "error", err)
	}

	reviewFile.Metadata = make(map[string]any)
	reviewFile.Metadata["file_chunk_ids"] = s.workspaceService.GetChunkIDs(ctx, fileChunks)
	reviewFile.Metadata["similar_chunk_ids"] = s.workspaceService.GetChunkIDs(ctx, similarChunks)
	reviewFile.Metadata["file_chunks_count"] = len(fileChunks)
	reviewFile.Metadata["similar_chunks_count"] = len(similarChunks)

	// Update the review file metadata
	if err := s.repo.UpdateReviewFileMetadata(ctx, reviewFile); err != nil {
		s.logger.Warn("Error updating review file metadata", "error", err)
	}

	// Build prompt options based on file language
	promptOptions := DefaultPromptOptions()
	promptOptions.Language = file.Language

	// Build the review prompt based on the LLM provider
	var messages []map[string]string

	// Use different message builders for different providers
	if s.config.DefaultLLMProvider == "gemini" {
		// Use Gemini-specific message list
		messages, err = BuildGeminiMessageList(file, content, diffInfo, similarChunks, promptOptions)
	} else if s.config.DefaultLLMProvider == "ollama" {
		// Use Ollama-specific message list
		messages, err = BuildOllamaMessageList(file, content, diffInfo, similarChunks, promptOptions)
	} else {
		// Use standard message list for Claude and others
		messages, err = BuildMessageList(file, content, diffInfo, similarChunks, promptOptions)
	}

	if err != nil {
		// Mark review file as failed
		reviewFile.MarkFileFailed()
		if err := s.repo.UpdateReviewFile(ctx, reviewFile); err != nil {
			s.logger.Error("Error updating review file status", "error", err)
		}
		return nil, fmt.Errorf("building review prompt: %w", err)
	}

	// Convert messages to LLM format
	llmMessages := make([]llm.Message, 0, len(messages))
	for _, msg := range messages {
		llmMessages = append(llmMessages, llm.Message{
			Role:    msg["role"],
			Content: msg["content"],
		})
	}

	// Call the LLM
	chatReq := llm.ChatRequest{
		Stream: false,
	}

	// Set model and parameters based on the provider
	switch s.config.DefaultLLMProvider {
	case "ollama":
		chatReq.Model = s.config.Ollama.Model
		chatReq.Temperature = s.config.Ollama.Temperature
		chatReq.MaxTokens = s.config.Ollama.MaxTokens
	case "claude":
		chatReq.Model = s.config.Claude.Model
		chatReq.Temperature = s.config.Claude.Temperature
		chatReq.MaxTokens = s.config.Claude.MaxTokens
	case "gemini":
		chatReq.Model = s.config.Gemini.Model
		chatReq.Temperature = s.config.Gemini.Temperature
		chatReq.MaxTokens = s.config.Gemini.MaxTokens
	default:
		// Set reasonable defaults
		chatReq.Model = "claude-3-7-sonnet-20250219" // Default model as a fallback
		chatReq.Temperature = 0.1
		chatReq.MaxTokens = 4096
	}

	// Set the messages
	chatReq.Messages = llmMessages

	// Make the request to the LLM
	response, err := s.llmClient.GenerateChat(ctx, chatReq)
	if err != nil {
		// Mark review file as failed
		reviewFile.MarkFileFailed()
		if err := s.repo.UpdateReviewFile(ctx, reviewFile); err != nil {
			s.logger.Error("Error updating review file status", "error", err)
		}
		return nil, fmt.Errorf("generating chat: %w", err)
	}

	jsonExtractor := extractor.NewJSONExtractor(s.logger)
	// Extract the JSON from the response
	llmResponse, err := jsonExtractor.ExtractLLMReviewOutput(response.Content)
	if err != nil {
		s.logger.Warn("Failed to extract LLM response as valid JSON",
			"error", err,
			"model", chatReq.Model,
			"response_length", len(response.Content))
	}

	// If we successfully extracted the LLM response, map it to our models
	if llmResponse != nil {
		// Set summary and assessment on the review file
		if llmResponse.Summary != "" {
			reviewFile.SetSummary(llmResponse.Summary)
		}

		if llmResponse.OverallAssessment != "" {
			reviewFile.SetAssessment(llmResponse.OverallAssessment)
		}

		// Create issues from the LLM response using our utility function
		issues := CreateIssuesFromLLMOutput(llmResponse, reviewID, file.ID)

		// Add issues to the review file
		for _, issue := range issues {
			// Save the issue to the database
			if err := s.repo.CreateIssue(ctx, issue); err != nil {
				s.logger.Error("Failed to save issue to database",
					"error", err,
					"issue_id", issue.ID,
					"file_id", issue.FileID,
					"review_id", issue.ReviewID)
				continue
			}

			reviewFile.AddIssue(issue)
		}
	}

	// Mark review file as completed
	reviewFile.MarkCompleted()
	if err := s.repo.UpdateReviewFile(ctx, reviewFile); err != nil {
		return nil, fmt.Errorf("updating review file: %w", err)
	}

	return reviewFile, nil
}

// ReviewFiles reviews multiple files concurrently
func (s *Service) ReviewFiles(ctx context.Context, reviewID string, files []*workspace.File, contents map[string]string, diffInfos map[string]string) ([]*ReviewFile, error) {
	var reviewFiles []*ReviewFile
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, file := range files {
		wg.Add(1)

		go func(f *workspace.File) {
			defer wg.Done()

			content := contents[f.ID]
			diffInfo := diffInfos[f.ID]

			reviewFile, err := s.ReviewFile(ctx, reviewID, f, content, diffInfo)
			if err != nil {
				s.logger.Warn("Error reviewing file", "file_id", f.ID, "error", err)
				// Continue with other files, don't add to results
				return
			}

			// Lock the mutex before appending to the shared slice
			mu.Lock()
			reviewFiles = append(reviewFiles, reviewFile)
			mu.Unlock()
		}(file) // Pass the file as an argument to the goroutine
	}

	// Wait for all goroutines to complete
	wg.Wait()

	return reviewFiles, nil
}

// CompleteReview completes a review by summarizing the findings
func (s *Service) CompleteReview(ctx context.Context, reviewID string) (*Review, error) {
	// Get the review
	review, err := s.repo.GetReview(ctx, reviewID)
	if err != nil {
		return nil, fmt.Errorf("getting review: %w", err)
	}

	// Get review files
	reviewFiles, err := s.repo.GetReviewFilesByReview(ctx, reviewID)
	if err != nil {
		return nil, fmt.Errorf("getting review files: %w", err)
	}

	// If no review files, mark as failed
	if len(reviewFiles) == 0 {
		review.MarkFailed()
		if err := s.repo.UpdateReview(ctx, review); err != nil {
			return nil, fmt.Errorf("updating review: %w", err)
		}
		return review, nil
	}

	// Create a result summary
	modelUsed := ""
	if len(reviewFiles) > 0 {
		// Use the model that was used in the review
		switch s.config.DefaultLLMProvider {
		case "ollama":
			modelUsed = s.config.Ollama.Model
		case "claude":
			modelUsed = s.config.Claude.Model
		case "gemini":
			modelUsed = s.config.Gemini.Model
		default:
			modelUsed = "claude-3-7-sonnet-20250219" // Default as fallback
		}
	}

	result := ReviewResult{
		Summary:         "Code review completed",
		TotalIssues:     0,
		ByType:          make(map[string]int),
		BySeverity:      make(map[string]int),
		ExecutionTime:   float64(time.Since(review.CreatedAt).Milliseconds()) / 1000.0,
		ProcessedFiles:  len(reviewFiles),
		ProcessedChunks: 0,
		Model:           modelUsed,
	}

	// Process all review files
	for _, rf := range reviewFiles {
		result.TotalIssues += rf.IssuesCount

		// Get issues for this file
		issues, err := s.repo.GetIssuesByReviewFile(ctx, rf.ID)
		if err != nil {
			s.logger.Warn("Error getting issues for review file", "review_file_id", rf.ID, "error", err)
			continue
		}

		// Tally up issues by type and severity
		for _, issue := range issues {
			result.ByType[string(issue.Type)]++
			result.BySeverity[string(issue.Severity)]++
		}
	}

	// Create a more detailed summary
	if result.TotalIssues == 0 {
		result.Summary = "No issues found in the code review."
	} else {
		result.Summary = fmt.Sprintf("Found %d issues in the code review.", result.TotalIssues)
	}

	// Set the result on the review
	review.SetResult(result)
	review.MarkCompleted()

	// Update the review
	if err := s.repo.UpdateReview(ctx, review); err != nil {
		return nil, fmt.Errorf("updating review: %w", err)
	}

	return review, nil
}

// findSimilarChunks finds chunks similar to the content
func (s *Service) findSimilarChunks(ctx context.Context, file *workspace.File, content string) ([]*workspace.Chunk, error) {
	// Create search options with config defaults
	opts := rag.NewSearchOptions().
		WithConfigDefaults(s.config).
		WithWorkspace(file.WorkspaceID).
		WithChunkType(workspace.ChunkTypeFile)
	// Exclude the current file
	if file.ID != "" {
		opts.WithExcludeFile(file.ID)
	}

	// Use the optimized vector store search
	scoredChunks, err := s.ragService.FindSimilarUsingStore(ctx, content, opts)
	if err != nil {
		return nil, fmt.Errorf("finding similar chunks: %w", err)
	}

	// Convert scored chunks to workspace chunks
	chunks := make([]*workspace.Chunk, 0, len(scoredChunks))
	for _, sc := range scoredChunks {
		chunks = append(chunks, sc.Chunk)
	}

	s.logger.Debug("Finding similar chunks", "file_id", file.ID, "workspace_id", file.WorkspaceID, "chunks", len(chunks))
	return chunks, nil
}
