package db_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/db"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/db/dbtest"
)

const (
	uniqueViolation     = "23505"
	foreignKeyViolation = "23503"
	checkViolation      = "23514"
	notNullViolation    = "23502"
)

func wantPgError(t *testing.T, err error, code string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != code {
		t.Errorf("err = %v, want SQLSTATE %s", err, code)
	}
}

func tables(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
		  AND table_name NOT IN ('goose_db_version', 'spatial_ref_sys')
		ORDER BY table_name`)
	if err != nil {
		t.Fatal(err)
	}
	names, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return names
}

func insertSpot(t *testing.T, pool *pgxpool.Pool, name string, placeID *string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(), `
		INSERT INTO spots (place_id, name, address, location)
		VALUES ($1, $2, '東京都', ST_SetSRID(ST_MakePoint(139.7671, 35.6812), 4326)::geography)
		RETURNING id`, placeID, name).Scan(&id)
	if err != nil {
		t.Fatalf("insert spot: %v", err)
	}
	return id
}

func insertUser(t *testing.T, pool *pgxpool.Pool, sub string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(), `
		INSERT INTO users (google_sub, email, display_name)
		VALUES ($1, $1 || '@example.com', $1)
		RETURNING id`, sub).Scan(&id)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return id
}

func count(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestMigrateCreatesTables(t *testing.T) {
	pool := dbtest.New(t)

	want := []string{"appearances", "sessions", "spots", "users", "visits"}
	if got := tables(t, pool); !slices.Equal(got, want) {
		t.Errorf("tables = %v, want %v", got, want)
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	pool := dbtest.New(t)
	insertSpot(t, pool, "spot", nil)

	if err := db.Migrate(context.Background(), pool); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	if n := count(t, pool, "SELECT count(*) FROM spots"); n != 1 {
		t.Errorf("spots = %d after the second Migrate, want 1", n)
	}
}

func TestMigrateDown(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()

	if err := db.MigrateDownTo(ctx, pool, 1); err != nil {
		t.Fatalf("down: %v", err)
	}
	if got := tables(t, pool); len(got) != 0 {
		t.Errorf("tables after down = %v, want none", got)
	}
	if n := count(t, pool, "SELECT count(*) FROM pg_proc WHERE proname = 'set_updated_at'"); n != 0 {
		t.Error("set_updated_at() remains after down")
	}

	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("up after down: %v", err)
	}
	if got := tables(t, pool); len(got) != 5 {
		t.Errorf("tables after up = %v, want 5 tables", got)
	}
}

func TestSpots(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()

	placeID := "ChIJ-test"
	id := insertSpot(t, pool, "with place", &placeID)

	t.Run("defaults", func(t *testing.T) {
		var (
			isClosed          bool
			fetchedAt, status *time.Time
			lng, lat          float64
		)
		err := pool.QueryRow(ctx, `
			SELECT is_closed, location_fetched_at, status_checked_at,
			       ST_X(location::geometry), ST_Y(location::geometry)
			FROM spots WHERE id = $1`, id).Scan(&isClosed, &fetchedAt, &status, &lng, &lat)
		if err != nil {
			t.Fatal(err)
		}
		if isClosed || fetchedAt != nil || status != nil {
			t.Errorf("is_closed = %v, location_fetched_at = %v, status_checked_at = %v", isClosed, fetchedAt, status)
		}
		if lng != 139.7671 || lat != 35.6812 {
			t.Errorf("location = (%v, %v)", lng, lat)
		}
	})

	t.Run("place_id is unique", func(t *testing.T) {
		_, err := pool.Exec(ctx, `
			INSERT INTO spots (place_id, name, address, location)
			VALUES ($1, 'dup', 'x', 'POINT(0 0)')`, placeID)
		wantPgError(t, err, uniqueViolation)
	})

	t.Run("several spots may have no place_id", func(t *testing.T) {
		insertSpot(t, pool, "unresolved 1", nil)
		insertSpot(t, pool, "unresolved 2", nil)
	})

	t.Run("location is required", func(t *testing.T) {
		_, err := pool.Exec(ctx, `INSERT INTO spots (name, address) VALUES ('x', 'x')`)
		wantPgError(t, err, notNullViolation)
	})
}

func TestUpdatedAtIsMaintained(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()

	spotID := insertSpot(t, pool, "spot", nil)
	userID := insertUser(t, pool, "sub")
	var visitID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO visits (user_id, spot_id) VALUES ($1, $2) RETURNING id`, userID, spotID,
	).Scan(&visitID); err != nil {
		t.Fatal(err)
	}

	for table, id := range map[string]string{"spots": spotID, "users": userID, "visits": visitID} {
		t.Run(table, func(t *testing.T) {
			old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
			// A session-local setting skips the trigger so the test can backdate the row.
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if _, err := tx.Exec(ctx, "SET LOCAL session_replication_role = replica"); err != nil {
				t.Fatal(err)
			}
			if _, err := tx.Exec(ctx, "UPDATE "+table+" SET updated_at = $1, created_at = $1 WHERE id = $2", old, id); err != nil {
				t.Fatal(err)
			}
			if err := tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}

			if _, err := pool.Exec(ctx, "UPDATE "+table+" SET created_at = created_at WHERE id = $1", id); err != nil {
				t.Fatal(err)
			}
			var createdAt, updatedAt time.Time
			if err := pool.QueryRow(ctx, "SELECT created_at, updated_at FROM "+table+" WHERE id = $1", id).Scan(&createdAt, &updatedAt); err != nil {
				t.Fatal(err)
			}
			if !createdAt.Equal(old) {
				t.Errorf("created_at = %v, want it unchanged", createdAt)
			}
			if time.Since(updatedAt) > time.Minute {
				t.Errorf("updated_at = %v, want now", updatedAt)
			}
		})
	}
}

