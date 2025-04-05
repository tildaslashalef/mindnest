// Package tui provides a terminal user interface for Mindnest
package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/tildaslashalef/mindnest/internal/app"
	"github.com/tildaslashalef/mindnest/internal/loggy"
	"github.com/tildaslashalef/mindnest/internal/review"
	"github.com/tildaslashalef/mindnest/internal/workspace"
)

// Chunk is an alias for workspace.Chunk to avoid import issues
type Chunk = workspace.Chunk

// Status represents the current status of the TUI
type Status int

const (
	// StatusInitializing is the initial status
	StatusInitializing Status = iota
	// StatusInit is the status when the TUI is initializing
	StatusInit
	// StatusWorkspaceInit is the status when the workspace is being initialized
	StatusWorkspaceInit
	// StatusReady is the status when the TUI is ready for user input
	StatusReady
	// StatusReviewing is the status when a review is in progress
	StatusReviewing
	// StatusProcessingFiles is the status when processing files
	StatusProcessingFiles
	// StatusGeneratingEmbeddings is the status when generating embeddings
	StatusGeneratingEmbeddings
	// StatusAnalyzingCode is the status when the LLM is analyzing code
	StatusAnalyzingCode
	// StatusViewingReview is the status when viewing a review
	StatusViewingReview
	// StatusError is the status when an error occurred
	StatusError
)

// ReviewOptions contains options for performing a review
type ReviewOptions struct {
	TargetDir   string
	Staged      bool
	CommitHash  string
	Branch      string
	BaseBranch  string
	AbsPath     string
	WorkspaceID string
}

// Model represents the TUI model
type Model struct {
	app             *app.App
	ctx             context.Context
	cancel          context.CancelFunc
	status          Status
	width           int
	height          int
	workspace       *workspace.Workspace
	reviewOptions   ReviewOptions
	review          *review.Review
	reviewFiles     []*review.ReviewFile
	currentIssueID  int
	issues          []*review.Issue
	currentFile     int
	totalFiles      int
	progressPercent float64
	statusMessage   string
	viewport        viewport.Model
	spinner         spinner.Model
	help            help.Model
	showHelp        bool
	errorMsg        string
	styles          Styles
	allChunks       []*Chunk
	filesToProcess  []string
	fileIDs         []string

	// Track step-by-step progress
	fileProcessingComplete      bool
	embeddingGenerationComplete bool
	commitHash                  string
	branchName                  string
	baseBranchName              string

	// Add glamour renderer for markdown
	renderer *glamour.TermRenderer

	// New field to track viewport readiness
	ready bool
}

// KeyMap defines the key bindings for the TUI
type KeyMap struct {
	Help        key.Binding
	Quit        key.Binding
	StartReview key.Binding
	NextIssue   key.Binding
	PrevIssue   key.Binding
	AcceptFix   key.Binding
}

// DefaultKeyMap returns the default key map
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		StartReview: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "start review"),
		),
		NextIssue: key.NewBinding(
			key.WithKeys("n", "right"),
			key.WithHelp("n", "next issue"),
		),
		PrevIssue: key.NewBinding(
			key.WithKeys("p", "left"),
			key.WithHelp("p", "previous issue"),
		),
		AcceptFix: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "accept/unaccept issue"),
		),
	}
}

// Keys is a global instance of the keymap for use in the model
var Keys = DefaultKeyMap()

// ShortHelp returns the short help text
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit, k.NextIssue, k.PrevIssue}
}

// FullHelp returns the full help text
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Help, k.Quit},
		{k.StartReview, k.NextIssue, k.PrevIssue, k.AcceptFix},
	}
}

// NewModel creates a new TUI model
func NewModel(application *app.App) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot

	h := help.New()
	h.ShowAll = false

	styles := DefaultStyles()

	s.Style = styles.Spinner

	// Create markdown renderer with dark theme optimized for terminal
	r, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(100),
	)

	ctx, cancel := context.WithCancel(context.Background())

	return Model{
		app:      application,
		ctx:      ctx,
		cancel:   cancel,
		status:   StatusInitializing,
		spinner:  s,
		help:     h,
		showHelp: false,
		styles:   styles,
		renderer: r,
		reviewOptions: ReviewOptions{
			Staged: true, // Default to staged changes
		},
		ready: true, // Initialize viewport readiness
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		tea.Sequence(
			waitForInitToFinish,
		),
	)
}

// SetReviewOptions sets the review options
func (m *Model) SetReviewOptions(options ReviewOptions) {
	// Ensure staged is true by default when no other review mode is specified
	if options.CommitHash == "" && options.Branch == "" {
		options.Staged = true
	}

	// Make sure we have an absolute path
	if options.AbsPath == "" && options.TargetDir != "" {
		absPath, err := filepath.Abs(options.TargetDir)
		if err == nil {
			options.AbsPath = absPath
		} else {
			options.AbsPath = options.TargetDir // Fallback to using target dir as is
		}
	}

	m.reviewOptions = options
}

// waitForInitToFinish is a command that simulates initialization
func waitForInitToFinish() tea.Msg {
	return statusChangeMsg{
		newStatus: StatusInit,
	}
}

// statusChangeMsg is a message for status changes
type statusChangeMsg struct {
	newStatus Status
	error     error
}

// workspaceMsg is a message for when workspace is loaded
type workspaceMsg struct {
	workspace *workspace.Workspace
	error     error
}

// reviewResultMsg is a message for when review is complete
type reviewResultMsg struct {
	review      *review.Review
	reviewFiles []*review.ReviewFile
	issues      []*review.Issue
	error       error
}

// fileProcessedMsg is a message for when a file is processed
type fileProcessedMsg struct {
	file            *workspace.File
	chunks          []*Chunk
	progressCurrent int
	progressTotal   int
	error           error
}

// startReviewProcessMsg is a message to start the review process
type startReviewProcessMsg struct{}

