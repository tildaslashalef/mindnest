package review

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/tildaslashalef/mindnest/internal/loggy"
	"github.com/tildaslashalef/mindnest/internal/ulid"
	"github.com/tildaslashalef/mindnest/internal/workspace"
)

// Repository defines operations for managing reviews in the database
type Repository interface {
	// CreateReview creates a new review
	CreateReview(ctx context.Context, review *Review) error

	// GetReview retrieves a review by ID
	GetReview(ctx context.Context, id string) (*Review, error)

	// GetReviewsByWorkspace retrieves reviews for a workspace
	GetReviewsByWorkspace(ctx context.Context, workspaceID string, limit, offset int) ([]*Review, error)

	// UpdateReview updates a review
	UpdateReview(ctx context.Context, review *Review) error

	// CreateReviewFile creates a new review file
	CreateReviewFile(ctx context.Context, reviewFile *ReviewFile) error

	// GetReviewFilesByReview retrieves review files for a review
	GetReviewFilesByReview(ctx context.Context, reviewID string) ([]*ReviewFile, error)

	// UpdateReviewFile updates a review file
	UpdateReviewFile(ctx context.Context, reviewFile *ReviewFile) error

	// CreateIssue creates a new issue
	CreateIssue(ctx context.Context, issue *Issue) error

	// GetIssue retrieves an issue by ID
	GetIssue(ctx context.Context, id string) (*Issue, error)

	// MarkIssueAsValid updates an issue
	MarkIssueAsValid(ctx context.Context, issueID string, isValid bool) error

	// GetIssuesByReviewFile retrieves issues for a review file
	GetIssuesByReviewFile(ctx context.Context, reviewFileID string) ([]*Issue, error)

	// UpdateReviewFileMetadata updates the metadata for a review file
	UpdateReviewFileMetadata(ctx context.Context, reviewFile *ReviewFile) error

	// GetReviewFile retrieves a review file by ID
	GetReviewFile(ctx context.Context, id string) (*ReviewFile, error)
}

// SQLRepository implements the Repository interface using a SQL database
type SQLRepository struct {
	db     *sql.DB
	logger *loggy.Logger
}

// NewSQLRepository creates a new SQL repository
func NewSQLRepository(db *sql.DB, logger *loggy.Logger) *SQLRepository {
	return &SQLRepository{
		db:     db,
		logger: logger,
	}
}

// CreateReview creates a new review
func (r *SQLRepository) CreateReview(ctx context.Context, review *Review) error {
	if review.ID == "" {
		review.ID = ulid.ReviewID()
	}

	now := time.Now()
	if review.CreatedAt.IsZero() {
		review.CreatedAt = now
	}
	if review.UpdatedAt.IsZero() {
		review.UpdatedAt = now
	}

	// Convert result to JSON if present
	var resultJSON []byte
	if !review.Result.IsEmpty() {
		var err error
		resultJSON, err = json.Marshal(review.Result)
		if err != nil {
			return fmt.Errorf("marshaling review result: %w", err)
		}
	}

	q := squirrel.Insert("reviews").
		Columns("id", "workspace_id", "review_type", "commit_hash", "branch_from", "branch_to", "status", "result", "created_at", "updated_at").
		Values(review.ID, review.WorkspaceID, review.Type, review.CommitHash, review.BranchFrom, review.BranchTo, review.Status, resultJSON, review.CreatedAt, review.UpdatedAt)

	query, args, err := q.ToSql()
	if err != nil {
		return fmt.Errorf("building create review query: %w", err)
	}

	_, err = r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("executing create review query: %w", err)
	}

	return nil
}

// GetReview retrieves a review by ID
func (r *SQLRepository) GetReview(ctx context.Context, id string) (*Review, error) {
	q := squirrel.Select("id", "workspace_id", "review_type", "commit_hash", "branch_from", "branch_to", "status", "result", "created_at", "updated_at").
		From("reviews").
		Where(squirrel.Eq{"id": id})

	query, args, err := q.ToSql()
	if err != nil {
		return nil, fmt.Errorf("building get review query: %w", err)
	}

	var review Review
	var resultJSON []byte
	err = r.db.QueryRowContext(ctx, query, args...).Scan(
		&review.ID,
		&review.WorkspaceID,
		&review.Type,
		&review.CommitHash,
		&review.BranchFrom,
		&review.BranchTo,
		&review.Status,
		&resultJSON,
		&review.CreatedAt,
		&review.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("review not found: %s", id)
		}
		return nil, fmt.Errorf("executing get review query: %w", err)
	}

	// Parse result JSON if present
	if len(resultJSON) > 0 {
		if err := json.Unmarshal(resultJSON, &review.Result); err != nil {
			return nil, fmt.Errorf("unmarshaling review result: %w", err)
		}
	}

	return &review, nil
}

