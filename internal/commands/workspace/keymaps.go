package workspace

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines keybindings for the workspace TUI
type KeyMap struct {
	Up        key.Binding
	Down      key.Binding
	NextIssue key.Binding
	PrevIssue key.Binding
	Help      key.Binding
	Quit      key.Binding
	Confirm   key.Binding
	GitHub    key.Binding // GitHub PR submission
}

// ShortHelp returns keybindings to show in the mini help view
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit, k.NextIssue, k.PrevIssue, k.Confirm, k.GitHub}
}

// FullHelp returns all keybindings for the help view
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Help, k.Quit},
		{k.NextIssue, k.PrevIssue, k.Confirm, k.GitHub},
	}
}

// DefaultKeyMap returns the default keybindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "esc", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		NextIssue: key.NewBinding(
			key.WithKeys("n", "right", "l"),
			key.WithHelp("n/→", "next issue"),
		),
		PrevIssue: key.NewBinding(
			key.WithKeys("p", "left", "h"),
			key.WithHelp("p/←", "previous issue"),
		),
		Confirm: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "confirm issue"),
		),
		GitHub: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "submit to GitHub PR"),
		),
	}
}
