
-- Drop vector graph indexes
DROP INDEX IF EXISTS idx_vector_graph_source;
DROP INDEX IF EXISTS idx_vector_graph_target;

-- Drop vector vector_graph table first 
DROP TABLE IF EXISTS vector_graph;

-- Drop vector index virtual table
DROP TABLE IF EXISTS vector_index;

-- Drop vector store indexes
DROP INDEX IF EXISTS idx_vector_store_chunk_id;
DROP INDEX IF EXISTS idx_vector_store_workspace_id;
DROP INDEX IF EXISTS idx_vector_store_created_at;

-- Drop vector store table
DROP TABLE IF EXISTS vector_store;

-- Drop original vectors virtual table
DROP TABLE IF EXISTS vectors;