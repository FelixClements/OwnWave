package testdb

import (
	"errors"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func RunMigrations(databaseURL, migrationsPath string) error {
	m, err := newMigrate(databaseURL, migrationsPath)
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// MigrateTo moves the database up or down to the given migration version.
func MigrateTo(databaseURL, migrationsPath string, version uint) error {
	m, err := newMigrate(databaseURL, migrationsPath)
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Migrate(version); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate to %d: %w", version, err)
	}
	return nil
}

func newMigrate(databaseURL, migrationsPath string) (*migrate.Migrate, error) {
	driverURL := databaseURL
	if strings.HasPrefix(driverURL, "postgres://") {
		driverURL = "pgx5" + driverURL[len("postgres"):]
	}
	if strings.HasPrefix(driverURL, "postgresql://") {
		driverURL = "pgx5" + driverURL[len("postgresql"):]
	}

	m, err := migrate.New("file://"+migrationsPath, driverURL)
	if err != nil {
		return nil, fmt.Errorf("migrate init: %w", err)
	}
	return m, nil
}
