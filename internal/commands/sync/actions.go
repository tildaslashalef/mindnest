package sync

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tildaslashalef/mindnest/internal/loggy"
	"github.com/tildaslashalef/mindnest/internal/sync"
)

// getEntitiesToSyncCmd is a tea.Cmd that calculates entities needing sync.
// NOTE: This command might not be strictly necessary if getEntitiesToSync
// is only called internally by simulateSync/performRealSync, but kept for now.
func (m Model) getEntitiesToSyncCmd() tea.Msg {
	ctx := context.Background() // Or get from app/context if needed
	wsCount, revCount, revFileCount, issueCount, fileCount, ws, revs, revFiles, issues, files, total := m.getEntitiesToSync(ctx)
	return EntitiesToSyncMsg{
		WorkspaceCount:  wsCount,
		ReviewCount:     revCount,
		ReviewFileCount: revFileCount,
		IssueCount:      issueCount,
		FileCount:       fileCount,
		TotalEntities:   total,
		// EntityTypes not strictly needed by TUI if SyncAll handles order internally?
		// For now, pass the default order. Needs review based on SyncAll behavior.
		EntityTypes:         []sync.EntityType{sync.EntityTypeWorkspace, sync.EntityTypeReview, sync.EntityTypeReviewFile, sync.EntityTypeIssue, sync.EntityTypeFile},
		UnsyncedWorkspaces:  ws,
		UnsyncedReviews:     revs,
		UnsyncedReviewFiles: revFiles,
		UnsyncedIssues:      issues,
		UnsyncedFiles:       files,
	}
}

// startSync initiates the sync process based on dryRun flag.
func (m Model) startSync() tea.Cmd {
	loggy.Debug("startSync called", "dryRun", m.dryRun)
	ctx := context.Background() // Consider passing from app or init
	if m.dryRun {
		return func() tea.Msg {
			return m.simulateSync(ctx)
		}
	}
	return func() tea.Msg {
		return m.performRealSync(ctx)
	}
}

