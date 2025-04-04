-- Create settings table to store application configuration
CREATE TABLE settings (
    id TEXT PRIMARY KEY,
    key TEXT NOT NULL,
    value TEXT,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

-- Create unique index for keys to ensure no duplicates
CREATE UNIQUE INDEX idx_settings_key ON settings(key);