// continueProcessingMsg is a message for continuing processing
type continueProcessingMsg struct {
	index int
}

// embedGenerationMsg is a message for embedding generation
type embedGenerationMsg struct{}

// reviewStartMsg is a message for starting code review
type reviewStartMsg struct{}

// issueAcceptedMsg is a message for when an issue is accepted
type issueAcceptedMsg struct {
	issueID string
	success bool
	error   error
}

// reviewSetupMsg is a message for setting up the review data
type reviewSetupMsg struct {
	fileIDs        []string
	filesToProcess []string
	totalFiles     int
	commitHash     string
	branchName     string
	baseBranchName string
}

// Update updates the model based on messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, Keys.Quit):
			// Clean up when quitting
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit

		case key.Matches(msg, Keys.Help):
			m.showHelp = !m.showHelp
			return m, nil

		case key.Matches(msg, Keys.StartReview):
			if m.status == StatusReady {
				// Start the review process
				m.status = StatusReviewing
				return m, startReviewProcess(m)
			}

		case key.Matches(msg, Keys.NextIssue):
			if m.status == StatusViewingReview && len(m.issues) > 0 {
				m.currentIssueID = (m.currentIssueID + 1) % len(m.issues)

				// Update viewport content when changing issues
				if m.viewport.Height > 0 {
					m.viewport.SetContent(m.getReviewView())
					m.viewport.GotoTop() // Reset scroll position
				}

				return m, nil
			}

		case key.Matches(msg, Keys.PrevIssue):
			if m.status == StatusViewingReview && len(m.issues) > 0 {
				m.currentIssueID = (m.currentIssueID - 1 + len(m.issues)) % len(m.issues)

				// Update viewport content when changing issues
				if m.viewport.Height > 0 {
					m.viewport.SetContent(m.getReviewView())
					m.viewport.GotoTop() // Reset scroll position
				}

				return m, nil
			}

		case key.Matches(msg, Keys.AcceptFix):
			if m.status == StatusViewingReview && len(m.issues) > 0 {
				issue := m.issues[m.currentIssueID]
				// Toggle the issue status
				return m, m.toggleIssueStatus(issue.ID, !issue.IsValid)
			}

		// Add case for up/down keys to scroll viewport
		case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
			if m.status == StatusViewingReview {
				m.viewport.LineUp(1)
				return m, nil
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
			if m.status == StatusViewingReview {
				m.viewport.LineDown(1)
				return m, nil
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("pgup"))):
			if m.status == StatusViewingReview {
				m.viewport.HalfViewUp()
				return m, nil
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("pgdown"))):
			if m.status == StatusViewingReview {
				m.viewport.HalfViewDown()
				return m, nil
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("home"))):
			if m.status == StatusViewingReview {
				m.viewport.GotoTop()
				return m, nil
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("end"))):
			if m.status == StatusViewingReview {
				m.viewport.GotoBottom()
				return m, nil
			}
		}

	case startReviewProcessMsg:
		// Start spinner for the review process
		return m, tea.Batch(
			m.spinner.Tick,
			performReview(m),
		)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		if !m.ready {
			// Initialize viewport now that we have the window dimensions
			headerHeight := 12 // Space for workspace info and issue title
			footerHeight := 3  // Space for help text
			viewportHeight := msg.Height - headerHeight - footerHeight

			// Ensure minimum viewport height
			if viewportHeight < 10 {
				viewportHeight = 10
			}

			m.viewport = viewport.New(msg.Width, viewportHeight)
			m.viewport.YPosition = headerHeight
			m.viewport.HighPerformanceRendering = false // Enable text selection
			m.ready = true

			// Set initial content if we have issues
			if len(m.issues) > 0 {
				m.viewport.SetContent(m.getReviewView())
			}
		} else {
			// Update viewport dimensions when window is resized
			headerHeight := 12
			footerHeight := 3
			viewportHeight := msg.Height - headerHeight - footerHeight

			if viewportHeight < 10 {
				viewportHeight = 10
			}

			m.viewport.Width = msg.Width
			m.viewport.Height = viewportHeight
		}

		// Update help view width
		m.help.Width = msg.Width
		return m, nil

	case statusChangeMsg:
		m.status = msg.newStatus
		if msg.error != nil {
			m.status = StatusError
			m.errorMsg = msg.error.Error()
			return m, nil
		}

		switch msg.newStatus {
		case StatusInit:
			return m, tea.Sequence(
				waitForWorkspaceInit,
			)

		case StatusWorkspaceInit:
			// Initialize the workspace
			return m, getOrCreateWorkspace(m)

		case StatusReviewing:
			// Start the review process with a spinner
			return m, tea.Batch(
				m.spinner.Tick,
				performReview(m),
			)
		}

	case workspaceMsg:
		if msg.error != nil {
			m.status = StatusError
			m.errorMsg = fmt.Sprintf("Failed to initialize workspace: %v", msg.error)
			return m, nil
		}

		m.workspace = msg.workspace
		m.reviewOptions.WorkspaceID = msg.workspace.ID
		m.status = StatusReady

		return m, nil

	case fileProcessedMsg:
		if msg.error != nil {
			loggy.Warn("Error processing file", "error", msg.error)
		} else {
			// Add chunks to model if they exist
			if msg.chunks != nil && len(msg.chunks) > 0 {
				if m.allChunks == nil {
					m.allChunks = make([]*Chunk, 0)
				}

				loggy.Debug("Adding chunks to model",
					"file", msg.file.Path,
					"chunks_count", len(msg.chunks),
					"current_all_chunks", len(m.allChunks))

				m.allChunks = append(m.allChunks, msg.chunks...)

				loggy.Debug("Updated chunks in model",
					"new_total_chunks", len(m.allChunks))
			} else {
				loggy.Debug("No chunks found in file", "file", msg.file.Path)
			}
		}

		m.currentFile = msg.progressCurrent
		m.totalFiles = msg.progressTotal

		// Calculate progress percentage for status only
		percent := float64(msg.progressCurrent) / float64(msg.progressTotal)
		m.progressPercent = percent

		// Update with a simplified progress message (only used for logs)
		//m.statusMessage = fmt.Sprintf("Processing file %d of %d", m.currentFile, m.totalFiles)

		// Log detailed message for debugging
		if msg.file != nil {
			loggy.Debug("Processing file",
				"current", msg.progressCurrent,
				"total", msg.progressTotal,
				"path", msg.file.Path)
		} else {
			loggy.Debug("Processing file (no name)",
				"current", msg.progressCurrent,
				"total", msg.progressTotal)
		}

		// Continue processing next file or move to next stage
		if msg.progressCurrent < msg.progressTotal {
			// Process next file after a short delay to allow UI updates
			return m, func() tea.Msg {
				// Wait a moment to update the UI
				time.Sleep(10 * time.Millisecond)
				return continueProcessingMsg{index: msg.progressCurrent}
			}
		}

		// All files processed, move to next stage
		loggy.Debug("All files processed, moving to embedding generation", "total_chunks", len(m.allChunks))
		return m, func() tea.Msg {
			// Wait a moment to update the UI
			time.Sleep(100 * time.Millisecond)
			return embedGenerationMsg{}
		}

	case continueProcessingMsg:
		// When starting processing, initialize model state
		if msg.index == 0 {
			m.status = StatusProcessingFiles
			// Make sure we have a fresh empty array of chunks
			m.allChunks = make([]*Chunk, 0)
			loggy.Info("Starting code review process in TUI mode")

			// Log files count to verify state
			loggy.Debug("Starting file processing with",
				"files_count", len(m.filesToProcess),
				"files", m.filesToProcess)

			// If we have no files to process, something went wrong
			if len(m.filesToProcess) == 0 {
				// Sanity check - should not happen if performReview worked correctly
				loggy.Error("No files to process when starting file processing",
					"file_ids_len", len(m.fileIDs))
				return m, func() tea.Msg {
					return reviewResultMsg{
						error: fmt.Errorf("no files to process - verify you have staged changes"),
					}
				}
			}
		}

		// Continue processing files
		return m, performContinueProcessing(m, msg.index)

	case embedGenerationMsg:
		// Start embedding generation
		m.status = StatusGeneratingEmbeddings
		// Mark file processing as complete
		m.fileProcessingComplete = true
		loggy.Debug("Starting embedding generation phase", "chunks_count", len(m.allChunks))

		// Ensure we have a valid allChunks array
		if m.allChunks == nil {
			m.allChunks = make([]*Chunk, 0)
			loggy.Warn("allChunks was nil when starting embedding generation")
		}

		return m, tea.Batch(
			m.spinner.Tick,
			performEmbeddingGeneration(m),
		)

	case reviewStartMsg:
		// Start the code review phase
		m.status = StatusAnalyzingCode
		// Mark embedding generation as complete
		m.embeddingGenerationComplete = true
		loggy.Debug("Starting code review analysis phase")

		return m, tea.Batch(
			m.spinner.Tick,
			performReviewStart(m),
		)

	case reviewResultMsg:
		if msg.error != nil {
			loggy.Error("Failed to complete review", "error", msg.error)
			m.status = StatusError
			m.errorMsg = fmt.Sprintf("Failed to complete review: %v", msg.error)
			return m, nil
		}

		loggy.Info("Review completed successfully",
			"review_id", msg.review.ID,
			"files", len(msg.reviewFiles),
			"issues", len(msg.issues))

		m.status = StatusViewingReview
		m.review = msg.review
		m.reviewFiles = msg.reviewFiles
		m.issues = msg.issues
		m.currentIssueID = 0

		// Initialize viewport with content
		headerHeight := 12
		footerHeight := 3
		viewportHeight := m.height - headerHeight - footerHeight

		if viewportHeight < 10 {
			viewportHeight = 10
		}

		m.viewport = viewport.New(m.width, viewportHeight)
		m.viewport.YPosition = headerHeight
		m.viewport.HighPerformanceRendering = false

		// Set initial content
		if len(m.issues) > 0 {
			m.viewport.SetContent(m.getReviewView())
		}

		return m, nil

	case spinner.TickMsg:
		var spinnerCmd tea.Cmd
		m.spinner, spinnerCmd = m.spinner.Update(msg)
		return m, spinnerCmd

	case issueAcceptedMsg:
		if msg.error != nil {
			loggy.Warn("Error toggling issue status", "issue_id", msg.issueID, "error", msg.error)
			m.statusMessage = fmt.Sprintf("Error toggling issue status: %v", msg.error)
		} else {
			// Update the UI to show the issue has been toggled
			loggy.Debug("Issue status toggled", "issue_id", msg.issueID)

			// Update the issue status in our local copy
			if m.currentIssueID < len(m.issues) {
				issue := m.issues[m.currentIssueID]
				issue.IsValid = !issue.IsValid // Toggle the status
				if issue.IsValid {
					m.statusMessage = "Issue accepted"
				} else {
					m.statusMessage = "Issue unaccepted"
				}
				// Update viewport content to reflect the new status
				m.viewport.SetContent(m.getReviewView())
			}
		}
		return m, nil

	case reviewSetupMsg:
		// Store the file data in the model
		m.fileIDs = msg.fileIDs
		m.filesToProcess = msg.filesToProcess
		m.totalFiles = msg.totalFiles
		m.currentFile = 0
		m.commitHash = msg.commitHash
		m.branchName = msg.branchName
		m.baseBranchName = msg.baseBranchName

		loggy.Debug("Review setup complete",
			"file_ids", len(m.fileIDs),
			"files_to_process", len(m.filesToProcess),
			"commit_hash", m.commitHash,
			"branch_name", m.branchName,
			"base_branch", m.baseBranchName,
			"files", m.filesToProcess)

		// Start file processing
		if len(m.filesToProcess) > 0 {
			return m, func() tea.Msg {
				return continueProcessingMsg{
					index: 0,
				}
			}
		}

		return m, func() tea.Msg {
			return reviewResultMsg{
				error: fmt.Errorf("no files to process after filtering"),
			}
		}
	}

	return m, tea.Batch(cmds...)
}

