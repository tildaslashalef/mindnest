package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tildaslashalef/mindnest/internal/loggy"
)

// Helper function to set up a temporary Git repository
func setupTempGitRepo(t *testing.T) string {
	// Create a temporary directory for the Git repository
	tempDir, err := os.MkdirTemp("", "git_test_*")
	require.NoError(t, err, "Failed to create temporary directory")

	// Initialize Git repository
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	err = cmd.Run()
	require.NoError(t, err, "Failed to initialize Git repository")

	// Configure Git user for commits
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tempDir
	err = cmd.Run()
	require.NoError(t, err, "Failed to set Git user name")

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tempDir
	err = cmd.Run()
	require.NoError(t, err, "Failed to set Git user email")

	// Create initial commit so we have a master branch
	createFile(t, tempDir, "README.md", "# Test Repository\n\nThis is a test repository.\n")
	stageFile(t, tempDir, "README.md")
	commitChanges(t, tempDir, "Initial commit")

	return tempDir
}

// Helper function to create a file in the repository
func createFile(t *testing.T, repoPath, filename, content string) {
	filePath := filepath.Join(repoPath, filename)
	err := os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err, "Failed to create file")
}

// Helper function to stage a file
func stageFile(t *testing.T, repoPath, filename string) {
	cmd := exec.Command("git", "add", filename)
	cmd.Dir = repoPath
	err := cmd.Run()
	require.NoError(t, err, "Failed to stage file")
}

// Helper function to commit changes
func commitChanges(t *testing.T, repoPath, message string) string {
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = repoPath
	err := cmd.Run()
	require.NoError(t, err, "Failed to commit changes")

	// Get the commit hash
	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	require.NoError(t, err, "Failed to get commit hash")
	return strings.TrimSpace(string(out)) // Trim whitespace including newlines
}

// Helper function to create a branch
func createBranch(t *testing.T, repoPath, branchName string) {
	cmd := exec.Command("git", "branch", branchName)
	cmd.Dir = repoPath
	err := cmd.Run()
	require.NoError(t, err, "Failed to create branch")
}

// Helper function to switch to a branch
func switchBranch(t *testing.T, repoPath, branchName string) {
	cmd := exec.Command("git", "checkout", branchName)
	cmd.Dir = repoPath
	err := cmd.Run()
	require.NoError(t, err, "Failed to switch to branch")
}

// Helper function to get current branch
func getCurrentBranch(t *testing.T, repoPath string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	require.NoError(t, err, "Failed to get current branch")
	return strings.TrimSpace(string(out))
}