// simulateSync performs a dry run of the sync process, matching original logic.
func (m *Model) simulateSync(ctx context.Context) tea.Msg {
	loggy.Info("Starting sync simulation (dry run)")
	start := time.Now()

	// Get entities to simulate syncing
	wsCount, revCount, revFileCount, issueCount, fileCount,
		unsyncedWS, unsyncedRevs, unsyncedRevFiles, unsyncedIssues, unsyncedFiles,
		totalEntities := m.getEntitiesToSync(ctx)

	if totalEntities == 0 {
		loggy.Info("Dry run: Nothing to sync.")
		return SyncCompleteMsg{
			Result:          &sync.SyncResult{Success: true, TotalItems: 0, Duration: time.Since(start)},
			WorkspaceCount:  wsCount,
			ReviewCount:     revCount,
			ReviewFileCount: revFileCount,
			IssueCount:      issueCount,
			FileCount:       fileCount,
		}
	}

	result := &sync.SyncResult{
		TotalItems:   totalEntities,
		SuccessItems: 0,
		FailedItems:  0,
		Duration:     0,
		Success:      true, // Assume success for dry run
	}

	currentProgress := 0

	// Simulate Workspaces
	for i, wsID := range unsyncedWS {
		time.Sleep(50 * time.Millisecond)
		currentProgress++
		progress := float64(currentProgress) / float64(totalEntities)
		result.SuccessItems++
		// Update model state directly for UI feedback during simulation
		m.lastProgress = SyncProgressMsg{
			EntityType:    sync.EntityTypeWorkspace,
			EntityID:      wsID,
			CurrentItem:   i + 1,
			TotalItems:    len(unsyncedWS),
			ItemsSynced:   result.SuccessItems, // Use cumulative success count
			ItemsFailed:   0,
			ErrorMessage:  "",
			ProgressValue: progress,
		}
		m.currentStage = fmt.Sprintf("Syncing %s", sync.EntityTypeWorkspace)
		m.progress.SetPercent(progress)
		// NOTE: In real TUI, we'd send this msg back via tea.Cmd, but direct mutation is ok for simulation
	}

	// Simulate Reviews
	for i, revID := range unsyncedRevs {
		time.Sleep(50 * time.Millisecond)
		currentProgress++
		progress := float64(currentProgress) / float64(totalEntities)
		result.SuccessItems++
		m.lastProgress = SyncProgressMsg{
			EntityType:    sync.EntityTypeReview,
			EntityID:      revID,
			CurrentItem:   i + 1,
			TotalItems:    len(unsyncedRevs),
			ItemsSynced:   result.SuccessItems,
			ItemsFailed:   0,
			ErrorMessage:  "",
			ProgressValue: progress,
		}
		m.currentStage = fmt.Sprintf("Syncing %s", sync.EntityTypeReview)
		m.progress.SetPercent(progress)
	}

	// Simulate Review Files
	for i, rfID := range unsyncedRevFiles {
		time.Sleep(30 * time.Millisecond)
		currentProgress++
		progress := float64(currentProgress) / float64(totalEntities)
		result.SuccessItems++
		m.lastProgress = SyncProgressMsg{
			EntityType:    sync.EntityTypeReviewFile,
			EntityID:      rfID,
			CurrentItem:   i + 1,
			TotalItems:    len(unsyncedRevFiles),
			ItemsSynced:   result.SuccessItems,
			ItemsFailed:   0,
			ErrorMessage:  "",
			ProgressValue: progress,
		}
		m.currentStage = fmt.Sprintf("Syncing %s", sync.EntityTypeReviewFile)
		m.progress.SetPercent(progress)
	}

	// Simulate Issues
	for i, issID := range unsyncedIssues {
		time.Sleep(30 * time.Millisecond)
		currentProgress++
		progress := float64(currentProgress) / float64(totalEntities)
		result.SuccessItems++
		m.lastProgress = SyncProgressMsg{
			EntityType:    sync.EntityTypeIssue,
			EntityID:      issID,
			CurrentItem:   i + 1,
			TotalItems:    len(unsyncedIssues),
			ItemsSynced:   result.SuccessItems,
			ItemsFailed:   0,
			ErrorMessage:  "",
			ProgressValue: progress,
		}
		m.currentStage = fmt.Sprintf("Syncing %s", sync.EntityTypeIssue)
		m.progress.SetPercent(progress)
	}

	// Simulate Files
	for i, fileID := range unsyncedFiles {
		time.Sleep(30 * time.Millisecond)
		currentProgress++
		progress := float64(currentProgress) / float64(totalEntities)
		result.SuccessItems++
		m.lastProgress = SyncProgressMsg{
			EntityType:    sync.EntityTypeFile,
			EntityID:      fileID, // Assuming fileID is path or identifier
			CurrentItem:   i + 1,
			TotalItems:    len(unsyncedFiles),
			ItemsSynced:   result.SuccessItems,
			ItemsFailed:   0,
			ErrorMessage:  "",
			ProgressValue: progress,
		}
		m.currentStage = fmt.Sprintf("Syncing %s", sync.EntityTypeFile)
		m.progress.SetPercent(progress)
	}

	result.Duration = time.Since(start)
	result.Success = true // Ensure success is true for dry run

	// Add counts to result for final display (from original function call)
	result.WorkspaceCount = wsCount
	result.ReviewCount = revCount
	result.ReviewFileCount = revFileCount
	result.IssueCount = issueCount
	result.FileCount = fileCount

	loggy.Info("Sync simulation finished", "duration", result.Duration, "items_simulated", result.SuccessItems)
	return SyncCompleteMsg{
		Result:          result,
		WorkspaceCount:  wsCount,
		ReviewCount:     revCount,
		ReviewFileCount: revFileCount,
		IssueCount:      issueCount,
		FileCount:       fileCount,
	}
}

