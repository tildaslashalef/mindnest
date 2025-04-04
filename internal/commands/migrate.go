package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/tildaslashalef/mindnest/internal/database"
	"github.com/tildaslashalef/mindnest/internal/utils"
	"github.com/urfave/cli/v2"
)

// MigrateCommand returns the CLI command for database migrations
func MigrateCommand() *cli.Command {
	return &cli.Command{
		Name:   "migrate",
		Usage:  "Manage database migrations",
		Hidden: true,
		Subcommands: []*cli.Command{
			{
				Name:  "up",
				Usage: "Apply all pending migrations",
				Action: func(c *cli.Context) error {
					// We no longer need the application instance for migrations
					// since they're embedded in the binary

					utils.PrintInfo("Applying embedded migrations")

					// Use the public RunMigrations function
					migrationsApplied, err := database.RunMigrations()
					if err != nil {
						utils.PrintError(fmt.Sprintf("Failed to apply migrations: %s", err))
						return fmt.Errorf("failed to apply migrations: %w", err)
					}

					if migrationsApplied > 0 {
						utils.PrintSuccess(fmt.Sprintf("Applied %d migration(s) successfully!", migrationsApplied))
					} else {
						utils.PrintSuccess("Database schema is already up-to-date")
					}
					return nil
				},
			},
			{
				Name:  "down",
				Usage: "Revert the last migration",
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:  "steps",
						Usage: "Number of migrations to revert (default: 1)",
						Value: 1,
					},
				},
				Action: func(c *cli.Context) error {
					// We no longer need the application instance for migrations
					// since they're embedded in the binary

					steps := c.Int("steps")

					utils.PrintWarning(fmt.Sprintf("Reverting %d embedded migration(s)", steps))

					// Use the public RevertMigrations function
					if err := database.RevertMigrations(steps); err != nil {
						utils.PrintError(fmt.Sprintf("Failed to revert migrations: %s", err))
						return fmt.Errorf("failed to revert migrations: %w", err)
					}

					utils.PrintSuccess("Migration(s) reverted successfully!")
					return nil
				},
			},
			{
				Name:  "create",
				Usage: "Create a new migration (development only)",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "name",
						Aliases:  []string{"n"},
						Usage:    "Name of the migration (eg: create_users_table)",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "path",
						Usage:    "Path where migration files will be created",
						Required: true,
					},
				},
				Action: func(c *cli.Context) error {
					name := c.String("name")
					path := c.String("path")

					utils.PrintWarning("Note: This command is intended for development use only.")
					utils.PrintWarning("New migrations will not be automatically embedded in the binary.")

					// Ensure the migrations directory exists
					if err := os.MkdirAll(path, 0755); err != nil {
						utils.PrintError(fmt.Sprintf("Failed to create migrations directory: %s", err))
						return fmt.Errorf("failed to create migrations directory: %w", err)
					}

					// Get the next migration number
					nextNumber, err := getNextMigrationNumber(path)
					if err != nil {
						utils.PrintError(fmt.Sprintf("Failed to determine next migration number: %s", err))
						return fmt.Errorf("failed to determine next migration number: %w", err)
					}

					utils.PrintInfo(fmt.Sprintf("Creating new migration: %s (version %d)", name, nextNumber))

					// Create filenames using sequential numbering format (000001, 000002, etc)
					// Use dots instead of underscores for up/down to match golang-migrate expectation
					upFilename := fmt.Sprintf("%06d_%s.up.sql", nextNumber, name)
					downFilename := fmt.Sprintf("%06d_%s.down.sql", nextNumber, name)

					// Create up migration file
					upFile := filepath.Join(path, upFilename)
					if err := os.WriteFile(upFile, []byte("-- Write your UP migration SQL here\n"), 0644); err != nil {
						utils.PrintError(fmt.Sprintf("Failed to create up migration file: %s", err))
						return fmt.Errorf("failed to create up migration file: %w", err)
					}

					// Create down migration file
					downFile := filepath.Join(path, downFilename)
					if err := os.WriteFile(downFile, []byte("-- Write your DOWN migration SQL here\n"), 0644); err != nil {
						utils.PrintError(fmt.Sprintf("Failed to create down migration file: %s", err))
						return fmt.Errorf("failed to create down migration file: %w", err)
					}

					utils.PrintSuccess("Migration created successfully!")
					utils.PrintInfo(fmt.Sprintf("Up migration: %s", upFile))
					utils.PrintInfo(fmt.Sprintf("Down migration: %s", downFile))
					utils.PrintWarning("Remember to copy these files to internal/migrations/sql/ and rebuild to embed them.")

					return nil
				},
			},
		},
	}
}

// getNextMigrationNumber determines the next migration number by scanning
// existing migrations and incrementing the highest number found
func getNextMigrationNumber(migrationsPath string) (int, error) {
	// Read all migration files
	entries, err := os.ReadDir(migrationsPath)
	if err != nil {
		if os.IsNotExist(err) {
			// If directory doesn't exist, start with 1
			return 1, nil
		}
		return 0, err
	}

	if len(entries) == 0 {
		// If no migrations exist yet, start with 1
		return 1, nil
	}

	// Extract numbers from filenames
	numbers := []int{}
	for _, entry := range entries {
		if !entry.IsDir() {
			// Look for both formats: xxx_name.up.sql or xxx_name_up.sql
			if strings.HasSuffix(entry.Name(), ".up.sql") || strings.HasSuffix(entry.Name(), "_up.sql") {
				// Extract the number prefix (e.g., "000001" from "000001_create_users_table.up.sql")
				parts := strings.Split(entry.Name(), "_")
				if len(parts) >= 2 {
					if num, err := strconv.Atoi(parts[0]); err == nil {
						numbers = append(numbers, num)
					}
				}
			}
		}
	}

	if len(numbers) == 0 {
		// If no valid migration numbers found, start with 1
		return 1, nil
	}

	// Find the highest number
	sort.Ints(numbers)
	highest := numbers[len(numbers)-1]

	// Return the next number
	return highest + 1, nil
}