func TestAppearances(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	spotID := insertSpot(t, pool, "spot", nil)
	otherSpotID := insertSpot(t, pool, "other", nil)

	insert := func(spot, youtubeID string, timestamp int) error {
		_, err := pool.Exec(ctx,
			`INSERT INTO appearances (spot_id, youtube_id, "timestamp") VALUES ($1, $2, $3)`,
			spot, youtubeID, timestamp)
		return err
	}

	if err := insert(spotID, "video1", 10); err != nil {
		t.Fatal(err)
	}
	t.Run("same spot, video and timestamp is rejected", func(t *testing.T) {
		wantPgError(t, insert(spotID, "video1", 10), uniqueViolation)
	})
	t.Run("another timestamp, video or spot is accepted", func(t *testing.T) {
		for _, err := range []error{
			insert(spotID, "video1", 20),
			insert(spotID, "video2", 10),
			insert(otherSpotID, "video1", 10),
		} {
			if err != nil {
				t.Error(err)
			}
		}
	})
	t.Run("negative timestamp is rejected", func(t *testing.T) {
		wantPgError(t, insert(spotID, "video3", -1), checkViolation)
	})
	t.Run("unknown spot is rejected", func(t *testing.T) {
		wantPgError(t, insert("00000000-0000-0000-0000-000000000000", "video1", 10), foreignKeyViolation)
	})
	t.Run("deleted with the spot", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "DELETE FROM spots WHERE id = $1", spotID); err != nil {
			t.Fatal(err)
		}
		if n := count(t, pool, "SELECT count(*) FROM appearances WHERE spot_id = $1", spotID); n != 0 {
			t.Errorf("%d appearances remain", n)
		}
		if n := count(t, pool, "SELECT count(*) FROM appearances"); n != 1 {
			t.Errorf("appearances of the other spot = %d, want 1", n)
		}
	})
}

func TestUsers(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	id := insertUser(t, pool, "sub")

	var public bool
	if err := pool.QueryRow(ctx, "SELECT profile_public FROM users WHERE id = $1", id).Scan(&public); err != nil {
		t.Fatal(err)
	}
	if public {
		t.Error("profile_public defaults to true, want false")
	}

	_, err := pool.Exec(ctx, `INSERT INTO users (google_sub, email, display_name) VALUES ('sub', 'a@example.com', 'a')`)
	wantPgError(t, err, uniqueViolation)
}

