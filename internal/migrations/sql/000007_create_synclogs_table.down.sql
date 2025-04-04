-- Drop indexes
DROP INDEX IF EXISTS idx_sync_logs_entity_type;
DROP INDEX IF EXISTS idx_sync_logs_entity_id;
DROP INDEX IF EXISTS idx_sync_logs_completed_at;

-- Drop sync_logs table
DROP TABLE IF EXISTS sync_logs;
