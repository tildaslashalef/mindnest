// Package git provides Git integration for the Mindnest application
package git

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/tildaslashalef/mindnest/internal/loggy"
)

// Service provides Git operations
type Service struct {
	logger *loggy.Logger
}

// NewService creates a new Git service
func NewService(logger *loggy.Logger) *Service {
	return &Service{
		logger: logger,
	}
}

// HasGitRepo checks if the provided path contains a valid Git repository
func (s *Service) HasGitRepo(path string) bool {
	// Try to open the repository using go-git
	_, err := git.PlainOpen(path)
	if err != nil {
		s.logger.Debug("Not a valid Git repository", "path", path, "error", err)
		return false
	}

	return true
}

// GetDiff retrieves a diff based on the request parameters
func (s *Service) GetDiff(req DiffRequest) (*DiffResult, error) {
	repo, err := git.PlainOpen(req.RepoPath)
	if err != nil {
		return nil, fmt.Errorf("opening git repository: %w", err)
	}

	switch req.DiffType {
	case DiffTypeStaged:
		return s.getStagedDiff(repo)
	case DiffTypeCommit:
		return s.getCommitDiff(repo, req.CommitID)
	case DiffTypeBranch:
		return s.getBranchDiff(repo, req.BranchOne, req.BranchTwo)
	default:
		return nil, fmt.Errorf("unsupported diff type: %s", req.DiffType)
	}
}

// getStagedDiff retrieves staged changes in the repository
func (s *Service) getStagedDiff(repo *git.Repository) (*DiffResult, error) {
	// Debug info
	s.logger.Debug("Starting getStagedDiff")

	worktree, err := repo.Worktree()
	if err != nil {
		s.logger.Debug("Error getting worktree", "error", err)
		return nil, fmt.Errorf("getting worktree: %w", err)
	}

	status, err := worktree.Status()
	if err != nil {
		s.logger.Debug("Error getting worktree status", "error", err)
		return nil, fmt.Errorf("getting worktree status: %w", err)
	}

	// Debug: Print status info
	s.logger.Debug("Worktree status retrieved", "status_length", len(status))

	for path, fileStatus := range status {
		s.logger.Debug("File status",
			"path", path,
			"staging", fmt.Sprintf("%d", fileStatus.Staging),
			"worktree", fmt.Sprintf("%d", fileStatus.Worktree))
	}

	if len(status) == 0 {
		s.logger.Debug("No changes detected in worktree status")
		return &DiffResult{
			Files: []ChangedFile{},
		}, nil
	}

	// Get the staging area (index)
	var files []ChangedFile
	for filePath, fileStatus := range status {
		// Only include staged files (Added, Modified, Deleted)
		if fileStatus.Staging != git.Unmodified {
			changeType := getChangeType(fileStatus.Staging)

			s.logger.Debug("Processing staged file",
				"path", filePath,
				"change_type", changeType,
				"staging_status", fmt.Sprintf("%d", fileStatus.Staging))

			// Get the content and patch from the index
			patch, content, err := getStagedFileContent(repo, worktree, filePath, changeType)
			if err != nil {
				s.logger.Warn("Failed to get staged file content", "path", filePath, "error", err)
				// Continue with other files even if one fails
				continue
			}

			s.logger.Debug("Retrieved file content",
				"path", filePath,
				"content_length", len(content),
				"patch_length", len(patch))

			files = append(files, ChangedFile{
				Path:       filePath,
				ChangeType: changeType,
				Content:    content,
				Patch:      patch,
			})
		} else {
			s.logger.Debug("Skipping unstaged file", "path", filePath, "staging", fmt.Sprintf("%d", fileStatus.Staging))
		}
	}

	s.logger.Debug("Completed getStagedDiff", "files_found", len(files))

	return &DiffResult{
		Files: files,
	}, nil
}

