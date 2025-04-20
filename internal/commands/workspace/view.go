package workspace

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	ws "github.com/tildaslashalef/mindnest/internal/workspace"
)

// View renders the UI based on the model state.
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	if m.lastError != "" {
		// Simple error view for now
		return m.styles.Error.Render("Error: "+m.lastError) + "\nPress q to quit."
	}

	if m.loading {
		return fmt.Sprintf("\n\n  %s Loading workspace data...\n\n", m.spinner.View())
	}

	// If showing GitHub modal, render that instead
	if m.showGitHubModal {
		return m.renderGitHubModal()
	}

	// --- Main View Composition using lipgloss.JoinVertical ---
	header := m.renderHeader()           // Get rendered header
	viewportContent := m.viewport.View() // Get rendered viewport content
	footer := m.renderFooter()           // Get rendered footer

	// Prepare status message line (always include for stable layout)
	statusLine := ""
	if m.statusMsg != "" {
		statusLine = m.styles.StatusText.Render(m.statusMsg)
	} else {
		// Render an empty line with the same expected height as the status message
		// to prevent layout jumps when the message appears/disappears.
		// Adjust height based on StatusText style if needed (e.g., if it has padding).
		statusLine = "" // Or potentially "\n" if StatusText has no height itself.
	}

	// Combine the parts vertically
	// Ensure viewport takes up remaining space - lipgloss usually handles this well with JoinVertical.
	finalView := lipgloss.JoinVertical(lipgloss.Left,
		header,
		statusLine, // Status line (potentially empty)
		viewportContent,
		footer,
	)

	return finalView
}

// renderHeader creates the top header string with workspace details.
func (m Model) renderHeader() string {
	wsName := "<No Workspace>"
	wsPath := ""
	if m.workspace != nil {
		wsName = m.workspace.Name
		wsPath = m.workspace.Path
	}
	issueCountStr := "0 Issues"
	if len(m.issues) > 0 {
		issueCountStr = fmt.Sprintf("Issue %d/%d", m.currentIssue+1, len(m.issues))
	} else if !m.loading {
		issueCountStr = "No Issues Found"
	}

	// Styling (using styles from model.go)
	nameStyle := m.styles.Title
	pathStyle := m.styles.Subtle
	countStyle := m.styles.Info
	headerBoxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color("#d5c4a1")). // Gruvbox light color
		Padding(0, 1).                               // Minimal padding
		MarginBottom(1)                              // Add margin below header

	nameLine := nameStyle.Render(fmt.Sprintf("Workspace: %s", wsName))
	pathLine := pathStyle.Render(fmt.Sprintf("Path: %s", wsPath))
	countLine := countStyle.Render(issueCountStr)

	headerContent := lipgloss.JoinVertical(lipgloss.Left,
		nameLine,
		pathLine,
		countLine,
	)

	// Adjust width dynamically
	availableWidth := m.width - 4 // Account for border
	finalHeader := headerBoxStyle.Width(availableWidth).Render(headerContent)

	return finalHeader
}

// renderFooter creates the bottom footer string (help text).
func (m Model) renderFooter() string {
	if m.showHelp {
		return m.help.View(m.keymap)
	} else {
		helpView := m.help.ShortHelpView(m.keymap.ShortHelp())
		return m.styles.Subtle.Render(helpView)
	}
}

// formatIssueContent formats the details of the currently selected issue for the viewport.
func (m Model) formatIssueContent() string {
	if len(m.issues) == 0 && !m.loading {
		return "No issues found for this workspace."
	} else if len(m.issues) == 0 && m.loading {
		return "Loading issues..." // Should be covered by main loading view, but defensive
	}
	if m.currentIssue < 0 || m.currentIssue >= len(m.issues) {
		return "Error: Invalid issue selection."
	}

	issue := m.issues[m.currentIssue]
	var content strings.Builder

	// Status badge
	statusBadge := ""
	if issue.IsValid {
		statusBadge = " " + m.styles.Success.Render("[ACCEPTED]")
	}

	// Title
	issueTitle := fmt.Sprintf("[%s] %s", issue.Severity, issue.Title)
	severityStyle := getSeverityStyle(m.styles, issue.Severity)
	headerLine := severityStyle.Render(issueTitle) + statusBadge
	content.WriteString(headerLine + "\n\n")

	// Metadata: Type, File, Lines, Created
	metaContent := fmt.Sprintf("Type: %s\nFile: %s\n",
		string(issue.Type),
		getFilePath(issue)) // Use helper
	if issue.LineStart > 0 {
		if issue.LineEnd > issue.LineStart {
			metaContent += fmt.Sprintf("Lines: %d-%d\n", issue.LineStart, issue.LineEnd)
		} else {
			metaContent += fmt.Sprintf("Line: %d\n", issue.LineStart)
		}
	}
	metaContent += fmt.Sprintf("Created: %s", issue.CreatedAt.Format("2006-01-02 15:04:05"))
	content.WriteString(m.styles.Subtle.Render(metaContent) + "\n\n")

	// Description
	content.WriteString(m.styles.Title.Render("Description") + "\n")
	desc := renderMarkdown(m, issue.Description)
	content.WriteString(desc + "\n")

	// Affected Code
	if issue.AffectedCode != "" {
		content.WriteString(m.styles.Title.Render("Affected Code") + "\n")
		affectedCode := renderCodeMarkdown(m, issue.AffectedCode, "go")
		content.WriteString(affectedCode + "\n")
	}

	// Suggestion Text
	if issue.Suggestion != "" {
		content.WriteString(m.styles.Title.Render("Suggestion") + "\n")
		suggestion := renderMarkdown(m, issue.Suggestion)
		content.WriteString(suggestion + "\n")
	}

	// Code Snippet (Suggested Fix)
	if issue.CodeSnippet != "" {
		content.WriteString(m.styles.Title.Render("Suggested Fix") + "\n")
		codeSnippet := renderCodeMarkdown(m, issue.CodeSnippet, "go")
		content.WriteString(codeSnippet + "\n")
	}

	return content.String()
}

