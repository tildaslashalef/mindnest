package review

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Init initializes the TUI model.
// It returns the initial command to execute, which includes
// starting the spinner and triggering the workspace initialization.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		getOrCreateWorkspace(m),
	)
}
