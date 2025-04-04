package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/tildaslashalef/mindnest/internal/config"
	"github.com/tildaslashalef/mindnest/internal/loggy"
	"github.com/tildaslashalef/mindnest/internal/review"
	"github.com/tildaslashalef/mindnest/internal/workspace"
)

// Service handles syncing data to the remote server
type Service struct {
	config           *config.Config
	client           *Client
	repo             Repository
	workspaceRepo    workspace.Repository
	workspaceService *workspace.Service
	reviewRepo       review.Repository
	reviewService    *review.Service
	logger           *loggy.Logger
	settingsRepo     config.SettingsRepository
}

// NewService creates a new sync service
func NewService(
	cfg *config.Config,
	syncRepo Repository,
	workspaceRepo workspace.Repository,
	workspaceService *workspace.Service,
	reviewRepo review.Repository,
	reviewService *review.Service,
	logger *loggy.Logger,
) *Service {
	// Create HTTP client
	client := NewClient(
		cfg.Server.URL,
		cfg.Server.Token,
		cfg.Server.Timeout,
		logger,
	)

	return &Service{
		config:           cfg,
		client:           client,
		repo:             syncRepo,
		workspaceRepo:    workspaceRepo,
		workspaceService: workspaceService,
		reviewRepo:       reviewRepo,
		reviewService:    reviewService,
		logger:           logger,
	}
}

// IsConfigured returns whether the sync service is configured
func (s *Service) IsConfigured() bool {
	enabled, err := s.settingsRepo.GetSetting(context.Background(), "sync.enabled")
	if err != nil {
		s.logger.Error("Failed to get sync enabled status", "error", err)
		return false
	}

	serverToken, err := s.settingsRepo.GetSetting(context.Background(), "sync.server_token")
	if err != nil {
		s.logger.Error("Failed to get sync server token", "error", err)
		return false
	}

	serverURL, err := s.settingsRepo.GetSetting(context.Background(), "sync.server_url")
	if err != nil {
		s.logger.Error("Failed to get sync server URL", "error", err)
		return false
	}

	return enabled == "true" && serverToken != "" && serverURL != ""
}

// SetToken updates the authentication token
func (s *Service) SetToken(token string) error {
	s.config.Server.Token = token
	s.client.SetToken(token)

	// Save to persistent storage if settings repo is available
	if s.settingsRepo != nil {
		ctx := context.Background()
		if err := s.settingsRepo.SetSetting(ctx, "sync.server_token", token); err != nil {
			s.logger.Warn("Failed to save token to settings", "error", err)
			// Continue anyway, the in-memory setting was updated
		} else {
			s.logger.Info("Saved sync token to persistent storage")
		}

		// Update enabled status if token is set or cleared
		enabledStr := "false"
		if token != "" {
			enabledStr = "true"
		}
		if err := s.settingsRepo.SetSetting(ctx, "sync.enabled", enabledStr); err != nil {
			s.logger.Warn("Failed to save enabled status to settings", "error", err)
		}
	}

	return nil
}

// VerifyToken verifies if the current token is valid
func (s *Service) VerifyToken(ctx context.Context) (bool, error) {
	if !s.IsConfigured() {
		return false, fmt.Errorf("sync service not configured")
	}

	return s.client.VerifyToken(ctx)
}

// GetSyncLogs retrieves sync logs
func (s *Service) GetSyncLogs(ctx context.Context, entityType EntityType, entityID string, limit int, offset int) ([]*SyncLog, error) {
	return s.repo.GetSyncLogs(ctx, entityType, entityID, limit, offset)
}

// GetFailedSyncLogs retrieves only failed sync logs for a specific entity type
func (s *Service) GetFailedSyncLogs(ctx context.Context, entityType EntityType, limit int) ([]string, error) {
	logs, err := s.repo.GetSyncLogs(ctx, entityType, "", limit, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get sync logs: %w", err)
	}

	// Extract entity IDs from failed logs
	var failedEntities []string

	// Create map to track most recent log for each entity
	// The key is the entity ID, and the value is a bool indicating whether the most recent sync was successful
	entitySyncStatus := make(map[string]bool)
	entityLastAttempt := make(map[string]time.Time)

	// Process logs in order (most recent first, according to GetSyncLogs' OrderBy)
	for _, log := range logs {
		// If we haven't seen this entity before, or if this log is more recent than what we've seen
		lastAttempt, exists := entityLastAttempt[log.EntityID]
		if !exists || log.CompletedAt.After(lastAttempt) {
			entitySyncStatus[log.EntityID] = log.Success
			entityLastAttempt[log.EntityID] = log.CompletedAt
		}
	}

	// Now collect entities whose most recent sync was a failure
	for entityID, wasSuccessful := range entitySyncStatus {
		if !wasSuccessful {
			failedEntities = append(failedEntities, entityID)
		}
	}

	return failedEntities, nil
}

