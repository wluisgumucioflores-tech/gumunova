package postgres

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // registers the pgx5:// driver
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect opens a connection pool and verifies the database is reachable.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// MigrateUp applies every pending migration found in files (embedded *.sql).
func MigrateUp(databaseURL string, files fs.FS) error {
	source, err := iofs.New(files, ".")
	if err != nil {
		return fmt.Errorf("open migration files: %w", err)
	}
	migrator, err := migrate.NewWithSourceInstance("iofs", source, toMigrateURL(databaseURL))
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer migrator.Close()

	if err := migrator.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

// toMigrateURL maps postgres:// URLs to the scheme of the pgx v5 migrate driver.
func toMigrateURL(databaseURL string) string {
	for _, prefix := range []string{"postgresql://", "postgres://"} {
		if rest, found := strings.CutPrefix(databaseURL, prefix); found {
			return "pgx5://" + rest
		}
	}
	return databaseURL
}
