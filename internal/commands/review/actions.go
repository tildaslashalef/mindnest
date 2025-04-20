package review

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tildaslashalef/mindnest/internal/loggy"
	"github.com/tildaslashalef/mindnest/internal/review"
)

// getOrCreateWorkspace gets or creates the workspace based on review options.
// It returns a command that will send a workspaceMsg.
func getOrCreateWorkspace(m Model) tea.Cmd {
	return func() tea.Msg {
		loggy.Debug("Attempting to get or create workspace", "target_dir", m.reviewOptions.TargetDir, "abs_path", m.reviewOptions.AbsPath)
		// Get current workspace or create if not found
		workspace, err := m.app.Workspace.GetCurrentWorkspace(m.ctx, m.app.Config)
		if err != nil {
			if err.Error() == "workspace not found" || err.Error() == "no config file found" {
				// Use the resolved AbsPath from reviewOptions
				cwd := m.reviewOptions.AbsPath
				if cwd == "" {
					// Fallback if somehow AbsPath wasn't resolved
					cwd, _ = filepath.Abs(m.reviewOptions.TargetDir)
				}
				dirName := filepath.Base(cwd)

				loggy.Info("Workspace not found, creating new one", "dir_name", dirName, "path", cwd)
				workspace, err = m.app.Workspace.CreateWorkspace(m.ctx, cwd, dirName, m.app.Config, "", "")
				if err != nil {
					loggy.Error("Failed to create workspace", "path", cwd, "error", err)
					return workspaceMsg{
						error: fmt.Errorf("failed to create workspace: %w", err),
					}
				}
				loggy.Info("Created new workspace", "name", workspace.Name, "id", workspace.ID, "path", workspace.Path)
			} else {
				loggy.Error("Error accessing workspace", "error", err)
				return workspaceMsg{
					error: fmt.Errorf("error accessing workspace: %w", err),
				}
			}
		} else {
			loggy.Info("Using existing workspace", "name", workspace.Name, "id", workspace.ID, "path", workspace.Path)
		}

		return workspaceMsg{
			workspace: workspace,
		}
	}
}

// startReview signals the Update loop to prepare for the review process.
func startReview() tea.Cmd {
	return func() tea.Msg {
		return startReviewProcessMsg{} // Defined in messages.go
	}
}