// waitForWorkspaceInit is a command that initializes the workspace
func waitForWorkspaceInit() tea.Msg {
	return statusChangeMsg{
		newStatus: StatusWorkspaceInit,
	}
}

// getOrCreateWorkspace is a command that gets or creates the workspace
func getOrCreateWorkspace(m Model) tea.Cmd {
	return func() tea.Msg {
		// Get current workspace
		workspace, err := m.app.Workspace.GetCurrentWorkspace(m.ctx, m.app.Config)
		if err != nil {
			if err.Error() == "workspace not found" || err.Error() == "no config file found" {
				// Create a new workspace
				cwd, _ := filepath.Abs(m.reviewOptions.TargetDir)
				dirName := filepath.Base(cwd)

				workspace, err = m.app.Workspace.CreateWorkspace(m.ctx, cwd, dirName, m.app.Config, "", "")
				if err != nil {
					return workspaceMsg{
						error: fmt.Errorf("failed to create workspace: %w", err),
					}
				}

				loggy.Info("Created new workspace", "name", workspace.Name, "path", workspace.Path)
			} else {
				return workspaceMsg{
					error: fmt.Errorf("error accessing workspace: %w", err),
				}
			}
		}

		return workspaceMsg{
			workspace: workspace,
		}
	}
}

// startReviewProcess begins the review process
func startReviewProcess(m Model) tea.Cmd {
	return func() tea.Msg {
		return startReviewProcessMsg{}
	}
}