// GetUnsyncedWorkspaces retrieves workspaces that need to be synced
func (s *Service) GetUnsyncedWorkspaces(ctx context.Context, limit int) ([]string, error) {
	return s.repo.GetUnsyncedWorkspaces(ctx, limit)
}

// GetUnsyncedReviews retrieves reviews that need to be synced
func (s *Service) GetUnsyncedReviews(ctx context.Context, workspaceID string, limit int) ([]string, error) {
	return s.repo.GetUnsyncedReviews(ctx, workspaceID, limit)
}

// GetUnsyncedReviewFiles retrieves review files that need to be synced
func (s *Service) GetUnsyncedReviewFiles(ctx context.Context, reviewID string, limit int) ([]string, error) {
	return s.repo.GetUnsyncedReviewFiles(ctx, reviewID, limit)
}

// GetUnsyncedIssues retrieves issues that need to be synced
func (s *Service) GetUnsyncedIssues(ctx context.Context, reviewID string, limit int) ([]string, error) {
	return s.repo.GetUnsyncedIssues(ctx, reviewID, limit)
}

// GetUnsyncedFiles retrieves files that need to be synced
func (s *Service) GetUnsyncedFiles(ctx context.Context, workspaceID string, limit int) ([]string, error) {
	return s.repo.GetUnsyncedFiles(ctx, workspaceID, limit)
}

// SyncWorkspace syncs a workspace to the server
func (s *Service) SyncWorkspace(ctx context.Context, workspaceID string) (*SyncResult, error) {
	// Create sync log
	syncLog := NewSyncLog(SyncTypeManual, EntityTypeWorkspace, workspaceID)
	start := time.Now()

	// Get workspace
	workspace, err := s.workspaceService.GetWorkspace(ctx, workspaceID)
	if err != nil {
		syncLog.MarkFailed(SyncErrorTypeClient, fmt.Sprintf("failed to get workspace: %v", err))
		if err := s.repo.CreateSyncLog(ctx, syncLog); err != nil {
			s.logger.Error("Failed to create sync log", "error", err)
		}
		return &SyncResult{
			Success:      false,
			ErrorType:    SyncErrorTypeClient,
			ErrorMessage: fmt.Sprintf("failed to get workspace: %v", err),
			Duration:     time.Since(start),
		}, err
	}

	// Create sync request
	req := CreateWorkspaceFromModel(workspace)

	// Send to server
	_, err = s.client.SyncWorkspace(ctx, req)
	if err != nil {
		// Determine error type
		errorType := SyncErrorTypeUnknown
		if apiErr, ok := err.(APIError); ok {
			switch apiErr.StatusCode {
			case 401, 403:
				errorType = SyncErrorTypeAuth
			case 500, 502, 503, 504:
				errorType = SyncErrorTypeServer
			default:
				errorType = SyncErrorTypeClient
			}
		} else {
			errorType = SyncErrorTypeNetwork
		}

		syncLog.MarkFailed(errorType, err.Error())
		if err := s.repo.CreateSyncLog(ctx, syncLog); err != nil {
			s.logger.Error("Failed to create sync log", "error", err)
		}

		return &SyncResult{
			Success:      false,
			ErrorType:    errorType,
			ErrorMessage: err.Error(),
			Duration:     time.Since(start),
		}, err
	}

	// Only update sync timestamp if sync was successful
	if err := s.repo.UpdateEntitySyncStatus(ctx, EntityTypeWorkspace, workspaceID); err != nil {
		s.logger.Error("Failed to update workspace sync status", "error", err, "workspace_id", workspaceID)
		return &SyncResult{
			Success:      false,
			ErrorType:    SyncErrorTypeClient,
			ErrorMessage: fmt.Sprintf("failed to update workspace sync status: %v", err),
			Duration:     time.Since(start),
		}, err
	}

	// Mark sync log as successful
	syncLog.MarkSuccessful(1)
	if err := s.repo.CreateSyncLog(ctx, syncLog); err != nil {
		s.logger.Error("Failed to create sync log", "error", err)
	}

	return &SyncResult{
		TotalItems:   1,
		SuccessItems: 1,
		FailedItems:  0,
		Success:      true,
		Duration:     time.Since(start),
	}, nil
}

