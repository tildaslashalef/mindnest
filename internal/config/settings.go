package config

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/tildaslashalef/mindnest/internal/loggy"
	"github.com/tildaslashalef/mindnest/internal/ulid"
)

// Settings represents a persistent setting in the database
type Settings struct {
	ID        string
	Key       string
	Value     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SettingsRepository defines operations for managing settings in the database
type SettingsRepository interface {
	// GetSetting retrieves a setting by key
	GetSetting(ctx context.Context, key string) (string, error)

	// GetSettings retrieves multiple settings by prefix
	GetSettings(ctx context.Context, prefix string) (map[string]string, error)

	// SetSetting sets a setting value
	SetSetting(ctx context.Context, key, value string) error

	// DeleteSetting deletes a setting
	DeleteSetting(ctx context.Context, key string) error
}

// SQLSettingsRepository implements SettingsRepository using a SQL database
type SQLSettingsRepository struct {
	db     *sql.DB
	logger *loggy.Logger
}

// NewSQLSettingsRepository creates a new SQL settings repository
func NewSQLSettingsRepository(db *sql.DB, logger *loggy.Logger) SettingsRepository {
	return &SQLSettingsRepository{
		db:     db,
		logger: logger,
	}
}

// GetSetting retrieves a setting by key
func (r *SQLSettingsRepository) GetSetting(ctx context.Context, key string) (string, error) {
	q := squirrel.Select("value").
		From("settings").
		Where(squirrel.Eq{"key": key}).
		Limit(1)

	query, args, err := q.ToSql()
	if err != nil {
		return "", fmt.Errorf("building get setting query: %w", err)
	}

	var value string
	err = r.db.QueryRowContext(ctx, query, args...).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil // Return empty string if not found
		}
		return "", fmt.Errorf("executing get setting query: %w", err)
	}

	// Decode if it's a token
	if key == "sync.server_token" && value != "" {
		return deobfuscateToken(value)
	}

	return value, nil
}

// GetSettings retrieves multiple settings by prefix
func (r *SQLSettingsRepository) GetSettings(ctx context.Context, prefix string) (map[string]string, error) {
	// Use LIKE query to get settings with a specific prefix
	q := squirrel.Select("key", "value").
		From("settings").
		Where(squirrel.Like{"key": prefix + "%"})

	query, args, err := q.ToSql()
	if err != nil {
		return nil, fmt.Errorf("building get settings query: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("executing get settings query: %w", err)
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var key, value string
		err := rows.Scan(&key, &value)
		if err != nil {
			return nil, fmt.Errorf("scanning setting row: %w", err)
		}

		// Deobfuscate token if necessary
		if key == "sync.server_token" && value != "" {
			value, err = deobfuscateToken(value)
			if err != nil {
				r.logger.Warn("Failed to deobfuscate token", "error", err)
				// Continue with other settings even if one fails
				continue
			}
		}

		settings[key] = value
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating setting rows: %w", err)
	}

	return settings, nil
}

// SetSetting sets a setting value
func (r *SQLSettingsRepository) SetSetting(ctx context.Context, key, value string) error {
	// Check if the setting already exists
	existingValue, err := r.GetSetting(ctx, key)
	if err != nil {
		return fmt.Errorf("checking for existing setting: %w", err)
	}

	// Obfuscate token if necessary
	storeValue := value
	if key == "sync.server_token" && value != "" {
		storeValue, err = obfuscateToken(value)
		if err != nil {
			return fmt.Errorf("obfuscating token: %w", err)
		}
	}

	now := time.Now().UTC()

	if existingValue == "" {
		// Insert new setting
		id := ulid.SettingID()
		q := squirrel.Insert("settings").
			Columns("id", "key", "value", "created_at", "updated_at").
			Values(id, key, storeValue, now, now)

		query, args, err := q.ToSql()
		if err != nil {
			return fmt.Errorf("building insert setting query: %w", err)
		}

		_, err = r.db.ExecContext(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("executing insert setting query: %w", err)
		}
	} else {
		// Update existing setting
		q := squirrel.Update("settings").
			Set("value", storeValue).
			Set("updated_at", now).
			Where(squirrel.Eq{"key": key})

		query, args, err := q.ToSql()
		if err != nil {
			return fmt.Errorf("building update setting query: %w", err)
		}

		_, err = r.db.ExecContext(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("executing update setting query: %w", err)
		}
	}

	return nil
}

// DeleteSetting deletes a setting
func (r *SQLSettingsRepository) DeleteSetting(ctx context.Context, key string) error {
	q := squirrel.Delete("settings").
		Where(squirrel.Eq{"key": key})

	query, args, err := q.ToSql()
	if err != nil {
		return fmt.Errorf("building delete setting query: %w", err)
	}

	_, err = r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("executing delete setting query: %w", err)
	}

	return nil
}