// performReview performs the actual code review
func performReview(m Model) tea.Cmd {
	return func() tea.Msg {
		if m.workspace == nil {
			return reviewResultMsg{
				error: fmt.Errorf("workspace not initialized"),
			}
		}

		ctx := m.ctx
		app := m.app
		workspace := m.workspace

		// Resolve the target directory
		absPath := m.reviewOptions.AbsPath
		if absPath == "" {
			wd, err := os.Getwd()
			if err != nil {
				return reviewResultMsg{
					error: fmt.Errorf("failed to get current directory: %w", err),
				}
			}
			absPath = wd
		}

		// Validate review options
		staged := m.reviewOptions.Staged
		commitHash := m.reviewOptions.CommitHash
		branch := m.reviewOptions.Branch

		if (staged && (commitHash != "" || branch != "")) ||
			(commitHash != "" && branch != "") {
			return reviewResultMsg{
				error: fmt.Errorf("invalid review options"),
			}
		}

		// Find files to process based on review mode
		var fileIDs []string
		var err error

		if staged || commitHash != "" || branch != "" {
			// Check if directory has a git repository using our service
			gitRepoExists := app.Workspace.HasGitRepo(absPath)
			if !gitRepoExists {
				return reviewResultMsg{
					error: fmt.Errorf("no Git repository found in %s", absPath),
				}
			}
		}

		// Store commit or branch information for diffInfo
		currentCommitHash := ""
		currentBranchName := ""

		if staged {
			loggy.Debug("Processing staged changes")
			fileIDs, err = app.Workspace.ParseStagedChanges(ctx, workspace.ID, absPath)
		} else if commitHash != "" {
			loggy.Debug("Processing commit changes", "hash", commitHash)
			fileIDs, err = app.Workspace.ParseCommitChanges(ctx, workspace.ID, absPath, commitHash)
			currentCommitHash = commitHash
		} else if branch != "" {
			loggy.Debug("Processing branch diff", "branch", branch, "base", m.reviewOptions.BaseBranch)
			fileIDs, err = app.Workspace.ParseBranchChanges(ctx, workspace.ID, absPath, m.reviewOptions.BaseBranch, branch)
			currentBranchName = branch
		}

		if err != nil {
			return reviewResultMsg{
				error: fmt.Errorf("failed to parse changes: %w", err),
			}
		}

		// Check if we found any files
		if len(fileIDs) == 0 {
			// No valid files found from processing the provided paths
			loggy.Warn("No files found from paths, check if your files are properly staged or selected")
			return reviewResultMsg{
				error: fmt.Errorf("no valid files found for review - make sure your changes are properly staged"),
			}
		}

		loggy.Debug("Retrieved file IDs for review", "count", len(fileIDs), "file_ids", fileIDs)

		// Get files by IDs to get their paths
		var filesToProcess []string
		for _, fileID := range fileIDs {
			file, err := app.Workspace.GetFile(ctx, fileID)
			if err != nil {
				loggy.Warn("Failed to get file", "file_id", fileID, "error", err)
				continue
			}
			filesToProcess = append(filesToProcess, file.Path)
		}

		// Return the data directly in a message to create a new model
		// This avoids the problem of the model fields not being properly updated
		return reviewSetupMsg{
			fileIDs:        fileIDs,
			filesToProcess: filesToProcess,
			totalFiles:     len(filesToProcess),
			commitHash:     currentCommitHash,
			branchName:     currentBranchName,
			baseBranchName: m.reviewOptions.BaseBranch,
		}
	}
}

