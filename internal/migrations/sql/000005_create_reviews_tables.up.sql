-- Create reviews table
CREATE TABLE IF NOT EXISTS reviews (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    review_type TEXT NOT NULL,
    commit_hash TEXT,
    branch_from TEXT,
    branch_to TEXT,
    status TEXT NOT NULL,
    result TEXT,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    synced_at TIMESTAMP,
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
);

-- Create review_files table
CREATE TABLE IF NOT EXISTS review_files (
    id TEXT PRIMARY KEY,
    review_id TEXT NOT NULL,
    file_id TEXT NOT NULL,
    status TEXT NOT NULL,
    issues_count INTEGER NOT NULL DEFAULT 0,
    summary TEXT,
    assessment TEXT,
    metadata TEXT,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    synced_at TIMESTAMP,
    FOREIGN KEY (review_id) REFERENCES reviews(id) ON DELETE CASCADE,
    FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE
);

-- Create indexes for reviews and review_files
CREATE INDEX IF NOT EXISTS idx_reviews_workspace_id ON reviews(workspace_id);
CREATE INDEX IF NOT EXISTS idx_reviews_status ON reviews(status);
CREATE INDEX IF NOT EXISTS idx_review_files_review_id ON review_files(review_id);
CREATE INDEX IF NOT EXISTS idx_review_files_file_id ON review_files(file_id); 