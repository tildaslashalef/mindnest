package commands

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/urfave/cli/v2"

	"github.com/tildaslashalef/mindnest/internal/app"
	"github.com/tildaslashalef/mindnest/internal/github"
	"github.com/tildaslashalef/mindnest/internal/loggy"
	"github.com/tildaslashalef/mindnest/internal/tui"
	"github.com/tildaslashalef/mindnest/internal/utils"
	"github.com/tildaslashalef/mindnest/internal/workspace"
)

// WorkspaceCommand returns the workspace command
func WorkspaceCommand() *cli.Command {
	return &cli.Command{
		Name:    "workspace",
		Aliases: []string{"ws"},
		Usage:   "Show workspace details in an interactive UI",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "name",
				Aliases: []string{"n"},
				Usage:   "Name of the workspace to show (defaults to workspace in current directory)",
			},
			&cli.StringFlag{
				Name:    "github-url",
				Aliases: []string{"g"},
				Usage:   "Set or update GitHub repository URL for the workspace (e.g., https://github.com/owner/repo)",
			},
			&cli.BoolFlag{
				Name:    "tui",
				Aliases: []string{"t"},
				Usage:   "Show the interactive TUI after command execution (default: true)",
				Value:   true,
			},
		},
		Action: workspaceShowAction,
	}
}

// handleWorkspaceShow handles the workspace show command
func workspaceShowAction(c *cli.Context) error {
	application, err := app.FromContext(c)
	if err != nil {
		return fmt.Errorf("failed to get application from context: %w", err)
	}

	ctx := context.Background()
	name := c.String("name")
	githubURL := c.String("github-url")
	var ws *workspace.Workspace

	if name != "" {
		// Find workspace by name by listing all workspaces
		workspaces, err := application.Workspace.ListWorkspaces(ctx)
		if err != nil {
			return fmt.Errorf("failed to list workspaces: %w", err)
		}

		// Find workspace with matching name
		var matchingWorkspaces []*workspace.Workspace
		for _, w := range workspaces {
			if strings.Contains(strings.ToLower(w.Name), strings.ToLower(name)) {
				matchingWorkspaces = append(matchingWorkspaces, w)
			}

			// Exact match takes priority
			if w.Name == name {
				ws = w
				break
			}
		}

		if ws == nil && len(matchingWorkspaces) > 0 {
			// No exact match found, use the first partial match
			ws = matchingWorkspaces[0]
		}

		if ws == nil {
			return fmt.Errorf("no workspace found with name: %s", name)
		}
	} else {
		// Get workspace for current directory
		currentDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		ws, err = application.Workspace.GetWorkspaceByPath(ctx, currentDir)
		if err != nil {
			return fmt.Errorf("failed to get workspace for current directory: %w", err)
		}
	}

	if ws == nil {
		utils.PrintError("No workspace found")
		return fmt.Errorf("no workspace found")
	}

	// Update GitRepoURL if provided
	if githubURL != "" {
		// Validate the URL format (basic validation)
		if !strings.HasPrefix(githubURL, "http://") && !strings.HasPrefix(githubURL, "https://") && !strings.HasPrefix(githubURL, "git@") {
			utils.PrintError("Invalid GitHub URL format: " + githubURL + " (should start with http://, https://, or git@)")
			return fmt.Errorf("invalid GitHub URL format: %s (should start with http://, https://, or git@)", githubURL)
		}

		// Verify we can extract owner/repo from it
		if _, _, err := application.GitHub.ExtractRepoDetailsFromURL(githubURL); err != nil {
			utils.PrintError("Unable to extract repository details from URL: " + err.Error())
			return fmt.Errorf("unable to extract repository details from URL: %w", err)
		}

		// Update the workspace
		ws.SetGitRepoURL(githubURL)
		if err := application.Workspace.UpdateWorkspace(ctx, ws); err != nil {
			utils.PrintError("Failed to update workspace with new GitHub URL: " + err.Error())
			return fmt.Errorf("failed to update workspace with new GitHub URL: %w", err)
		}

		utils.PrintSuccess("Updated workspace with new GitHub URL " + githubURL)

		// If only updating the URL, return without starting the TUI
		if c.Command.Name == "workspace" && c.String("name") == "" && !c.Bool("tui") {
			return nil
		}
	}

	// Create and start the Bubble Tea program
	m := NewWorkspaceModel(application, ws)
	p := tea.NewProgram(m, tea.WithAltScreen())

	loggy.Debug("Starting workspace TUI", "workspace_id", ws.ID, "name", ws.Name)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run workspace UI: %w", err)
	}

	return nil
}

