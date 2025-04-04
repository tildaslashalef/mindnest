-- Create issues table
CREATE TABLE issues (
    id TEXT PRIMARY KEY,
    review_id TEXT NOT NULL,
    file_id TEXT NOT NULL,
    type TEXT NOT NULL, 
    severity TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    line_start INTEGER,
    line_end INTEGER,
    suggestion TEXT,
    affected_code TEXT,
    code_snippet TEXT,
    is_valid BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    synced_at TIMESTAMP,
    FOREIGN KEY (review_id) REFERENCES reviews(id) ON DELETE CASCADE,
    FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE
);

-- Create indexes for issues lookups
CREATE INDEX idx_issues_review_id ON issues(review_id);
CREATE INDEX idx_issues_file_id ON issues(file_id);
CREATE INDEX idx_issues_type ON issues(type);
CREATE INDEX idx_issues_severity ON issues(severity);
CREATE INDEX idx_issues_line_start ON issues(line_start); 