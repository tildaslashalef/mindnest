package config

import (
	"context"
	"database/sql"

	"github.com/tildaslashalef/mindnest/internal/loggy"
)

// SettingsService provides operations for managing application settings
type SettingsService struct {
	repo   SettingsRepository
	config *Config
	logger *loggy.Logger
}

// NewSettingsService creates a new settings service
func NewSettingsService(db *sql.DB, config *Config, logger *loggy.Logger) *SettingsService {
	repo := NewSQLSettingsRepository(db, logger)

	return &SettingsService{
		repo:   repo,
		config: config,
		logger: logger,
	}
}

// GetSetting retrieves a setting by key
func (s *SettingsService) GetSetting(ctx context.Context, key string) (string, error) {
	return s.repo.GetSetting(ctx, key)
}

// GetSettings retrieves multiple settings by prefix
func (s *SettingsService) GetSettings(ctx context.Context, prefix string) (map[string]string, error) {
	return s.repo.GetSettings(ctx, prefix)
}

// SetSetting sets a setting value
func (s *SettingsService) SetSetting(ctx context.Context, key, value string) error {
	return s.repo.SetSetting(ctx, key, value)
}

// DeleteSetting deletes a setting
func (s *SettingsService) DeleteSetting(ctx context.Context, key string) error {
	return s.repo.DeleteSetting(ctx, key)
}

// GetRepository returns the underlying repository
func (s *SettingsService) GetRepository() SettingsRepository {
	return s.repo
}

// LoadSyncSettings loads sync settings from the database into the Config
func (s *SettingsService) LoadSyncSettings(ctx context.Context) error {
	return LoadSyncSettings(ctx, s.config, s.repo)
}

// SaveSyncSettings saves sync settings from the Config to the database
func (s *SettingsService) SaveSyncSettings(ctx context.Context) error {
	return SaveSyncSettings(ctx, s.config, s.repo)
}

// SetToken sets the sync token with proper obfuscation
func (s *SettingsService) SetToken(ctx context.Context, token string) error {
	// Store the token in config
	s.config.Server.Token = token

	// Save to database with automatic obfuscation
	return s.repo.SetSetting(ctx, "sync.server_token", token)
}

// SetServerURL sets the sync server URL
func (s *SettingsService) SetServerURL(ctx context.Context, url string) error {
	// Store the URL in config
	s.config.Server.URL = url

	// Save to database
	return s.repo.SetSetting(ctx, "sync.server_url", url)
}

// SetDeviceName sets the sync device name
func (s *SettingsService) SetDeviceName(ctx context.Context, name string) error {
	// Store the name in config
	s.config.Server.DeviceName = name

	// Save to database
	return s.repo.SetSetting(ctx, "sync.device_name", name)
}

// SetSyncEnabled sets whether sync is enabled
func (s *SettingsService) SetSyncEnabled(ctx context.Context, enabled bool) error {
	// Store the flag in config
	s.config.Server.Enabled = enabled

	// Convert to string for storage
	enabledStr := "false"
	if enabled {
		enabledStr = "true"
	}

	// Save to database
	return s.repo.SetSetting(ctx, "sync.enabled", enabledStr)
}