// WorkspaceKeyMap defines keybindings for the workspace TUI
type WorkspaceKeyMap struct {
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
func (k WorkspaceKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit, k.NextIssue, k.PrevIssue, k.Confirm, k.GitHub}
}

// FullHelp returns all keybindings for the help view
func (k WorkspaceKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Help, k.Quit},
		{k.NextIssue, k.PrevIssue, k.Confirm, k.GitHub},
	}
}

// DefaultWorkspaceKeyMap returns the default keybindings
func DefaultWorkspaceKeyMap() WorkspaceKeyMap {
	return WorkspaceKeyMap{
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

// Message types
type (
	// InitMsg is the initial message sent to the model
	InitMsg struct{}

	// LoadDataMsg is sent when data has been loaded
	LoadDataMsg struct {
		Workspace *workspace.Workspace
		Issues    []*workspace.Issue
		Error     error
	}

	// IssueStatusMsg is sent when an issue's status has been updated
	IssueStatusMsg struct {
		IssueID string
		IsValid bool
		Success bool
		Error   error
	}

	// GitHubPRSubmitMsg is sent when an issue is submitted to GitHub PR
	GitHubPRSubmitMsg struct {
		IssueID string
		Success bool
		Error   error
	}
)

// GitHubPRDetails contains the details for submitting to a GitHub PR
type GitHubPRDetails struct {
	PRNumber   int
	ReviewText string // Combined text with description, suggestion, and code snippet
}

// WorkspaceModel represents the state of our TUI
type WorkspaceModel struct {
	app          *app.App
	workspace    *workspace.Workspace
	issues       []*workspace.Issue
	currentIssue int

	// UI components
	keymap    WorkspaceKeyMap
	help      help.Model
	viewport  viewport.Model
	spinner   spinner.Model
	renderer  *glamour.TermRenderer
	styles    tui.Styles
	prInput   textinput.Model // Input for PR number
	textInput textarea.Model  // Input for review text

	// UI state
	ready    bool
	loading  bool
	showHelp bool
	error    string
	width    int
	height   int
	status   string

	// GitHub PR submission state
	showGitHubModal   bool
	prDetails         GitHubPRDetails
	prSubmitting      bool
	prError           string
	editingReviewText bool           // Whether we're editing the review text
	lineNumber        int            // Line for displaying/editing in the review modal
	reviewViewport    viewport.Model // For scrolling through review text
}

// NewWorkspaceModel creates a new workspace model
func NewWorkspaceModel(a *app.App, ws *workspace.Workspace) WorkspaceModel {
	keymap := DefaultWorkspaceKeyMap()
	help := help.New()

	s := spinner.New()
	s.Spinner = spinner.Dot

	// Create markdown renderer with dark theme optimized for terminal
	r, _ := glamour.NewTermRenderer(
		// Use dark style optimized for terminal
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(100),
	)

	// Initialize PR number input
	prInput := textinput.New()
	prInput.Placeholder = "Enter PR number"
	prInput.CharLimit = 10
	prInput.Width = 20

	// Initialize review text input
	textInput := textarea.New()
	textInput.Placeholder = "Review text..."
	textInput.SetWidth(128)
	textInput.SetHeight(25)
	textInput.CharLimit = 5000
	textInput.ShowLineNumbers = false

	return WorkspaceModel{
		app:          a,
		workspace:    ws,
		currentIssue: 0,
		loading:      true,
		keymap:       keymap,
		help:         help,
		spinner:      s,
		renderer:     r,
		styles:       tui.DefaultStyles(),
		prInput:      prInput,
		textInput:    textInput,
	}
}

// Init initializes the model
func (m WorkspaceModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.loadWorkspaceData,
	)
}

// loadWorkspaceData loads the workspace data
func (m WorkspaceModel) loadWorkspaceData() tea.Msg {
	ctx := context.Background()

	if m.workspace == nil {
		return LoadDataMsg{Error: fmt.Errorf("workspace not initialized")}
	}

	// Load workspace issues
	issues, err := m.app.Workspace.GetWorkspaceIssues(ctx, m.workspace.ID)
	if err != nil {
		return LoadDataMsg{Error: fmt.Errorf("failed to load workspace issues: %w", err)}
	}

	return LoadDataMsg{
		Workspace: m.workspace,
		Issues:    issues,
		Error:     nil,
	}
}

// Update handles messages and updates the model
func (m WorkspaceModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// If showing GitHub modal, handle its specific keys
		if m.showGitHubModal {
			return m.handleGitHubModalKeyPress(msg)
		}

		switch {
		case key.Matches(msg, m.keymap.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keymap.Help):
			m.showHelp = !m.showHelp
			return m, nil

		case key.Matches(msg, m.keymap.NextIssue):
			if !m.loading && len(m.issues) > 0 {
				m.currentIssue = (m.currentIssue + 1) % len(m.issues)
				m.viewport.SetContent(m.formatIssueContent())
				m.viewport.GotoTop()
				// Clear status message when navigating
				m.status = ""
				return m, nil
			}

		case key.Matches(msg, m.keymap.PrevIssue):
			if !m.loading && len(m.issues) > 0 {
				m.currentIssue = (m.currentIssue - 1 + len(m.issues)) % len(m.issues)
				m.viewport.SetContent(m.formatIssueContent())
				m.viewport.GotoTop()
				// Clear status message when navigating
				m.status = ""
				return m, nil
			}

		case key.Matches(msg, m.keymap.Up):
			if m.ready {
				var cmd tea.Cmd
				m.viewport, cmd = m.viewport.Update(msg)
				cmds = append(cmds, cmd)
			}

		case key.Matches(msg, m.keymap.Down):
			if m.ready {
				var cmd tea.Cmd
				m.viewport, cmd = m.viewport.Update(msg)
				cmds = append(cmds, cmd)
			}

		case key.Matches(msg, m.keymap.Confirm):
			if !m.loading && len(m.issues) > 0 {
				issue := m.issues[m.currentIssue]
				// Toggle the issue status
				return m, m.toggleIssueStatus(issue.ID, !issue.IsValid)
			}

		case key.Matches(msg, m.keymap.GitHub):
			if !m.loading && len(m.issues) > 0 && m.issues[m.currentIssue].IsValid {
				// Show GitHub PR submission modal
				return m.showGitHubPRModal()
			} else if !m.loading && len(m.issues) > 0 && !m.issues[m.currentIssue].IsValid {
				m.status = "Issue must be accepted before submitting to GitHub"
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		if !m.ready {
			// Initialize viewport now that we have the window dimensions
			m.viewport = viewport.New(msg.Width, msg.Height-7) // Reserve space for header and footer
			m.viewport.YPosition = 3
			m.ready = true

			// Update viewport content
			m.viewport.SetContent(m.formatIssueContent())
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 7
		}

		// Update help view width
		m.help.Width = msg.Width

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case LoadDataMsg:
		m.loading = false

		if msg.Error != nil {
			m.error = fmt.Sprintf("Error: %v", msg.Error)
			return m, nil
		}

		m.workspace = msg.Workspace
		m.issues = msg.Issues

		// Update viewport content if there are issues
		if len(m.issues) > 0 {
			m.viewport.SetContent(m.formatIssueContent())
		}

	case IssueStatusMsg:
		if msg.Error != nil {
			m.status = fmt.Sprintf("Error updating issue: %v", msg.Error)
		} else {
			// Update the issue in our local copy
			if m.currentIssue < len(m.issues) {
				m.issues[m.currentIssue].IsValid = msg.IsValid
				if msg.IsValid {
					m.status = "Issue marked as accepted"
				} else {
					m.status = "Issue marked as unaccepted"
				}

				// Update the view to reflect the new status
				m.viewport.SetContent(m.formatIssueContent())
			}
		}

	case GitHubPRSubmitMsg:
		m.prSubmitting = false

		if msg.Error != nil {
			m.prError = fmt.Sprintf("Error submitting to GitHub: %v", msg.Error)
		} else {
			// Hide GitHub modal
			m.showGitHubModal = false
			m.prError = ""
			m.status = "Issue successfully submitted to GitHub PR"
		}
	}

	return m, tea.Batch(cmds...)
}

// toggleIssueStatus toggles an issue's valid status
func (m WorkspaceModel) toggleIssueStatus(issueID string, isValid bool) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		var err error
		if isValid {
			// Mark as valid
			err = m.app.Review.MarkIssueAsValid(ctx, issueID)
		} else {
			// Mark as invalid using the new service method
			err = m.app.Review.MarkIssueAsInvalid(ctx, issueID)
		}

		return IssueStatusMsg{
			IssueID: issueID,
			IsValid: isValid,
			Success: err == nil,
			Error:   err,
		}
	}
}

