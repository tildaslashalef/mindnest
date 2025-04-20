package sync

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/tildaslashalef/mindnest/internal/loggy"
)

// Update handles messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width
		m.progress.Width = msg.Width - 10 // Adjust progress bar width
		m.ready = true                    // Consider ready once we have dimensions

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keymap.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keymap.Help):
			m.help.ShowAll = !m.help.ShowAll
			m.showHelp = !m.showHelp
			return m, nil
		case key.Matches(msg, m.keymap.Enter):
			if !m.syncing && m.ready && m.result == nil {
				m.syncing = true
				m.status = "Starting sync..."
				cmds = append(cmds, m.startSync(), m.spinner.Tick)
			} else if m.result != nil || m.error != "" {
				// If sync is done or errored, enter quits
				return m, tea.Quit
			}
		}

	case spinner.TickMsg:
		if m.loading || m.syncing {
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		cmds = append(cmds, cmd)

	case SyncStartMsg:
		m.loading = false
		m.ready = true
		m.status = "Ready to sync. Press Enter to start."
		loggy.Debug("Received SyncStartMsg, calculating items to sync.")
		cmds = append(cmds, m.getEntitiesToSyncCmd)

	case EntitiesToSyncMsg:
		m.workspaceCount = msg.WorkspaceCount
		m.reviewCount = msg.ReviewCount
		m.reviewFileCount = msg.ReviewFileCount
		m.issueCount = msg.IssueCount
		m.fileCount = msg.FileCount
		m.entityTypes = msg.EntityTypes // Store the order
		m.status = fmt.Sprintf("Found %d items to sync.", msg.TotalEntities)
		loggy.Debug("Received EntitiesToSyncMsg", "total_items", msg.TotalEntities)

	case SyncProgressMsg:
		m.lastProgress = msg
		m.currentStage = string(msg.EntityType)
		cmd = m.progress.SetPercent(msg.ProgressValue)
		cmds = append(cmds, cmd)
		if msg.ErrorMessage != "" {
			m.status = fmt.Sprintf("Error syncing %s %s: %s", msg.EntityType, msg.EntityID, msg.ErrorMessage)
		} else {
			m.status = fmt.Sprintf("Syncing %s (%d/%d)", msg.EntityType, msg.CurrentItem, msg.TotalItems)
		}
		loggy.Debug("SyncProgress", "type", msg.EntityType, "id", msg.EntityID, "current", msg.CurrentItem, "total", msg.TotalItems, "progress", msg.ProgressValue)

	case SyncCompleteMsg:
		m.syncing = false
		m.result = msg.Result
		if msg.Error != nil {
			m.error = msg.Error.Error()
			m.status = "Sync completed with errors."
			loggy.Error("Sync completed with error", "error", msg.Error)
		} else {
			m.status = "Sync complete! Press Enter or q to quit."
			loggy.Info("Sync completed successfully", "result", msg.Result)
		}
		// Store final counts from the message
		m.workspaceCount = msg.WorkspaceCount
		m.reviewCount = msg.ReviewCount
		m.reviewFileCount = msg.ReviewFileCount
		m.issueCount = msg.IssueCount
		m.fileCount = msg.FileCount

	default:
		// Handle spinner and progress updates even if no other match
		if m.loading || m.syncing {
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}
