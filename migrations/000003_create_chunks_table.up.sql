-- Create chunks table
CREATE TABLE chunks (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    file_id TEXT NOT NULL,
    name TEXT NOT NULL,
    content TEXT NOT NULL,
    start_line INTEGER,
    end_line INTEGER,
    start_offset INTEGER,
    end_offset INTEGER,
    chunk_type TEXT NOT NULL, -- e.g., "function", "method", "if statement", ...
    signature TEXT, -- function signature, method signature, etc.
    parent_id TEXT,
    child_ids JSON,
    metadata JSON,
    vector_id INTEGER,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE,
    FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE
);

-- Create indexes for chunk lookups
CREATE INDEX idx_chunks_workspace_id ON chunks(workspace_id);
CREATE INDEX idx_chunks_file_id ON chunks(file_id);
CREATE INDEX idx_chunks_chunk_type ON chunks(chunk_type);
CREATE INDEX idx_chunks_updated_at ON chunks(updated_at DESC); 