// SyncReview syncs a review to the server
func (s *Service) SyncReview(ctx context.Context, reviewID string) (*SyncResult, error) {
	// Create sync log
	syncLog := NewSyncLog(SyncTypeManual, EntityTypeReview, reviewID)
	start := time.Now()

	// Get review
	review, err := s.reviewService.GetReview(ctx, reviewID)
	if err != nil {
		syncLog.MarkFailed(SyncErrorTypeClient, fmt.Sprintf("failed to get review: %v", err))
		if err := s.repo.CreateSyncLog(ctx, syncLog); err != nil {
			s.logger.Error("Failed to create sync log", "error", err)
		}
		return &SyncResult{
			Success:      false,
			ErrorType:    SyncErrorTypeClient,
			ErrorMessage: fmt.Sprintf("failed to get review: %v", err),
			Duration:     time.Since(start),
		}, err
	}

	// Create sync request
	var resultJSON json.RawMessage
	if !review.Result.IsEmpty() {
		resultJSON, err = json.Marshal(review.Result)
		if err != nil {
			syncLog.MarkFailed(SyncErrorTypeClient, fmt.Sprintf("failed to marshal review result: %v", err))
			if err := s.repo.CreateSyncLog(ctx, syncLog); err != nil {
				s.logger.Error("Failed to create sync log", "error", err)
			}
			return &SyncResult{
				Success:      false,
				ErrorType:    SyncErrorTypeClient,
				ErrorMessage: fmt.Sprintf("failed to marshal review result: %v", err),
				Duration:     time.Since(start),
			}, err
		}
	}

	req := &SyncReviewRequest{
		ID:          review.ID,
		WorkspaceID: review.WorkspaceID,
		Type:        review.Type,
		CommitHash:  review.CommitHash,
		BranchFrom:  review.BranchFrom,
		BranchTo:    review.BranchTo,
		Status:      string(review.Status),
		Result:      resultJSON,
		CreatedAt:   review.CreatedAt,
		UpdatedAt:   review.UpdatedAt,
	}

	// Send to server
	_, err = s.client.SyncReview(ctx, req)
	if err != nil {
		// Determine error type
		errorType := SyncErrorTypeUnknown
		if apiErr, ok := err.(APIError); ok {
			switch apiErr.StatusCode {
			case 401, 403:
				errorType = SyncErrorTypeAuth
			case 500, 502, 503, 504:
				errorType = SyncErrorTypeServer
			default:
				errorType = SyncErrorTypeClient
			}
		} else {
			errorType = SyncErrorTypeNetwork
		}

		syncLog.MarkFailed(errorType, err.Error())
		if err := s.repo.CreateSyncLog(ctx, syncLog); err != nil {
			s.logger.Error("Failed to create sync log", "error", err)
		}

		return &SyncResult{
			Success:      false,
			ErrorType:    errorType,
			ErrorMessage: err.Error(),
			Duration:     time.Since(start),
		}, err
	}

	// Update sync timestamp
	if err := s.repo.UpdateEntitySyncStatus(ctx, EntityTypeReview, reviewID); err != nil {
		s.logger.Error("Failed to update review sync status", "error", err, "review_id", reviewID)
	}

	// Mark sync log as successful
	syncLog.MarkSuccessful(1)
	if err := s.repo.CreateSyncLog(ctx, syncLog); err != nil {
		s.logger.Error("Failed to create sync log", "error", err)
		return &SyncResult{
			Success:      false,
			ErrorType:    SyncErrorTypeClient,
			ErrorMessage: fmt.Sprintf("failed to create sync log: %v", err),
			Duration:     time.Since(start),
		}, err
	}

	return &SyncResult{
		TotalItems:   1,
		SuccessItems: 1,
		FailedItems:  0,
		Success:      true,
		Duration:     time.Since(start),
	}, nil
}