// getCommitDiff retrieves changes in a specific commit
func (s *Service) getCommitDiff(repo *git.Repository, commitID string) (*DiffResult, error) {
	hash := plumbing.NewHash(commitID)
	commit, err := repo.CommitObject(hash)
	if err != nil {
		return nil, fmt.Errorf("getting commit object: %w", err)
	}

	// Get the commit's parent
	parentCommit, err := getParentCommit(commit)
	if err != nil {
		return nil, fmt.Errorf("getting parent commit: %w", err)
	}

	// Get the diff between the commit and its parent
	changes, err := getCommitChanges(commit, parentCommit)
	if err != nil {
		return nil, fmt.Errorf("getting commit changes: %w", err)
	}

	// Process the changes
	files, err := processChanges(changes)
	if err != nil {
		return nil, fmt.Errorf("processing changes: %w", err)
	}

	// Create the commit info
	commitInfo := &Commit{
		Hash:      commit.Hash.String(),
		Author:    commit.Author.Name,
		Email:     commit.Author.Email,
		Message:   commit.Message,
		Timestamp: commit.Author.When,
	}

	return &DiffResult{
		Files:      files,
		CommitInfo: commitInfo,
	}, nil
}

// getBranchDiff retrieves changes between two branches
func (s *Service) getBranchDiff(repo *git.Repository, branch1, branch2 string) (*DiffResult, error) {
	// Get the reference for branch1
	branch1Ref, err := repo.Reference(plumbing.NewBranchReferenceName(branch1), true)
	if err != nil {
		return nil, fmt.Errorf("getting reference for branch1: %w", err)
	}

	// Get the reference for branch2
	branch2Ref, err := repo.Reference(plumbing.NewBranchReferenceName(branch2), true)
	if err != nil {
		return nil, fmt.Errorf("getting reference for branch2: %w", err)
	}

	// Get the commit object for branch1
	branch1Commit, err := repo.CommitObject(branch1Ref.Hash())
	if err != nil {
		return nil, fmt.Errorf("getting commit for branch1: %w", err)
	}

	// Get the commit object for branch2
	branch2Commit, err := repo.CommitObject(branch2Ref.Hash())
	if err != nil {
		return nil, fmt.Errorf("getting commit for branch2: %w", err)
	}

	// Get the changes between the two branches
	changes, err := getBranchChanges(branch1Commit, branch2Commit)
	if err != nil {
		return nil, fmt.Errorf("getting branch changes: %w", err)
	}

	// Process the changes
	files, err := processChanges(changes)
	if err != nil {
		return nil, fmt.Errorf("processing changes: %w", err)
	}

	return &DiffResult{
		Files: files,
	}, nil
}

// getParentCommit returns the parent commit of the given commit
func getParentCommit(commit *object.Commit) (*object.Commit, error) {
	if commit.NumParents() == 0 {
		// For first commit, compare with empty tree
		return nil, nil
	}

	parent, err := commit.Parent(0)
	if err != nil {
		return nil, fmt.Errorf("getting parent commit: %w", err)
	}

	return parent, nil
}

// getCommitChanges returns the changes between a commit and its parent
func getCommitChanges(commit, parentCommit *object.Commit) (object.Changes, error) {
	var changes object.Changes

	if parentCommit == nil {
		// This is the first commit
		commitTree, err := commit.Tree()
		if err != nil {
			return nil, fmt.Errorf("getting commit tree: %w", err)
		}

		slog.Debug("Handling initial commit",
			"commit_hash", commit.Hash.String(),
			"commit_message", commit.Message)

		// For initial commits, we want to diff empty -> commit (not commit -> empty)
		// This makes "new files" appear as "Insert" rather than "Delete"
		emptyTree := &object.Tree{}
		changes, err = emptyTree.Diff(commitTree)
		if err != nil {
			return nil, fmt.Errorf("getting diff with empty tree: %w", err)
		}
	} else {
		// Get the trees for both commits
		commitTree, err := commit.Tree()
		if err != nil {
			return nil, fmt.Errorf("getting commit tree: %w", err)
		}

		parentTree, err := parentCommit.Tree()
		if err != nil {
			return nil, fmt.Errorf("getting parent tree: %w", err)
		}

		// Get the changes between the two trees
		changes, err = parentTree.Diff(commitTree)
		if err != nil {
			return nil, fmt.Errorf("getting diff: %w", err)
		}
	}

	return changes, nil
}

// getBranchChanges returns the changes between two branches
func getBranchChanges(branch1Commit, branch2Commit *object.Commit) (object.Changes, error) {
	// Get the trees for both branches
	branch1Tree, err := branch1Commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("getting branch1 tree: %w", err)
	}

	branch2Tree, err := branch2Commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("getting branch2 tree: %w", err)
	}

	// Get the changes between the two trees
	changes, err := branch1Tree.Diff(branch2Tree)
	if err != nil {
		return nil, fmt.Errorf("getting diff: %w", err)
	}

	return changes, nil
}