// formatIssueContent formats the current issue for display
func (m WorkspaceModel) formatIssueContent() string {
	if len(m.issues) == 0 {
		return "No issues found for this workspace"
	}

	issue := m.issues[m.currentIssue]
	var content strings.Builder

	// Status badge for accepted issues
	status := ""
	if issue.IsValid {
		status = " " + m.styles.Success.Render("[ACCEPTED]")
	}

	// Issue title with id number
	issueTitle := fmt.Sprintf("Issue #%d: %s", m.currentIssue+1, issue.Title)

	// Create a single header with title
	headerContent := m.styles.Title.Render(issueTitle) + status

	// Create a single bordered box for the issue header that matches workspace header style
	headerBox := lipgloss.NewStyle().
		BorderStyle(lipgloss.DoubleBorder()).        // Use double border to match workspace header
		BorderForeground(lipgloss.Color("#b8bb26")). // Gruvbox green
		Padding(0, 2).                               // Reduced padding to match workspace header
		Margin(0, 0).                                // No margin
		Width(min(m.width-4, 80)).                   // Reasonable width
		Render(headerContent)

	content.WriteString("\n" + headerBox + "\n\n")

	// Issue type and severity
	var severityStyle lipgloss.Style
	switch issue.Severity {
	case workspace.IssueSeverityCritical:
		severityStyle = m.styles.HighSeverity
	case workspace.IssueSeverityHigh:
		severityStyle = m.styles.HighSeverity
	case workspace.IssueSeverityMedium:
		severityStyle = m.styles.MediumSeverity
	case workspace.IssueSeverityLow:
		severityStyle = m.styles.LowSeverity
	default:
		severityStyle = m.styles.InfoSeverity
	}

	// Get file path from metadata if available
	filePath := "Unknown file"
	if issue.Metadata != nil {
		if path, ok := issue.Metadata["file_path"].(string); ok && path != "" {
			filePath = path
		}
	}

	// Format metadata in a clean and organized way
	metaContent := fmt.Sprintf("Type: %s    Severity: %s\n",
		string(issue.Type),
		severityStyle.Render(string(issue.Severity)))

	metaContent += fmt.Sprintf("File: %s\n", filePath)

	// Line numbers
	if issue.LineStart > 0 {
		if issue.LineEnd > issue.LineStart {
			metaContent += fmt.Sprintf("Lines: %d-%d\n", issue.LineStart, issue.LineEnd)
		} else {
			metaContent += fmt.Sprintf("Line: %d\n", issue.LineStart)
		}
	}

	metaContent += fmt.Sprintf("Created: %s", issue.CreatedAt.Format("2006-01-02 15:04:05"))

	// Style and add metadata
	content.WriteString(m.styles.Subtle.Render(metaContent) + "\n\n")

	// Issue description
	descriptionTitle := m.styles.Info.Render("Description")
	content.WriteString(descriptionTitle + "\n")

	// Render description with markdown
	description := issue.Description
	renderedDesc, err := m.renderer.Render(description)
	if err == nil {
		content.WriteString(renderedDesc + "\n")
	} else {
		content.WriteString(description + "\n\n")
	}

	// Show the suggestion
	if issue.Suggestion != "" {
		suggestionTitle := m.styles.Info.Render("Suggestion")
		content.WriteString(suggestionTitle + "\n")

		// Render suggestion with markdown
		renderedSuggestion, err := m.renderer.Render(issue.Suggestion)
		if err == nil {
			content.WriteString(renderedSuggestion + "\n")
		} else {
			content.WriteString(issue.Suggestion + "\n\n")
		}
	}

	// Show affected code if available
	if issue.AffectedCode != "" {
		affectedCodeTitle := m.styles.Info.Render("Affected Code")
		content.WriteString(affectedCodeTitle + "\n")

		// Use markdown code block for syntax highlighting
		codeBlock := "```go\n" + issue.AffectedCode + "\n```"
		renderedCode, err := m.renderer.Render(codeBlock)
		if err == nil {
			content.WriteString(renderedCode + "\n")
		} else {
			// Fallback to basic box if rendering fails
			codeStyle := m.styles.CodeBlock.Copy().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#665c54")). // Gruvbox color
				Padding(1, 2).
				Width(min(m.width-10, 80)) // Limit width but keep it reasonable
			content.WriteString(codeStyle.Render(issue.AffectedCode) + "\n\n")
		}
	}

	// Show suggested fix if available
	if issue.CodeSnippet != "" {
		suggestedFixTitle := m.styles.Info.Render("Suggested Fix")
		content.WriteString(suggestedFixTitle + "\n")

		// Use markdown code block for syntax highlighting
		codeBlock := "```go\n" + issue.CodeSnippet + "\n```"
		renderedCode, err := m.renderer.Render(codeBlock)
		if err == nil {
			content.WriteString(renderedCode + "\n")
		} else {
			// Fallback to basic box if rendering fails
			codeStyle := m.styles.CodeBlock.Copy().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#665c54")). // Gruvbox color
				Padding(1, 2).
				Width(min(m.width-10, 80)) // Limit width but keep it reasonable
			content.WriteString(codeStyle.Render(issue.CodeSnippet) + "\n\n")
		}
	}

	// Actions
	content.WriteString("\n")
	if issue.IsValid {
		content.WriteString(m.styles.Subtle.Render("Press 'c' to mark as unaccepted, 'g' to submit to GitHub PR, use n/p to navigate between issues.") + "\n")
	} else {
		content.WriteString(m.styles.Subtle.Render("Press 'c' to accept this issue, use n/p to navigate between issues.") + "\n")
	}

	return content.String()
}

