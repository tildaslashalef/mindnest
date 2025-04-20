package review

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
	"github.com/tildaslashalef/mindnest/internal/loggy"
	"github.com/tildaslashalef/mindnest/internal/review"
)

// View renders the UI based on the model's current state.
func (m Model) View() string {
	if !m.ready {
		// Don't render until dimensions are known
		return "Initializing...\n"
	}

	var mainContent string
	var footer string

	switch m.status {
	case StatusInitializing, StatusInit, StatusWorkspaceInit:
		mainContent = m.renderInitializingView()
	case StatusReady:
		mainContent = m.renderReadyView()
	case StatusReviewing, StatusProcessingFiles, StatusGeneratingEmbeddings, StatusAnalyzingCode:
		mainContent = m.renderReviewingView()
	case StatusViewingReview:
		mainContent = m.renderReviewView()
	case StatusError:
		mainContent = m.renderErrorView()
	default:
		mainContent = "Unknown status"
	}

	// Always show help at the bottom if toggled
	if m.showHelp {
		footer = m.help.View(Keys) // Keys defined in keymaps.go
	} else {
		footer = m.help.ShortHelpView(Keys.ShortHelp())
	}

	// Combine main content and footer
	// Ensure footer doesn't get pushed off screen if main content is tall
	return lipgloss.JoinVertical(lipgloss.Left,
		mainContent,
		footer,
	)
}

// renderInitializingView displays the initial loading state.
func (m Model) renderInitializingView() string {
	statusLine := m.styles.StatusText.Render("Initializing...")
	spinner := m.spinner.View()
	return lipgloss.JoinVertical(lipgloss.Center,
		renderBanner(m.styles), // Use shared banner renderer
		"\n",
		spinner+" "+statusLine,
	)
}

// renderReadyView displays the state when ready for user action.
func (m Model) renderReadyView() string {
	var b strings.Builder

	b.WriteString(renderBanner(m.styles))
	b.WriteString("\n\n")
	if m.workspace != nil {
		b.WriteString(m.styles.Title.Render(fmt.Sprintf("Workspace: %s", m.workspace.Name)))
		b.WriteString("\n")
		b.WriteString(m.styles.Subtle.Render(m.workspace.Path))
		b.WriteString("\n\n")
	}
	b.WriteString(m.styles.Paragraph.Render("Press 'r' to start a review based on your options."))
	b.WriteString("\n")
	b.WriteString(m.styles.Paragraph.Render("Press '?' for help, 'q' to quit."))

	// Center the content vertically and horizontally
	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		b.String(),
	)
}

// renderReviewingView displays progress during the review phases.
func (m Model) renderReviewingView() string {
	// var progress string
	// if m.totalFiles > 0 {
	// 	progress = fmt.Sprintf("%.0f%% (%d/%d)", m.progressPercent*100, m.currentFile, m.totalFiles)
	// }

	status := m.getStatusLine() // Get the status message
	spinner := m.spinner.View()

	// Simple progress bar - REMOVED
	// barWidth := m.width - 10 // Adjust width as needed
	// filledWidth := int(float64(barWidth) * m.progressPercent)
	// emptyWidth := barWidth - filledWidth
	// progressBar := m.styles.ProgressBarFull.Render(strings.Repeat("█", filledWidth)) +
	// 	m.styles.ProgressBarEmpty.Render(strings.Repeat("░", emptyWidth))

	content := lipgloss.JoinVertical(lipgloss.Center,
		renderBanner(m.styles),
		"\n",
		spinner+" "+status,
	)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}

// renderReviewView displays the results of the review (issues).
func (m Model) renderReviewView() string {
	header := m.getReviewHeaderInfo()
	body := m.viewport.View()
	footer := m.getReviewFooterInfo()

	// Combine header, body (viewport), and footer
	fullContent := lipgloss.JoinVertical(lipgloss.Left,
		header,
		body, // Viewport content already has its own padding/style
		footer,
	)

	// Return the combined view
	return fullContent
}

