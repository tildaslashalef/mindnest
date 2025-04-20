package workspace

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/tildaslashalef/mindnest/internal/app"
	ws "github.com/tildaslashalef/mindnest/internal/workspace"
)

// Model represents the state of the workspace TUI.
// Note: Init, Update, View methods are in separate files.
type Model struct {
	app          *app.App
	workspace    *ws.Workspace   // The workspace being viewed
	issues       []*ws.Issue     // Issues loaded for the workspace
	currentIssue int             // Index of the currently focused issue
	prDetails    GitHubPRDetails // Temp storage for PR submission details
	styles       Styles          // Using local styles from styles.go
	keymap       KeyMap          // Defined in keymaps.go
	lastError    string          // Store the last error message
	statusMsg    string          // Short status message for feedback
	width        int             // Terminal width
	height       int             // Terminal height
	ready        bool            // Flag indicating if UI dimensions are set
	loading      bool            // Flag indicating data is being loaded
	showHelp     bool            // Flag to toggle help view
	prSubmitting bool            // Flag indicating PR submission is in progress
	prError      string          // Error message specific to PR submission

	// UI Components
	help              help.Model
	viewport          viewport.Model // For displaying issue details
	spinner           spinner.Model
	renderer          *glamour.TermRenderer // For markdown rendering
	prInput           textinput.Model       // Input field for PR number
	textInput         textarea.Model        // Input field for review comment text
	showGitHubModal   bool                  // Flag to show/hide PR submission modal
	editingReviewText bool                  // Flag indicating which modal field is active
}

// NewModel creates a new workspace TUI model.
func NewModel(a *app.App, initialWs *ws.Workspace) Model {
	keymap := DefaultKeyMap()
	help := help.New()

	s := spinner.New()
	s.Spinner = spinner.Dot
	styles := DefaultStyles() // Use local DefaultStyles
	s.Style = styles.Spinner

	r, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(100),
	)

	prInput := textinput.New()
	prInput.Placeholder = "Enter PR number"
	prInput.CharLimit = 10
	prInput.Width = 20
	prInput.Prompt = "> "
	prInput.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#b8bb26"))
	prInput.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ebdbb2"))

	textInput := textarea.New()
	textInput.Placeholder = "Review text... (Tab to switch, Enter in PR# field to submit, Esc to cancel)"
	// Dimensions will be set later based on window size
	textInput.CharLimit = 5000
	textInput.ShowLineNumbers = false

	return Model{
		app:          a,
		workspace:    initialWs, // Store the initial workspace
		currentIssue: 0,
		loading:      true, // Start in loading state
		keymap:       keymap,
		help:         help,
		spinner:      s,
		renderer:     r,
		styles:       styles,
		prInput:      prInput,
		textInput:    textInput,
		ready:        false, // Not ready until size is known
	}
}

// Init initializes the model - now implicitly handled by bubbletea calling Update.
// The initial load command is returned here.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		loadWorkspaceData(m), // Trigger initial data load (func in commands.go)
	)
}
