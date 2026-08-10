package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func runMigrations(databaseURL string) error {
	migrationsPath := os.Getenv("MIGRATIONS_PATH")
	if migrationsPath == "" {
		migrationsPath = "/migrations"
	}

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

	ver, dirty, err := m.Version()
	if err == nil {
		slog.Info("migrations applied", "version", ver, "dirty", dirty)
	}
	return nil
}
