-- Drop indexes
DROP INDEX IF EXISTS idx_workspaces_path;
DROP INDEX IF EXISTS idx_workspaces_updated_at;

-- Drop workspaces table
DROP TABLE IF EXISTS workspaces; 