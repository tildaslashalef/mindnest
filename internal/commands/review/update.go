package review

import (
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/tildaslashalef/mindnest/internal/loggy"
)

// Update handles messages and updates the model state.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	// Process messages
	switch msg := msg.(type) {

	// --- Core Bubble Tea Messages ---
	case tea.WindowSizeMsg:
		// Update viewport and help dimensions
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width

		// Set viewport dimensions, typically leaving space for header/footer/status
		// Adjust vertical padding as needed for your layout
		verticalPadding := 10 // Example padding, adjust based on header/footer size
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - verticalPadding
		m.ready = true // Mark viewport as ready
		loggy.Debug("Window resized", "width", m.width, "height", m.height, "viewport_height", m.viewport.Height)
		// Update content if viewing review, as width might change rendering
		if m.status == StatusViewingReview {
			m.viewport.SetContent(m.renderReviewContent()) // Use helper from view.go
		}
		return m, nil // No command usually needed

	case tea.KeyMsg:
		// Handle key presses based on current status and key bindings
		switch {
		// Global keys
		case key.Matches(msg, Keys.Quit):
			loggy.Info("Quit key pressed, shutting down TUI")
			if m.cancel != nil {
				m.cancel() // Cancel context if necessary
			}
			return m, tea.Quit

		case key.Matches(msg, Keys.Help):
			m.showHelp = !m.showHelp
			loggy.Debug("Toggled help visibility", "show", m.showHelp)
			return m, nil

		// Keys specific to StatusReady
		case key.Matches(msg, Keys.StartReview) && m.status == StatusReady:
			loggy.Info("Start review key pressed")
			m.status = StatusReviewing
			m.statusMessage = "Starting review process..."
			cmds = append(cmds, m.spinner.Tick, prepareReviewData(m)) // Start spinner and prepare data
			return m, tea.Batch(cmds...)

		// Keys specific to StatusViewingReview
		case key.Matches(msg, Keys.NextIssue) && m.status == StatusViewingReview:
			if len(m.issues) > 0 {
				m.currentIssueID = (m.currentIssueID + 1) % len(m.issues)
				m.viewport.SetContent(m.renderReviewContent()) // Update viewport content
				m.viewport.GotoTop()                           // Reset scroll position
				loggy.Debug("Navigated to next issue", "index", m.currentIssueID)
			}
			return m, nil

		case key.Matches(msg, Keys.PrevIssue) && m.status == StatusViewingReview:
			if len(m.issues) > 0 {
				m.currentIssueID = (m.currentIssueID - 1 + len(m.issues)) % len(m.issues)
				m.viewport.SetContent(m.renderReviewContent()) // Update viewport content
				m.viewport.GotoTop()                           // Reset scroll position
				loggy.Debug("Navigated to previous issue", "index", m.currentIssueID)
			}
			return m, nil

		case key.Matches(msg, Keys.AcceptFix) && m.status == StatusViewingReview:
			if len(m.issues) > 0 && m.currentIssueID < len(m.issues) {
				issue := m.issues[m.currentIssueID]
				newStatus := !issue.IsValid // Toggle the current status
				loggy.Info("Toggle issue status key pressed", "issue_id", issue.ID, "mark_valid", newStatus)
				cmds = append(cmds, toggleIssueStatusCmd(m, issue.ID, newStatus))
				// Optimistically update UI slightly while waiting for confirmation
				m.statusMessage = fmt.Sprintf("Updating status for issue %s...", issue.ID)
				return m, tea.Batch(cmds...)
			}
			return m, nil

		// Viewport scrolling keys (only active when viewing review)
		default:
			if m.status == StatusViewingReview {
				// Pass scrolling keys to the viewport
				m.viewport, cmd = m.viewport.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

	// --- Custom Message Handling ---
	case statusChangeMsg:
		loggy.Debug("Received status change message", "new_status", msg.newStatus, "error", msg.error)
		m.status = msg.newStatus
		if msg.error != nil {
			m.status = StatusError
			m.errorMsg = msg.error.Error()
			loggy.Error("Status changed to error", "error", msg.error)
		} else {
			m.errorMsg = ""
		}
		// Optionally reset status message or set based on new status
		m.statusMessage = getDefaultStatusMessage(m.status) // Use a helper if needed
		return m, nil                                       // Usually no command needed when just changing status display

	case workspaceMsg:
		loggy.Debug("Received workspace message", "error", msg.error)
		if msg.error != nil {
			m.status = StatusError
			m.errorMsg = msg.error.Error()
			loggy.Error("Failed to get/create workspace", "error", msg.error)
		} else {
			m.workspace = msg.workspace
			m.status = StatusReady
			m.statusMessage = fmt.Sprintf("Workspace '%s' ready.", m.workspace.Name)
			loggy.Info("Workspace ready", "id", m.workspace.ID, "name", m.workspace.Name)
		}
		return m, nil

	case reviewSetupMsg:
		loggy.Debug("Received review setup message", "files_count", msg.totalFiles)
		m.fileIDs = msg.fileIDs
		m.filesToProcess = msg.filesToProcess
		m.totalFiles = msg.totalFiles
		m.commitHash = msg.commitHash
		m.branchName = msg.branchName
		m.baseBranchName = msg.baseBranchName
		m.currentFile = 0
		m.progressPercent = 0.0
		m.allChunks = []*Chunk{} // Reset chunks for new review
		m.fileProcessingComplete = false
		m.embeddingGenerationComplete = false
		// Start processing the first file
		m.status = StatusProcessingFiles
		m.statusMessage = fmt.Sprintf("Processing %d files...", m.totalFiles)
		if m.totalFiles > 0 {
			cmds = append(cmds, processNextFile(m, 0)) // Process file 0
		} else {
			// No files to process, move to next relevant state (e.g., embedding or completion?)
			loggy.Warn("Review setup complete, but no files to process.")
			m.status = StatusReady // Or StatusError? Go back to ready for now.
			m.statusMessage = "No files found for review."
		}
		return m, tea.Batch(cmds...)

	case fileProcessedMsg:
		m.currentFile = msg.progressCurrent
		if msg.progressTotal > 0 {
			m.progressPercent = float64(m.currentFile) / float64(msg.progressTotal)
		}
		if msg.error != nil {
			// Log error, but continue processing other files
			loggy.Warn("Error processing file", "file_index", m.currentFile, "error", msg.error)
			m.statusMessage = fmt.Sprintf("Error processing file %d/%d: %v", m.currentFile, msg.progressTotal, msg.error)
		} else if msg.file != nil {
			m.statusMessage = fmt.Sprintf("Processed file %d/%d: %s (%d chunks)", m.currentFile, msg.progressTotal, filepath.Base(msg.file.Path), len(msg.chunks))
			// Collect chunks
			if len(msg.chunks) > 0 {
				m.allChunks = append(m.allChunks, msg.chunks...)
			}
		}

		// Check if file processing is complete
		if m.currentFile >= msg.progressTotal {
			m.fileProcessingComplete = true
			m.status = StatusGeneratingEmbeddings
			m.statusMessage = fmt.Sprintf("Generating embeddings for %d chunks...", len(m.allChunks))
			loggy.Info("File processing complete", "total_files", msg.progressTotal, "total_chunks", len(m.allChunks))
			cmds = append(cmds, generateEmbeddings(m)) // Start embedding generation
		} else {
			// Process the next file
			cmds = append(cmds, processNextFile(m, m.currentFile))
		}
		return m, tea.Batch(cmds...)

	case embedGenerationMsg:
		// This message primarily triggers the *command* in commands.go.
		// The command function itself handles the async work.
		// We might update status here if needed before command starts.
		loggy.Debug("Embed generation command triggered")
		m.status = StatusGeneratingEmbeddings
		m.statusMessage = fmt.Sprintf("Generating embeddings for %d chunks...", len(m.allChunks))
		cmds = append(cmds, m.spinner.Tick) // Ensure spinner continues
		// The actual work starts via the command returned previously or triggered here.
		// If generateEmbeddings command was triggered by previous step, no new cmd needed here.
		// If this msg is meant to START it, then: cmds = append(cmds, generateEmbeddings(m))
		return m, tea.Batch(cmds...)

	case reviewStartMsg:
		// This message signals that embeddings (if any) are done, start LLM review.
		loggy.Info("Embedding generation complete, starting LLM analysis")
		m.embeddingGenerationComplete = true
		m.status = StatusAnalyzingCode
		m.statusMessage = "Analyzing code with LLM..."
		cmds = append(cmds, m.spinner.Tick, performLLMReview(m))
		return m, tea.Batch(cmds...)

	case reviewResultMsg:
		loggy.Debug("Received review result message", "error", msg.error)
		if msg.error != nil {
			m.status = StatusError
			m.errorMsg = msg.error.Error()
			loggy.Error("Review process failed", "error", msg.error)
		} else {
			m.review = msg.review
			m.reviewFiles = msg.reviewFiles
			m.issues = msg.issues
			m.currentIssueID = 0 // Start at the first issue
			m.status = StatusViewingReview
			loggy.Info("Review analysis complete", "review_id", msg.review.ID, "files_reviewed", len(msg.reviewFiles), "issues_found", len(msg.issues))
			if len(m.issues) == 0 {
				m.statusMessage = "Review complete: No issues found!"
			} else {
				m.statusMessage = fmt.Sprintf("Review complete: %d issues found. Displaying issue 1/%d.", len(m.issues), len(m.issues))
			}
			// Update viewport content now that we have issues
			m.viewport.SetContent(m.renderReviewContent()) // Use helper from view.go
			m.viewport.GotoTop()
		}
		return m, nil

	case issueAcceptedMsg:
		loggy.Debug("Received issue accepted message", "issue_id", msg.issueID, "success", msg.success, "error", msg.error)
		if msg.success {
			// Find the issue in the model and update its status
			found := false
			for i := range m.issues {
				if m.issues[i].ID == msg.issueID {
					m.issues[i].IsValid = !m.issues[i].IsValid // Toggle status
					m.statusMessage = fmt.Sprintf("Issue #%d status updated.", m.currentIssueID+1)
					loggy.Info("Updated issue status in model", "issue_id", msg.issueID, "is_valid", m.issues[i].IsValid)
					found = true
					// Re-render viewport content with updated status
					m.viewport.SetContent(m.renderReviewContent())
					break
				}
			}
			if !found {
				loggy.Warn("Received success message for issue not found in current list", "issue_id", msg.issueID)
				m.statusMessage = fmt.Sprintf("Failed to update status locally for issue %s", msg.issueID)
			}
		} else {
			// Handle error - display message to user?
			m.statusMessage = fmt.Sprintf("Error updating status for issue %s: %v", msg.issueID, msg.error)
			loggy.Error("Failed to update issue status via command", "issue_id", msg.issueID, "error", msg.error)
		}
		return m, nil

	// --- Spinner Message ---
	case spinner.TickMsg:
		// Only advance spinner if it's relevant to the current status
		if m.status < StatusReady || m.status == StatusReviewing || m.status == StatusProcessingFiles || m.status == StatusGeneratingEmbeddings || m.status == StatusAnalyzingCode {
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		} else {
			// If spinner is not active for the current state, don't update it or return its Tick cmd
			return m, nil
		}

		// --- Other/Unhandled Messages ---
		// Can optionally log unhandled messages
		// default:
		// 	 loggy.Trace("Unhandled message type", "type", fmt.Sprintf("%T", msg))
	}

	// --- Handle Component Updates ---
	// Update help model
	m.help, cmd = m.help.Update(msg)
	cmds = append(cmds, cmd)

	// Update viewport model (if not handled specifically for scrolling earlier)
	// This ensures messages like mouse events are passed to the viewport
	if m.status == StatusViewingReview {
		// Avoid double-updating if already handled in KeyMsg scrolling
		if _, ok := msg.(tea.KeyMsg); !ok {
			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd)
		}
	} else {
		// Pass other messages like mouse clicks even when not in review view?
		// _, cmd = m.viewport.Update(msg)
		// cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// Helper to get a default status message based on state
func getDefaultStatusMessage(status Status) string {
	switch status {
	case StatusInitializing, StatusInit:
		return "Initializing..."
	case StatusWorkspaceInit:
		return "Initializing workspace..."
	case StatusReady:
		return "Ready. Press 'r' to start review or '?' for help."
	case StatusReviewing:
		return "Starting review process..."
	case StatusProcessingFiles:
		return "Processing files..."
	case StatusGeneratingEmbeddings:
		return "Generating embeddings..."
	case StatusAnalyzingCode:
		return "Analyzing code with LLM..."
	case StatusViewingReview:
		return "Viewing review results. Use n/p to navigate issues."
	case StatusError:
		return "An error occurred. Press 'q' to quit."
	default:
		return ""
	}
}