// SyncReviewFile syncs a review file to the server
func (s *Service) SyncReviewFile(ctx context.Context, reviewFileID string) (*SyncResult, error) {
	// Create sync log
	syncLog := NewSyncLog(SyncTypeManual, EntityTypeReviewFile, reviewFileID)
	start := time.Now()

	// Get review file
	reviewFile, err := s.reviewService.GetReviewFile(ctx, reviewFileID)
	if err != nil {
		syncLog.MarkFailed(SyncErrorTypeClient, fmt.Sprintf("failed to get review file: %v", err))
		if err := s.repo.CreateSyncLog(ctx, syncLog); err != nil {
			s.logger.Error("Failed to create sync log", "error", err)
		}
		return &SyncResult{
			Success:      false,
			ErrorType:    SyncErrorTypeClient,
			ErrorMessage: fmt.Sprintf("failed to get review file: %v", err),
			Duration:     time.Since(start),
		}, err
	}

	var metadataJSON json.RawMessage
	if reviewFile.Metadata != nil {
		metadataJSON, err = json.Marshal(reviewFile.Metadata)
		if err != nil {
			syncLog.MarkFailed(SyncErrorTypeClient, fmt.Sprintf("failed to marshal review file metadata: %v", err))
			if err := s.repo.CreateSyncLog(ctx, syncLog); err != nil {
				s.logger.Error("Failed to create sync log", "error", err)
			}
			return &SyncResult{
				Success:      false,
				ErrorType:    SyncErrorTypeClient,
				ErrorMessage: fmt.Sprintf("failed to marshal review file metadata: %v", err),
				Duration:     time.Since(start),
			}, err
		}
	}

	// Create sync request
	req := &SyncReviewFileRequest{
		ID:          reviewFile.ID,
		ReviewID:    reviewFile.ReviewID,
		FileID:      reviewFile.FileID,
		Status:      reviewFile.Status,
		IssuesCount: reviewFile.IssuesCount,
		Summary:     reviewFile.Summary,
		Assessment:  reviewFile.Assessment,
		Metadata:    metadataJSON,
		CreatedAt:   reviewFile.CreatedAt,
		UpdatedAt:   reviewFile.UpdatedAt,
	}

	// Send to server
	_, err = s.client.SyncReviewFile(ctx, req)
	if err != nil {
		// Determine error type
		errorType := SyncErrorTypeUnknown
		if apiErr, ok := err.(APIError); ok {
			switch apiErr.StatusCode {
			case 401, 403:
				errorType = SyncErrorTypeAuth
			case 500, 502, 503, 504:
				errorType = SyncErrorTypeServer
			default:
				errorType = SyncErrorTypeClient
			}
		} else {
			errorType = SyncErrorTypeNetwork
		}

		syncLog.MarkFailed(errorType, err.Error())
		if err := s.repo.CreateSyncLog(ctx, syncLog); err != nil {
			s.logger.Error("Failed to create sync log", "error", err)
		}

		return &SyncResult{
			Success:      false,
			ErrorType:    errorType,
			ErrorMessage: err.Error(),
			Duration:     time.Since(start),
		}, err
	}

	// Update sync timestamp
	if err := s.repo.UpdateEntitySyncStatus(ctx, EntityTypeReviewFile, reviewFileID); err != nil {
		s.logger.Error("Failed to update review file sync status", "error", err, "review_file_id", reviewFileID)
		return &SyncResult{
			Success:      false,
			ErrorType:    SyncErrorTypeClient,
			ErrorMessage: fmt.Sprintf("failed to update review file sync status: %v", err),
			Duration:     time.Since(start),
		}, err
	}

	// Mark sync log as successful
	syncLog.MarkSuccessful(1)
	if err := s.repo.CreateSyncLog(ctx, syncLog); err != nil {
		s.logger.Error("Failed to create sync log", "error", err)
	}

	return &SyncResult{
		TotalItems:   1,
		SuccessItems: 1,
		FailedItems:  0,
		Success:      true,
		Duration:     time.Since(start),
	}, nil
}

