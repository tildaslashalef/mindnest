package workspace

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// Update handles messages and updates the model state.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// If showing GitHub modal, handle its specific keys
		if m.showGitHubModal {
			// Need to return the updated model from the handler
			newModel, cmd := m.handleGitHubModalKeyPress(msg)
			return newModel, cmd
		}

		// Handle regular keys
		switch {
		case key.Matches(msg, m.keymap.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keymap.Help):
			m.showHelp = !m.showHelp
			return m, nil

		case key.Matches(msg, m.keymap.NextIssue):
			if !m.loading && len(m.issues) > 0 {
				m.currentIssue = (m.currentIssue + 1) % len(m.issues)
				if m.ready { // Only update viewport if ready
					m.viewport.SetContent(m.formatIssueContent()) // Use helper from view.go
					m.viewport.GotoTop()
				}
				m.statusMsg = "" // Clear status message
			}
			return m, nil

		case key.Matches(msg, m.keymap.PrevIssue):
			if !m.loading && len(m.issues) > 0 {
				m.currentIssue = (m.currentIssue - 1 + len(m.issues)) % len(m.issues)
				if m.ready { // Only update viewport if ready
					m.viewport.SetContent(m.formatIssueContent()) // Use helper from view.go
					m.viewport.GotoTop()
				}
				m.statusMsg = "" // Clear status message
			}
			return m, nil

		case key.Matches(msg, m.keymap.Confirm):
			if !m.loading && len(m.issues) > 0 && m.currentIssue < len(m.issues) {
				issue := m.issues[m.currentIssue]
				cmds = append(cmds, toggleIssueStatus(m, issue.ID, !issue.IsValid)) // Use func from commands.go
				m.statusMsg = fmt.Sprintf("Updating status for issue #%d...", m.currentIssue+1)
			}
			return m, tea.Batch(cmds...)

		case key.Matches(msg, m.keymap.GitHub):
			if !m.loading && len(m.issues) > 0 && m.currentIssue < len(m.issues) {
				if m.issues[m.currentIssue].IsValid {
					// showGitHubPRModal updates the model directly and returns it
					return m.showGitHubPRModal() // Use helper func below
				} else {
					m.statusMsg = "Issue must be accepted before submitting to GitHub PR"
				}
			}
			return m, nil // No command needed if not showing modal

		// Viewport scrolling keys
		default:
			if m.ready {
				m.viewport, cmd = m.viewport.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Recalculate viewport size, leaving space for header, status, and footer
		headerHeightEstimate := 5 // Estimate height of rendered header
		statusLineHeight := 1     // Reserve 1 line for the status message
		footerHeightEstimate := 2 // Estimate height of rendered footer (short help)
		totalReservedHeight := headerHeightEstimate + statusLineHeight + footerHeightEstimate
		vpHeight := m.height - totalReservedHeight
		if vpHeight < 1 {
			vpHeight = 1 // Minimum height of 1
		}

		if !m.ready {
			// Initialize viewport with calculated dimensions
			m.viewport = viewport.New(msg.Width, vpHeight)
			// m.viewport.YPosition = headerHeightEstimate + statusLineHeight // Not needed with JoinVertical
			m.ready = true
		} else {
			// Update existing viewport dimensions
			m.viewport.Width = msg.Width
			m.viewport.Height = vpHeight
		}
		// Update content on resize
		if len(m.issues) > 0 {
			m.viewport.SetContent(m.formatIssueContent())
		}
		m.help.Width = msg.Width
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	// --- Custom Messages ---
	case LoadDataMsg:
		m.loading = false
		if msg.Error != nil {
			m.lastError = fmt.Sprintf("Error loading data: %v", msg.Error)
		} else {
			m.workspace = msg.Workspace
			m.issues = msg.Issues
			m.currentIssue = 0 // Reset to first issue
			if m.ready && len(m.issues) > 0 {
				m.viewport.SetContent(m.formatIssueContent())
				m.viewport.GotoTop()
			} else if len(m.issues) == 0 {
				m.statusMsg = "No issues found in this workspace."
			}
		}
		return m, nil

	case IssueStatusMsg:
		if msg.Error != nil {
			m.statusMsg = fmt.Sprintf("Error updating issue #%d: %v", m.currentIssue+1, msg.Error)
		} else {
			// Update the issue in our local copy
			found := false
			for i := range m.issues {
				if m.issues[i].ID == msg.IssueID {
					m.issues[i].IsValid = msg.IsValid
					if msg.IsValid {
						m.statusMsg = fmt.Sprintf("Issue #%d marked as accepted", m.currentIssue+1)
					} else {
						m.statusMsg = fmt.Sprintf("Issue #%d marked as unaccepted", m.currentIssue+1)
					}
					// Update the view to reflect the new status
					if m.ready {
						m.viewport.SetContent(m.formatIssueContent())
					}
					found = true
					break
				}
			}
			if !found {
				m.statusMsg = fmt.Sprintf("Error: Could not find issue %s locally after update.", msg.IssueID)
			}
		}
		return m, nil

	case GitHubPRSubmitMsg:
		m.prSubmitting = false
		if msg.Error != nil {
			m.prError = fmt.Sprintf("Error submitting to GitHub: %v", msg.Error)
			// Keep modal open to show error
		} else {
			m.showGitHubModal = false
			m.prError = ""
			m.statusMsg = fmt.Sprintf("Issue #%d successfully submitted to GitHub PR", m.currentIssue+1)
		}
		// Need to return the model state here
		return m, nil
	}

	// Pass messages to viewport and help components as well
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)
	m.help, cmd = m.help.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// --- Update Helper Functions ---