// View renders the UI
func (m WorkspaceModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	if m.error != "" {
		return m.styles.Error.Render(m.error)
	}

	if m.loading {
		return fmt.Sprintf("\n\n  %s Loading workspace data...\n\n", m.spinner.View())
	}

	// If showing GitHub modal, render that instead of normal view
	if m.showGitHubModal {
		return m.renderGitHubModal()
	}

	// Main content
	var s strings.Builder

	// Create a single workspace info header with all information
	wsName := fmt.Sprintf("Workspace: %s", m.workspace.Name)
	wsPath := fmt.Sprintf("Path: %s", m.workspace.Path)
	issueCount := fmt.Sprintf("Issues: %d/%d", m.currentIssue+1, len(m.issues))

	// Style the workspace name (normal size)
	styledName := m.styles.Title.Render(wsName)

	// Style the path (dimmed and smaller)
	styledPath := m.styles.Subtle.Copy().
		Foreground(lipgloss.Color("#928374")). // Dimmed Gruvbox color
		Render(wsPath)

	// Style the issue count
	styledCount := m.styles.Info.Render(issueCount)

	// Combine all info in a single block with proper layout
	headerContent := lipgloss.JoinVertical(
		lipgloss.Left,
		styledName,
		styledPath,
		styledCount,
	)

	// Create a single bordered box for all workspace info with all borders visible
	// Use double borders for better visibility
	headerBox := lipgloss.NewStyle().
		BorderStyle(lipgloss.DoubleBorder()).        // Use double border for prominence
		BorderForeground(lipgloss.Color("#d5c4a1")). // Gruvbox light color
		Padding(0, 2).                               // Reduce vertical padding
		Margin(0, 0).                                // No margin
		Width(min(m.width-4, 80)).                   // Reasonable width
		Render(headerContent)

	s.WriteString(headerBox + "\n")

	// Status message if any
	if m.status != "" {
		s.WriteString(m.styles.StatusText.Render(m.status) + "\n")
	}

	// Main content
	s.WriteString(m.viewport.View())

	// Footer with help
	var footerContent string
	if m.showHelp {
		footerContent = "\n" + m.help.View(m.keymap)
	} else {
		helpView := m.help.ShortHelpView(m.keymap.ShortHelp())
		footerContent = "\n" + m.styles.Subtle.Render(helpView)
	}
	s.WriteString(footerContent)

	return s.String()
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// showGitHubPRModal initializes and shows the GitHub PR submission modal
func (m WorkspaceModel) showGitHubPRModal() (tea.Model, tea.Cmd) {
	m.showGitHubModal = true
	m.prError = ""

	// Initialize in editing mode directly
	m.editingReviewText = true

	// Reset PR number input but don't focus it yet (we'll focus the text area)
	m.prInput.Reset()

	// Calculate text area width based on available space
	textWidth := min(m.width-14, 70) // Leave room for borders and padding

	// Get the current issue
	if m.currentIssue < len(m.issues) {
		issue := m.issues[m.currentIssue]

		// Initialize review text with description, suggestion, and code snippet
		var reviewText strings.Builder

		// Add description
		reviewText.WriteString("## Description\n")
		reviewText.WriteString(issue.Description)
		reviewText.WriteString("\n\n")

		// Add suggestion if available
		if issue.Suggestion != "" {
			reviewText.WriteString("## Suggestion\n")
			reviewText.WriteString(issue.Suggestion)
			reviewText.WriteString("\n\n")
		}

		// Add code snippet if available
		if issue.CodeSnippet != "" {
			reviewText.WriteString("## Suggested Code Fix\n")
			reviewText.WriteString("```go\n")
			reviewText.WriteString(issue.CodeSnippet)
			reviewText.WriteString("\n```")
		}

		// Set review text and focus the text input
		m.textInput.SetValue(reviewText.String())
		m.textInput.Focus()
		m.textInput.SetWidth(textWidth)
		// Textarea doesn't use Prompt/PromptStyle/TextStyle in the same way as textinput
		// Instead we'll set the style directly
		m.textInput.Blur()
		m.textInput.Focus()

		// Initialize the PR input too
		m.prInput.Prompt = "> "
		m.prInput.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#b8bb26"))
		m.prInput.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ebdbb2"))

		// Initialize review viewport with proper dimensions
		viewHeight := min(m.height-20, 20) // Allow space for other UI elements
		viewWidth := min(m.width-20, 78)   // Allow space for borders
		m.reviewViewport = viewport.New(viewWidth, viewHeight)
		m.reviewViewport.SetContent(m.textInput.Value())
	}

	return m, nil
}

// handleGitHubModalKeyPress handles key presses in the GitHub modal
func (m WorkspaceModel) handleGitHubModalKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "esc":
		// Cancel submission and close modal
		m.showGitHubModal = false
		m.prError = ""
		return m, nil

	case "tab":
		// Toggle between inputs
		if m.prInput.Focused() {
			m.prInput.Blur()
			m.editingReviewText = true
			m.textInput.Focus()
		} else {
			m.editingReviewText = false
			m.textInput.Blur()
			m.prInput.Focus()
		}
		return m, nil

	case "enter":
		// If already submitting, do nothing
		if m.prSubmitting {
			return m, nil
		}

		// If PR input is focused, try to submit
		if m.prInput.Focused() {
			// Validate PR number
			prNum := m.prInput.Value()
			if prNum == "" {
				m.prError = "Please enter a valid PR number"
				return m, nil
			}

			prNumber, err := strconv.Atoi(prNum)
			if err != nil || prNumber <= 0 {
				m.prError = "Please enter a valid PR number"
				return m, nil
			}

			// Set submission details
			m.prDetails.PRNumber = prNumber
			m.prDetails.ReviewText = m.textInput.Value()

			// Start submission
			m.prSubmitting = true

			// Submit to GitHub
			if m.currentIssue < len(m.issues) {
				issue := m.issues[m.currentIssue]
				return m, m.submitIssueToGitHub(issue.ID)
			}

			// No issue selected
			m.prSubmitting = false
			m.prError = "No issue selected"
			return m, nil
		} else {
			// If text input is focused, just add a newline
			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd
		}
	}

	// Handle input based on which field is focused
	if m.prInput.Focused() {
		m.prInput, cmd = m.prInput.Update(msg)
		return m, cmd
	} else {
		// Handle text area input and scrolling
		switch msg.String() {
		case "up", "down", "left", "right", "home", "end", "pgup", "pgdown":
			// Let the text input handle navigation keys
			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd
		default:
			// For regular typing, pass to text input
			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd
		}
	}
}

