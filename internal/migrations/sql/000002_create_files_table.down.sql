-- Drop indexes
DROP INDEX IF EXISTS idx_files_workspace_id;
DROP INDEX IF EXISTS idx_files_path;
DROP INDEX IF EXISTS idx_files_language;
DROP INDEX IF EXISTS idx_files_updated_at;

-- Drop files table
DROP TABLE IF EXISTS files; 