// renderGitHubModal renders the modal for submitting issue to GitHub PR.
func (m Model) renderGitHubModal() string {
	modalWidth := min(m.width-4, 80) // Calculate consistent modal width

	// Modal container styling
	modalStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#fabd2f")). // Gruvbox yellow
		Padding(1, 2).
		Width(modalWidth)

	var modalContent strings.Builder

	// Title
	modalTitle := m.styles.Title.Render("Submit to GitHub PR")
	modalContent.WriteString(modalTitle + "\n\n")

	// Error (if any)
	if m.prError != "" {
		modalContent.WriteString(m.styles.Error.Render(m.prError) + "\n\n")
	}

	// Issue Context
	if m.currentIssue < len(m.issues) {
		issue := m.issues[m.currentIssue]
		lineInfo := ""
		if issue.LineStart > 0 {
			lineInfo = fmt.Sprintf(", Line: %d", issue.LineStart)
			if issue.LineEnd > issue.LineStart {
				lineInfo = fmt.Sprintf(", Lines: %d-%d", issue.LineStart, issue.LineEnd)
			}
		}
		modalContent.WriteString(fmt.Sprintf("Submitting Issue: %s%s\nFile: %s\n\n",
			issue.Title,
			lineInfo,
			getFilePath(issue)))
	}

	// PR Number Input
	modalContent.WriteString("PR Number:\n")
	modalContent.WriteString(m.prInput.View() + "\n\n")

	// Review Text Area
	modalContent.WriteString("Review Comment Text:\n")
	// Adjust text area width dynamically within modal constraints
	textAreaWidth := modalWidth - 4 // Account for modal padding
	m.textInput.SetWidth(textAreaWidth)
	// Adjust height based on available space
	// Estimate height used by other elements (title, error, context, pr input, instructions, borders/padding)
	heightUsedByOthers := 15 // Rough estimate, adjust as needed
	textAreaHeight := m.height - heightUsedByOthers
	if textAreaHeight < 3 {
		textAreaHeight = 3 // Minimum height
	}
	m.textInput.SetHeight(textAreaHeight)
	modalContent.WriteString(m.textInput.View() + "\n\n")

	// Instructions/Status
	if m.prSubmitting {
		modalContent.WriteString(fmt.Sprintf("%s Submitting...\n", m.spinner.View()))
	} else {
		modalContent.WriteString(m.styles.Subtle.Render("Tab: Switch | Enter (in PR#): Submit | Esc: Cancel"))
	}

	// Render the modal centered on screen
	finalModal := lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		modalStyle.Render(modalContent.String()),
	)

	return finalModal
}

// --- View Rendering Helpers ---

// getFilePath extracts the file path from issue metadata.
func getFilePath(issue *ws.Issue) string {
	if issue != nil && issue.Metadata != nil {
		if path, ok := issue.Metadata["file_path"].(string); ok && path != "" {
			return path
		}
	}
	return "<unknown>"
}

// getSeverityStyle maps issue severity to a lipgloss style.
func getSeverityStyle(styles Styles, severity ws.IssueSeverity) lipgloss.Style {
	switch severity {
	case ws.IssueSeverityCritical, ws.IssueSeverityHigh:
		return styles.HighSeverity
	case ws.IssueSeverityMedium:
		return styles.MediumSeverity
	case ws.IssueSeverityLow:
		return styles.LowSeverity
	default:
		return styles.InfoSeverity // Use Info style for others/unknown
	}
}

// renderMarkdown renders plain markdown text using the glamour renderer.
func renderMarkdown(m Model, text string) string {
	if m.renderer != nil {
		rendered, err := m.renderer.Render(text)
		if err == nil {
			return rendered
		}
	}
	// Fallback to plain text if renderer fails or is nil
	return m.styles.Paragraph.Render(text)
}

// renderCodeMarkdown renders text assumed to be code within a markdown code block.
func renderCodeMarkdown(m Model, code, lang string) string {
	if lang == "" {
		lang = "text" // Default language for highlighting
	}
	md := fmt.Sprintf("```%s\n%s\n```", lang, code)
	if m.renderer != nil {
		rendered, err := m.renderer.Render(md)
		if err == nil {
			return rendered
		}
	}
	// Fallback to basic code block style
	return m.styles.CodeBlock.Render(code)
}

// min helper function (could be moved to utils)
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
