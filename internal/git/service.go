// Package git provides Git integration for the Mindnest application
package git

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
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
	repo   *git.Repository
}

// NewService creates a new Git service
func NewService(logger *loggy.Logger) *Service {
	return &Service{
		logger: logger,
	}
}

// InitRepo initializes the git repository for the service
func (s *Service) InitRepo(repoPath string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("opening git repo: %w", err)
	}

	s.repo = repo
	return nil
}

// ensureRepo ensures the repository is initialized before performing operations
func (s *Service) ensureRepo() error {
	if s.repo == nil {
		return fmt.Errorf("git repository not initialized")
	}
	return nil
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
	if err := s.ensureRepo(); err != nil {
		return nil, err
	}

	switch req.DiffType {
	case DiffTypeStaged:
		return s.getStagedDiff()
	case DiffTypeCommit:
		return s.getCommitDiff(req.CommitID)
	case DiffTypeBranch:
		return s.getBranchDiff(req.BranchOne, req.BranchTwo)
	default:
		return nil, fmt.Errorf("unsupported diff type: %s", req.DiffType)
	}
}

// getStagedDiff retrieves staged changes in the repository
func (s *Service) getStagedDiff() (*DiffResult, error) {
	// Debug info
	s.logger.Debug("Starting getStagedDiff")

	worktree, err := s.repo.Worktree()
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
			patch, content, err := getStagedFileContent(s.repo, worktree, filePath, changeType)
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
func (s *Service) getCommitDiff(commitID string) (*DiffResult, error) {
	hash := plumbing.NewHash(commitID)
	commit, err := s.repo.CommitObject(hash)
	if err != nil {
		return nil, fmt.Errorf("getting commit object: %w", err)
	}

	s.logger.Debug("Processing commit",
		"hash", commitID,
		"message", commit.Message,
		"author", commit.Author.Name)

	// Get the commit's parent
	parentCommit, err := s.getParentCommit(commit)
	if err != nil {
		return nil, fmt.Errorf("getting parent commit: %w", err)
	}

	if parentCommit != nil {
		s.logger.Debug("Found parent commit",
			"parent_hash", parentCommit.Hash.String(),
			"parent_message", parentCommit.Message)
	} else {
		s.logger.Debug("No parent commit found - this is the initial commit")
	}

	// Get the diff between the commit and its parent
	changes, err := s.getCommitChanges(commit, parentCommit)
	if err != nil {
		return nil, fmt.Errorf("getting commit changes: %w", err)
	}

	s.logger.Debug("Retrieved commit changes", "changes_count", len(changes))

	// Process the changes
	files, err := s.processChanges(changes)
	if err != nil {
		return nil, fmt.Errorf("processing changes: %w", err)
	}

	s.logger.Debug("Processed changed files", "files_count", len(files))
	for _, file := range files {
		s.logger.Debug("Changed file details",
			"path", file.Path,
			"old_path", file.OldPath,
			"change_type", file.ChangeType,
			"content_length", len(file.Content),
			"patch_length", len(file.Patch))
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

// getParentCommit returns the parent commit of the given commit
func (s *Service) getParentCommit(commit *object.Commit) (*object.Commit, error) {
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

// getChangesFromEmptyTree gets changes by comparing an empty tree with the given commit
func getChangesFromEmptyTree(commit *object.Commit) (object.Changes, error) {
	commitTree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("getting commit tree: %w", err)
	}

	// For initial commits, we want to diff empty -> commit (not commit -> empty)
	// This makes "new files" appear as "Insert" rather than "Delete"
	emptyTree := &object.Tree{}
	changes, err := emptyTree.Diff(commitTree)
	if err != nil {
		return nil, fmt.Errorf("getting diff with empty tree: %w", err)
	}

	return changes, nil
}

// getCommitChanges retrieves the changes between a commit and its parent
func (s *Service) getCommitChanges(commit, parentCommit *object.Commit) (object.Changes, error) {
	var changes object.Changes
	var err error

	if parentCommit == nil {
		// For first commit, compare with empty tree
		changes, err = getChangesFromEmptyTree(commit)
		s.logger.Debug("Getting changes from empty tree", "error", err)
	} else {
		// Get the trees for both commits
		currentTree, err := commit.Tree()
		if err != nil {
			return nil, fmt.Errorf("getting current tree: %w", err)
		}

		parentTree, err := parentCommit.Tree()
		if err != nil {
			return nil, fmt.Errorf("getting parent tree: %w", err)
		}

		// Get the changes between the two trees
		changes, err = parentTree.Diff(currentTree)
		s.logger.Debug("Getting changes between trees",
			"current_tree", currentTree.Hash.String(),
			"parent_tree", parentTree.Hash.String(),
			"error", err)
	}

	if err != nil {
		return nil, fmt.Errorf("getting changes: %w", err)
	}

	// Log each change
	for i, change := range changes {
		s.logger.Debug("Found change",
			"index", i,
			"from", change.From.Name,
			"to", change.To.Name,
			"from_hash", change.From.TreeEntry.Hash.String(),
			"to_hash", change.To.TreeEntry.Hash.String())
	}

	return changes, nil
}

// getBranchDiff retrieves changes between two branches
func (s *Service) getBranchDiff(branch1, branch2 string) (*DiffResult, error) {
	// Get the reference for branch1
	branch1Ref, err := s.repo.Reference(plumbing.NewBranchReferenceName(branch1), true)
	if err != nil {
		return nil, fmt.Errorf("getting reference for branch1: %w", err)
	}

	// Get the reference for branch2
	branch2Ref, err := s.repo.Reference(plumbing.NewBranchReferenceName(branch2), true)
	if err != nil {
		return nil, fmt.Errorf("getting reference for branch2: %w", err)
	}

	// Get the commit object for branch1
	branch1Commit, err := s.repo.CommitObject(branch1Ref.Hash())
	if err != nil {
		return nil, fmt.Errorf("getting commit for branch1: %w", err)
	}

	// Get the commit object for branch2
	branch2Commit, err := s.repo.CommitObject(branch2Ref.Hash())
	if err != nil {
		return nil, fmt.Errorf("getting commit for branch2: %w", err)
	}

	// Get the changes between the two branches
	changes, err := s.getBranchChanges(branch1Commit, branch2Commit)
	if err != nil {
		return nil, fmt.Errorf("getting branch changes: %w", err)
	}

	// Process the changes
	files, err := s.processChanges(changes)
	if err != nil {
		return nil, fmt.Errorf("processing changes: %w", err)
	}

	return &DiffResult{
		Files: files,
	}, nil
}

// getBranchChanges returns the changes between two branches
func (s *Service) getBranchChanges(branch1Commit, branch2Commit *object.Commit) (object.Changes, error) {
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
func (s *Service) processChanges(changes object.Changes) ([]ChangedFile, error) {
	var files []ChangedFile

	s.logger.Debug("Starting to process changes", "total_changes", len(changes))

	for i, change := range changes {
		s.logger.Debug("Processing change",
			"index", i,
			"from_name", change.From.Name,
			"to_name", change.To.Name)

		file, err := s.processChange(change)
		if err != nil {
			s.logger.Error("Failed to process change",
				"error", err,
				"from_name", change.From.Name,
				"to_name", change.To.Name)
			continue
		}

		files = append(files, file)
		s.logger.Debug("Successfully processed change",
			"index", i,
			"path", file.Path,
			"old_path", file.OldPath,
			"change_type", file.ChangeType)
	}

	return files, nil
}

// processChange converts a single go-git Change to our ChangedFile model
func (s *Service) processChange(change *object.Change) (ChangedFile, error) {
	var result ChangedFile
	var err error

	// Get the paths
	fromName := ""
	if change.From.Name != "" {
		fromName = filepath.Clean(change.From.Name)
	}

	toName := ""
	if change.To.Name != "" {
		toName = filepath.Clean(change.To.Name)
	}

	// Normalize the path we'll use
	path := toName
	if path == "" {
		path = fromName
	}

	s.logger.Debug("Normalized path",
		"original", change.To.Name,
		"cleaned", toName,
		"normalized", path)

	// Determine change type
	changeType := getChangeTypeFromChange(change)
	s.logger.Debug("Change type determined",
		"path", path,
		"old_path", fromName,
		"change_type", changeType)

	// Generate patch
	patch, err := change.Patch()
	if err != nil {
		return result, fmt.Errorf("generating patch: %w", err)
	}
	patchStr := patch.String()
	s.logger.Debug("Generated patch",
		"path", path,
		"patch_length", len(patchStr))

	// Get file content
	s.logger.Debug("Attempting to get file content",
		"path", path,
		"is_initial_commit", change.From.Name == "",
		"has_new_file", change.To.Name != "")

	switch changeType {
	case ChangeTypeAdded:
		blob, err := s.repo.BlobObject(change.To.TreeEntry.Hash)
		if err != nil {
			return result, fmt.Errorf("getting blob for added file: %w", err)
		}
		reader, err := blob.Reader()
		if err != nil {
			return result, fmt.Errorf("getting reader for added file: %w", err)
		}
		defer reader.Close()

		content, err := io.ReadAll(reader)
		if err != nil {
			return result, fmt.Errorf("reading added file content: %w", err)
		}

		result.Content = string(content)

	case ChangeTypeDeleted:
		blob, err := s.repo.BlobObject(change.From.TreeEntry.Hash)
		if err != nil {
			return result, fmt.Errorf("getting blob for deleted file: %w", err)
		}
		reader, err := blob.Reader()
		if err != nil {
			return result, fmt.Errorf("getting reader for deleted file: %w", err)
		}
		defer reader.Close()

		content, err := io.ReadAll(reader)
		if err != nil {
			return result, fmt.Errorf("reading deleted file content: %w", err)
		}

		result.Content = string(content)

	case ChangeTypeModified:
		blob, err := s.repo.BlobObject(change.To.TreeEntry.Hash)
		if err != nil {
			return result, fmt.Errorf("getting blob for modified file: %w", err)
		}
		reader, err := blob.Reader()
		if err != nil {
			return result, fmt.Errorf("getting reader for modified file: %w", err)
		}
		defer reader.Close()

		content, err := io.ReadAll(reader)
		if err != nil {
			return result, fmt.Errorf("reading modified file content: %w", err)
		}

		result.Content = string(content)
	}

	// Build the result
	result = ChangedFile{
		Path:       path,
		OldPath:    fromName,
		Content:    result.Content,
		Patch:      patchStr,
		ChangeType: changeType,
	}

	return result, nil
}

// getChangeTypeFromChange determines the type of change from a git object.Change
func getChangeTypeFromChange(change *object.Change) ChangeType {
	// If From is empty (zero hash) and To exists, it's an addition
	if change.From.TreeEntry.Hash.IsZero() && !change.To.TreeEntry.Hash.IsZero() {
		return ChangeTypeAdded
	}

	// If To is empty (zero hash) and From exists, it's a deletion
	if !change.From.TreeEntry.Hash.IsZero() && change.To.TreeEntry.Hash.IsZero() {
		return ChangeTypeDeleted
	}

	// Otherwise it's a modification
	return ChangeTypeModified
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
func (s *Service) ListBranches() ([]string, error) {
	branches := []string{}

	branchIter, err := s.repo.Branches()
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
func (s *Service) ListCommits(limit int) ([]*Commit, error) {
	headRef, err := s.repo.Head()
	if err != nil {
		return nil, fmt.Errorf("getting HEAD: %w", err)
	}

	// Get the HEAD commit
	commit, err := s.repo.CommitObject(headRef.Hash())
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
