// Package db opens the PostgreSQL connection pool and applies the migrations.
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/YutoMaeda1209/ShikisiMap/src/backend/migrations"
)

// Open creates a connection pool and checks that the database is reachable.
func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	return pool, nil
}

// Migrate applies every pending migration.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	provider, closeDB, err := newProvider(pool)
	if err != nil {
		return err
	}
	defer closeDB()

	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

func newProvider(pool *pgxpool.Pool) (*goose.Provider, func(), error) {
	sqlDB := stdlib.OpenDBFromPool(pool)
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	if err != nil {
		sqlDB.Close()
		return nil, nil, fmt.Errorf("load migrations: %w", err)
	}
	return provider, func() { sqlDB.Close() }, nil
}