// View renders the UI
func (m Model) View() string {
	// Get the view based on the current status
	var content string
	var footer string

	switch m.status {
	case StatusInitializing, StatusInit, StatusWorkspaceInit:
		content = m.getInitializingView()
	case StatusReady:
		content = m.getReadyView()
		footer = m.help.View(Keys)
	case StatusReviewing, StatusProcessingFiles, StatusGeneratingEmbeddings, StatusAnalyzingCode:
		content = m.getReviewingView()
	case StatusViewingReview:
		if len(m.issues) == 0 {
			content = lipgloss.JoinVertical(lipgloss.Left,
				"",
				m.styles.Success.Render("✅ No issues found! Code looks good."),
				"",
				m.styles.Paragraph.Render("The code passed the review criteria. Good job!"),
			)
		} else {
			// Get header info
			headerInfo := m.getReviewHeaderInfo()

			// Get viewport content (this contains the issue details)
			viewportContent := m.viewport.View()

			// Get footer info
			footerInfo := m.getReviewFooterInfo()

			// Combine all components with proper spacing
			content = lipgloss.JoinVertical(lipgloss.Left,
				headerInfo,
				"", // Add spacing
				viewportContent,
				"", // Add spacing
				footerInfo,
			)
		}
		footer = m.help.View(Keys)
	case StatusError:
		content = m.getErrorView()
	}

	// Build the final view with banner and content
	banner := renderBanner(m.styles)
	return lipgloss.JoinVertical(lipgloss.Left,
		banner,
		content,
		footer,
	)
}

// getStatusLine returns the status line
func (m Model) getStatusLine() string {
	var statusText string

	switch m.status {
	case StatusInitializing, StatusInit, StatusWorkspaceInit:
		statusText = "Initializing..."
	case StatusReady:
		if m.workspace != nil {
			statusText = fmt.Sprintf("Ready - Workspace: %s", m.workspace.Name)
		} else {
			statusText = "Ready"
		}
	case StatusReviewing:
		statusText = "Starting review..."
	case StatusProcessingFiles:
		statusText = fmt.Sprintf("Processing files (%d/%d)", m.currentFile, m.totalFiles)
	case StatusGeneratingEmbeddings:
		statusText = "Generating embeddings..."
	case StatusAnalyzingCode:
		statusText = "Analyzing code with LLM..."
	case StatusViewingReview:
		if len(m.issues) > 0 {
			issue := m.issues[m.currentIssueID]
			if issue.IsValid {
				statusText = fmt.Sprintf("Viewing accepted issue %d of %d", m.currentIssueID+1, len(m.issues))
			} else {
				statusText = fmt.Sprintf("Viewing issue %d of %d", m.currentIssueID+1, len(m.issues))
			}
		} else {
			statusText = "No issues found"
		}
	case StatusError:
		statusText = "Error: " + m.errorMsg
	}

	// If we have a specific status message, use it
	if m.statusMessage != "" && (m.status == StatusProcessingFiles ||
		m.status == StatusGeneratingEmbeddings ||
		m.status == StatusAnalyzingCode) {
		statusText = m.statusMessage
	}

	// Create a styled status line
	return m.styles.StatusBar.Copy().Width(m.width - 4).Render(statusText)
}

// getInitializingView returns the view for initializing status
func (m Model) getInitializingView() string {
	return lipgloss.JoinVertical(lipgloss.Left,
		"",
		m.styles.Title.Render("Initializing Mindnest..."),
		"",
		lipgloss.JoinHorizontal(lipgloss.Left, m.spinner.View(), " Loading workspace..."),
	)
}

// getReadyView returns the view for ready status
func (m Model) getReadyView() string {
	workspaceInfo := ""
	if m.workspace != nil {
		workspaceInfo = m.styles.Info.Render(fmt.Sprintf("Current workspace: %s (%s)",
			m.workspace.Name,
			m.workspace.Path))
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		"",
		m.styles.Title.Render("Mindnest is ready"),
		"",
		workspaceInfo,
		"",
		m.styles.Paragraph.Render("Press 'r' to start a review"),
		m.styles.Paragraph.Render("Press 'q' to quit"),
	)
}

// getReviewingView returns the view for reviewing the code
func (m Model) getReviewingView() string {
	// Show step-by-step progress
	spinner := m.spinner.View()
	checkmark := m.styles.Success.Render("✓")

	// Build steps with appropriate indicators
	var fileProcessingStep, embeddingStep, reviewingStep string

	// Step 1: File processing
	if m.status == StatusProcessingFiles {
		fileProcessingStep = fmt.Sprintf("%s Processing files...", spinner)
	} else if m.fileProcessingComplete {
		fileProcessingStep = fmt.Sprintf("%s Files processed", checkmark)
	} else {
		fileProcessingStep = "Processing files..."
	}

	// Step 2: Embedding generation
	if m.status == StatusGeneratingEmbeddings {
		embeddingStep = fmt.Sprintf("%s Generating embeddings...", spinner)
	} else if m.embeddingGenerationComplete {
		embeddingStep = fmt.Sprintf("%s Embeddings generated", checkmark)
	} else {
		embeddingStep = "Generating embeddings..."
	}

	// Step 3: Reviewing code
	if m.status == StatusAnalyzingCode {
		reviewingStep = fmt.Sprintf("%s Reviewing code...", spinner)
	} else if m.status == StatusViewingReview {
		reviewingStep = fmt.Sprintf("%s Code review complete", checkmark)
	} else {
		reviewingStep = "Reviewing code..."
	}

	// Style the steps
	fileProcessingStep = m.styles.Info.Render(fileProcessingStep)
	embeddingStep = m.styles.Info.Render(embeddingStep)
	reviewingStep = m.styles.Info.Render(reviewingStep)

	return lipgloss.JoinVertical(lipgloss.Left,
		"",
		fileProcessingStep,
		embeddingStep,
		reviewingStep,
		"",
	)
}

