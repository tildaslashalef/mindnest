package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/fatih/color"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/tildaslashalef/mindnest/internal/app"
	"github.com/tildaslashalef/mindnest/internal/database"
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
					// App is already initialized in Before hook
					application, err := getAppFromContext(c)
					if err != nil {
						return err
					}

					migrationsPath := application.Config.Database.MigrationsPath

					fmt.Println(color.CyanString("Applying migrations from %s", migrationsPath))

					// Use the public RunMigrations function
					if err := database.RunMigrations(migrationsPath); err != nil {
						return fmt.Errorf("failed to apply migrations: %w", err)
					}

					fmt.Println(color.GreenString("✓ Migrations applied successfully!"))
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
					application, err := getAppFromContext(c)
					if err != nil {
						return err
					}

					migrationsPath := application.Config.Database.MigrationsPath
					steps := c.Int("steps")

					fmt.Println(color.YellowString("Reverting %d migration(s) using %s", steps, migrationsPath))

					// Use the public RevertMigrations function
					if err := database.RevertMigrations(migrationsPath, steps); err != nil {
						return fmt.Errorf("failed to revert migrations: %w", err)
					}

					fmt.Println(color.GreenString("✓ Migration(s) reverted successfully!"))
					return nil
				},
			},
			{
				Name:  "create",
				Usage: "Create a new migration",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "name",
						Aliases:  []string{"n"},
						Usage:    "Name of the migration (eg: create_users_table)",
						Required: true,
					},
				},
				Action: func(c *cli.Context) error {
					name := c.String("name")

					// Get migrations path from context
					application, err := getAppFromContext(c)
					if err != nil {
						return err
					}

					// Get migrations directory from config (remove file:// prefix)
					migrationsPath := strings.TrimPrefix(application.Config.Database.MigrationsPath, "file://")

					// Ensure the migrations directory exists
					if err := os.MkdirAll(migrationsPath, 0755); err != nil {
						return fmt.Errorf("failed to create migrations directory: %w", err)
					}

					// Get the next migration number
					nextNumber, err := getNextMigrationNumber(migrationsPath)
					if err != nil {
						return fmt.Errorf("failed to determine next migration number: %w", err)
					}

					fmt.Println(color.CyanString("Creating new migration: %s (version %d)", name, nextNumber))

					// Create filenames using sequential numbering format (000001, 000002, etc)
					// Use dots instead of underscores for up/down to match golang-migrate expectation
					upFilename := fmt.Sprintf("%06d_%s.up.sql", nextNumber, name)
					downFilename := fmt.Sprintf("%06d_%s.down.sql", nextNumber, name)

					// Create up migration file
					upFile := filepath.Join(migrationsPath, upFilename)
					if err := os.WriteFile(upFile, []byte("-- Write your UP migration SQL here\n"), 0644); err != nil {
						return fmt.Errorf("failed to create up migration file: %w", err)
					}

					// Create down migration file
					downFile := filepath.Join(migrationsPath, downFilename)
					if err := os.WriteFile(downFile, []byte("-- Write your DOWN migration SQL here\n"), 0644); err != nil {
						return fmt.Errorf("failed to create down migration file: %w", err)
					}

					fmt.Println(color.GreenString("✓ Migration created successfully!"))
					fmt.Printf("  Up migration: %s\n", upFile)
					fmt.Printf("  Down migration: %s\n", downFile)

					return nil
				},
			},
		},
	}
}

// getAppFromContext retrieves the app instance from the CLI context
func getAppFromContext(c *cli.Context) (*app.App, error) {
	if c.App.Metadata == nil || c.App.Metadata["app"] == nil {
		return nil, fmt.Errorf("app not initialized in context")
	}
	application, ok := c.App.Metadata["app"].(*app.App)
	if !ok {
		return nil, fmt.Errorf("invalid app type in context")
	}
	return application, nil
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