func (r *SQLRepository) GetReviewFile(ctx context.Context, id string) (*ReviewFile, error) {
	q := squirrel.Select("id", "review_id", "file_id", "status", "issues_count", "summary", "assessment", "metadata", "created_at", "updated_at").
		From("review_files").
		Where(squirrel.Eq{"id": id})

	query, args, err := q.ToSql()
	if err != nil {
		return nil, fmt.Errorf("building get review file query: %w", err)
	}

	var reviewFile ReviewFile
	var summary, assessment sql.NullString
	var metadataBytes []byte
	err = r.db.QueryRowContext(ctx, query, args...).Scan(
		&reviewFile.ID,
		&reviewFile.ReviewID,
		&reviewFile.FileID,
		&reviewFile.Status,
		&reviewFile.IssuesCount,
		&summary,
		&assessment,
		&metadataBytes,
		&reviewFile.CreatedAt,
		&reviewFile.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("executing get review file query: %w", err)
	}

	if summary.Valid {
		reviewFile.Summary = summary.String
	}
	if assessment.Valid {
		reviewFile.Assessment = assessment.String
	}

	if len(metadataBytes) > 0 {
		if err := json.Unmarshal(metadataBytes, &reviewFile.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshaling metadata: %w", err)
		}
	}

	return &reviewFile, nil
}

// GetReviewsByWorkspace retrieves reviews for a workspace
func (r *SQLRepository) GetReviewsByWorkspace(ctx context.Context, workspaceID string, limit, offset int) ([]*Review, error) {
	q := squirrel.Select("id", "workspace_id", "review_type", "commit_hash", "branch_from", "branch_to", "status", "result", "created_at", "updated_at").
		From("reviews").
		Where(squirrel.Eq{"workspace_id": workspaceID}).
		OrderBy("updated_at DESC")

	if limit > 0 {
		q = q.Limit(uint64(limit))
	}
	if offset > 0 {
		q = q.Offset(uint64(offset))
	}

	query, args, err := q.ToSql()
	if err != nil {
		return nil, fmt.Errorf("building get reviews by workspace query: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("executing get reviews by workspace query: %w", err)
	}
	defer rows.Close()

	var reviews []*Review
	for rows.Next() {
		var review Review
		var resultJSON []byte
		err := rows.Scan(
			&review.ID,
			&review.WorkspaceID,
			&review.Type,
			&review.CommitHash,
			&review.BranchFrom,
			&review.BranchTo,
			&review.Status,
			&resultJSON,
			&review.CreatedAt,
			&review.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning review row: %w", err)
		}

		// Parse result JSON if present
		if len(resultJSON) > 0 {
			if err := json.Unmarshal(resultJSON, &review.Result); err != nil {
				return nil, fmt.Errorf("unmarshaling review result: %w", err)
			}
		}

		reviews = append(reviews, &review)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating review rows: %w", err)
	}

	return reviews, nil
}

// UpdateReview updates a review
func (r *SQLRepository) UpdateReview(ctx context.Context, review *Review) error {
	review.UpdatedAt = time.Now()

	// Convert result to JSON if present
	var resultJSON []byte
	if !review.Result.IsEmpty() {
		var err error
		resultJSON, err = json.Marshal(review.Result)
		if err != nil {
			return fmt.Errorf("marshaling review result: %w", err)
		}
	}

	q := squirrel.Update("reviews").
		Set("status", review.Status).
		Set("result", resultJSON).
		Set("updated_at", review.UpdatedAt).
		Where(squirrel.Eq{"id": review.ID})

	query, args, err := q.ToSql()
	if err != nil {
		return fmt.Errorf("building update review query: %w", err)
	}

	_, err = r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("executing update review query: %w", err)
	}

	return nil
}