// getReviewView returns the view for viewing a review
func (m Model) getReviewView() string {
	if len(m.issues) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left,
			"",
			m.styles.Success.Render("✅ No issues found! Code looks good."),
			"",
			m.styles.Paragraph.Render("The code passed the review criteria. Good job!"),
		)
	}

	issue := m.issues[m.currentIssueID]
	var content strings.Builder

	// Get file path from metadata if available
	filePath := "Unknown file"
	if issue.Metadata != nil {
		if path, ok := issue.Metadata["file_path"].(string); ok && path != "" {
			filePath = path
		}
	}

	// Issue type and severity
	var severityStyle lipgloss.Style
	switch issue.Severity {
	case review.IssueSeverityCritical, review.IssueSeverityHigh:
		severityStyle = m.styles.HighSeverity
	case review.IssueSeverityMedium:
		severityStyle = m.styles.MediumSeverity
	case review.IssueSeverityLow:
		severityStyle = m.styles.LowSeverity
	default:
		severityStyle = m.styles.InfoSeverity
	}

	// Format metadata in a clean and organized way
	metaContent := fmt.Sprintf("Type: %s    Severity: %s\n",
		string(issue.Type),
		severityStyle.Render(string(issue.Severity)))

	metaContent += fmt.Sprintf("File: %s\n", filePath)

	// Line numbers
	if issue.LineStart > 0 {
		if issue.LineEnd > issue.LineStart {
			metaContent += fmt.Sprintf("Lines: %d-%d\n", issue.LineStart, issue.LineEnd)
		} else {
			metaContent += fmt.Sprintf("Line: %d\n", issue.LineStart)
		}
	}

	metaContent += fmt.Sprintf("Created: %s", issue.CreatedAt.Format("2006-01-02 15:04:05"))

	// Style and add metadata with extra spacing
	content.WriteString(m.styles.Subtle.Render(metaContent) + "\n\n")

	// Issue description
	descriptionTitle := m.styles.Info.Render("Description")
	content.WriteString(descriptionTitle + "\n")

	// Render description with markdown
	description := issue.Description
	renderedDesc, err := m.renderer.Render(description)
	if err == nil {
		content.WriteString(renderedDesc + "\n\n")
	} else {
		content.WriteString(description + "\n\n")
	}

	// Show the suggestion
	if issue.Suggestion != "" {
		suggestionTitle := m.styles.Info.Render("Suggestion")
		content.WriteString(suggestionTitle + "\n")

		// Render suggestion with markdown
		renderedSuggestion, err := m.renderer.Render(issue.Suggestion)
		if err == nil {
			content.WriteString(renderedSuggestion + "\n\n")
		} else {
			content.WriteString(issue.Suggestion + "\n\n")
		}
	}

	// Show affected code if available
	if issue.AffectedCode != "" {
		affectedCodeTitle := m.styles.Info.Render("Affected Code")
		content.WriteString(affectedCodeTitle + "\n")

		// Use markdown code block for syntax highlighting
		codeBlock := "```go\n" + issue.AffectedCode + "\n```"
		renderedCode, err := m.renderer.Render(codeBlock)
		if err == nil {
			content.WriteString(renderedCode + "\n\n")
		} else {
			// Fallback to basic box if rendering fails
			codeStyle := m.styles.CodeBlock.Copy().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#665c54")). // Gruvbox color
				Padding(1, 2).
				Width(min(m.width-10, 80)) // Limit width but keep it reasonable
			content.WriteString(codeStyle.Render(issue.AffectedCode) + "\n\n")
		}
	}

	// Show suggested fix if available
	if issue.CodeSnippet != "" {
		suggestedFixTitle := m.styles.Info.Render("Suggested Fix")
		content.WriteString(suggestedFixTitle + "\n")

		// Use markdown code block for syntax highlighting
		codeBlock := "```go\n" + issue.CodeSnippet + "\n```"
		renderedCode, err := m.renderer.Render(codeBlock)
		if err == nil {
			content.WriteString(renderedCode + "\n\n")
		} else {
			// Fallback to basic box if rendering fails
			codeStyle := m.styles.CodeBlock.Copy().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#665c54")). // Gruvbox color
				Padding(1, 2).
				Width(min(m.width-10, 80)) // Limit width but keep it reasonable
			content.WriteString(codeStyle.Render(issue.CodeSnippet) + "\n\n")
		}
	}

	return content.String()
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// getErrorView returns the view for error status
func (m Model) getErrorView() string {
	return lipgloss.JoinVertical(lipgloss.Left,
		"",
		m.styles.Error.Render("❌ ERROR:"),
		"",
		m.styles.Paragraph.Render(m.errorMsg),
		"",
		m.styles.Paragraph.Render("Press 'q' to quit"),
	)
}

// renderBanner returns the banner text
func renderBanner(styles Styles) string {
	banner := `
███╗   ███╗██╗███╗   ██╗██████╗ ███╗   ██╗███████╗███████╗████████╗
████╗ ████║██║████╗  ██║██╔══██╗████╗  ██║██╔════╝██╔════╝╚══██╔══╝
██╔████╔██║██║██╔██╗ ██║██║  ██║██╔██╗ ██║█████╗  ███████╗   ██║   
██║╚██╔╝██║██║██║╚██╗██║██║  ██║██║╚██╗██║██╔══╝  ╚════██║   ██║   
██║ ╚═╝ ██║██║██║ ╚████║██████╔╝██║ ╚████║███████╗███████║   ██║   
╚═╝     ╚═╝╚═╝╚═╝  ╚═══╝╚═════╝ ╚═╝  ╚═══╝╚══════╝╚══════╝   ╚═╝   
`

	return styles.Banner.Render(banner)
}

