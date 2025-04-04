-- Drop issues table and its indexes
DROP INDEX IF EXISTS idx_issues_line_start;
DROP INDEX IF EXISTS idx_issues_severity;
DROP INDEX IF EXISTS idx_issues_type;
DROP INDEX IF EXISTS idx_issues_file_id;
DROP INDEX IF EXISTS idx_issues_review_id;
DROP TABLE IF EXISTS issues; 