// performRealSync executes the actual sync process with the server, matching original logic.
func (m Model) performRealSync(ctx context.Context) tea.Msg {
	loggy.Info("Starting real sync")

	// Get counts for the final message display
	wsCount, revCount, revFileCount, issueCount, fileCount, _, _, _, _, _, totalEntities := m.getEntitiesToSync(ctx)

	if totalEntities == 0 {
		loggy.Info("Real sync: Nothing to sync.")
		return SyncCompleteMsg{
			Result:          &sync.SyncResult{Success: true, TotalItems: 0, Duration: 0},
			WorkspaceCount:  wsCount,
			ReviewCount:     revCount,
			ReviewFileCount: revFileCount,
			IssueCount:      issueCount,
			FileCount:       fileCount,
		}
	}

	// Perform actual sync using the app service method from original code
	// NOTE: SyncAll does not provide granular progress updates in the original code.
	// The UI will show a generic spinner/status during this call.
	syncResult, syncErr := m.app.Sync.SyncAll(ctx)

	loggy.Info("Real sync finished", "duration", syncResult.Duration, "error", syncErr)

	if syncErr != nil {
		loggy.Error("Sync operation failed", "error", syncErr)
		// Pass original counts with error message
		return SyncCompleteMsg{
			Result:          syncResult, // Pass potentially partial result
			Error:           fmt.Errorf("sync failed: %w", syncErr),
			WorkspaceCount:  wsCount,
			ReviewCount:     revCount,
			ReviewFileCount: revFileCount,
			IssueCount:      issueCount,
			FileCount:       fileCount,
		}
	}

	if syncResult == nil {
		loggy.Error("Sync finished with nil result and nil error")
		// Pass original counts with error message
		return SyncCompleteMsg{
			Error:           fmt.Errorf("sync finished with unknown state"),
			WorkspaceCount:  wsCount,
			ReviewCount:     revCount,
			ReviewFileCount: revFileCount,
			IssueCount:      issueCount,
			FileCount:       fileCount,
		}
	}

	loggy.Info("Sync completed successfully", "result", syncResult)
	// Pass counts from initial check, as SyncResult might not have them broken down.
	return SyncCompleteMsg{
		Result:          syncResult,
		WorkspaceCount:  wsCount,
		ReviewCount:     revCount,
		ReviewFileCount: revFileCount,
		IssueCount:      issueCount,
		FileCount:       fileCount,
	}
}

