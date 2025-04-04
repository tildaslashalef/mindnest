package utils

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/goombaio/namegenerator"
)

// GenerateProjectName creates a random, memorable project name using namegenerator
func GenerateProjectName() string {
	seed := time.Now().UTC().UnixNano()
	nameGenerator := namegenerator.NewNameGenerator(seed)

	// Generate a name like "wispy-dust"
	name := nameGenerator.Generate()

	// Some names might have underscores; convert to hyphens for consistency
	name = strings.ReplaceAll(name, "_", "-")

	return name
}

// SanitizeDirectoryName cleans up a directory name to be used as a workspace name
func SanitizeDirectoryName(dirName string) string {
	// Replace spaces with hyphens and convert to lowercase
	name := strings.ToLower(strings.ReplaceAll(dirName, " ", "-"))

	// Replace other non-alphanumeric characters with hyphens
	// (except for already existing hyphens and periods)
	replacer := strings.NewReplacer(
		"_", "-",
		".", "-",
		",", "-",
		";", "-",
		":", "-",
		"/", "-",
		"\\", "-",
	)
	name = replacer.Replace(name)

	// Replace multiple consecutive hyphens with a single hyphen
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}

	// Remove leading and trailing hyphens
	name = strings.Trim(name, "-")

	return name
}

// CopyToClipboard copies the given text to the system clipboard
func CopyToClipboard(text string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		cmd = exec.Command("xclip", "-selection", "clipboard")
	case "windows":
		cmd = exec.Command("clip")
	default:
		return fmt.Errorf("unsupported platform for clipboard operations")
	}

	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}
