package sync

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tildaslashalef/mindnest/internal/app"
	"github.com/tildaslashalef/mindnest/internal/sync"
)

// Model is the Bubble Tea model for the sync TUI
type Model struct {
	app      *app.App
	dryRun   bool
	keymap   KeyMap
	help     help.Model
	spinner  spinner.Model
	progress progress.Model
	styles   Styles

	// UI state
	ready        bool
	loading      bool
	showHelp     bool
	error        string
	status       string
	width        int
	height       int
	lastProgress SyncProgressMsg
	result       *sync.SyncResult
	syncing      bool
	currentStage string
	entityTypes  []sync.EntityType

	// Item counts by type
	workspaceCount  int
	reviewCount     int
	reviewFileCount int
	issueCount      int
	fileCount       int
}

// NewModel initializes and returns a new Model
func NewModel(a *app.App, dryRun bool) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	p := progress.New(progress.WithDefaultGradient())

	styles := DefaultStyles()

	return Model{
		app:      a,
		dryRun:   dryRun,
		keymap:   DefaultKeyMap(),
		help:     help.New(),
		spinner:  s,
		progress: p,
		styles:   styles,
		loading:  true,
		ready:    false,
		showHelp: false,
		status:   "Initializing...",
	}
}

// Init initializes the model and returns the initial command
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, tea.Cmd(func() tea.Msg { return SyncStartMsg{} }))
}