func TestGitService(t *testing.T) {
	// Create a logger for testing
	logger := loggy.NewNoopLogger()

	// Create the Git service
	service := NewService(logger)

	t.Run("GetDiff_StagedChanges", func(t *testing.T) {
		// Set up a temporary Git repository
		repoPath := setupTempGitRepo(t)
		defer os.RemoveAll(repoPath)

		// Initialize the repository in the service
		err := service.InitRepo(repoPath)
		require.NoError(t, err, "InitRepo should not return an error")

		// Create a file
		createFile(t, repoPath, "test.go", "package main\n\nfunc main() {\n\tprintln(\"Hello, World!\")\n}\n")

		// Stage the file
		stageFile(t, repoPath, "test.go")

		// Get the diff
		req := DiffRequest{
			RepoPath: repoPath,
			DiffType: DiffTypeStaged,
		}

		diff, err := service.GetDiff(req)
		require.NoError(t, err, "GetDiff should not return an error")
		require.NotNil(t, diff, "Diff should not be nil")

		// Verify the diff
		assert.GreaterOrEqual(t, len(diff.Files), 1, "Should have at least one changed file")
		foundTestGo := false
		for _, file := range diff.Files {
			if file.Path == "test.go" {
				foundTestGo = true
				assert.Equal(t, ChangeTypeAdded, file.ChangeType, "Change type should be added")
				assert.Contains(t, file.Content, "func main()", "Content should contain expected code")
				break
			}
		}
		assert.True(t, foundTestGo, "Should find test.go in changed files")
	})

	t.Run("GetDiff_CommitChanges", func(t *testing.T) {
		// Set up a temporary Git repository
		repoPath := setupTempGitRepo(t)
		defer os.RemoveAll(repoPath)

		// Initialize the repository in the service
		err := service.InitRepo(repoPath)
		require.NoError(t, err, "InitRepo should not return an error")

		// Create and commit a file
		createFile(t, repoPath, "test.go", "package main\n\nfunc main() {\n\tprintln(\"Hello, World!\")\n}\n")
		stageFile(t, repoPath, "test.go")
		commitChanges(t, repoPath, "Initial commit for test.go")

		// Modify the file
		createFile(t, repoPath, "test.go", "package main\n\nfunc main() {\n\tprintln(\"Hello, Git!\")\n}\n")
		stageFile(t, repoPath, "test.go")
		commitHash := commitChanges(t, repoPath, "Update message")

		// Get the diff for the second commit
		req := DiffRequest{
			RepoPath: repoPath,
			DiffType: DiffTypeCommit,
			CommitID: commitHash,
		}

		diff, err := service.GetDiff(req)
		require.NoError(t, err, "GetDiff should not return an error")
		require.NotNil(t, diff, "Diff should not be nil")

		// Verify the diff
		assert.Len(t, diff.Files, 1, "Should have one changed file")
		assert.Equal(t, "test.go", diff.Files[0].Path, "Changed file path should match")
		assert.Equal(t, ChangeTypeModified, diff.Files[0].ChangeType, "Change type should be modified")
		assert.Contains(t, diff.Files[0].Content, "Hello, Git!", "Content should contain updated message")

		// Verify commit info
		assert.NotNil(t, diff.CommitInfo, "Commit info should not be nil")
		assert.Equal(t, commitHash, diff.CommitInfo.Hash, "Commit hash should match")
		assert.Equal(t, "Test User", diff.CommitInfo.Author, "Commit author should match")
		assert.Equal(t, "test@example.com", diff.CommitInfo.Email, "Commit email should match")
		assert.Equal(t, "Update message", strings.TrimSpace(diff.CommitInfo.Message), "Commit message should match")
	})

	t.Run("GetDiff_BranchComparison", func(t *testing.T) {
		// Set up a temporary Git repository
		repoPath := setupTempGitRepo(t)
		defer os.RemoveAll(repoPath)

		// Initialize the repository in the service
		err := service.InitRepo(repoPath)
		require.NoError(t, err, "InitRepo should not return an error")

		// Create a feature branch
		createBranch(t, repoPath, "feature")

		// Remember the current branch (should be master/main)
		mainBranch := getCurrentBranch(t, repoPath)

		// Switch to feature branch
		switchBranch(t, repoPath, "feature")

		// Add a new file on feature branch
		createFile(t, repoPath, "feature.go", "package main\n\nfunc feature() {\n\tprintln(\"Feature function\")\n}\n")
		stageFile(t, repoPath, "feature.go")
		commitChanges(t, repoPath, "Add feature file")

		// Switch back to main branch
		switchBranch(t, repoPath, mainBranch)

		// Add a different file on main branch
		createFile(t, repoPath, "master.go", "package main\n\nfunc master() {\n\tprintln(\"Master function\")\n}\n")
		stageFile(t, repoPath, "master.go")
		commitChanges(t, repoPath, "Add master file")

		// Compare branches
		req := DiffRequest{
			RepoPath:  repoPath,
			DiffType:  DiffTypeBranch,
			BranchOne: mainBranch,
			BranchTwo: "feature",
		}

		diff, err := service.GetDiff(req)
		require.NoError(t, err, "GetDiff should not return an error")
		require.NotNil(t, diff, "Diff should not be nil")

		// Verify the diff should contain at least master.go
		foundMasterGo := false
		for _, file := range diff.Files {
			if file.Path == "master.go" {
				foundMasterGo = true
				break
			}
		}
		assert.True(t, foundMasterGo, "Should find master.go in the branch comparison")
	})

	t.Run("ListBranches", func(t *testing.T) {
		// Set up a temporary Git repository
		repoPath := setupTempGitRepo(t)
		defer os.RemoveAll(repoPath)

		// Initialize the repository in the service
		err := service.InitRepo(repoPath)
		require.NoError(t, err, "InitRepo should not return an error")

		// Get the default branch name
		mainBranch := getCurrentBranch(t, repoPath)

		// Create branches
		createBranch(t, repoPath, "feature1")
		createBranch(t, repoPath, "feature2")

		// List branches
		branches, err := service.ListBranches()
		require.NoError(t, err, "ListBranches should not return an error")

		// Verify branches
		assert.GreaterOrEqual(t, len(branches), 3, "Should have at least three branches")
		assert.Contains(t, branches, mainBranch, "Should include main branch")
		assert.Contains(t, branches, "feature1", "Should include feature1 branch")
		assert.Contains(t, branches, "feature2", "Should include feature2 branch")
	})

	t.Run("ListCommits", func(t *testing.T) {
		// Set up a temporary Git repository
		repoPath := setupTempGitRepo(t)
		defer os.RemoveAll(repoPath)

		// Initialize the repository in the service
		err := service.InitRepo(repoPath)
		require.NoError(t, err, "InitRepo should not return an error")

		// Create and commit files
		createFile(t, repoPath, "file1.go", "package main\n\nfunc file1() {}\n")
		stageFile(t, repoPath, "file1.go")
		commit1 := commitChanges(t, repoPath, "Add file1")

		// Sleep to ensure different timestamps
		time.Sleep(100 * time.Millisecond)

		createFile(t, repoPath, "file2.go", "package main\n\nfunc file2() {}\n")
		stageFile(t, repoPath, "file2.go")
		commit2 := commitChanges(t, repoPath, "Add file2")

		// List commits with limit
		commits, err := service.ListCommits(2)
		require.NoError(t, err, "ListCommits should not return an error")

		// Verify commits
		assert.Len(t, commits, 2, "Should have two commits")

		// The newer commit should be first
		assert.Equal(t, commit2, commits[0].Hash, "First commit hash should match the latest commit")
		assert.Equal(t, "Add file2", strings.TrimSpace(commits[0].Message), "First commit message should match")
		assert.Equal(t, "Test User", commits[0].Author, "First commit author should match")

		assert.Equal(t, commit1, commits[1].Hash, "Second commit hash should match the first commit")
		assert.Equal(t, "Add file1", strings.TrimSpace(commits[1].Message), "Second commit message should match")
	})

	t.Run("InitRepo_NonExistentRepo", func(t *testing.T) {
		// Try to initialize a non-existent repository
		err := service.InitRepo("/path/that/does/not/exist")
		assert.Error(t, err, "InitRepo should return an error for non-existent repository")
	})

	t.Run("InitRepo_MultipleInits", func(t *testing.T) {
		// Set up a temporary Git repository
		repoPath := setupTempGitRepo(t)
		defer os.RemoveAll(repoPath)

		// First initialization should succeed
		err := service.InitRepo(repoPath)
		assert.NoError(t, err, "First InitRepo should succeed")

		// Second initialization of the same repo should succeed (idempotent)
		err = service.InitRepo(repoPath)
		assert.NoError(t, err, "Second InitRepo should succeed (idempotent)")
	})
}
