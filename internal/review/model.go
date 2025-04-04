// Package review provides functionality for code review using LLMs
package review

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/tildaslashalef/mindnest/internal/extractor"
	"github.com/tildaslashalef/mindnest/internal/workspace"
)

// ReviewType represents the type of code review being performed
type ReviewType string

const (
	// ReviewTypeStaged represents a review of staged changes
	ReviewTypeStaged ReviewType = "staged"
	// ReviewTypeCommit represents a review of changes in a commit
	ReviewTypeCommit ReviewType = "commit"
	// ReviewTypeBranch represents a review of differences between branches
	ReviewTypeBranch ReviewType = "branch"
)

// ReviewStatus represents the status of a code review
type ReviewStatus string

const (
	// ReviewStatusPending indicates the review is waiting to start
	ReviewStatusPending ReviewStatus = "pending"
	// ReviewStatusRunning indicates the review is in progress
	ReviewStatusRunning ReviewStatus = "running"
	// ReviewStatusCompleted indicates the review has completed successfully
	ReviewStatusCompleted ReviewStatus = "completed"
	// ReviewStatusFailed indicates the review failed
	ReviewStatusFailed ReviewStatus = "failed"
)

// ReviewFileStatus represents the status of a file in a review
type ReviewFileStatus string

const (
	// ReviewFileStatusPending indicates a file review is waiting to start
	ReviewFileStatusPending ReviewFileStatus = "pending"
	// ReviewFileStatusRunning indicates a file review is in progress
	ReviewFileStatusRunning ReviewFileStatus = "running"
	// ReviewFileStatusCompleted indicates a file review is completed
	ReviewFileStatusCompleted ReviewFileStatus = "completed"
	// ReviewFileStatusFailed indicates a file review has failed
	ReviewFileStatusFailed ReviewFileStatus = "failed"
)

// IssueType represents the type of issue identified during code review
type IssueType string

const (
	// IssueTypeBug represents a potential bug or error
	IssueTypeBug IssueType = "bug"
	// IssueTypeSecurity represents a security vulnerability
	IssueTypeSecurity IssueType = "security"
	// IssueTypePerformance represents a performance issue
	IssueTypePerformance IssueType = "performance"
	// IssueTypeDesign represents a design or architectural issue
	IssueTypeDesign IssueType = "design"
	// IssueTypeStyle represents a code style issue
	IssueTypeStyle IssueType = "style"
	// IssueTypeComplexity represents an unnecessary complexity issue
	IssueTypeComplexity IssueType = "complexity"
	// IssueTypeBestPractice represents a deviation from best practices
	IssueTypeBestPractice IssueType = "best_practice"
)

// IssueSeverity represents the severity of an issue
type IssueSeverity string

const (
	// IssueSeverityCritical represents a critical issue
	IssueSeverityCritical IssueSeverity = "critical"
	// IssueSeverityHigh represents a high-severity issue
	IssueSeverityHigh IssueSeverity = "high"
	// IssueSeverityMedium represents a medium-severity issue
	IssueSeverityMedium IssueSeverity = "medium"
	// IssueSeverityLow represents a low-severity issue
	IssueSeverityLow IssueSeverity = "low"
)

// Review represents a code review session
type Review struct {
	ID          string       `json:"id"`
	WorkspaceID string       `json:"workspace_id"`
	Type        ReviewType   `json:"type"`
	CommitHash  string       `json:"commit_hash,omitempty"`
	BranchFrom  string       `json:"branch_from,omitempty"`
	BranchTo    string       `json:"branch_to,omitempty"`
	Status      ReviewStatus `json:"status"`
	Result      ReviewResult `json:"result,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	SyncedAt    *time.Time   `json:"synced_at,omitempty"`
}

// ReviewFile represents a file included in a code review
type ReviewFile struct {
	ID          string           `json:"id"`
	ReviewID    string           `json:"review_id"`
	FileID      string           `json:"file_id"`
	Status      ReviewFileStatus `json:"status"`
	IssuesCount int              `json:"issues_count"`
	Summary     string           `json:"summary,omitempty"`
	Assessment  string           `json:"assessment,omitempty"`
	Metadata    map[string]any   `json:"metadata,omitempty"`
	File        *workspace.File  `json:"file,omitempty"`
	Issues      []Issue          `json:"issues,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	SyncedAt    *time.Time       `json:"synced_at,omitempty"`
}