// SyncIssue syncs an issue to the server
func (s *Service) SyncIssue(ctx context.Context, issueID string) (*SyncResult, error) {
	// Create sync log
	syncLog := NewSyncLog(SyncTypeManual, EntityTypeIssue, issueID)
	start := time.Now()

	// Get issue
	issue, err := s.reviewService.GetIssue(ctx, issueID)
	if err != nil {
		syncLog.MarkFailed(SyncErrorTypeClient, fmt.Sprintf("failed to get issue: %v", err))
		if err := s.repo.CreateSyncLog(ctx, syncLog); err != nil {
			s.logger.Error("Failed to create sync log", "error", err)
		}
		return &SyncResult{
			Success:      false,
			ErrorType:    SyncErrorTypeClient,
			ErrorMessage: fmt.Sprintf("failed to get issue: %v", err),
			Duration:     time.Since(start),
		}, err
	}
	req := &SyncIssueRequest{
		ID:           issue.ID,
		ReviewID:     issue.ReviewID,
		FileID:       issue.FileID,
		Type:         issue.Type,
		Severity:     issue.Severity,
		Title:        issue.Title,
		Description:  issue.Description,
		LineStart:    issue.LineStart,
		LineEnd:      issue.LineEnd,
		Suggestion:   issue.Suggestion,
		AffectedCode: issue.AffectedCode,
		CodeSnippet:  issue.CodeSnippet,
		IsValid:      issue.IsValid,
		CreatedAt:    issue.CreatedAt,
		UpdatedAt:    issue.UpdatedAt,
	}

	// Send to server
	_, err = s.client.SyncIssue(ctx, req)
	if err != nil {
		syncLog.MarkFailed(SyncErrorTypeClient, fmt.Sprintf("failed to sync issue: %v", err))
		if err := s.repo.CreateSyncLog(ctx, syncLog); err != nil {
			s.logger.Error("Failed to create sync log", "error", err)
		}
		return &SyncResult{
			Success:      false,
			ErrorType:    SyncErrorTypeClient,
			ErrorMessage: fmt.Sprintf("failed to sync issue: %v", err),
			Duration:     time.Since(start),
		}, err
	}

	// Update sync timestamp
	if err := s.repo.UpdateEntitySyncStatus(ctx, EntityTypeIssue, issueID); err != nil {
		s.logger.Error("Failed to update issue sync status", "error", err, "issue_id", issueID)
	}

	// Mark sync log as successful
	syncLog.MarkSuccessful(1)
	if err := s.repo.CreateSyncLog(ctx, syncLog); err != nil {
		s.logger.Error("Failed to create sync log", "error", err)
	}

	return &SyncResult{
		TotalItems:   1,
		SuccessItems: 1,
		FailedItems:  0,
		Success:      true,
		Duration:     time.Since(start),
	}, nil
}

func (s *Service) SyncFile(ctx context.Context, fileID string) (*SyncResult, error) {
	// Create sync log
	syncLog := NewSyncLog(SyncTypeManual, EntityTypeFile, fileID)
	start := time.Now()

	// Get file
	file, err := s.workspaceService.GetFile(ctx, fileID)
	if err != nil {
		syncLog.MarkFailed(SyncErrorTypeClient, fmt.Sprintf("failed to get file: %v", err))
		if err := s.repo.CreateSyncLog(ctx, syncLog); err != nil {
			s.logger.Error("Failed to create sync log", "error", err)
		}
		return &SyncResult{
			Success:      false,
			ErrorType:    SyncErrorTypeClient,
			ErrorMessage: fmt.Sprintf("failed to get file: %v", err),
			Duration:     time.Since(start),
		}, err
	}

	req := &SyncFileRequest{
		ID:          file.ID,
		WorkspaceID: file.WorkspaceID,
		Path:        file.Path,
		Language:    file.Language,
		LastParsed:  file.LastParsed,
		Metadata:    file.Metadata,
		CreatedAt:   file.CreatedAt,
		UpdatedAt:   file.UpdatedAt,
	}

	// Send to server
	_, err = s.client.SyncFile(ctx, req)
	if err != nil {
		syncLog.MarkFailed(SyncErrorTypeClient, fmt.Sprintf("failed to sync file: %v", err))
		if err := s.repo.CreateSyncLog(ctx, syncLog); err != nil {
			s.logger.Error("Failed to create sync log", "error", err)
		}
		return &SyncResult{
			Success:      false,
			ErrorType:    SyncErrorTypeClient,
			ErrorMessage: fmt.Sprintf("failed to sync file: %v", err),
			Duration:     time.Since(start),
		}, err
	}

	// Update sync timestamp
	if err := s.repo.UpdateEntitySyncStatus(ctx, EntityTypeFile, fileID); err != nil {
		s.logger.Error("Failed to update file sync status", "error", err, "file_id", fileID)
		return &SyncResult{
			Success:      false,
			ErrorType:    SyncErrorTypeClient,
			ErrorMessage: fmt.Sprintf("failed to update file sync status: %v", err),
			Duration:     time.Since(start),
		}, err
	}

	// Mark sync log as successful
	syncLog.MarkSuccessful(1)
	if err := s.repo.CreateSyncLog(ctx, syncLog); err != nil {
		s.logger.Error("Failed to create sync log", "error", err)
	}

	return &SyncResult{
		TotalItems:   1,
		SuccessItems: 1,
		FailedItems:  0,
		Success:      true,
		Duration:     time.Since(start),
	}, nil
}