// renderGitHubModal renders the GitHub PR submission modal
func (m WorkspaceModel) renderGitHubModal() string {
	var sb strings.Builder

	// Add minimal top padding to ensure the top border is visible
	sb.WriteString("\n")

	// Modal container styling with explicit full border
	modalStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#fabd2f")). // Gruvbox yellow
		Padding(1, 2).
		Width(min(m.width-4, 80))

	// Modal content
	var modalContent strings.Builder

	// Modal title
	modalTitle := m.styles.Title.Render("Submit to GitHub PR")
	modalContent.WriteString(modalTitle + "\n\n")

	// Show error if any
	if m.prError != "" {
		modalContent.WriteString(m.styles.Error.Render(m.prError) + "\n\n")
	}

	// Show file info for the current issue being submitted
	if m.currentIssue < len(m.issues) {
		issue := m.issues[m.currentIssue]

		// Line information
		if issue.LineStart > 0 {
			lineInfo := fmt.Sprintf("Line: %d", issue.LineStart)
			if issue.LineEnd > issue.LineStart {
				lineInfo = fmt.Sprintf("Lines: %d-%d", issue.LineStart, issue.LineEnd)
			}
			modalContent.WriteString(fmt.Sprintf("File: %s, %s\n\n",
				getFilePath(issue),
				lineInfo))
		}
	}

	// PR Number input
	prLabel := m.styles.Subtle.Render("PR Number:")
	modalContent.WriteString(prLabel + "\n")

	// Use the textinput's built-in view rather than custom styling
	modalContent.WriteString(m.prInput.View() + "\n\n")

	// Review text section
	reviewLabel := m.styles.Subtle.Render("Review Text:")
	modalContent.WriteString(reviewLabel + "\n")

	// Calculate maximum space for text area
	maxWidth := min(m.width-14, 70) // Leave room for borders and padding

	// Set up the text area
	borderColor := lipgloss.Color("#83a598") // Default blue border
	if m.editingReviewText {
		borderColor = lipgloss.Color("#b8bb26") // Green when focused
	}

	// Create border style once
	borderStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(maxWidth + 4) // Account for border and padding

	// Format the content
	var reviewContent string
	if m.editingReviewText {
		// When editing, just show the input
		reviewContent = m.textInput.View()
	} else {
		// When viewing, replace \n with actual visible linebreaks
		// This helps debug what's actually in the text
		reviewContent = strings.ReplaceAll(m.textInput.Value(), "\n", "↵\n")
	}

	// If we're editing, let the textarea render itself
	if m.editingReviewText {
		textAreaView := borderStyle.Render(reviewContent)
		modalContent.WriteString(textAreaView + "\n\n")
	} else {
		// Otherwise, apply inner styling separate from border
		innerContent := lipgloss.NewStyle().
			Padding(1, 2).
			Foreground(lipgloss.Color("#ebdbb2")).
			Width(maxWidth).
			Render(reviewContent)

		// Wrap the content with a border
		textAreaView := borderStyle.Render(innerContent)
		modalContent.WriteString(textAreaView + "\n\n")
	}

	// Instructions
	if m.prSubmitting {
		modalContent.WriteString(fmt.Sprintf("%s Submitting to GitHub...\n", m.spinner.View()))
	} else {
		modalContent.WriteString(m.styles.Info.Render("Instructions:") + "\n")
		modalContent.WriteString("- Tab to switch between PR number and review text\n")
		modalContent.WriteString("- Edit review text directly\n")
		modalContent.WriteString("- Press Enter to submit (when PR number is focused)\n")
		modalContent.WriteString("- Press Esc to cancel\n")
	}

	// Render the full modal with all borders visible
	sb.WriteString(modalStyle.Render(modalContent.String()))

	return sb.String()
}