// prepareReviewData determines which files need processing based on review options.
// It returns a command that sends a reviewSetupMsg.
func prepareReviewData(m Model) tea.Cmd {
	return func() tea.Msg {
		if m.workspace == nil {
			return statusChangeMsg{error: fmt.Errorf("workspace not initialized for review prep")} // Send error status
		}

		ctx := m.ctx
		app := m.app
		workspace := m.workspace
		absPath := m.reviewOptions.AbsPath

		// Validate review options
		staged := m.reviewOptions.Staged
		commitHash := m.reviewOptions.CommitHash
		branch := m.reviewOptions.Branch

		if (staged && (commitHash != "" || branch != "")) || (commitHash != "" && branch != "") {
			loggy.Error("Invalid review options: multiple modes selected", "staged", staged, "commit", commitHash, "branch", branch)
			return statusChangeMsg{error: fmt.Errorf("invalid review options: select only one of staged, commit, or branch")}
		}

		// Find files to process based on review mode
		var fileIDs []string
		var err error
		var currentCommitHash, currentBranchName, baseBranchName string

		if staged || commitHash != "" || branch != "" {
			loggy.Debug("Checking for Git repository", "path", absPath)
			if !app.Workspace.HasGitRepo(absPath) {
				loggy.Error("Git repository required but not found", "path", absPath)
				return statusChangeMsg{error: fmt.Errorf("no Git repository found in %s (required for staged/commit/branch review)", absPath)}
			}
		}

		if staged {
			loggy.Info("Processing staged changes")
			fileIDs, err = app.Workspace.ParseStagedChanges(ctx, workspace.ID, absPath)
		} else if commitHash != "" {
			loggy.Info("Processing commit changes", "hash", commitHash)
			fileIDs, err = app.Workspace.ParseCommitChanges(ctx, workspace.ID, absPath, commitHash)
			currentCommitHash = commitHash
		} else if branch != "" {
			baseBranchName = m.reviewOptions.BaseBranch
			loggy.Info("Processing branch diff", "branch", branch, "base", baseBranchName)
			fileIDs, err = app.Workspace.ParseBranchChanges(ctx, workspace.ID, absPath, baseBranchName, branch)
			currentBranchName = branch
		} else {
			// If no mode specified, potentially default to all tracked files or error?
			// Current logic seems to imply one mode must be active based on RunWithOptions.
			// If we reach here, it might indicate an issue.
			loggy.Warn("No review mode specified (staged/commit/branch), reviewing all files in workspace (behavior might change)")
			// Example: Process all files (implement if needed)
			// fileIDs, err = app.Workspace.GetAllFileIDs(ctx, workspace.ID)
			return statusChangeMsg{error: fmt.Errorf("internal error: no review mode determined")}
		}

		if err != nil {
			loggy.Error("Failed to parse changes", "error", err)
			return statusChangeMsg{error: fmt.Errorf("failed to parse changes: %w", err)}
		}

		if len(fileIDs) == 0 {
			loggy.Warn("No files found for review in the selected mode.")
			return statusChangeMsg{error: fmt.Errorf("no files found for review - check your selection (staged files, commit hash, or branch names)")}
		}

		loggy.Debug("Retrieved file IDs for review", "count", len(fileIDs))

		// Get file paths for progress display
		var filesToProcess []string
		for _, fileID := range fileIDs {
			file, err := app.Workspace.GetFile(ctx, fileID)
			if err != nil {
				loggy.Warn("Failed to get file details during setup", "file_id", fileID, "error", err)
				// Decide whether to skip or error out. Skipping for now.
				continue
			}
			filesToProcess = append(filesToProcess, file.Path)
		}

		if len(filesToProcess) == 0 && len(fileIDs) > 0 {
			loggy.Error("Found file IDs but failed to retrieve any file paths")
			return statusChangeMsg{error: fmt.Errorf("internal error: failed to retrieve file details for review")}
		}

		// Send the setup data back to the Update loop
		return reviewSetupMsg{ // Defined in messages.go
			fileIDs:        fileIDs,
			filesToProcess: filesToProcess,
			totalFiles:     len(filesToProcess),
			commitHash:     currentCommitHash,
			branchName:     currentBranchName,
			baseBranchName: baseBranchName,
		}
	}
}

// processNextFile processes a single file at the given index.
// It returns a command that sends a fileProcessedMsg.
func processNextFile(m Model, currentIndex int) tea.Cmd {
	return func() tea.Msg {
		if m.workspace == nil {
			return statusChangeMsg{error: fmt.Errorf("workspace not initialized for file processing")}
		}

		// Ensure index is valid
		if currentIndex < 0 || currentIndex >= len(m.filesToProcess) {
			loggy.Error("Invalid index for file processing", "index", currentIndex, "total", len(m.filesToProcess))
			// This indicates a logic error, potentially finish processing
			return embedGenerationMsg{} // Signal to move to next phase
		}

		app := m.app
		ctx := m.ctx
		workspace := m.workspace
		path := m.filesToProcess[currentIndex]
		relPath, _ := filepath.Rel(workspace.Path, path)

		loggy.Debug("Processing file", "index", currentIndex+1, "of", m.totalFiles, "path", relPath)

		// Use RefreshFile which parses and returns chunks
		file, chunks, err := app.Workspace.RefreshFile(ctx, workspace.ID, path)
		if err != nil {
			loggy.Warn("Failed to process file", "path", relPath, "error", err)
			// Send message indicating failure for this file, but allow process to continue
			return fileProcessedMsg{ // Defined in messages.go
				progressCurrent: currentIndex + 1,
				progressTotal:   m.totalFiles,
				error:           fmt.Errorf("failed processing %s: %w", relPath, err),
			}
		}

		loggy.Debug("Successfully parsed file", "path", relPath, "chunks", len(chunks), "file_id", file.ID)

		return fileProcessedMsg{
			file:            file,
			chunks:          chunks,
			progressCurrent: currentIndex + 1,
			progressTotal:   m.totalFiles,
		}
	}
}

