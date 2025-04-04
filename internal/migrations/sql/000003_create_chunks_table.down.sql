-- Drop indexes
DROP INDEX IF EXISTS idx_chunks_workspace_id;
DROP INDEX IF EXISTS idx_chunks_file_id;
DROP INDEX IF EXISTS idx_chunks_chunk_type;
DROP INDEX IF EXISTS idx_chunks_updated_at;

-- Drop chunks table
DROP TABLE IF EXISTS chunks; 