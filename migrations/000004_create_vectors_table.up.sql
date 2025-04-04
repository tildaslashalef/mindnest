-- Create vectors virtual table using vec0
-- This requires the sqlite-vec extension to be loaded
CREATE VIRTUAL TABLE IF NOT EXISTS vectors USING vec0 (
    embedding float[768] -- embedding size for nomic-embed-text
); 