// getEntitiesToSync calculates which entities need to be synced, matching original logic.
// It combines unsynced entities and previously failed sync logs.
func (m Model) getEntitiesToSync(ctx context.Context) (
	workspaceCount int, reviewCount int, reviewFileCount int, issueCount int, fileCount int,
	unsyncedWorkspaces []string, unsyncedReviews []string, unsyncedReviewFiles []string, unsyncedIssues []string, unsyncedFiles []string,
	totalEntitiesToSync int) {

	loggy.Debug("Calculating entities to sync based on unsynced and failed logs")

	// Reset counts
	workspaceCount = 0
	reviewCount = 0
	reviewFileCount = 0
	issueCount = 0
	fileCount = 0
	totalEntitiesToSync = 0

	// --- Workspaces --- //
	unsyncedWS, err := m.app.Sync.GetUnsyncedWorkspaces(ctx, 100) // Limit might need adjustment
	if err != nil {
		loggy.Error("Error getting unsynced workspaces", "error", err)
	}
	failedWSLogs, err := m.app.Sync.GetFailedSyncLogs(ctx, sync.EntityTypeWorkspace, 100)
	if err != nil {
		loggy.Error("Error getting failed workspace sync logs", "error", err)
	}
	wsMap := make(map[string]struct{})
	for _, id := range unsyncedWS {
		wsMap[id] = struct{}{}
	}
	for _, log := range failedWSLogs {
		wsMap[log] = struct{}{}
	}
	unsyncedWorkspaces = make([]string, 0, len(wsMap))
	for id := range wsMap {
		unsyncedWorkspaces = append(unsyncedWorkspaces, id)
	}
	workspaceCount = len(unsyncedWorkspaces)
	totalEntitiesToSync += workspaceCount

	// --- Reviews --- //
	unsyncedRevMap := make(map[string]struct{})
	for _, wsID := range unsyncedWorkspaces {
		revs, err := m.app.Sync.GetUnsyncedReviews(ctx, wsID, 100)
		if err != nil {
			loggy.Error("Error getting unsynced reviews for workspace", "error", err, "workspace_id", wsID)
			continue
		}
		for _, id := range revs {
			unsyncedRevMap[id] = struct{}{}
		}
	}
	failedRevLogs, err := m.app.Sync.GetFailedSyncLogs(ctx, sync.EntityTypeReview, 100)
	if err != nil {
		loggy.Error("Error getting failed review sync logs", "error", err)
	}
	for _, log := range failedRevLogs {
		unsyncedRevMap[log] = struct{}{}
	}
	unsyncedReviews = make([]string, 0, len(unsyncedRevMap))
	for id := range unsyncedRevMap {
		unsyncedReviews = append(unsyncedReviews, id)
	}
	reviewCount = len(unsyncedReviews)
	totalEntitiesToSync += reviewCount

	// --- Review Files --- //
	unsyncedRevFileMap := make(map[string]struct{})
	for _, revID := range unsyncedReviews {
		rfs, err := m.app.Sync.GetUnsyncedReviewFiles(ctx, revID, 100)
		if err != nil {
			loggy.Error("Error getting unsynced review files for review", "error", err, "review_id", revID)
			continue
		}
		for _, id := range rfs {
			unsyncedRevFileMap[id] = struct{}{}
		}
	}
	failedRevFileLogs, err := m.app.Sync.GetFailedSyncLogs(ctx, sync.EntityTypeReviewFile, 100)
	if err != nil {
		loggy.Error("Error getting failed review file sync logs", "error", err)
	}
	for _, log := range failedRevFileLogs {
		unsyncedRevFileMap[log] = struct{}{}
	}
	unsyncedReviewFiles = make([]string, 0, len(unsyncedRevFileMap))
	for id := range unsyncedRevFileMap {
		unsyncedReviewFiles = append(unsyncedReviewFiles, id)
	}
	reviewFileCount = len(unsyncedReviewFiles)
	totalEntitiesToSync += reviewFileCount

	// --- Issues --- //
	unsyncedIssueMap := make(map[string]struct{})
	// Get issues linked to unsynced reviews
	for _, revID := range unsyncedReviews {
		issues, err := m.app.Sync.GetUnsyncedIssues(ctx, revID, 100)
		if err != nil {
			loggy.Error("Error getting unsynced issues for review", "error", err, "review_id", revID)
			continue
		}
		for _, id := range issues {
			unsyncedIssueMap[id] = struct{}{}
		}
	}
	// Get directly modified issues (not linked to a review or review unsynced)
	directIssues, err := m.app.Sync.GetUnsyncedIssues(ctx, "", 100) // Assuming empty review ID gets direct ones
	if err != nil {
		loggy.Error("Error getting directly unsynced issues", "error", err)
	}
	for _, id := range directIssues {
		unsyncedIssueMap[id] = struct{}{}
	}
	failedIssueLogs, err := m.app.Sync.GetFailedSyncLogs(ctx, sync.EntityTypeIssue, 100)
	if err != nil {
		loggy.Error("Error getting failed issue sync logs", "error", err)
	}
	for _, log := range failedIssueLogs {
		unsyncedIssueMap[log] = struct{}{}
	}
	unsyncedIssues = make([]string, 0, len(unsyncedIssueMap))
	for id := range unsyncedIssueMap {
		unsyncedIssues = append(unsyncedIssues, id)
	}
	issueCount = len(unsyncedIssues)
	totalEntitiesToSync += issueCount

	// --- Files --- //
	unsyncedFileMap := make(map[string]struct{}) // Using Path as ID for files
	// Get files linked to unsynced workspaces
	for _, wsID := range unsyncedWorkspaces {
		files, err := m.app.Sync.GetUnsyncedFiles(ctx, wsID, 100)
		if err != nil {
			loggy.Error("Error getting unsynced files for workspace", "error", err, "workspace_id", wsID)
			continue
		}
		for _, path := range files {
			unsyncedFileMap[path] = struct{}{}
		}
	}
	// Get directly modified files (if applicable)
	directFiles, err := m.app.Sync.GetUnsyncedFiles(ctx, "", 100)
	if err != nil {
		loggy.Error("Error getting directly unsynced files", "error", err)
	}
	for _, path := range directFiles {
		unsyncedFileMap[path] = struct{}{}
	}
	failedFileLogs, err := m.app.Sync.GetFailedSyncLogs(ctx, sync.EntityTypeFile, 100)
	if err != nil {
		loggy.Error("Error getting failed file sync logs", "error", err)
	}
	for _, log := range failedFileLogs {
		unsyncedFileMap[log] = struct{}{}
	}
	unsyncedFiles = make([]string, 0, len(unsyncedFileMap))
	for path := range unsyncedFileMap {
		unsyncedFiles = append(unsyncedFiles, path)
	}
	fileCount = len(unsyncedFiles)
	totalEntitiesToSync += fileCount

	loggy.Debug("Finished calculating entities", "workspaces", workspaceCount, "reviews", reviewCount, "reviewFiles", reviewFileCount, "issues", issueCount, "files", fileCount, "total", totalEntitiesToSync)
	return
}