// processChanges converts go-git Changes to our ChangedFile model
func processChanges(changes object.Changes) ([]ChangedFile, error) {
	var files []ChangedFile

	for _, change := range changes {
		file, err := processChange(change)
		if err != nil {
			// Log the error but continue processing other changes
			slog.Warn("Failed to process change", "error", err)
			continue
		}

		files = append(files, file)
	}

	return files, nil
}

// processChange converts a single go-git Change to our ChangedFile model
func processChange(change *object.Change) (ChangedFile, error) {
	var file ChangedFile

	// Determine the change type
	action, err := change.Action()
	if err != nil {
		return file, fmt.Errorf("determining change action: %w", err)
	}

	// Convert the action to a string for comparison
	actionStr := action.String()

	// For initial commits - reverse the order of the diff
	// In initial commits, go-git reports files backward: empty tree to commit
	// This makes new files appear as "Delete" operations
	isInitialCommitNewFile := false

	// If action is Delete but the To.Name exists and the To.Hash is not zero
	// then this is actually a new file in an initial commit
	if actionStr == "Delete" && change.To.Name != "" && !change.To.TreeEntry.Hash.IsZero() {
		isInitialCommitNewFile = true
	}

	if isInitialCommitNewFile {
		// This is actually a new file in the first commit
		file.ChangeType = ChangeTypeAdded
		file.Path = change.To.Name
	} else {
		// Regular case - set the change type and path based on action
		switch actionStr {
		case "Insert":
			file.ChangeType = ChangeTypeAdded
			file.Path = change.To.Name
		case "Delete":
			file.ChangeType = ChangeTypeDeleted
			file.Path = change.From.Name
		case "Modify":
			file.ChangeType = ChangeTypeModified
			file.Path = change.To.Name
		default:
			// Handle renames and other changes
			if change.From.Name != change.To.Name {
				file.ChangeType = ChangeTypeRenamed
				file.OldPath = change.From.Name
				file.Path = change.To.Name
			} else {
				file.ChangeType = ChangeTypeModified
				file.Path = change.To.Name
			}
		}
	}

	// Get the patch
	patch, err := change.Patch()
	if err != nil {
		return file, fmt.Errorf("getting patch: %w", err)
	}

	// Convert patch to string
	var buf bytes.Buffer
	err = patch.Encode(&buf)
	if err != nil {
		return file, fmt.Errorf("encoding patch: %w", err)
	}
	file.Patch = buf.String()

	// Get the content for non-deleted files and initial commit files
	if file.ChangeType != ChangeTypeDeleted || isInitialCommitNewFile {
		// For initial commits, we need to get content from the To field
		// even though the action might be "Delete"
		if isInitialCommitNewFile || change.To.TreeEntry.Mode.IsFile() {
			// Get the file from the tree
			var treeFile *object.File
			var fileErr error

			if isInitialCommitNewFile {
				// For initial commit, look in the To tree
				treeFile, fileErr = change.To.Tree.File(change.To.Name)
			} else {
				treeFile, fileErr = change.To.Tree.File(change.To.Name)
			}

			if fileErr != nil {
				return file, fmt.Errorf("getting file from tree: %w", fileErr)
			}

			// Get the content
			content, fileErr := treeFile.Contents()
			if fileErr != nil {
				return file, fmt.Errorf("getting file contents: %w", fileErr)
			}

			file.Content = content
		}
	}

	return file, nil
}