// showGitHubPRModal prepares the model state for displaying the GitHub PR modal.
func (m Model) showGitHubPRModal() (tea.Model, tea.Cmd) {
	m.showGitHubModal = true
	m.prError = ""
	m.editingReviewText = true // Start by editing review text
	m.prInput.Reset()
	m.prInput.Blur()    // Ensure PR input is not focused initially
	m.textInput.Reset() // Reset text area as well

	if m.currentIssue < len(m.issues) {
		issue := m.issues[m.currentIssue]
		// Initialize review text with details from the issue
		var reviewText strings.Builder
		reviewText.WriteString(fmt.Sprintf("## Mindnest Found Issue: %s\n\n", issue.Title))
		reviewText.WriteString("**Severity:** " + string(issue.Severity) + "\n")
		reviewText.WriteString("**Type:** " + string(issue.Type) + "\n\n")
		reviewText.WriteString("**Description:**\n")
		reviewText.WriteString(issue.Description)
		reviewText.WriteString("\n\n")
		if issue.Suggestion != "" {
			reviewText.WriteString("**Suggestion:**\n")
			reviewText.WriteString(issue.Suggestion)
			reviewText.WriteString("\n\n")
		}
		if issue.CodeSnippet != "" {
			reviewText.WriteString("**Suggested Code Fix:**\n")
			reviewText.WriteString("```go\n" + issue.CodeSnippet + "\n```\n") // Assume go, maybe make dynamic later
		}
		m.textInput.SetValue(reviewText.String())
	}

	// Focus the text area
	cmd := m.textInput.Focus()
	return m, cmd
}

// handleGitHubModalKeyPress handles key presses specifically when the GitHub modal is visible.
func (m Model) handleGitHubModalKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg.String() {
	case "esc":
		m.showGitHubModal = false
		m.prError = ""
		m.editingReviewText = false
		m.textInput.Blur()
		m.prInput.Blur()
		return m, nil

	case "tab":
		if m.prInput.Focused() {
			m.prInput.Blur()
			m.editingReviewText = true
			cmd = m.textInput.Focus()
		} else {
			m.editingReviewText = false
			m.textInput.Blur()
			cmd = m.prInput.Focus()
		}
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)

	case "enter":
		if m.prSubmitting {
			return m, nil // Ignore enter while submitting
		}
		// If PR input is focused, initiate submission
		if m.prInput.Focused() {
			prNumStr := m.prInput.Value()
			prNum, err := strconv.Atoi(prNumStr)
			if err != nil || prNum <= 0 {
				m.prError = "Invalid PR number."
				return m, nil
			}
			m.prDetails.PRNumber = prNum
			m.prDetails.ReviewText = m.textInput.Value()
			m.prSubmitting = true
			m.prError = "" // Clear previous error
			if m.currentIssue < len(m.issues) {
				issueID := m.issues[m.currentIssue].ID
				cmds = append(cmds, submitIssueToGitHub(m, issueID), m.spinner.Tick) // Start submission cmd + spinner
			} else {
				m.prError = "Error: No issue selected for submission."
				m.prSubmitting = false
			}
		} else {
			// If text area is focused, pass Enter to it (might add newline)
			m.textInput, cmd = m.textInput.Update(msg)
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)
	}

	// Pass other key messages to the focused input component
	if m.prInput.Focused() {
		m.prInput, cmd = m.prInput.Update(msg)
		cmds = append(cmds, cmd)
	} else if m.editingReviewText {
		// Ensure the text area gets the message when it should be focused
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)
	} else {
		// If neither is explicitly focused (shouldn't normally happen in modal)
		// Maybe pass to text area by default?
		// Or handle as no-op?
		// For now, assume one is always focused based on m.editingReviewText / m.prInput.Focused()
	}

	return m, tea.Batch(cmds...)
}
