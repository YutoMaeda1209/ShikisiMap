// Package dbtest provides a migrated, empty database for tests.
package dbtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/db"
)

// EnvVar names the environment variable holding the connection string of a
// PostgreSQL server with PostGIS installed. The user must be allowed to
// create databases and the postgis extension.
const EnvVar = "TEST_DATABASE_URL"

// New creates a database with every migration applied and returns a pool
// connected to it. The database is dropped when the test ends.
//
// The test is skipped when TEST_DATABASE_URL is not set.
func New(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := NewEmpty(t)
	if err := db.Migrate(context.Background(), pool); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	return pool
}

// NewEmpty is like New but applies no migrations.
func NewEmpty(t *testing.T) *pgxpool.Pool {
	t.Helper()

	adminURL := os.Getenv(EnvVar)
	if adminURL == "" {
		t.Skipf("%s is not set", EnvVar)
	}
	ctx := context.Background()

	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatalf("connect to %s: %v", EnvVar, err)
	}
	name := "shikisimap_test_" + randomSuffix(t)
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		admin.Close(ctx)
		t.Fatalf("create test database: %v", err)
	}
	t.Cleanup(func() {
		defer admin.Close(ctx)
		if _, err := admin.Exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)"); err != nil {
			t.Errorf("drop test database: %v", err)
		}
	})

	u, err := url.Parse(adminURL)
	if err != nil {
		t.Fatalf("parse %s: %v", EnvVar, err)
	}
	u.Path = "/" + name
	pool, err := db.Open(ctx, u.String())
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	// Cleanups run last-in first-out, so the pool closes before the drop.
	t.Cleanup(pool.Close)
	return pool
}

func randomSuffix(t *testing.T) string {
	t.Helper()
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}
