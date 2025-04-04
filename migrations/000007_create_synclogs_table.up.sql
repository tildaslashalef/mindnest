-- Create sync_logs table to track sync history
CREATE TABLE sync_logs (
    id TEXT PRIMARY KEY,
    sync_type TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    success BOOLEAN NOT NULL,
    error_message TEXT,
    items_synced INTEGER NOT NULL,
    started_at TIMESTAMP NOT NULL,
    completed_at TIMESTAMP NOT NULL
);

-- Create indexes for sync_logs
CREATE INDEX idx_sync_logs_entity_type ON sync_logs(entity_type);
CREATE INDEX idx_sync_logs_entity_id ON sync_logs(entity_id);
CREATE INDEX idx_sync_logs_completed_at ON sync_logs(completed_at DESC);