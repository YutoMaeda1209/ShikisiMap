package scheduler_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/db/dbtest"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/scheduler"
)

func exec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func count(t *testing.T, pool *pgxpool.Pool, sql string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), sql).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func addSessions(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	exec(t, pool, `INSERT INTO users (id, google_sub, email, display_name) VALUES ('11111111-0000-4000-8000-000000000001', 's', 'e', 'n') ON CONFLICT DO NOTHING`)
	exec(t, pool, `
		INSERT INTO sessions (user_id, token_hash, expires_at) VALUES
		('11111111-0000-4000-8000-000000000001', 'expired', now() - interval '1 second'),
		('11111111-0000-4000-8000-000000000001', 'long-expired', now() - interval '40 days'),
		('11111111-0000-4000-8000-000000000001', 'valid', now() + interval '1 hour')`)
}

func TestDeletesExpiredSessions(t *testing.T) {
	pool := dbtest.New(t)
	addSessions(t, pool)
	exec(t, pool, `INSERT INTO spots (name, address, location) VALUES ('spot', 'addr', 'POINT(100 50)')`)

	result, err := scheduler.New(pool).RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.SessionsDeleted != 2 || result.Skipped {
		t.Errorf("result = %+v", result)
	}
	if n := count(t, pool, `SELECT count(*) FROM sessions WHERE token_hash = 'valid'`); n != 1 {
		t.Error("the valid session was deleted")
	}
	if n := count(t, pool, `SELECT count(*) FROM users`); n != 1 {
		t.Error("the user was deleted")
	}
	if n := count(t, pool, `SELECT count(*) FROM spots`); n != 1 {
		t.Error("the spot was touched")
	}
}

func TestOnlyOneRunAtATime(t *testing.T) {
	pool := dbtest.New(t)
	s := scheduler.New(pool)
	ctx := context.Background()

	// Another instance holds the lock on its own connection.
	other, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Release()
	if _, err := other.Exec(ctx, `SELECT pg_advisory_lock(741120260001)`); err != nil {
		t.Fatal(err)
	}
	addSessions(t, pool)

	result, err := s.RunOnce(ctx)
	if err != nil || !result.Skipped {
		t.Errorf("result = %+v, err = %v; want the run skipped", result, err)
	}
	if n := count(t, pool, `SELECT count(*) FROM sessions`); n != 3 {
		t.Errorf("sessions = %d, want nothing deleted", n)
	}

	if _, err := other.Exec(ctx, `SELECT pg_advisory_unlock(741120260001)`); err != nil {
		t.Fatal(err)
	}
	// The lock is released after each run, so runs can follow each other.
	for i := range 2 {
		result, err := s.RunOnce(ctx)
		if err != nil || result.Skipped {
			t.Errorf("run %d: result = %+v, err = %v", i, result, err)
		}
	}
	if n := count(t, pool, `SELECT count(*) FROM sessions`); n != 1 {
		t.Errorf("sessions = %d, want only the valid one", n)
	}
}