func TestVisits(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	spotID := insertSpot(t, pool, "spot", nil)
	otherSpotID := insertSpot(t, pool, "other", nil)
	userID := insertUser(t, pool, "sub")
	otherUserID := insertUser(t, pool, "other")
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	insert := func(user, spot string, visitedAt time.Time) error {
		_, err := pool.Exec(ctx,
			`INSERT INTO visits (user_id, spot_id, visited_at, memo) VALUES ($1, $2, $3, 'memo')`,
			user, spot, visitedAt)
		return err
	}

	if err := insert(userID, spotID, at); err != nil {
		t.Fatal(err)
	}
	t.Run("same user, spot and time is rejected", func(t *testing.T) {
		wantPgError(t, insert(userID, spotID, at), uniqueViolation)
	})
	t.Run("same instant in another time zone is rejected", func(t *testing.T) {
		wantPgError(t, insert(userID, spotID, at.In(time.FixedZone("JST", 9*60*60))), uniqueViolation)
	})
	t.Run("ON CONFLICT DO NOTHING skips the duplicate", func(t *testing.T) {
		tag, err := pool.Exec(ctx, `
			INSERT INTO visits (user_id, spot_id, visited_at) VALUES ($1, $2, $3)
			ON CONFLICT (user_id, spot_id, visited_at) DO NOTHING`, userID, spotID, at)
		if err != nil {
			t.Fatal(err)
		}
		if tag.RowsAffected() != 0 {
			t.Errorf("rows affected = %d, want 0", tag.RowsAffected())
		}
	})
	t.Run("another time, spot or user is accepted", func(t *testing.T) {
		for _, err := range []error{
			insert(userID, spotID, at.Add(time.Second)),
			insert(userID, otherSpotID, at),
			insert(otherUserID, spotID, at),
		} {
			if err != nil {
				t.Error(err)
			}
		}
	})
	t.Run("memo and visited_at are optional", func(t *testing.T) {
		var (
			memo      *string
			visitedAt time.Time
		)
		err := pool.QueryRow(ctx,
			`INSERT INTO visits (user_id, spot_id) VALUES ($1, $2) RETURNING memo, visited_at`,
			otherUserID, otherSpotID).Scan(&memo, &visitedAt)
		if err != nil {
			t.Fatal(err)
		}
		if memo != nil || time.Since(visitedAt) > time.Minute {
			t.Errorf("memo = %v, visited_at = %v", memo, visitedAt)
		}
	})
	t.Run("unknown spot is rejected", func(t *testing.T) {
		wantPgError(t, insert(userID, "00000000-0000-0000-0000-000000000000", at), foreignKeyViolation)
	})
	t.Run("a visited spot cannot be deleted", func(t *testing.T) {
		_, err := pool.Exec(ctx, "DELETE FROM spots WHERE id = $1", spotID)
		wantPgError(t, err, foreignKeyViolation)
	})
}

func TestDeletingUserCascades(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	spotID := insertSpot(t, pool, "spot", nil)
	userID := insertUser(t, pool, "sub")
	otherUserID := insertUser(t, pool, "other")

	for i, user := range []string{userID, userID, otherUserID} {
		if _, err := pool.Exec(ctx,
			`INSERT INTO visits (user_id, spot_id, visited_at) VALUES ($1, $2, now() + $3 * interval '1 hour')`,
			user, spotID, i); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO sessions (user_id, token_hash, expires_at) VALUES ($1, $2, now() + interval '30 days')`,
			user, "hash"+string(rune('a'+i))); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := pool.Exec(ctx, "DELETE FROM users WHERE id = $1", userID); err != nil {
		t.Fatal(err)
	}
	if n := count(t, pool, "SELECT count(*) FROM visits"); n != 1 {
		t.Errorf("visits = %d, want only the other user's 1", n)
	}
	if n := count(t, pool, "SELECT count(*) FROM sessions"); n != 1 {
		t.Errorf("sessions = %d, want only the other user's 1", n)
	}
	if n := count(t, pool, "SELECT count(*) FROM spots"); n != 1 {
		t.Errorf("spots = %d, want 1", n)
	}
}

func TestSessions(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	userID := insertUser(t, pool, "sub")

	insert := func(user any, hash string) error {
		_, err := pool.Exec(ctx,
			`INSERT INTO sessions (user_id, token_hash, expires_at) VALUES ($1, $2, now() + interval '30 days')`,
			user, hash)
		return err
	}
	if err := insert(userID, "hash"); err != nil {
		t.Fatal(err)
	}
	t.Run("token_hash is unique", func(t *testing.T) {
		wantPgError(t, insert(userID, "hash"), uniqueViolation)
	})
	t.Run("a user may have several sessions", func(t *testing.T) {
		if err := insert(userID, "hash2"); err != nil {
			t.Error(err)
		}
	})
	t.Run("unknown user is rejected", func(t *testing.T) {
		wantPgError(t, insert("00000000-0000-0000-0000-000000000000", "hash3"), foreignKeyViolation)
	})
	t.Run("expires_at is required", func(t *testing.T) {
		_, err := pool.Exec(ctx, `INSERT INTO sessions (user_id, token_hash) VALUES ($1, 'hash4')`, userID)
		wantPgError(t, err, notNullViolation)
	})
}