// performContinueProcessing continues processing files
func performContinueProcessing(m Model, currentIndex int) tea.Cmd {
	return func() tea.Msg {
		if m.workspace == nil {
			return reviewResultMsg{
				error: fmt.Errorf("workspace not initialized"),
			}
		}

		app := m.app
		ctx := m.ctx
		workspace := m.workspace

		// Fix: Make a local copy of filesToProcess to ensure it's not modified elsewhere
		filesToProcess := m.filesToProcess

		// Debug the filesToProcess to check if they exist
		loggy.Debug("Continue processing files",
			"current_index", currentIndex,
			"files_count", len(filesToProcess),
			"files", filesToProcess)

		// Check if we're done with all files
		if currentIndex >= len(filesToProcess) {
			loggy.Debug("Finished processing all files", "total_processed", len(filesToProcess), "total_chunks", len(m.allChunks))

			// Move to embedding generation phase
			return embedGenerationMsg{}
		}

		// Process the current file
		path := filesToProcess[currentIndex]
		relPath, _ := filepath.Rel(workspace.Path, path)
		if relPath == "" {
			relPath = filepath.Base(path)
		}

		loggy.Debug("Processing file", "index", currentIndex+1, "of", len(filesToProcess), "path", path, "rel_path", relPath)

		// Use RefreshFile which will parse the file and return chunks directly
		file, chunks, err := app.Workspace.RefreshFile(ctx, workspace.ID, path)
		if err != nil {
			loggy.Warn("Failed to process file", "path", path, "rel_path", relPath, "error", err)

			// Continue to next file even if this one failed
			return fileProcessedMsg{
				progressCurrent: currentIndex + 1,
				progressTotal:   len(filesToProcess),
				error:           err,
			}
		}

		// Log successful parsing
		loggy.Debug("Successfully parsed file",
			"path", path,
			"rel_path", relPath,
			"chunks", len(chunks),
			"file_id", file.ID)

		// Add chunks to our collection - this will be handled in the Update method
		// as the m here is a copy, not a reference to the current model state
		return fileProcessedMsg{
			file:            file,
			chunks:          chunks,
			progressCurrent: currentIndex + 1,
			progressTotal:   len(filesToProcess),
		}
	}
}

// performEmbeddingGeneration starts the embedding generation process
func performEmbeddingGeneration(m Model) tea.Cmd {
	return func() tea.Msg {
		if m.workspace == nil {
			return reviewResultMsg{
				error: fmt.Errorf("workspace not initialized"),
			}
		}

		ctx := m.ctx
		app := m.app

		// If no chunks to process, skip to next phase
		if m.allChunks == nil || len(m.allChunks) == 0 {
			loggy.Debug("No chunks to process for embedding generation, skipping to next phase")
			return reviewStartMsg{}
		}

		loggy.Debug("Starting embedding generation", "chunks_count", len(m.allChunks))

		// Process chunks in smaller batches to avoid overloading the API
		batchSize := 20
		if app.Config.RAG.BatchSize > 0 {
			batchSize = app.Config.RAG.BatchSize
		}

		// Track successful chunks
		var processingError error
		successfulChunks := 0

		// Process in smaller batches for more reliability
		for i := 0; i < len(m.allChunks); i += batchSize {
			end := i + batchSize
			if end > len(m.allChunks) {
				end = len(m.allChunks)
			}

			batch := m.allChunks[i:end]
			loggy.Debug("Processing embedding batch",
				"batch", i/batchSize+1,
				"of", (len(m.allChunks)+batchSize-1)/batchSize,
				"size", len(batch))

			// Try to process the batch
			err := app.RAG.ProcessChunks(ctx, batch)
			if err != nil {
				// Log error but continue with other batches
				loggy.Warn("Error processing embedding batch",
					"batch", i/batchSize+1,
					"error", err)
				processingError = err
			} else {
				successfulChunks += len(batch)
				loggy.Debug("Successfully processed batch",
					"batch", i/batchSize+1,
					"chunks", len(batch))
			}
		}

		// Check if we had partial success
		if successfulChunks > 0 {
			loggy.Info("Successfully processed embeddings",
				"successful_chunks", successfulChunks,
				"total_chunks", len(m.allChunks))

			// Continue with what we have
			return reviewStartMsg{}
		}

		// If we had no success at all, report the error
		if processingError != nil {
			loggy.Error("Failed to process any embeddings", "error", processingError)
			return reviewResultMsg{
				error: fmt.Errorf("failed to generate embeddings: %w", processingError),
			}
		}

		// Move to next phase
		return reviewStartMsg{}
	}
}

