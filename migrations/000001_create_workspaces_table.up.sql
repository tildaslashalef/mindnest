-- Create workspaces table
CREATE TABLE workspaces (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    path TEXT NOT NULL,
    git_repo_url TEXT,
    description TEXT,
    model_config JSON NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    synced_at TIMESTAMP
);

-- Create index for path lookups
CREATE UNIQUE INDEX idx_workspaces_path ON workspaces(path);

-- Create index for faster queries by update time
CREATE INDEX idx_workspaces_updated_at ON workspaces(updated_at DESC); 