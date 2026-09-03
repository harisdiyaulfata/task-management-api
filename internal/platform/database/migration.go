package database

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// RunMigrations executes pending *.up.sql migrations before HTTP traffic is
// accepted. migrate.ErrNoChange is a successful, idempotent startup outcome.
func RunMigrations(postgresDSN, migrationsPath string) error {
	absPath, err := filepath.Abs(migrationsPath)
	if err != nil {
		return fmt.Errorf("resolve migrations path: %w", err)
	}
	// golang-migrate's file driver expects forward slashes, including on Windows.
	sourceURL := "file://" + strings.ReplaceAll(absPath, `\`, "/")
	migrator, err := migrate.New(sourceURL, postgresDSN)
	if err != nil {
		return fmt.Errorf("initialize migrations: %w", err)
	}
	defer func() {
		_, _ = migrator.Close()
	}()

	if err := migrator.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}
