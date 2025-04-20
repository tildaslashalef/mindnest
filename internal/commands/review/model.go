package review

import (
	"context"
	"path/filepath"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/glamour"
	"github.com/tildaslashalef/mindnest/internal/app"
	"github.com/tildaslashalef/mindnest/internal/review"
	"github.com/tildaslashalef/mindnest/internal/workspace"
)

// Model represents the TUI model state.
// It holds all the necessary data for the UI.
// Note: Methods like Init, Update, View are now in separate files.
type Model struct {
	app             *app.App
	ctx             context.Context
	cancel          context.CancelFunc
	status          Status // Defined in types.go
	width           int
	height          int
	workspace       *workspace.Workspace
	reviewOptions   ReviewOptions // Defined in types.go
	review          *review.Review
	reviewFiles     []*review.ReviewFile
	currentIssueID  int             // Index for the currently viewed issue
	issues          []*review.Issue // All issues for the current review
	currentFile     int             // Progress counter for file processing
	totalFiles      int             // Total files to process
	progressPercent float64         // Overall progress percentage
	statusMessage   string          // Generic status message string
	errorMsg        string          // Holds the latest error message
	styles          Styles          // Defined in styles.go
	allChunks       []*Chunk        // Defined in types.go (alias for workspace.Chunk)
	filesToProcess  []string        // List of absolute file paths to process
	fileIDs         []string        // Corresponding file IDs from the workspace service

	// Components from bubbletea/bubbles
	viewport viewport.Model
	spinner  spinner.Model
	help     help.Model
	showHelp bool // Toggles help visibility

	// Markdown rendering
	renderer *glamour.TermRenderer

	// State tracking for multi-step processes
	fileProcessingComplete      bool
	embeddingGenerationComplete bool
	commitHash                  string // Used if reviewing a specific commit
	branchName                  string // Used if reviewing a specific branch
	baseBranchName              string // Used if reviewing a specific branch

	// Viewport readiness flag
	ready bool // Ensures viewport has dimensions before rendering
}

// NewModel creates a new TUI model with initial state.
func NewModel(application *app.App) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot

	h := help.New()
	h.ShowAll = false // Start with short help

	styles := DefaultStyles() // Defined in styles.go

	s.Style = styles.Spinner

	// Create markdown renderer
	r, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(100), // Adjust wrap width as needed
	)

	ctx, cancel := context.WithCancel(context.Background())

	// Initialize viewport - size will be set on WindowSizeMsg
	vp := viewport.New(10, 10)
	vp.Style = styles.Paragraph // Use a base style for the viewport

	return Model{
		app:           application,
		ctx:           ctx,
		cancel:        cancel,
		status:        StatusInitializing, // Defined in types.go
		spinner:       s,
		help:          h,
		showHelp:      false,
		styles:        styles,
		renderer:      r,
		viewport:      vp,
		reviewOptions: ReviewOptions{ // Defined in types.go
			// Default will be set later based on cli flags or RunWithOptions
		},
		ready: false, // Viewport isn't ready until size is set
	}
}

// SetReviewOptions updates the review options in the model.
// It also ensures the absolute path is resolved.
func (m *Model) SetReviewOptions(options ReviewOptions) {
	// Resolve absolute path if not provided
	if options.AbsPath == "" && options.TargetDir != "" {
		absPath, err := filepath.Abs(options.TargetDir)
		if err == nil {
			options.AbsPath = absPath
		} else {
			// Log or handle error? For now, fallback to original TargetDir
			options.AbsPath = options.TargetDir
			// Consider logging: loggy.Warn("Failed to get absolute path", "target", options.TargetDir, "error", err)
		}
	}
	m.reviewOptions = options
}