// generateEmbeddings processes all collected chunks to generate embeddings.
// It returns a command that signals completion (reviewStartMsg) or failure (statusChangeMsg).
func generateEmbeddings(m Model) tea.Cmd {
	return func() tea.Msg {
		if m.workspace == nil {
			return statusChangeMsg{error: fmt.Errorf("workspace not initialized for embedding generation")}
		}

		ctx := m.ctx
		app := m.app

		if len(m.allChunks) == 0 {
			loggy.Debug("No chunks collected, skipping embedding generation.")
			return reviewStartMsg{} // Move to next phase
		}

		loggy.Info("Starting embedding generation", "chunks_count", len(m.allChunks))

		batchSize := app.Config.RAG.BatchSize // Use configured batch size
		if batchSize <= 0 {
			batchSize = 20 // Default batch size
		}

		var processingError error
		successfulChunks := 0
		totalBatches := (len(m.allChunks) + batchSize - 1) / batchSize

		for i := 0; i < len(m.allChunks); i += batchSize {
			end := i + batchSize
			if end > len(m.allChunks) {
				end = len(m.allChunks)
			}
			batch := m.allChunks[i:end]
			currentBatchNum := i/batchSize + 1

			loggy.Debug("Processing embedding batch", "batch", currentBatchNum, "of", totalBatches, "size", len(batch))

			err := app.RAG.ProcessChunks(ctx, batch)
			if err != nil {
				loggy.Warn("Error processing embedding batch", "batch", currentBatchNum, "error", err)
				// Store the first error encountered, but continue processing other batches
				if processingError == nil {
					processingError = err
				}
			} else {
				successfulChunks += len(batch)
				loggy.Debug("Successfully processed batch", "batch", currentBatchNum, "chunks", len(batch))
			}
		}

		if successfulChunks > 0 {
			loggy.Info("Embedding generation complete", "successful_chunks", successfulChunks, "total_chunks", len(m.allChunks))
			if processingError != nil {
				loggy.Warn("Some errors occurred during embedding generation", "first_error", processingError)
				// Decide if partial success is okay. Proceeding for now.
			}
			return reviewStartMsg{} // Signal to start the LLM review phase
		}

		// Only reach here if successfulChunks == 0
		if processingError != nil {
			loggy.Error("Failed to process any embeddings", "error", processingError)
			return statusChangeMsg{error: fmt.Errorf("failed to generate embeddings: %w", processingError)}
		}

		// Should ideally not happen if allChunks was not empty, but handle defensively
		loggy.Warn("No embeddings processed and no error reported")
		return reviewStartMsg{} // Proceed anyway?
	}
}

