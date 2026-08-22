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
	driverURL := databaseURL
	if strings.HasPrefix(driverURL, "postgres://") {
		driverURL = "pgx5" + driverURL[len("postgres"):]
	}
	if strings.HasPrefix(driverURL, "postgresql://") {
		driverURL = "pgx5" + driverURL[len("postgresql"):]
	}

	src := "file://" + migrationsPath
	m, err := migrate.New(src, driverURL)
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}
