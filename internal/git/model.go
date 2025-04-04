// Package git provides Git integration for the Mindnest application
package git

import (
	"strings"
	"time"
)

// DiffType represents the source of a diff
type DiffType string

const (
	// DiffTypeStaged represents staged changes in a Git repository
	DiffTypeStaged DiffType = "staged"
	// DiffTypeCommit represents changes in a specific commit
	DiffTypeCommit DiffType = "commit"
	// DiffTypeBranch represents changes between two branches
	DiffTypeBranch DiffType = "branch"
)

// ChangeType represents the type of change to a file
type ChangeType string

const (
	// ChangeTypeAdded represents a file that was added
	ChangeTypeAdded ChangeType = "added"
	// ChangeTypeModified represents a file that was modified
	ChangeTypeModified ChangeType = "modified"
	// ChangeTypeDeleted represents a file that was deleted
	ChangeTypeDeleted ChangeType = "deleted"
	// ChangeTypeRenamed represents a file that was renamed
	ChangeTypeRenamed ChangeType = "renamed"
)

// ChangedFile represents a file that was changed in a diff
type ChangedFile struct {
	Path       string     `json:"path"`
	OldPath    string     `json:"old_path,omitempty"` // Only used for renamed files
	ChangeType ChangeType `json:"change_type"`
	Content    string     `json:"content,omitempty"`
	Patch      string     `json:"patch,omitempty"`
	Language   string     `json:"language,omitempty"`
}

// FileLine represents a line of code in a file with metadata
type FileLine struct {
	LineNum     int    `json:"line_num"`
	Content     string `json:"content"`
	AddedInDiff bool   `json:"added_in_diff"`
}

// Commit represents a Git commit
type Commit struct {
	Hash      string    `json:"hash"`
	Author    string    `json:"author"`
	Email     string    `json:"email"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// DiffRequest represents a request to get a diff
type DiffRequest struct {
	RepoPath  string   `json:"repo_path"`
	DiffType  DiffType `json:"diff_type"`
	CommitID  string   `json:"commit_id,omitempty"`
	BranchOne string   `json:"branch_one,omitempty"`
	BranchTwo string   `json:"branch_two,omitempty"`
}

// DiffResult represents the result of a diff operation
type DiffResult struct {
	Files      []ChangedFile `json:"files"`
	CommitInfo *Commit       `json:"commit_info,omitempty"`
	Error      error         `json:"error,omitempty"`
}

// IsGo returns true if the file is a Go file
func (f *ChangedFile) IsGo() bool {
	return strings.HasSuffix(f.Path, ".go")
}