// SyncAll syncs all workspaces, reviews, and issues
func (s *Service) SyncAll(ctx context.Context) (*SyncResult, error) {
	if !s.IsConfigured() {
		return nil, fmt.Errorf("sync service not configured")
	}

	start := time.Now()
	result := &SyncResult{
		Success: true,
	}

	// Track counts for each entity type
	var workspaceCount, reviewCount, reviewFileCount, issueCount, fileCount int

	// Get unsynced workspaces
	workspaceIDs, err := s.repo.GetUnsyncedWorkspaces(ctx, 100) // Increased limit
	if err != nil {
		s.logger.Error("Failed to get unsynced workspaces", "error", err)
		return nil, err
	}

	unsyncedWorkspaceCount := len(workspaceIDs)
	s.logger.Debug("Found unsynced workspaces", "count", unsyncedWorkspaceCount)

	// Get failed workspaces from previous sync attempts
	failedWorkspaces, err := s.GetFailedSyncLogs(ctx, EntityTypeWorkspace, 100)
	if err != nil {
		s.logger.Error("Failed to get failed workspace logs", "error", err)
	} else {
		s.logger.Debug("Found failed workspaces", "count", len(failedWorkspaces))
		// Combine with unsynced workspaces (no duplicates)
		workspacesMap := make(map[string]bool)
		for _, id := range workspaceIDs {
			workspacesMap[id] = true
		}
		for _, id := range failedWorkspaces {
			workspacesMap[id] = true
		}

		// Convert back to slice
		workspaceIDs = make([]string, 0, len(workspacesMap))
		for id := range workspacesMap {
			workspaceIDs = append(workspaceIDs, id)
		}
	}

	workspaceCount = len(workspaceIDs)
	s.logger.Debug("Syncing workspaces", "count", workspaceCount, "new_unsynced", unsyncedWorkspaceCount, "previously_failed", len(failedWorkspaces))

	// Sync workspaces
	for _, id := range workspaceIDs {
		wsResult, err := s.SyncWorkspace(ctx, id)
		if err != nil {
			s.logger.Error("Failed to sync workspace", "error", err, "workspace_id", id)
			result.FailedItems++
			result.Success = false
			if result.ErrorMessage == "" {
				result.ErrorMessage = err.Error()
				result.ErrorType = wsResult.ErrorType
			}
		} else {
			result.SuccessItems++
		}
		result.TotalItems++
	}

	// Get unsynced files
	var fileMap = make(map[string]bool)
	for _, wsID := range workspaceIDs {
		files, err := s.repo.GetUnsyncedFiles(ctx, wsID, 100) // Increased limit
		if err != nil {
			s.logger.Error("Failed to get unsynced files", "error", err)
			continue
		}
		for _, fileID := range files {
			fileMap[fileID] = true
		}
	}

	// Also check for any directly modified files (without a specific workspace filter)
	directlyModifiedFiles, err := s.repo.GetUnsyncedFiles(ctx, "", 100)
	if err != nil {
		s.logger.Error("Failed to get directly modified files", "error", err)
	} else {
		s.logger.Debug("Found directly modified files", "count", len(directlyModifiedFiles))
		for _, fileID := range directlyModifiedFiles {
			fileMap[fileID] = true
		}
	}

	unsyncedFileCount := len(fileMap)
	s.logger.Debug("Found unsynced files", "count", unsyncedFileCount)

	// Get failed files from previous sync attempts
	failedFiles, err := s.GetFailedSyncLogs(ctx, EntityTypeFile, 100)
	if err != nil {
		s.logger.Error("Failed to get failed file logs", "error", err)
	} else {
		s.logger.Debug("Found failed files", "count", len(failedFiles))
		// Add failed files
		for _, id := range failedFiles {
			fileMap[id] = true
		}
	}

	// Convert map to slice
	fileIDs := make([]string, 0, len(fileMap))
	for id := range fileMap {
		fileIDs = append(fileIDs, id)
	}

	fileCount = len(fileIDs)
	s.logger.Debug("Syncing files", "count", fileCount, "new_unsynced", unsyncedFileCount, "previously_failed", len(failedFiles))

	// Sync files
	for _, id := range fileIDs {
		fileResult, err := s.SyncFile(ctx, id)
		if err != nil {
			s.logger.Error("Failed to sync file", "error", err, "file_id", id)
			result.FailedItems++
			result.Success = false
			if result.ErrorMessage == "" {
				result.ErrorMessage = err.Error()
				result.ErrorType = fileResult.ErrorType
			}
		} else {
			result.SuccessItems++
		}
		result.TotalItems++
	}

	// Get unsynced reviews
	var reviewsMap = make(map[string]bool)
	for _, wsID := range workspaceIDs {
		reviewIDs, err := s.repo.GetUnsyncedReviews(ctx, wsID, 100) // Increased limit
		if err != nil {
			s.logger.Error("Failed to get unsynced reviews", "error", err)
			continue
		}
		for _, reviewID := range reviewIDs {
			reviewsMap[reviewID] = true
		}
	}

	unsyncedReviewCount := len(reviewsMap)
	s.logger.Debug("Found unsynced reviews", "count", unsyncedReviewCount)

	// Get failed reviews from previous sync attempts
	failedReviews, err := s.GetFailedSyncLogs(ctx, EntityTypeReview, 100)
	if err != nil {
		s.logger.Error("Failed to get failed review logs", "error", err)
	} else {
		s.logger.Debug("Found failed reviews", "count", len(failedReviews))
		// Add failed reviews
		for _, id := range failedReviews {
			reviewsMap[id] = true
		}
	}

	reviewCount = len(reviewsMap)
	s.logger.Info("Syncing reviews", "count", reviewCount, "new_unsynced", unsyncedReviewCount, "previously_failed", len(failedReviews))

	// Sync reviews
	for id := range reviewsMap {
		reviewResult, err := s.SyncReview(ctx, id)
		if err != nil {
			s.logger.Error("Failed to sync review", "error", err, "review_id", id)
			result.FailedItems++
			result.Success = false
			if result.ErrorMessage == "" {
				result.ErrorMessage = err.Error()
				result.ErrorType = reviewResult.ErrorType
			}
		} else {
			result.SuccessItems++
		}
		result.TotalItems++
	}

	// Get unsynced review files
	var reviewFileMap = make(map[string]bool)
	for reviewID := range reviewsMap {
		files, err := s.repo.GetUnsyncedReviewFiles(ctx, reviewID, 100) // Increased limit
		if err != nil {
			s.logger.Error("Failed to get unsynced review files", "error", err)
			continue
		}
		for _, fileID := range files {
			reviewFileMap[fileID] = true
		}
	}

	// Also check for any directly modified review files (without a specific review filter)
	directlyModifiedReviewFiles, err := s.repo.GetUnsyncedReviewFiles(ctx, "", 100)
	if err != nil {
		s.logger.Error("Failed to get directly modified review files", "error", err)
	} else {
		s.logger.Info("Found directly modified review files", "count", len(directlyModifiedReviewFiles))
		for _, fileID := range directlyModifiedReviewFiles {
			reviewFileMap[fileID] = true
		}
	}

	unsyncedReviewFileCount := len(reviewFileMap)
	s.logger.Debug("Found unsynced review files", "count", unsyncedReviewFileCount)

	// Get failed review files from previous sync attempts
	failedReviewFiles, err := s.GetFailedSyncLogs(ctx, EntityTypeReviewFile, 100)
	if err != nil {
		s.logger.Error("Failed to get failed review file logs", "error", err)
	} else {
		s.logger.Debug("Found failed review files", "count", len(failedReviewFiles))
		// Add failed review files
		for _, id := range failedReviewFiles {
			reviewFileMap[id] = true
		}
	}

	// Convert map to slice
	reviewFileIDs := make([]string, 0, len(reviewFileMap))
	for id := range reviewFileMap {
		reviewFileIDs = append(reviewFileIDs, id)
	}

	reviewFileCount = len(reviewFileIDs)
	s.logger.Debug("Syncing review files", "count", reviewFileCount, "new_unsynced", unsyncedReviewFileCount, "previously_failed", len(failedReviewFiles))

	// Sync review files
	for _, id := range reviewFileIDs {
		reviewFileResult, err := s.SyncReviewFile(ctx, id)
		if err != nil {
			s.logger.Error("Failed to sync review file", "error", err, "review_file_id", id)
			result.FailedItems++
			result.Success = false
			if result.ErrorMessage == "" {
				result.ErrorMessage = err.Error()
				result.ErrorType = reviewFileResult.ErrorType
			}
		} else {
			result.SuccessItems++
		}
		result.TotalItems++
	}

	// Get unsynced issues
	var issueMap = make(map[string]bool)
	for reviewID := range reviewsMap {
		issues, err := s.repo.GetUnsyncedIssues(ctx, reviewID, 100) // Increased limit
		if err != nil {
			s.logger.Error("Failed to get unsynced issues", "error", err)
			continue
		}
		for _, issueID := range issues {
			issueMap[issueID] = true
		}
	}

	// Also check for any directly modified issues (without a specific review filter)
	directlyModifiedIssues, err := s.repo.GetUnsyncedIssues(ctx, "", 100)
	if err != nil {
		s.logger.Error("Failed to get directly modified issues", "error", err)
	} else {
		s.logger.Debug("Found directly modified issues", "count", len(directlyModifiedIssues))
		for _, issueID := range directlyModifiedIssues {
			issueMap[issueID] = true
		}
	}

	unsyncedIssueCount := len(issueMap)
	s.logger.Debug("Found unsynced issues", "count", unsyncedIssueCount)

	// Get failed issues from previous sync attempts
	failedIssues, err := s.GetFailedSyncLogs(ctx, EntityTypeIssue, 100)
	if err != nil {
		s.logger.Error("Failed to get failed issue logs", "error", err)
	} else {
		s.logger.Debug("Found failed issues", "count", len(failedIssues))
		// Add failed issues
		for _, id := range failedIssues {
			issueMap[id] = true
		}
	}

	// Convert map to slice
	issueIDs := make([]string, 0, len(issueMap))
	for id := range issueMap {
		issueIDs = append(issueIDs, id)
	}

	issueCount = len(issueIDs)
	s.logger.Debug("Syncing issues", "count", issueCount, "new_unsynced", unsyncedIssueCount, "previously_failed", len(failedIssues))

	// Sync issues
	for _, id := range issueIDs {
		issueResult, err := s.SyncIssue(ctx, id)
		if err != nil {
			s.logger.Error("Failed to sync issue", "error", err, "issue_id", id)
			result.FailedItems++
			result.Success = false
			if result.ErrorMessage == "" {
				result.ErrorMessage = err.Error()
				result.ErrorType = issueResult.ErrorType
			}
		} else {
			result.SuccessItems++
		}
		result.TotalItems++
	}

	// If nothing was synced, log it
	if result.TotalItems == 0 {
		s.logger.Debug("No items to sync - everything is up to date")
	}

	// Store the breakdown in the result
	result.WorkspaceCount = workspaceCount
	result.FileCount = fileCount
	result.ReviewCount = reviewCount
	result.ReviewFileCount = reviewFileCount
	result.IssueCount = issueCount

	result.Duration = time.Since(start)
	return result, nil
}

// SetSettingsRepository sets the settings repository for the service
func (s *Service) SetSettingsRepository(repo config.SettingsRepository) {
	s.settingsRepo = repo

	// Also set the repository for the client
	if s.client != nil {
		s.client.SetSettingsRepository(repo)
	}
}

// SaveSettings persists the current sync settings to the database
func (s *Service) SaveSettings(ctx context.Context) error {
	if s.settingsRepo == nil {
		return fmt.Errorf("settings repository not initialized")
	}

	return config.SaveSyncSettings(ctx, s.config, s.settingsRepo)
}