// performLLMReview initiates the LLM review process for all relevant files.
// It returns a command that sends a reviewResultMsg with the final outcome.
func performLLMReview(m Model) tea.Cmd {
	return func() tea.Msg {
		if m.workspace == nil {
			return statusChangeMsg{error: fmt.Errorf("workspace not initialized for LLM review")}
		}

		ctx := m.ctx
		app := m.app
		workspace := m.workspace
		fileIDs := m.fileIDs // Use the file IDs determined during setup

		loggy.Info("Starting LLM code review analysis", "files_count", len(fileIDs))

		if len(fileIDs) == 0 {
			loggy.Warn("No file IDs available to start LLM review.")
			return statusChangeMsg{error: fmt.Errorf("no files found to review")}
		}

		// Determine review type based on model state captured during setup
		reviewType := review.ReviewTypeStaged // Default
		if m.commitHash != "" {
			reviewType = review.ReviewTypeCommit
		} else if m.branchName != "" {
			reviewType = review.ReviewTypeBranch
		}

		loggy.Debug("Creating review object", "type", reviewType, "commit", m.commitHash, "branch", m.branchName, "base_branch", m.baseBranchName)
		reviewObj, err := app.Review.CreateReview(ctx, workspace.ID, reviewType, m.commitHash, m.baseBranchName, m.branchName)
		if err != nil {
			loggy.Error("Failed to create review object", "error", err)
			return statusChangeMsg{error: fmt.Errorf("failed to create review: %w", err)}
		}
		loggy.Info("Created review object", "review_id", reviewObj.ID)

		var reviewFiles []*review.ReviewFile
		var allIssues []*review.Issue
		filesReviewedCount := 0
		// TODO: Consider parallelizing file reviews if safe and beneficial
		for i, fileID := range fileIDs {
			file, err := app.Workspace.GetFile(ctx, fileID)
			if err != nil {
				loggy.Warn("Failed to get file for LLM analysis", "file_id", fileID, "error", err)
				continue // Skip this file
			}
			relPath, _ := filepath.Rel(workspace.Path, file.Path)
			loggy.Debug("Reviewing file with LLM", "index", i+1, "of", len(fileIDs), "path", relPath)

			contentBytes, err := os.ReadFile(file.Path)
			if err != nil {
				loggy.Warn("Failed to read file content for LLM analysis", "path", relPath, "error", err)
				continue // Skip this file
			}
			content := string(contentBytes)

			// Prepare diff information string for context
			var diffInfo string
			switch reviewType {
			case review.ReviewTypeStaged:
				diffInfo = "Staged changes"
			case review.ReviewTypeCommit:
				diffInfo = fmt.Sprintf("Changes from commit %s", m.commitHash)
			case review.ReviewTypeBranch:
				diffInfo = fmt.Sprintf("Changes between branches %s...%s", m.baseBranchName, m.branchName)
			default:
				diffInfo = "Code changes"
			}

			// Call the review service for the individual file
			reviewedFile, err := app.Review.ReviewFile(ctx, reviewObj.ID, file, content, diffInfo)
			if err != nil {
				// Log error but attempt to continue with other files
				loggy.Warn("Failed to review file with LLM", "path", relPath, "error", err)
				// Mark file as failed? The review service might already do this.
				continue
			}

			filesReviewedCount++
			reviewFiles = append(reviewFiles, reviewedFile)
			loggy.Debug("LLM analysis complete for file", "path", relPath, "issues_found", reviewedFile.IssuesCount)

			// Collect issues immediately after reviewing the file
			if reviewedFile != nil && reviewedFile.IssuesCount > 0 {
				issues, err := app.Review.GetIssuesByReviewFile(ctx, reviewedFile.ID)
				if err != nil {
					loggy.Warn("Failed to retrieve issues after review", "file_id", reviewedFile.ID, "path", relPath, "error", err)
				} else {
					allIssues = append(allIssues, issues...)
					loggy.Debug("Collected issues for file", "path", relPath, "count", len(issues))
				}
			}
		}

		loggy.Debug("LLM analysis finished for all applicable files", "reviewed_count", filesReviewedCount, "total_issues", len(allIssues))

		// Complete the overall review process
		completedReview, err := app.Review.CompleteReview(ctx, reviewObj.ID)
		if err != nil {
			// Log the completion error, but still return results gathered so far
			loggy.Warn("Failed to finalize review object state", "review_id", reviewObj.ID, "error", err)
		}

		loggy.Info("Code review process complete", "review_id", reviewObj.ID)

		// Return all collected results
		return reviewResultMsg{ // Defined in messages.go
			review:      completedReview, // May be nil if CompleteReview failed
			reviewFiles: reviewFiles,
			issues:      allIssues,
		}
	}
}

// toggleIssueStatusCmd updates the validity status of an issue.
// It returns a command that sends an issueAcceptedMsg.
func toggleIssueStatusCmd(m Model, issueID string, markAsValid bool) tea.Cmd {
	return func() tea.Msg {
		ctx := m.ctx
		app := m.app

		var err error
		if markAsValid {
			loggy.Debug("Marking issue as valid", "issue_id", issueID)
			err = app.Review.MarkIssueAsValid(ctx, issueID)
		} else {
			loggy.Debug("Marking issue as invalid", "issue_id", issueID)
			err = app.Review.MarkIssueAsInvalid(ctx, issueID)
		}

		if err != nil {
			loggy.Error("Failed to update issue status", "issue_id", issueID, "mark_valid", markAsValid, "error", err)
		}

		return issueAcceptedMsg{ // Defined in messages.go
			issueID: issueID,
			success: err == nil,
			error:   err,
		}
	}
}