// performReviewStart starts the code review process
func performReviewStart(m Model) tea.Cmd {
	return func() tea.Msg {
		if m.workspace == nil {
			return reviewResultMsg{
				error: fmt.Errorf("workspace not initialized"),
			}
		}

		ctx := m.ctx
		app := m.app
		workspace := m.workspace

		// Use fileIDs directly from the model
		fileIDs := m.fileIDs

		loggy.Debug("Starting code review", "file_ids_count", len(fileIDs), "files_count", len(m.filesToProcess))

		// Validate we have files to process
		if len(fileIDs) == 0 {
			return reviewResultMsg{
				error: fmt.Errorf("no files to process for review"),
			}
		}

		// Determine the review type based on the model state
		reviewType := review.ReviewTypeStaged
		if m.commitHash != "" {
			reviewType = review.ReviewTypeCommit
		} else if m.branchName != "" {
			reviewType = review.ReviewTypeBranch
		}

		// Create a new review
		reviewObj, err := app.Review.CreateReview(ctx, workspace.ID, reviewType, m.commitHash, m.baseBranchName, m.branchName)
		if err != nil {
			loggy.Error("Failed to create review", "error", err)
			return reviewResultMsg{
				error: fmt.Errorf("failed to create review: %w", err),
			}
		}

		loggy.Debug("Created review", "review_id", reviewObj.ID, "type", reviewType)

		// Analyze files and generate issues
		loggy.Debug("Starting file analysis", "review_id", reviewObj.ID, "files", len(fileIDs))

		// Prepare for file review
		var reviewFiles []*review.ReviewFile
		for i, fileID := range fileIDs {
			file, err := app.Workspace.GetFile(ctx, fileID)
			if err != nil {
				loggy.Warn("Failed to get file for analysis", "file_id", fileID, "error", err)
				continue
			}

			loggy.Debug("Reviewing file", "index", i+1, "of", len(fileIDs), "path", file.Path)

			// Read file content
			content, err := os.ReadFile(file.Path)
			if err != nil {
				loggy.Warn("Failed to read file content", "path", filepath.Base(file.Path), "error", err)
				continue
			}

			// Get diff info based on review type
			var diffInfo string
			switch reviewType {
			case review.ReviewTypeStaged:
				diffInfo = "Staged changes"
			case review.ReviewTypeCommit:
				diffInfo = fmt.Sprintf("Changes from commit %s", m.commitHash)
			case review.ReviewTypeBranch:
				diffInfo = fmt.Sprintf("Changes from '%s' to '%s' branch", m.baseBranchName, m.branchName)
			default:
				diffInfo = "Code changes"
			}

			// Review the file with content
			reviewedFile, err := app.Review.ReviewFile(ctx, reviewObj.ID, file, string(content), diffInfo)
			if err != nil {
				loggy.Warn("Failed to review file", "path", filepath.Base(file.Path), "error", err)
				continue
			}

			reviewFiles = append(reviewFiles, reviewedFile)
			loggy.Debug("File reviewed successfully", "path", file.Path, "issues", reviewedFile.IssuesCount)
		}

		// Complete the review
		loggy.Debug("Completing review", "review_id", reviewObj.ID, "files_reviewed", len(reviewFiles))
		completedReview, err := app.Review.CompleteReview(ctx, reviewObj.ID)
		if err != nil {
			loggy.Warn("Failed to complete review", "error", err)
		} else {
			loggy.Debug("Review completed successfully", "review_id", reviewObj.ID)
		}

		// Get issues from review files
		var allIssues []*review.Issue
		for _, rf := range reviewFiles {
			if rf != nil && rf.IssuesCount > 0 {
				issues, err := app.Review.GetIssuesByReviewFile(ctx, rf.ID)
				if err != nil {
					loggy.Warn("Failed to get issues", "file_id", rf.ID, "error", err)
					continue
				}
				allIssues = append(allIssues, issues...)
				loggy.Debug("Retrieved issues", "file_id", rf.ID, "issues_count", len(issues))
			}
		}

		loggy.Info("Review completed", "total_issues", len(allIssues))

		// Return result
		return reviewResultMsg{
			review:      completedReview,
			reviewFiles: reviewFiles,
			issues:      allIssues,
		}
	}
}

// toggleIssueStatus toggles an issue's valid status
func (m Model) toggleIssueStatus(issueID string, isValid bool) tea.Cmd {
	return func() tea.Msg {
		ctx := m.ctx
		app := m.app

		var err error
		if isValid {
			err = app.Review.MarkIssueAsValid(ctx, issueID)
		} else {
			err = app.Review.MarkIssueAsInvalid(ctx, issueID)
		}

		return issueAcceptedMsg{
			issueID: issueID,
			success: err == nil,
			error:   err,
		}
	}
}

// getReviewHeaderInfo returns the header info for the review view
func (m Model) getReviewHeaderInfo() string {
	if len(m.issues) == 0 {
		return ""
	}

	issue := m.issues[m.currentIssueID]

	// Create a single workspace info header with all information
	wsName := fmt.Sprintf("Workspace: %s", m.workspace.Name)
	wsPath := fmt.Sprintf("Path: %s", m.workspace.Path)
	issueCount := fmt.Sprintf("Issues: %d/%d", m.currentIssueID+1, len(m.issues))

	// Style the workspace name (normal size)
	styledName := m.styles.Title.Render(wsName)

	// Style the path (dimmed and smaller)
	styledPath := m.styles.Subtle.Copy().
		Foreground(lipgloss.Color("#928374")). // Dimmed Gruvbox color
		Render(wsPath)

	// Style the issue count
	styledCount := m.styles.Info.Render(issueCount)

	// Combine all info in a single block with proper layout
	headerContent := lipgloss.JoinVertical(
		lipgloss.Left,
		styledName,
		styledPath,
		styledCount,
	)

	// Create a single bordered box for all workspace info with all borders visible
	// Use double borders for better visibility
	headerBox := lipgloss.NewStyle().
		BorderStyle(lipgloss.DoubleBorder()).        // Use double border for prominence
		BorderForeground(lipgloss.Color("#d5c4a1")). // Gruvbox light color
		Padding(0, 2).                               // Reduce vertical padding
		Margin(0, 0).                                // No margin
		Width(min(m.width-4, 80)).                   // Reasonable width
		Render(headerContent)

	// Issue title with id number
	issueTitle := fmt.Sprintf("Issue #%d: %s", m.currentIssueID+1, issue.Title)

	// Add status badge for accepted issues
	status := ""
	if issue.IsValid {
		status = " " + m.styles.Success.Render("[ACCEPTED]")
	}

	// Create a single header with title
	issueTitleContent := m.styles.Title.Render(issueTitle) + status

	// Create bordered box for issue header
	issueTitleBox := lipgloss.NewStyle().
		BorderStyle(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color("#b8bb26")).
		Padding(0, 2).
		Margin(0, 0).
		Width(min(m.width-4, 80)).
		Render(issueTitleContent)

	return lipgloss.JoinVertical(lipgloss.Left,
		headerBox,
		"\n"+issueTitleBox,
	)
}

// getReviewFooterInfo returns the footer info for the review view
func (m Model) getReviewFooterInfo() string {
	if len(m.issues) == 0 {
		return ""
	}

	issue := m.issues[m.currentIssueID]

	// Actions footer
	var footerText string
	if issue.IsValid {
		footerText = m.styles.Subtle.Render("Press 'c' to mark as unaccepted, use n/p to navigate between issues.")
	} else {
		footerText = m.styles.Subtle.Render("Press 'c' to accept this issue, use n/p to navigate between issues.")
	}

	// Add scrolling hint
	scrollHint := m.styles.Subtle.Render("Use up/down arrows or j/k to scroll content.")

	return lipgloss.JoinVertical(lipgloss.Left,
		footerText,
		scrollHint,
	)
}
