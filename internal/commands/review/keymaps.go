package review

import (
	"github.com/charmbracelet/bubbles/key"
)

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
		// Note: Additional keys for scrolling (up, down, pgup, pgdown, home, end)
		// were handled directly in the original Update's KeyMsg switch.
		// They remain there for now but could be added to KeyMap if desired.
	}
}

// Keys is a global instance of the keymap for use in the model
var Keys = DefaultKeyMap()

// ShortHelp returns the short help text for the help bubble
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit, k.NextIssue, k.PrevIssue}
}

// FullHelp returns the full help text for the help bubble
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Help, k.Quit},
		{k.StartReview, k.NextIssue, k.PrevIssue, k.AcceptFix},
	}
}