// renderErrorView displays an error message.
func (m Model) renderErrorView() string {
	errorTitle := m.styles.Error.Render("Error")
	errorBody := m.styles.Paragraph.Render(m.errorMsg)
	quitMsg := m.styles.Subtle.Render("Press 'q' to quit.")

	content := lipgloss.JoinVertical(lipgloss.Center,
		errorTitle,
		"\n",
		errorBody,
		"\n",
		quitMsg,
	)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}

// renderBanner renders the application title/banner.
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

// getStatusLine constructs the status message string shown during processing.
func (m Model) getStatusLine() string {
	// This combines the spinner (if active) and the status message
	// The spinner itself is handled in the main view logic based on status
	return m.styles.StatusText.Render(m.statusMessage)
}

// getReviewHeaderInfo renders the top section of the review view (workspace, issue count).
func (m Model) getReviewHeaderInfo() string {
	// Basic header if workspace info isn't available yet
	if m.workspace == nil {
		return m.styles.Header.Render("Review Results")
	}

	wsInfo := fmt.Sprintf("Workspace: %s (%s)", m.workspace.Name, m.styles.Subtle.Render(m.workspace.Path))
	issueCount := ""
	if len(m.issues) > 0 {
		issueCount = fmt.Sprintf("Issue: %d/%d", m.currentIssueID+1, len(m.issues))
	} else {
		issueCount = "No issues found"
	}

	left := m.styles.Header.Render(wsInfo)
	right := m.styles.Header.Render(issueCount)

	// Use lipgloss spacing - adjust width based on model width
	spacerWidth := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if spacerWidth < 0 {
		spacerWidth = 0
	}
	spacer := lipgloss.NewStyle().Width(spacerWidth).Render("")

	return lipgloss.JoinHorizontal(lipgloss.Left, left, spacer, right)
}

