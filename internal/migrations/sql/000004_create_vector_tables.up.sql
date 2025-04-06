-- Create the original vectors virtual table for backward compatibility
CREATE VIRTUAL TABLE IF NOT EXISTS vectors USING vec0 (
    embedding float[768] -- embedding size for nomic-embed-text
);

-- Create a comprehensive vector store table with metadata
CREATE TABLE IF NOT EXISTS vector_store (
    id TEXT PRIMARY KEY,
    chunk_id TEXT NOT NULL,
    workspace_id TEXT NOT NULL,
    vector BLOB NOT NULL,       -- The serialized vector in binary format
    vector_type TEXT NOT NULL,  -- float32, int8, binary
    dimensions INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    metadata TEXT,              -- JSON metadata
    UNIQUE(chunk_id)
);

-- Create indexes for vector_store
CREATE INDEX IF NOT EXISTS idx_vector_store_chunk_id ON vector_store(chunk_id);
CREATE INDEX IF NOT EXISTS idx_vector_store_workspace_id ON vector_store(workspace_id);
CREATE INDEX IF NOT EXISTS idx_vector_store_created_at ON vector_store(created_at);

-- Create optimized vector index using standard vec0 virtual table
-- This provides vector search with L2 distance
CREATE VIRTUAL TABLE IF NOT EXISTS vector_index USING vec0 (
    id TEXT,                   -- Reference to vector_store.id
    embedding float[768] distance_metric=cosine,  -- The vector embedding with 768 dimensions 
    workspace_id TEXT          -- For filtering by workspace
);

CREATE TABLE IF NOT EXISTS vector_graph (
    source_id TEXT NOT NULL,        -- ID of the source chunk/node
    target_id TEXT NOT NULL,        -- ID of the target chunk/node
    relationship_type TEXT NOT NULL, -- Type of relationship (calls, imports, inherits, etc.)
    weight REAL DEFAULT 1.0,        -- Relationship strength/confidence
    direction TEXT DEFAULT 'outgoing', -- Relationship direction (outgoing, incoming, bidirectional)
    metadata TEXT,                  -- JSON metadata about the relationship
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (source_id, target_id, relationship_type),
    FOREIGN KEY (source_id) REFERENCES vector_store(id),
    FOREIGN KEY (target_id) REFERENCES vector_store(id)
);

-- Create indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_vector_graph_source ON vector_graph(source_id, relationship_type);
CREATE INDEX IF NOT EXISTS idx_vector_graph_target ON vector_graph(target_id, relationship_type); 