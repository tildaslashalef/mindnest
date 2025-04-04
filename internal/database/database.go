// Package database provides SQLite database management for Mindnest
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/mattn/go-sqlite3"
	"github.com/tildaslashalef/mindnest/internal/config"
	"github.com/tildaslashalef/mindnest/internal/loggy"
)

var (
	// ErrNotInitialized is returned when the database has not been initialized
	ErrNotInitialized = errors.New("database not initialized")

	db     *sql.DB
	dbLock sync.Mutex
)

// DB returns the database connection
func DB() (*sql.DB, error) {
	if db == nil {
		return nil, ErrNotInitialized
	}
	return db, nil
}

// InitDB initializes the database connection and migrates the schema
func InitDB(cfg *config.Config) error {
	dbLock.Lock()
	defer dbLock.Unlock()

	if db != nil {
		// Database already initialized
		return nil
	}

	// Create the database connection
	loggy.Info("Initializing database", "path", cfg.Database.Path)

	// Enable sqlite-vec extension
	sqlite_vec.Auto()

	// Build connection string
	connStr := buildSQLiteDSN(&cfg.Database)

	var err error
	db, err = sql.Open("sqlite3", connStr)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}

	// Set connection pool settings
	db.SetConnMaxLifetime(cfg.Database.ConnMaxLife)
	db.SetMaxOpenConns(1) // SQLite supports only one writer at a time
	db.SetMaxIdleConns(1)

	// Verify the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		db = nil
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Initialize vector extension
	if err := initVectorExtension(); err != nil {
		loggy.Warn("Failed to initialize vector extension", "error", err)
		// Don't fail the entire database initialization if vector extension fails
		// This allows the application to still work without vector functionality
	}

	loggy.Info("Database initialized successfully")
	return nil
}

// buildSQLiteDSN builds a SQLite DSN with additional parameters
func buildSQLiteDSN(cfg *config.DatabaseConfig) string {
	if cfg.Path == ":memory:" || strings.HasPrefix(cfg.Path, "file::memory:") {
		return cfg.Path
	}

	params := url.Values{}
	params.Add("_busy_timeout", strconv.Itoa(cfg.BusyTimeout))
	params.Add("_journal_mode", cfg.JournalMode)
	params.Add("_synchronous", cfg.SynchronousMode)
	if cfg.CacheSize != 0 {
		params.Add("_cache_size", strconv.Itoa(cfg.CacheSize))
	}
	params.Add("_foreign_keys", strconv.FormatBool(cfg.ForeignKeys))
	params.Add("cache", "shared")

	return fmt.Sprintf("%s?%s", cfg.Path, params.Encode())
}

// initVectorExtension initializes the sqlite-vec extension for vector embeddings
// This is an internal function called by InitDB
func initVectorExtension() error {
	if db == nil {
		return ErrNotInitialized
	}

	loggy.Info("Initializing sqlite-vec extension")

	// Check if vec_version() function exists
	var version string
	err := db.QueryRow("SELECT vec_version()").Scan(&version)
	if err == nil {
		// Extension already loaded
		loggy.Info("sqlite-vec extension already loaded", "version", version)
		return nil
	}

	loggy.Debug("vec_version() not available, sqlite-vec was not registered correctly", "error", err)
	loggy.Warn("Vectors and embeddings functionality may not work properly")

	return nil
}

// CloseDB closes the database connection
func CloseDB() error {
	dbLock.Lock()
	defer dbLock.Unlock()

	if db == nil {
		return nil
	}

	err := db.Close()
	db = nil
	return err
}

// QueryRowContext executes a query that returns a single row with context
func QueryRowContext(ctx context.Context, query string, args ...interface{}) (*sql.Row, error) {
	if db == nil {
		return nil, ErrNotInitialized
	}
	return db.QueryRowContext(ctx, query, args...), nil
}

// QueryContext executes a query that returns rows with context
func QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if db == nil {
		return nil, ErrNotInitialized
	}
	return db.QueryContext(ctx, query, args...)
}

// ExecContext executes a query without returning any rows with context
func ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if db == nil {
		return nil, ErrNotInitialized
	}
	return db.ExecContext(ctx, query, args...)
}

// BeginTx starts a new transaction with the provided context and options
func BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	if db == nil {
		return nil, ErrNotInitialized
	}
	return db.BeginTx(ctx, opts)
}

// WithTransaction executes a function within a database transaction
func WithTransaction(ctx context.Context, fn func(*sql.Tx) error) error {
	if db == nil {
		return ErrNotInitialized
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			// Rollback on panic and re-panic
			_ = tx.Rollback()
			panic(p)
		}
	}()

	err = fn(tx)
	if err != nil {
		// Rollback on error
		if rbErr := tx.Rollback(); rbErr != nil {
			// If rollback fails, log the error but return the original error
			loggy.Error("Failed to rollback transaction", "error", rbErr)
		}
		return err
	}

	// Commit the transaction if no error
	return tx.Commit()
}

// RunMigrations applies all pending migrations
func RunMigrations(migrationsPath string) error {
	if db == nil {
		return ErrNotInitialized
	}

	driver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
	if err != nil {
		return fmt.Errorf("failed to create database driver: %w", err)
	}

	// Remove 'file://' prefix if present and ensure path is clean
	cleanPath := strings.TrimPrefix(migrationsPath, "file://")
	sourceURL := "file://" + cleanPath

	loggy.Info("Using migrations source URL", "url", sourceURL)

	// Create migration instance
	m, err := migrate.NewWithDatabaseInstance(
		sourceURL,
		"sqlite3",
		driver,
	)
	if err != nil {
		loggy.Error("Failed to create migration instance", "error", err)
		return fmt.Errorf("failed to create migration instance: %w", err)
	}
	defer m.Close()

	// Apply migrations
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		loggy.Error("Failed to apply migrations", "error", err)
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	// Get the version after migration
	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("failed to get migration version: %w", err)
	}

	loggy.Info("Database migration complete",
		"version", version,
		"dirty", dirty,
		"error", err == migrate.ErrNilVersion,
	)

	return nil
}

// RevertMigrations reverts migrations back by the specified number of steps
func RevertMigrations(migrationsPath string, steps int) error {
	if db == nil {
		return ErrNotInitialized
	}

	driver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
	if err != nil {
		return fmt.Errorf("failed to create database driver: %w", err)
	}

	// Remove 'file://' prefix if present and ensure path is clean
	cleanPath := strings.TrimPrefix(migrationsPath, "file://")
	sourceURL := "file://" + cleanPath

	loggy.Info("Using migrations source URL", "url", sourceURL)

	// Create migration instance
	m, err := migrate.NewWithDatabaseInstance(
		sourceURL,
		"sqlite3",
		driver,
	)
	if err != nil {
		loggy.Error("Failed to create migration instance", "error", err)
		return fmt.Errorf("failed to create migration instance: %w", err)
	}
	defer m.Close()

	// Migrate down
	if err := m.Steps(-steps); err != nil && err != migrate.ErrNoChange {
		loggy.Error("Failed to revert migrations", "error", err)
		return fmt.Errorf("failed to revert migrations: %w", err)
	}

	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("failed to get migration version: %w", err)
	}

	loggy.Info("Database migration reversion complete",
		"version", version,
		"dirty", dirty,
		"error", err == migrate.ErrNilVersion,
	)

	return nil
}
