package review

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
			Light: "#b8bb26", // Gruvbox light green
			Dark:  "#b8bb26", // Gruvbox dark green
		},
		Secondary: lipgloss.AdaptiveColor{
			Light: "#fe8019", // Gruvbox light orange
			Dark:  "#fe8019", // Gruvbox dark orange
		},
		Accent: lipgloss.AdaptiveColor{
			Light: "#d3869b", // Gruvbox light purple
			Dark:  "#d3869b", // Gruvbox dark purple
		},
		Success: lipgloss.AdaptiveColor{
			Light: "#98971a", // Gruvbox light green
			Dark:  "#b8bb26", // Gruvbox dark green
		},
		Warning: lipgloss.AdaptiveColor{
			Light: "#d79921", // Gruvbox light yellow
			Dark:  "#fabd2f", // Gruvbox dark yellow
		},
		Error: lipgloss.AdaptiveColor{
			Light: "#cc241d", // Gruvbox light red
			Dark:  "#fb4934", // Gruvbox dark red
		},
		Info: lipgloss.AdaptiveColor{
			Light: "#458588", // Gruvbox light blue
			Dark:  "#83a598", // Gruvbox dark blue
		},
		Subtle: lipgloss.AdaptiveColor{
			Light: "#928374", // Gruvbox light gray
			Dark:  "#7c6f64", // Gruvbox dark gray
		},
		HighlightLo: lipgloss.AdaptiveColor{
			Light: "#d5c4a1", // Gruvbox light bg highlights
			Dark:  "#3c3836", // Gruvbox dark bg highlights
		},
		HighlightMd: lipgloss.AdaptiveColor{
			Light: "#bdae93", // Gruvbox light bg highlights
			Dark:  "#504945", // Gruvbox dark bg highlights
		},
		HighlightHi: lipgloss.AdaptiveColor{
			Light: "#a89984", // Gruvbox light bg highlights
			Dark:  "#665c54", // Gruvbox dark bg highlights
		},
		Border: lipgloss.AdaptiveColor{
			Light: "#d5c4a1", // Gruvbox light border
			Dark:  "#504945", // Gruvbox dark border
		},
		Text: lipgloss.AdaptiveColor{
			Light: "#3c3836", // Gruvbox light text
			Dark:  "#fbf1c7", // Gruvbox dark text
		},
		TextDim: lipgloss.AdaptiveColor{
			Light: "#7c6f64", // Gruvbox light text dim
			Dark:  "#a89984", // Gruvbox dark text dim
		},
		Background: lipgloss.AdaptiveColor{
			Light: "#fbf1c7", // Gruvbox light bg
			Dark:  "#282828", // Gruvbox dark bg
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
	Banner           lipgloss.Style
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
	ProgressBarEmpty lipgloss.Style
	ProgressBarFull  lipgloss.Style
}

// DefaultStyles returns default styles for the TUI
func DefaultStyles() Styles {
	theme := DefaultTheme

	return Styles{
		StatusText: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Primary),

		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Text),

		Paragraph: lipgloss.NewStyle().
			Foreground(theme.Text),

		Subtle: lipgloss.NewStyle().
			Foreground(theme.TextDim),

		Error: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Error),

		Success: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Success),

		Warning: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Warning),

		Info: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Info),

		Banner: lipgloss.NewStyle().
			Foreground(theme.Primary),

		Spinner: lipgloss.NewStyle().
			Foreground(theme.Secondary),

		StatusBar: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Text).
			Background(theme.HighlightLo).
			PaddingLeft(1).
			PaddingRight(1),

		CodeBlock: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border).
			Padding(1, 2),

		HighSeverity: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Error),

		MediumSeverity: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Warning),

		LowSeverity: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Info),

		InfoSeverity: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Subtle),

		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Text).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border).
			PaddingLeft(1).
			PaddingRight(1),

		Border: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border),

		Section: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border).
			Padding(1, 2),

		ProgressBarEmpty: lipgloss.NewStyle().
			Foreground(theme.HighlightLo),

		ProgressBarFull: lipgloss.NewStyle().
			Foreground(theme.Primary),
	}
}

// DefaultStyle is the default style for the TUI
var DefaultStyle = DefaultStyles()