// renderReviewContent prepares the detailed content for the viewport in the review view.
func (m Model) renderReviewContent() string {
	if len(m.issues) == 0 {
		return m.styles.Success.Render("\n✅ No issues found!")
	}

	if m.currentIssueID < 0 || m.currentIssueID >= len(m.issues) {
		return m.styles.Error.Render("Error: Invalid issue index.")
	}

	issue := m.issues[m.currentIssueID]

	var sb strings.Builder

	// Issue Title and Status
	issueTitle := fmt.Sprintf("[%s] %s", issue.Severity, issue.Title)
	statusBadge := ""
	if issue.IsValid {
		statusBadge = " " + m.styles.Success.Render("[ACCEPTED]")
	}
	severityStyle := getSeverityStyle(m.styles, issue.Severity)
	sb.WriteString(severityStyle.Render(issueTitle) + statusBadge)
	sb.WriteString("\n\n")

	// File Path and Line Number
	filePath := "<unknown file>" // Default
	if m.app != nil && m.app.Workspace != nil {
		file, err := m.app.Workspace.GetFile(m.ctx, issue.FileID)
		if err == nil && file != nil {
			relPath, relErr := filepath.Rel(m.workspace.Path, file.Path)
			if relErr == nil {
				filePath = relPath
			} else {
				filePath = file.Path
			}
		} else if err != nil {
			loggy.Warn("Could not get file details for issue rendering", "file_id", issue.FileID, "error", err)
		}
	}
	location := fmt.Sprintf("File: %s", m.styles.Subtle.Render(filePath))
	if issue.LineStart > 0 {
		location += fmt.Sprintf(":%d", issue.LineStart)
		if issue.LineEnd > issue.LineStart {
			location += fmt.Sprintf("-%d", issue.LineEnd)
		}
	}
	sb.WriteString(location)
	sb.WriteString("\n\n")

	// Affected Code Snippet (if available)
	if issue.AffectedCode != "" {
		sb.WriteString("Affected Code:\n")
		// Render as a code block using glamour if possible
		affectedCodeMarkdown := "```\n" + issue.AffectedCode + "\n```"
		if m.renderer != nil {
			renderedAffected, err := m.renderer.Render(affectedCodeMarkdown)
			if err == nil {
				affectedCodeMarkdown = renderedAffected
			} else {
				loggy.Warn("Failed to render affected code markdown", "error", err)
				// Fallback to simple code block style
				affectedCodeMarkdown = m.styles.CodeBlock.Render(issue.AffectedCode)
			}
		}
		sb.WriteString(affectedCodeMarkdown)
		sb.WriteString("\n\n")
	}

	// Issue Description (render as Markdown)
	sb.WriteString("Description:\n")
	descMarkdown := issue.Description
	if m.renderer != nil {
		renderedDesc, err := m.renderer.Render(descMarkdown)
		if err == nil {
			descMarkdown = renderedDesc
		} else {
			loggy.Warn("Failed to render description markdown", "error", err)
			descMarkdown = wordwrap.String(issue.Description, m.viewport.Width-2)
		}
	}
	sb.WriteString(m.styles.Paragraph.Render(descMarkdown))
	sb.WriteString("\n\n")

	// Suggestion Text + Code Snippet (if available)
	if issue.Suggestion != "" || issue.CodeSnippet != "" {
		sb.WriteString("Suggestion:\n")

		// Render Suggestion text (if present) as Markdown
		if issue.Suggestion != "" {
			suggestionMarkdown := issue.Suggestion
			if m.renderer != nil {
				renderedSuggestion, err := m.renderer.Render(suggestionMarkdown)
				if err == nil {
					suggestionMarkdown = renderedSuggestion
				} else {
					loggy.Warn("Failed to render suggestion markdown", "error", err)
					suggestionMarkdown = wordwrap.String(issue.Suggestion, m.viewport.Width-2)
				}
			}
			sb.WriteString(m.styles.Paragraph.Render(suggestionMarkdown))
			sb.WriteString("\n") // Add newline separation if both are present
		}

		// Render Code Snippet (if present) as a code block
		if issue.CodeSnippet != "" {
			codeSnippetMarkdown := "```\n" + issue.CodeSnippet + "\n```"
			if m.renderer != nil {
				renderedSnippet, err := m.renderer.Render(codeSnippetMarkdown)
				if err == nil {
					codeSnippetMarkdown = renderedSnippet
				} else {
					loggy.Warn("Failed to render code snippet markdown", "error", err)
					codeSnippetMarkdown = m.styles.CodeBlock.Render(issue.CodeSnippet)
				}
			}
			sb.WriteString(codeSnippetMarkdown)
		}
		sb.WriteString("\n") // Add final newline after suggestion section
	}

	return sb.String()
}

// getReviewFooterInfo renders the bottom help line for the review view.
func (m Model) getReviewFooterInfo() string {
	if len(m.issues) == 0 {
		return m.styles.Subtle.Render("Press 'q' to quit.")
	}

	issue := m.issues[m.currentIssueID]
	var actionText string
	if issue.IsValid {
		actionText = fmt.Sprintf("Press '%s' to unaccept", Keys.AcceptFix.Keys()[0])
	} else {
		actionText = fmt.Sprintf("Press '%s' to accept", Keys.AcceptFix.Keys()[0])
	}

	navText := fmt.Sprintf(" | '%s'/'%s' to navigate", Keys.PrevIssue.Keys()[0], Keys.NextIssue.Keys()[0])
	scrollHint := " | Use arrows/j/k/pgup/pgdn to scroll"

	return m.styles.Subtle.Render(actionText + navText + scrollHint)
}

// getSeverityStyle returns the appropriate lipgloss style based on issue severity.
func getSeverityStyle(styles Styles, severity review.IssueSeverity) lipgloss.Style {
	switch severity {
	case review.IssueSeverityHigh:
		return styles.HighSeverity
	case review.IssueSeverityMedium:
		return styles.MediumSeverity
	case review.IssueSeverityLow:
		return styles.LowSeverity
	default:
		return styles.Subtle
	}
}