// LoadSyncSettings loads sync settings from the database into a Config object
func LoadSyncSettings(ctx context.Context, cfg *Config, repo SettingsRepository) error {
	settings, err := repo.GetSettings(ctx, "sync.")
	if err != nil {
		return fmt.Errorf("loading sync settings: %w", err)
	}

	// Update config only for non-empty values
	if url, ok := settings["sync.server_url"]; ok && url != "" {
		cfg.Server.URL = url
	}

	if token, ok := settings["sync.server_token"]; ok && token != "" {
		cfg.Server.Token = token
	}

	if deviceName, ok := settings["sync.device_name"]; ok && deviceName != "" {
		cfg.Server.DeviceName = deviceName
	}

	if enabled, ok := settings["sync.enabled"]; ok && enabled != "" {
		if enabled == "true" {
			cfg.Server.Enabled = true
		} else {
			cfg.Server.Enabled = false
		}
	}

	return nil
}

// SaveSyncSettings saves sync settings from a Config object to the database
func SaveSyncSettings(ctx context.Context, cfg *Config, repo SettingsRepository) error {
	// Save server URL
	if err := repo.SetSetting(ctx, "sync.server_url", cfg.Server.URL); err != nil {
		return fmt.Errorf("saving server URL: %w", err)
	}

	// Save token
	if err := repo.SetSetting(ctx, "sync.server_token", cfg.Server.Token); err != nil {
		return fmt.Errorf("saving server token: %w", err)
	}

	// Save device name
	if err := repo.SetSetting(ctx, "sync.device_name", cfg.Server.DeviceName); err != nil {
		return fmt.Errorf("saving device name: %w", err)
	}

	// Save enabled status
	enabledStr := "false"
	if cfg.Server.Enabled {
		enabledStr = "true"
	}
	if err := repo.SetSetting(ctx, "sync.enabled", enabledStr); err != nil {
		return fmt.Errorf("saving enabled status: %w", err)
	}

	return nil
}

// Simple token obfuscation functions
// These provide basic obfuscation, not true encryption

// obfuscateToken performs basic token obfuscation
func obfuscateToken(token string) (string, error) {
	// Reverse the token
	runes := []rune(token)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	reversed := string(runes)

	// Base64 encode and add a marker
	encoded := base64.StdEncoding.EncodeToString([]byte(reversed))
	return "OBFS:" + encoded, nil
}

// deobfuscateToken reverses the obfuscation
func deobfuscateToken(obfuscated string) (string, error) {
	// Check if it's obfuscated
	if !strings.HasPrefix(obfuscated, "OBFS:") {
		return obfuscated, nil // Not obfuscated
	}

	// Extract the encoded part
	encoded := strings.TrimPrefix(obfuscated, "OBFS:")

	// Base64 decode
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decoding obfuscated token: %w", err)
	}

	// Reverse the string
	runes := []rune(string(decoded))
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}

	return string(runes), nil
}