func getFilePath(issue *workspace.Issue) string {
	if issue.Metadata != nil {
		if path, ok := issue.Metadata["file_path"].(string); ok && path != "" {
			return path
		}
	}
	return "Unknown file"
}

// submitIssueToGitHub submits an issue to GitHub PR
func (m WorkspaceModel) submitIssueToGitHub(issueID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// Check if GitHub service is initialized
		if m.app.GitHub == nil {
			return GitHubPRSubmitMsg{
				IssueID: issueID,
				Success: false,
				Error:   fmt.Errorf("GitHub service not initialized"),
			}
		}

		// Get current issue for metadata only
		var issue *workspace.Issue
		if m.currentIssue < len(m.issues) {
			issue = m.issues[m.currentIssue]
		} else {
			return GitHubPRSubmitMsg{
				IssueID: issueID,
				Success: false,
				Error:   fmt.Errorf("issue not found"),
			}
		}

		// Create a PR comment with workspace ID to allow repo details lookup
		prComment := &github.PRComment{
			// Owner and Repo will be determined by the service based on workspace
			WorkspaceID: m.workspace.ID,
			PRNumber:    m.prDetails.PRNumber,
			FilePath:    getFilePath(issue),
			LineStart:   issue.LineStart,
			LineEnd:     issue.LineEnd,
			Commentary:  m.textInput.Value(), // Use the edited text directly
		}

		// Check if workspace has a GitRepoURL - fail early if not
		if m.workspace.GitRepoURL == "" {
			loggy.Error("Cannot submit GitHub PR comment: workspace has no Git repository URL",
				"workspace_id", m.workspace.ID,
				"workspace_name", m.workspace.Name)

			return GitHubPRSubmitMsg{
				IssueID: issueID,
				Success: false,
				Error:   fmt.Errorf("workspace '%s' has no Git repository URL configured - please set GitRepoURL for this workspace", m.workspace.Name),
			}
		}

		loggy.Debug("Submitting GitHub PR comment",
			"issue_id", issue.ID,
			"workspace_id", m.workspace.ID,
			"pr_number", prComment.PRNumber,
			"file", prComment.FilePath,
			"line", prComment.LineStart)

		// Use the agnostic SubmitPRComment method
		err := m.app.GitHub.SubmitPRComment(ctx, prComment)

		return GitHubPRSubmitMsg{
			IssueID: issueID,
			Success: err == nil,
			Error:   err,
		}
	}
}
