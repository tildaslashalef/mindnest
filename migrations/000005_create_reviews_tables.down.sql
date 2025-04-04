-- Drop indexes
DROP INDEX IF EXISTS idx_review_files_file_id;
DROP INDEX IF EXISTS idx_review_files_review_id;
DROP INDEX IF EXISTS idx_reviews_status;
DROP INDEX IF EXISTS idx_reviews_workspace_id;

-- Drop tables
DROP TABLE IF EXISTS review_files;
DROP TABLE IF EXISTS reviews; 