// CreateReviewFile creates a new review file
func (r *SQLRepository) CreateReviewFile(ctx context.Context, reviewFile *ReviewFile) error {
	if reviewFile.ID == "" {
		reviewFile.ID = ulid.ReviewID()
	}

	now := time.Now()
	if reviewFile.CreatedAt.IsZero() {
		reviewFile.CreatedAt = now
	}
	if reviewFile.UpdatedAt.IsZero() {
		reviewFile.UpdatedAt = now
	}

	q := squirrel.Insert("review_files").
		Columns("id", "review_id", "file_id", "status", "issues_count", "created_at", "updated_at").
		Values(reviewFile.ID, reviewFile.ReviewID, reviewFile.FileID, reviewFile.Status, reviewFile.IssuesCount, reviewFile.CreatedAt, reviewFile.UpdatedAt)

	query, args, err := q.ToSql()
	if err != nil {
		return fmt.Errorf("building create review file query: %w", err)
	}

	_, err = r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("executing create review file query: %w", err)
	}

	return nil
}

// GetReviewFilesByReview retrieves review files for a review
func (r *SQLRepository) GetReviewFilesByReview(ctx context.Context, reviewID string) ([]*ReviewFile, error) {
	q := squirrel.Select("rf.id", "rf.review_id", "rf.file_id", "rf.status", "rf.issues_count", "rf.created_at", "rf.updated_at",
		"f.id", "f.workspace_id", "f.path", "f.language", "f.last_parsed", "f.created_at", "f.updated_at").
		From("review_files rf").
		Join("files f ON rf.file_id = f.id").
		Where(squirrel.Eq{"rf.review_id": reviewID}).
		OrderBy("rf.updated_at DESC")

	query, args, err := q.ToSql()
	if err != nil {
		return nil, fmt.Errorf("building get review files query: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("executing get review files query: %w", err)
	}
	defer rows.Close()

	var reviewFiles []*ReviewFile
	for rows.Next() {
		var rf ReviewFile
		var file workspace.File
		var lastParsed sql.NullTime

		err := rows.Scan(
			&rf.ID,
			&rf.ReviewID,
			&rf.FileID,
			&rf.Status,
			&rf.IssuesCount,
			&rf.CreatedAt,
			&rf.UpdatedAt,
			&file.ID,
			&file.WorkspaceID,
			&file.Path,
			&file.Language,
			&lastParsed,
			&file.CreatedAt,
			&file.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning review file row: %w", err)
		}

		// Set last parsed time if valid
		if lastParsed.Valid {
			parsedTime := lastParsed.Time
			file.LastParsed = &parsedTime
		}

		rf.File = &file
		reviewFiles = append(reviewFiles, &rf)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating review file rows: %w", err)
	}

	return reviewFiles, nil
}

// UpdateReviewFile updates a review file
func (r *SQLRepository) UpdateReviewFile(ctx context.Context, reviewFile *ReviewFile) error {
	reviewFile.UpdatedAt = time.Now()

	q := squirrel.Update("review_files").
		Set("status", reviewFile.Status).
		Set("issues_count", reviewFile.IssuesCount).
		Set("summary", reviewFile.Summary).
		Set("assessment", reviewFile.Assessment).
		Set("updated_at", reviewFile.UpdatedAt).
		Where(squirrel.Eq{"id": reviewFile.ID})

	query, args, err := q.ToSql()
	if err != nil {
		return fmt.Errorf("building update review file query: %w", err)
	}

	_, err = r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("executing update review file query: %w", err)
	}

	return nil
}

func (r *SQLRepository) UpdateReviewFileMetadata(ctx context.Context, reviewFile *ReviewFile) error {
	metadataJSON, err := json.Marshal(reviewFile.Metadata)
	if err != nil {
		return fmt.Errorf("marshaling review file metadata: %w", err)
	}

	q := squirrel.Update("review_files").
		Set("metadata", metadataJSON).
		Where(squirrel.Eq{"id": reviewFile.ID})

	query, args, err := q.ToSql()
	if err != nil {
		return fmt.Errorf("building update review file metadata query: %w", err)
	}

	_, err = r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("executing update review file metadata query: %w", err)
	}

	return nil
}

