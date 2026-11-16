package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// MigrateDownTo rolls the migrations back to version. Only tests roll back.
func MigrateDownTo(ctx context.Context, pool *pgxpool.Pool, version int64) error {
	provider, closeDB, err := newProvider(pool)
	if err != nil {
		return err
	}
	defer closeDB()
	_, err = provider.DownTo(ctx, version)
	return err
}
