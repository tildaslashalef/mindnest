package sync

import (
	"fmt"
	"strings"
)

// View renders the sync TUI.
func (m Model) View() string {
	if m.error != "" {
		return m.styles.Error.Render(fmt.Sprintf("Error: %s\n\nPress q to quit.", m.error))
	}

	if m.result != nil {
		var sb strings.Builder
		sb.WriteString(m.styles.Title.Render("Sync Complete"))
		sb.WriteString("\n\n")
		sb.WriteString(fmt.Sprintf("Workspaces:    %d synced\n", m.workspaceCount))
		sb.WriteString(fmt.Sprintf("Reviews:       %d synced\n", m.reviewCount))
		sb.WriteString(fmt.Sprintf("Review Files:  %d synced\n", m.reviewFileCount))
		sb.WriteString(fmt.Sprintf("Issues:        %d synced\n", m.issueCount))
		sb.WriteString(fmt.Sprintf("Files:         %d synced\n", m.fileCount))
		sb.WriteString("\n\nPress q to quit.")
		return m.styles.Paragraph.Render(sb.String())
	}

	if !m.ready {
		return fmt.Sprintf("%s Initializing... %s", m.spinner.View(), m.status)
	}

	if m.syncing {
		var sb strings.Builder
		sb.WriteString(m.styles.Title.Render(fmt.Sprintf("%s Syncing %s...", m.spinner.View(), m.currentStage)))
		sb.WriteString("\n\n")

		if m.lastProgress.TotalItems > 0 {
			sb.WriteString(fmt.Sprintf("%s: %d/%d",
				m.lastProgress.EntityType,
				m.lastProgress.CurrentItem,
				m.lastProgress.TotalItems))
			sb.WriteString("\n")
			sb.WriteString(m.progress.ViewAs(m.lastProgress.ProgressValue))
			sb.WriteString("\n")
			status := fmt.Sprintf("Synced: %d, Failed: %d", m.lastProgress.ItemsSynced, m.lastProgress.ItemsFailed)
			sb.WriteString(m.styles.StatusText.Render(status))
		} else {
			sb.WriteString(fmt.Sprintf("Preparing %s sync...", m.currentStage))
		}

		sb.WriteString("\n\n")
		helpView := m.help.View(m.keymap)
		sb.WriteString(helpView)

		return sb.String()
	}

	// Ready state, show summary and prompt
	var sb strings.Builder
	sb.WriteString(m.styles.Title.Render("Ready to Sync"))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("Workspaces:    %d\n", m.workspaceCount))
	sb.WriteString(fmt.Sprintf("Reviews:       %d\n", m.reviewCount))
	sb.WriteString(fmt.Sprintf("Review Files:  %d\n", m.reviewFileCount))
	sb.WriteString(fmt.Sprintf("Issues:        %d\n", m.issueCount))
	sb.WriteString(fmt.Sprintf("Files:         %d\n", m.fileCount))
	sb.WriteString("\n")
	if m.dryRun {
		sb.WriteString(m.styles.Warning.Render("Performing DRY RUN (no data will be sent)"))
		sb.WriteString("\n")
	}
	sb.WriteString("Press Enter to start sync, ? for help, q to quit.")
	sb.WriteString("\n\n")

	if m.showHelp {
		helpView := m.help.View(m.keymap)
		sb.WriteString(helpView)
	}

	return sb.String()
}
