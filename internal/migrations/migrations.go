// Package migrations provides embedded SQL migrations for the application
package migrations

import (
	"embed"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/tildaslashalef/mindnest/internal/loggy"
)

//go:embed sql
var migrationsFS embed.FS

// GetSource creates a migrate.Source from the embedded migrations
func GetSource() (source.Driver, error) {
	// Access the embedded migrations directory
	migrationFS, err := fs.Sub(migrationsFS, "sql")
	if err != nil {
		return nil, fmt.Errorf("failed to access embedded migrations: %w", err)
	}

	// Create a new source from the embedded filesystem
	source, err := iofs.New(migrationFS, ".")
	if err != nil {
		loggy.Error("Failed to create migration source", "error", err)
		return nil, fmt.Errorf("failed to create migration source: %w", err)
	}

	return source, nil
}
