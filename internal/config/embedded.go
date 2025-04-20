package config

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/tildaslashalef/mindnest/internal/loggy"
)

//go:embed env.sample
var configFS embed.FS

// SetupConfigDirectory ensures the config directory exists and contains necessary files
func SetupConfigDirectory(configDir string, backupExisting bool) error {
	// Ensure config directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	// Extract sample env file (with backup if it exists)
	sampleEnvPath := filepath.Join(configDir, ".env")
	if err := ExtractEmbeddedFile("env.sample", sampleEnvPath, backupExisting); err != nil {
		loggy.Warn("Failed to extract sample env file", "error", err)
		// Continue anyway, this is not critical
	}

	return nil
}

// ExtractEmbeddedFile extracts an embedded file to the target path if it doesn't exist
// If backupExisting is true and the file exists, it will be backed up before overwriting
func ExtractEmbeddedFile(embeddedPath, targetPath string, backupExisting bool) error {
	// Check if target file already exists
	if _, err := os.Stat(targetPath); err == nil {
		// File exists
		if backupExisting {
			// Create backup file path with timestamp
			timeStamp := time.Now().Format("2006-01-02")
			backupPath := fmt.Sprintf("%s.%s.bak", targetPath, timeStamp)

			// Copy the existing file to a backup
			existingData, err := os.ReadFile(targetPath)
			if err != nil {
				return fmt.Errorf("failed to read existing file for backup: %w", err)
			}

			if err := os.WriteFile(backupPath, existingData, 0644); err != nil {
				return fmt.Errorf("failed to write backup file: %w", err)
			}

			loggy.Info("Created backup of existing file", "original", targetPath, "backup", backupPath)
		} else {
			// Skip extraction if not backing up
			return nil
		}
	}

	// Read embedded file
	fileData, err := configFS.ReadFile(embeddedPath)
	if err != nil {
		return err
	}

	// Ensure target directory exists
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return err
	}

	// Write file to target path
	if err := os.WriteFile(targetPath, fileData, 0644); err != nil {
		return err
	}

	loggy.Info("Extracted embedded file", "source", embeddedPath, "target", targetPath)
	return nil
}

// ListEmbeddedFiles lists all embedded config files for debugging
func ListEmbeddedFiles() []string {
	var files []string

	err := fs.WalkDir(configFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		loggy.Error("Failed to list embedded files", "error", err)
	}

	return files
}