// CreateIssue creates a new issue
func (r *SQLRepository) CreateIssue(ctx context.Context, issue *Issue) error {
	if issue.ID == "" {
		issue.ID = ulid.IssueID()
	}

	now := time.Now()
	if issue.CreatedAt.IsZero() {
		issue.CreatedAt = now
	}
	if issue.UpdatedAt.IsZero() {
		issue.UpdatedAt = now
	}

	q := squirrel.Insert("issues").
		Columns("id", "review_id", "file_id", "type", "severity", "title", "description", "line_start", "line_end", "suggestion", "affected_code", "code_snippet", "is_valid", "created_at", "updated_at").
		Values(issue.ID, issue.ReviewID, issue.FileID, issue.Type, issue.Severity, issue.Title, issue.Description, issue.LineStart, issue.LineEnd, issue.Suggestion, issue.AffectedCode, issue.CodeSnippet, issue.IsValid, issue.CreatedAt, issue.UpdatedAt)

	query, args, err := q.ToSql()
	if err != nil {
		return fmt.Errorf("building create issue query: %w", err)
	}

	_, err = r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("executing create issue query: %w", err)
	}

	return nil
}

func (r *SQLRepository) GetIssue(ctx context.Context, id string) (*Issue, error) {
	q := squirrel.Select("id", "review_id", "file_id", "type", "severity", "title", "description", "line_start", "line_end", "suggestion", "affected_code", "code_snippet", "is_valid", "created_at", "updated_at").
		From("issues").
		Where(squirrel.Eq{"id": id})

	query, args, err := q.ToSql()
	if err != nil {
		return nil, fmt.Errorf("building get issue query: %w", err)
	}

	var issue Issue
	err = r.db.QueryRowContext(ctx, query, args...).Scan(
		&issue.ID,
		&issue.ReviewID,
		&issue.FileID,
		&issue.Type,
		&issue.Severity,
		&issue.Title,
		&issue.Description,
		&issue.LineStart,
		&issue.LineEnd,
		&issue.Suggestion,
		&issue.AffectedCode,
		&issue.CodeSnippet,
		&issue.IsValid,
		&issue.CreatedAt,
		&issue.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("executing get issue query: %w", err)
	}

	return &issue, nil
}

func (r *SQLRepository) MarkIssueAsValid(ctx context.Context, issueID string, isValid bool) error {
	q := squirrel.Update("issues").
		Set("is_valid", isValid).
		Where(squirrel.Eq{"id": issueID})

	query, args, err := q.ToSql()
	if err != nil {
		return fmt.Errorf("building mark issue as valid query: %w", err)
	}

	_, err = r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("executing mark issue as valid query: %w", err)
	}

	return nil
}

// GetIssuesByReviewFile retrieves issues for a review file
func (r *SQLRepository) GetIssuesByReviewFile(ctx context.Context, reviewFileID string) ([]*Issue, error) {
	q := squirrel.Select("i.id", "i.review_id", "i.file_id", "i.type", "i.severity", "i.title", "i.description",
		"i.line_start", "i.line_end", "i.suggestion", "i.affected_code", "i.code_snippet", "f.path AS file_path").
		From("issues i").
		Join("review_files rf ON i.file_id = rf.file_id AND i.review_id = rf.review_id").
		Join("files f ON i.file_id = f.id").
		Where(squirrel.Eq{"rf.id": reviewFileID}).
		OrderBy("i.line_start ASC")

	query, args, err := q.ToSql()
	if err != nil {
		return nil, fmt.Errorf("building get issues query: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("executing get issues query: %w", err)
	}
	defer rows.Close()

	var issues []*Issue
	for rows.Next() {
		var issue Issue
		var filePath string
		err := rows.Scan(
			&issue.ID,
			&issue.ReviewID,
			&issue.FileID,
			&issue.Type,
			&issue.Severity,
			&issue.Title,
			&issue.Description,
			&issue.LineStart,
			&issue.LineEnd,
			&issue.Suggestion,
			&issue.AffectedCode,
			&issue.CodeSnippet,
			&filePath,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning issue row: %w", err)
		}

		// Initialize metadata and store file path
		if issue.Metadata == nil {
			issue.Metadata = make(map[string]interface{})
		}
		issue.Metadata["file_path"] = filePath

		issues = append(issues, &issue)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating issue rows: %w", err)
	}

	return issues, nil
}
