package workspace

import (
	"github.com/charmbracelet/lipgloss"
)

// Theme represents the color theme for the TUI
type Theme struct {
	Primary     lipgloss.AdaptiveColor
	Secondary   lipgloss.AdaptiveColor
	Accent      lipgloss.AdaptiveColor
	Success     lipgloss.AdaptiveColor
	Warning     lipgloss.AdaptiveColor
	Error       lipgloss.AdaptiveColor
	Info        lipgloss.AdaptiveColor
	Subtle      lipgloss.AdaptiveColor
	HighlightLo lipgloss.AdaptiveColor
	HighlightMd lipgloss.AdaptiveColor
	HighlightHi lipgloss.AdaptiveColor
	Border      lipgloss.AdaptiveColor
	Text        lipgloss.AdaptiveColor
	TextDim     lipgloss.AdaptiveColor
	Background  lipgloss.AdaptiveColor
}

// GruvboxTheme creates a new Gruvbox-inspired theme
func GruvboxTheme() Theme {
	return Theme{
		// Gruvbox-inspired colors
		Primary: lipgloss.AdaptiveColor{
			Light: "#b8bb26", Dark: "#b8bb26",
		},
		Secondary: lipgloss.AdaptiveColor{
			Light: "#fe8019", Dark: "#fe8019",
		},
		Accent: lipgloss.AdaptiveColor{
			Light: "#d3869b", Dark: "#d3869b",
		},
		Success: lipgloss.AdaptiveColor{
			Light: "#98971a", Dark: "#b8bb26",
		},
		Warning: lipgloss.AdaptiveColor{
			Light: "#d79921", Dark: "#fabd2f",
		},
		Error: lipgloss.AdaptiveColor{
			Light: "#cc241d", Dark: "#fb4934",
		},
		Info: lipgloss.AdaptiveColor{
			Light: "#458588", Dark: "#83a598",
		},
		Subtle: lipgloss.AdaptiveColor{
			Light: "#928374", Dark: "#7c6f64",
		},
		HighlightLo: lipgloss.AdaptiveColor{
			Light: "#d5c4a1", Dark: "#3c3836",
		},
		HighlightMd: lipgloss.AdaptiveColor{
			Light: "#bdae93", Dark: "#504945",
		},
		HighlightHi: lipgloss.AdaptiveColor{
			Light: "#a89984", Dark: "#665c54",
		},
		Border: lipgloss.AdaptiveColor{
			Light: "#d5c4a1", Dark: "#504945",
		},
		Text: lipgloss.AdaptiveColor{
			Light: "#3c3836", Dark: "#fbf1c7",
		},
		TextDim: lipgloss.AdaptiveColor{
			Light: "#7c6f64", Dark: "#a89984",
		},
		Background: lipgloss.AdaptiveColor{
			Light: "#fbf1c7", Dark: "#282828",
		},
	}
}

// DefaultTheme is the default theme for the TUI
var DefaultTheme = GruvboxTheme()

// Styles contains predefined styles for the TUI
type Styles struct {
	StatusText       lipgloss.Style
	Title            lipgloss.Style
	Paragraph        lipgloss.Style
	Subtle           lipgloss.Style
	Error            lipgloss.Style
	Success          lipgloss.Style
	Warning          lipgloss.Style
	Info             lipgloss.Style
	Banner           lipgloss.Style // May not be used here, but keep for consistency?
	Spinner          lipgloss.Style
	StatusBar        lipgloss.Style
	CodeBlock        lipgloss.Style
	HighSeverity     lipgloss.Style
	MediumSeverity   lipgloss.Style
	LowSeverity      lipgloss.Style
	InfoSeverity     lipgloss.Style
	Header           lipgloss.Style
	Border           lipgloss.Style
	Section          lipgloss.Style
	ProgressBarEmpty lipgloss.Style // Not used, but keep for potential reuse
	ProgressBarFull  lipgloss.Style // Not used
}

// DefaultStyles returns default styles for the TUI
func DefaultStyles() Styles {
	theme := DefaultTheme
	return Styles{
		StatusText:       lipgloss.NewStyle().Bold(true).Foreground(theme.Primary),
		Title:            lipgloss.NewStyle().Bold(true).Foreground(theme.Text),
		Paragraph:        lipgloss.NewStyle().Foreground(theme.Text),
		Subtle:           lipgloss.NewStyle().Foreground(theme.TextDim),
		Error:            lipgloss.NewStyle().Bold(true).Foreground(theme.Error),
		Success:          lipgloss.NewStyle().Bold(true).Foreground(theme.Success),
		Warning:          lipgloss.NewStyle().Bold(true).Foreground(theme.Warning),
		Info:             lipgloss.NewStyle().Bold(true).Foreground(theme.Info),
		Banner:           lipgloss.NewStyle().Foreground(theme.Primary),
		Spinner:          lipgloss.NewStyle().Foreground(theme.Secondary),
		StatusBar:        lipgloss.NewStyle().Bold(true).Foreground(theme.Text).Background(theme.HighlightLo).Padding(0, 1),
		CodeBlock:        lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(theme.Border).Padding(1, 2),
		HighSeverity:     lipgloss.NewStyle().Bold(true).Foreground(theme.Error),
		MediumSeverity:   lipgloss.NewStyle().Bold(true).Foreground(theme.Warning),
		LowSeverity:      lipgloss.NewStyle().Bold(true).Foreground(theme.Info),   // Mapping low to info color
		InfoSeverity:     lipgloss.NewStyle().Bold(true).Foreground(theme.Subtle), // Mapping info to subtle color
		Header:           lipgloss.NewStyle().Bold(true).Foreground(theme.Text).BorderStyle(lipgloss.RoundedBorder()).BorderForeground(theme.Border).Padding(0, 1),
		Border:           lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(theme.Border),
		Section:          lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(theme.Border).Padding(1, 2),
		ProgressBarEmpty: lipgloss.NewStyle().Foreground(theme.HighlightLo),
		ProgressBarFull:  lipgloss.NewStyle().Foreground(theme.Primary),
	}
}