// Issue represents an issue identified during a code review
type Issue struct {
	ID           string                 `json:"id"`
	ReviewID     string                 `json:"review_id"`
	FileID       string                 `json:"file_id"`
	Type         IssueType              `json:"type"`
	Severity     IssueSeverity          `json:"severity"`
	Title        string                 `json:"title"`
	Description  string                 `json:"description"`
	LineStart    int                    `json:"line_start,omitempty"`
	LineEnd      int                    `json:"line_end,omitempty"`
	Suggestion   string                 `json:"suggestion,omitempty"`
	AffectedCode string                 `json:"affected_code,omitempty"`
	CodeSnippet  string                 `json:"code_snippet,omitempty"`
	IsValid      bool                   `json:"is_valid,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// ReviewResult represents the overall result of a code review
// It can be stored as JSON in the database
type ReviewResult struct {
	Summary         string         `json:"summary"`
	TotalIssues     int            `json:"total_issues"`
	ByType          map[string]int `json:"by_type"`
	BySeverity      map[string]int `json:"by_severity"`
	ExecutionTime   float64        `json:"execution_time"`
	ProcessedFiles  int            `json:"processed_files"`
	ProcessedChunks int            `json:"processed_chunks"`
	Model           string         `json:"model"`
}

// ReviewOutput represents the expected structure of a code review response
type ReviewOutput struct {
	Summary           string        `json:"summary"`
	Issues            []ReviewIssue `json:"issues"`
	OverallAssessment string        `json:"overall_assessment"`
}

// ReviewIssue represents a single issue found during code review
type ReviewIssue struct {
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Description string `json:"description"`
	LineStart   int    `json:"line_start"`
	LineEnd     int    `json:"line_end"`
	Suggestion  string `json:"suggestion"`
	CodeSnippet string `json:"code_snippet"`
}

// IsEmpty checks if the ReviewResult is empty
func (r ReviewResult) IsEmpty() bool {
	return r.Summary == "" && r.TotalIssues == 0 && r.Model == ""
}

// Value implements the driver.Valuer interface for database serialization.
func (r ReviewResult) Value() (driver.Value, error) {
	return json.Marshal(r)
}

// Scan implements the sql.Scanner interface for database deserialization.
func (r *ReviewResult) Scan(src interface{}) error {
	var source []byte
	switch src := src.(type) {
	case string:
		source = []byte(src)
	case []byte:
		source = src
	case nil:
		return nil
	default:
		return errors.New("incompatible type for ReviewResult")
	}

	if len(source) == 0 {
		return nil
	}

	return json.Unmarshal(source, &r)
}

// NewReview creates a new review instance
func NewReview(workspaceID string, reviewType ReviewType) *Review {
	now := time.Now()
	return &Review{
		ID:          "", // Will be set by the repository
		WorkspaceID: workspaceID,
		Type:        reviewType,
		Status:      ReviewStatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// NewReviewFile creates a new review file instance
func NewReviewFile(reviewID, fileID string) *ReviewFile {
	now := time.Now()
	return &ReviewFile{
		ID:          "", // Will be set by the repository
		ReviewID:    reviewID,
		FileID:      fileID,
		Status:      ReviewFileStatusPending,
		IssuesCount: 0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// NewIssue creates a new issue instance
func NewIssue(reviewID, fileID string, issueType IssueType, severity IssueSeverity) *Issue {
	return &Issue{
		ID:       "", // Will be set by the repository
		ReviewID: reviewID,
		FileID:   fileID,
		Type:     issueType,
		Severity: severity,
	}
}

// MarkCompleted marks a review as completed
func (r *Review) MarkCompleted() {
	r.Status = ReviewStatusCompleted
	r.UpdatedAt = time.Now()
}

// MarkFailed marks a review as failed
func (r *Review) MarkFailed() {
	r.Status = ReviewStatusFailed
	r.UpdatedAt = time.Now()
}

// SetResult sets the result of a review
func (r *Review) SetResult(result ReviewResult) {
	r.Result = result
	r.UpdatedAt = time.Now()
}

// MarkCompleted marks a review file as completed
func (rf *ReviewFile) MarkCompleted() {
	rf.Status = ReviewFileStatusCompleted
	rf.UpdatedAt = time.Now()
}

// MarkFileFailed marks a review file as failed
func (rf *ReviewFile) MarkFileFailed() {
	rf.Status = ReviewFileStatusFailed
	rf.UpdatedAt = time.Now()
}

// AddIssue adds an issue to a review file
func (rf *ReviewFile) AddIssue(issue *Issue) {
	rf.Issues = append(rf.Issues, *issue)
	rf.IssuesCount++
	rf.UpdatedAt = time.Now()
}

// SetSummary sets the summary of a review file
func (rf *ReviewFile) SetSummary(summary string) {
	rf.Summary = summary
}

// SetAssessment sets the assessment of a review file
func (rf *ReviewFile) SetAssessment(assessment string) {
	rf.Assessment = assessment
}

// MapStringToIssueType maps a string to an IssueType
func MapStringToIssueType(issueTypeStr string) IssueType {
	switch strings.ToLower(issueTypeStr) {
	case "bug":
		return IssueTypeBug
	case "security":
		return IssueTypeSecurity
	case "performance":
		return IssueTypePerformance
	case "design":
		return IssueTypeDesign
	case "style":
		return IssueTypeStyle
	case "complexity":
		return IssueTypeComplexity
	case "best_practice":
		return IssueTypeBestPractice
	default:
		return IssueTypeBug
	}
}

// MapStringToIssueSeverity maps a string to an IssueSeverity
func MapStringToIssueSeverity(severityStr string) IssueSeverity {
	switch strings.ToLower(severityStr) {
	case "critical":
		return IssueSeverityCritical
	case "high":
		return IssueSeverityHigh
	case "medium":
		return IssueSeverityMedium
	case "low":
		return IssueSeverityLow
	default:
		return IssueSeverityMedium
	}
}

// CreateIssuesFromLLMOutput converts an extractor.LLMReviewOutput to Issue objects
func CreateIssuesFromLLMOutput(llmOutput *extractor.LLMReviewOutput, reviewID, fileID string) []*Issue {
	if llmOutput == nil || len(llmOutput.Issues) == 0 {
		return []*Issue{}
	}

	issues := make([]*Issue, 0, len(llmOutput.Issues))

	for _, llmIssue := range llmOutput.Issues {
		issueType := MapStringToIssueType(llmIssue.Type)
		severity := MapStringToIssueSeverity(llmIssue.Severity)

		issue := NewIssue(reviewID, fileID, issueType, severity)
		issue.Title = llmIssue.Title
		issue.Description = llmIssue.Description
		issue.LineStart = llmIssue.LineStart
		issue.LineEnd = llmIssue.LineEnd
		issue.Suggestion = llmIssue.Suggestion
		issue.AffectedCode = llmIssue.AffectedCode
		issue.CodeSnippet = llmIssue.CodeSnippet

		issues = append(issues, issue)
	}

	return issues
}
