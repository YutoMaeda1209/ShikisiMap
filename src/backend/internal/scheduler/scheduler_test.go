package scheduler_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/db/dbtest"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/places"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/scheduler"
)

type fakePlaces struct {
	details map[string]places.Details
	errs    map[string]error
	calls   []string
	// quota is the number of calls allowed; negative means unlimited.
	quota int
}

func (f *fakePlaces) SearchText(context.Context, string) ([]places.Place, error) {
	return nil, errors.New("unexpected SearchText call")
}

func (f *fakePlaces) Details(_ context.Context, placeID string) (places.Details, error) {
	if f.quota == 0 {
		return places.Details{}, places.ErrQuotaExceeded
	}
	f.quota--
	f.calls = append(f.calls, placeID)
	if err := f.errs[placeID]; err != nil {
		return places.Details{}, err
	}
	if d, ok := f.details[placeID]; ok {
		return d, nil
	}
	return places.Details{Location: places.LatLng{Lat: 1, Lng: 2}, BusinessStatus: "OPERATIONAL"}, nil
}

func exec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func count(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// addSpot inserts a spot whose location was fetched the given number of days
// ago; a negative number means never.
func addSpot(t *testing.T, pool *pgxpool.Pool, placeID string, fetchedDaysAgo int) {
	t.Helper()
	var place, fetched any
	if placeID != "" {
		place = placeID
	}
	if fetchedDaysAgo >= 0 {
		fetched = fetchedDaysAgo
	}
	exec(t, pool, `
		INSERT INTO spots (place_id, name, address, location, location_fetched_at, status_checked_at)
		VALUES ($1, COALESCE($1, 'unresolved'), 'addr', 'POINT(100 50)',
		        now() - make_interval(days => $2), now() - make_interval(days => $2))`, place, fetched)
}

func mustRun(t *testing.T, s *scheduler.Scheduler) scheduler.Result {
	t.Helper()
	result, err := s.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestRefreshSpots(t *testing.T) {
	pool := dbtest.New(t)
	addSpot(t, pool, "fresh", 24)
	addSpot(t, pool, "due", 26)
	addSpot(t, pool, "never", -1)
	addSpot(t, pool, "closing", 40)
	addSpot(t, pool, "gone", 30)
	addSpot(t, pool, "", -1) // unresolved: nothing to ask Google
	exec(t, pool, `UPDATE spots SET is_closed = true WHERE place_id = 'due'`)

	api := &fakePlaces{quota: -1,
		details: map[string]places.Details{
			"due":     {Location: places.LatLng{Lat: 35.5, Lng: 139.5}, BusinessStatus: "OPERATIONAL"},
			"closing": {Location: places.LatLng{Lat: 3, Lng: 4}, BusinessStatus: "CLOSED_PERMANENTLY"},
		},
		errs: map[string]error{"gone": fmt.Errorf("%w: expired", places.ErrNotFound)},
	}
	result := mustRun(t, scheduler.New(pool, api, 0))

	if result.SpotsRefreshed != 3 || result.SpotsClosed != 1 || result.SpotsFailed != 1 || result.QuotaExceeded {
		t.Errorf("result = %+v", result)
	}
	if len(api.calls) != 4 {
		t.Errorf("calls = %v, want due, never, closing and gone", api.calls)
	}

	check := func(placeID, condition string) {
		t.Helper()
		if n := count(t, pool, `SELECT count(*) FROM spots WHERE place_id = $1 AND `+condition, placeID); n != 1 {
			t.Errorf("spot %q does not satisfy: %s", placeID, condition)
		}
	}
	const justNow = "> now() - interval '1 minute'"
	// Refreshed: new location and both timestamps. A manual closure stays.
	check("due", `ST_Y(location::geometry) = 35.5 AND ST_X(location::geometry) = 139.5 AND is_closed
		AND location_fetched_at `+justNow+` AND status_checked_at `+justNow)
	check("never", `ST_Y(location::geometry) = 1 AND NOT is_closed AND location_fetched_at `+justNow)
	check("closing", `is_closed AND ST_Y(location::geometry) = 3 AND location_fetched_at `+justNow)
	// Not due: untouched.
	check("fresh", `ST_Y(location::geometry) = 50 AND location_fetched_at < now() - interval '23 days'`)
	// Failed: the location stays, only the attempt is recorded.
	check("gone", `ST_Y(location::geometry) = 50 AND location_fetched_at < now() - interval '29 days' AND status_checked_at `+justNow)
	if n := count(t, pool, `SELECT count(*) FROM spots WHERE place_id IS NULL AND location_fetched_at IS NULL AND status_checked_at IS NULL`); n != 1 {
		t.Error("the unresolved spot was touched")
	}

	t.Run("next run retries only the failed spot", func(t *testing.T) {
		api.calls = nil
		result := mustRun(t, scheduler.New(pool, api, 0))
		if len(api.calls) != 1 || api.calls[0] != "gone" || result.SpotsRefreshed != 0 || result.SpotsClosed != 0 {
			t.Errorf("calls = %v, result = %+v", api.calls, result)
		}
	})
}

func TestRefreshIsLimitedAndRotates(t *testing.T) {
	pool := dbtest.New(t)
	// "bad" was attempted longest ago, so it comes first every time it is due.
	addSpot(t, pool, "bad", 60)
	for i := range 5 {
		addSpot(t, pool, fmt.Sprintf("p%d", i), 30+i)
	}
	api := &fakePlaces{quota: -1, errs: map[string]error{"bad": errors.New("boom")}}
	s := scheduler.New(pool, api, 2)

	seen := map[string]int{}
	for run := range 4 {
		api.calls = nil
		mustRun(t, s)
		if len(api.calls) > 2 {
			t.Fatalf("run %d made %d calls, limit is 2", run, len(api.calls))
		}
		for _, c := range api.calls {
			seen[c]++
		}
	}
	// A spot that always fails must not starve the others.
	for i := range 5 {
		if seen[fmt.Sprintf("p%d", i)] != 1 {
			t.Errorf("calls per spot = %v, want every p* refreshed exactly once", seen)
			break
		}
	}
	if seen["bad"] < 2 {
		t.Errorf("calls per spot = %v, want the failing spot retried", seen)
	}
}

func TestRefreshStopsAtQuota(t *testing.T) {
	pool := dbtest.New(t)
	for i := range 4 {
		addSpot(t, pool, fmt.Sprintf("p%d", i), 30)
	}
	api := &fakePlaces{quota: 1}
	result := mustRun(t, scheduler.New(pool, api, 10))
	if !result.QuotaExceeded || result.SpotsRefreshed != 1 || result.SpotsFailed != 0 || len(api.calls) != 1 {
		t.Errorf("result = %+v, calls = %v", result, api.calls)
	}
	// The spots that were not reached are still due.
	if n := count(t, pool, `SELECT count(*) FROM spots WHERE location_fetched_at < now() - interval '25 days'`); n != 3 {
		t.Errorf("due spots = %d, want 3", n)
	}
}

func TestDeletesExpiredSessions(t *testing.T) {
	pool := dbtest.New(t)
	exec(t, pool, `INSERT INTO users (id, google_sub, email, display_name) VALUES ('11111111-0000-4000-8000-000000000001', 's', 'e', 'n')`)
	exec(t, pool, `
		INSERT INTO sessions (user_id, token_hash, expires_at) VALUES
		('11111111-0000-4000-8000-000000000001', 'expired', now() - interval '1 second'),
		('11111111-0000-4000-8000-000000000001', 'long-expired', now() - interval '40 days'),
		('11111111-0000-4000-8000-000000000001', 'valid', now() + interval '1 hour')`)
	addSpot(t, pool, "due", 30)

	// Without Places API the sessions are still cleaned up.
	result := mustRun(t, scheduler.New(pool, nil, 0))
	if result.SessionsDeleted != 2 || result.SpotsRefreshed != 0 {
		t.Errorf("result = %+v", result)
	}
	if n := count(t, pool, `SELECT count(*) FROM sessions WHERE token_hash = 'valid'`); n != 1 {
		t.Error("the valid session was deleted")
	}
	if n := count(t, pool, `SELECT count(*) FROM users`); n != 1 {
		t.Error("the user was deleted")
	}
}

func TestOnlyOneRunAtATime(t *testing.T) {
	pool := dbtest.New(t)
	addSpot(t, pool, "due", 30)
	api := &fakePlaces{quota: -1}
	s := scheduler.New(pool, api, 0)
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

	result := mustRun(t, s)
	if !result.Skipped || len(api.calls) != 0 {
		t.Errorf("result = %+v, calls = %v; want the run skipped", result, api.calls)
	}

	if _, err := other.Exec(ctx, `SELECT pg_advisory_unlock(741120260001)`); err != nil {
		t.Fatal(err)
	}
	// The lock is released after each run, so runs can follow each other.
	for i := range 2 {
		if result := mustRun(t, s); result.Skipped {
			t.Errorf("run %d was skipped although no other run is in progress", i)
		}
	}
	if len(api.calls) != 1 {
		t.Errorf("calls = %v", api.calls)
	}
}
