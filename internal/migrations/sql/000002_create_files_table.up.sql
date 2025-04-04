-- Create files table
CREATE TABLE files (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    path TEXT NOT NULL,
    language TEXT NOT NULL,
    last_parsed TIMESTAMP,
    metadata JSON,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    synced_at TIMESTAMP,
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE,
    UNIQUE(workspace_id, path)
);

-- Create indexes for file lookups
CREATE INDEX idx_files_workspace_id ON files(workspace_id);
CREATE INDEX idx_files_path ON files(path);
CREATE INDEX idx_files_language ON files(language);
CREATE INDEX idx_files_updated_at ON files(updated_at DESC); 