// getStagedFileContent gets the content and patch for a staged file
func getStagedFileContent(repo *git.Repository, worktree *git.Worktree, filePath string, changeType ChangeType) (string, string, error) {
	var content, patch string

	// If the file is deleted, there's no content to retrieve
	if changeType == ChangeTypeDeleted {
		// Just return an empty content
		return "", "", nil
	}

	// Get the file info
	fileInfo, err := worktree.Filesystem.Stat(filePath)
	if err != nil {
		return "", "", fmt.Errorf("getting file info: %w", err)
	}

	// Read the file content
	file, err := worktree.Filesystem.Open(filePath)
	if err != nil {
		return "", "", fmt.Errorf("opening file: %w", err)
	}
	defer file.Close()

	// Read the content
	contentBuf := make([]byte, fileInfo.Size())
	_, err = file.Read(contentBuf)
	if err != nil {
		return "", "", fmt.Errorf("reading file: %w", err)
	}

	content = string(contentBuf)

	// For the patch, we need to compare with HEAD
	headRef, err := repo.Head()
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			// No HEAD yet (new repo), just return the content
			return "", content, nil
		}
		return "", "", fmt.Errorf("getting HEAD: %w", err)
	}

	headCommit, err := repo.CommitObject(headRef.Hash())
	if err != nil {
		return "", "", fmt.Errorf("getting HEAD commit: %w", err)
	}

	headTree, err := headCommit.Tree()
	if err != nil {
		return "", "", fmt.Errorf("getting HEAD tree: %w", err)
	}

	// Try to get the file from HEAD
	headFile, err := headTree.File(filePath)
	if err != nil {
		if errors.Is(err, object.ErrFileNotFound) {
			// File is new, no patch needed
			return "", content, nil
		}
		return "", "", fmt.Errorf("getting file from HEAD: %w", err)
	}

	// Get the content from HEAD
	headContent, err := headFile.Contents()
	if err != nil {
		return "", "", fmt.Errorf("getting HEAD file content: %w", err)
	}

	// Generate a diff between HEAD and staged
	patch = generatePatch(filePath, headContent, content)

	return patch, content, nil
}

// generatePatch generates a simple unified diff
func generatePatch(filePath, oldContent, newContent string) string {
	// This is a very simple patch generator
	// For real-world usage, consider using a more sophisticated diff library
	var buf strings.Builder

	buf.WriteString(fmt.Sprintf("--- a/%s\n", filePath))
	buf.WriteString(fmt.Sprintf("+++ b/%s\n", filePath))

	// Split content into lines
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	// Very simple diff - this is not a proper diff algorithm
	// but serves as a placeholder for demonstration
	buf.WriteString("@@ -1,0 +1,0 @@\n")
	for _, line := range oldLines {
		buf.WriteString(fmt.Sprintf("-%s\n", line))
	}
	for _, line := range newLines {
		buf.WriteString(fmt.Sprintf("+%s\n", line))
	}

	return buf.String()
}

// getChangeType converts go-git StatusCode to our ChangeType
func getChangeType(code git.StatusCode) ChangeType {
	switch code {
	case git.Added, git.Untracked:
		return ChangeTypeAdded
	case git.Modified, git.UpdatedButUnmerged:
		return ChangeTypeModified
	case git.Deleted:
		return ChangeTypeDeleted
	case git.Renamed:
		return ChangeTypeRenamed
	default:
		return ChangeTypeModified // Default case
	}
}

// ListBranches returns a list of all branches in the repository
func (s *Service) ListBranches(repoPath string) ([]string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("opening git repository: %w", err)
	}

	branches := []string{}

	branchIter, err := repo.Branches()
	if err != nil {
		return nil, fmt.Errorf("getting branches: %w", err)
	}

	err = branchIter.ForEach(func(ref *plumbing.Reference) error {
		// Remove the refs/heads/ prefix
		name := strings.TrimPrefix(ref.Name().String(), "refs/heads/")
		branches = append(branches, name)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("iterating branches: %w", err)
	}

	return branches, nil
}

// ListCommits returns a list of commits in the repository
func (s *Service) ListCommits(repoPath string, limit int) ([]*Commit, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("opening git repository: %w", err)
	}

	headRef, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("getting HEAD: %w", err)
	}

	// Get the HEAD commit
	commit, err := repo.CommitObject(headRef.Hash())
	if err != nil {
		return nil, fmt.Errorf("getting HEAD commit: %w", err)
	}

	// Prepare the commit iterator
	commitIter := object.NewCommitIterCTime(commit, nil, nil)
	defer commitIter.Close()

	var commits []*Commit
	count := 0

	// Iterate through commits
	err = commitIter.ForEach(func(c *object.Commit) error {
		if limit > 0 && count >= limit {
			return storer.ErrStop
		}

		commits = append(commits, &Commit{
			Hash:      c.Hash.String(),
			Author:    c.Author.Name,
			Email:     c.Author.Email,
			Message:   c.Message,
			Timestamp: c.Author.When,
		})

		count++
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("iterating commits: %w", err)
	}

	return commits, nil
}
