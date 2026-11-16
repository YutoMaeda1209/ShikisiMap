// Package scheduler runs the periodic jobs: refreshing the location and the
// business status of the spots, and deleting expired sessions.
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/places"
)

const (
	// DefaultSpotLimit is how many spots one run refreshes at most. It keeps
	// a run well under the daily quota of Place Details.
	DefaultSpotLimit = 20

	// Google allows keeping coordinates for 30 days. Refreshing after 25
	// leaves room for runs that are skipped or cut short by the limit.
	refreshAfter = 25 * 24 * time.Hour

	interval = 24 * time.Hour
	// The first run waits so that a restart loop does not hammer the API.
	startDelay = time.Minute

	// advisoryLockKey identifies the job among PostgreSQL advisory locks.
	// The value is arbitrary but must not change.
	advisoryLockKey = 7411_2026_0001
)

// Scheduler runs the periodic jobs.
type Scheduler struct {
	pool      *pgxpool.Pool
	places    places.API
	spotLimit int
}

// New returns a Scheduler. With a nil api, the spots are not refreshed and
// only the sessions are cleaned up.
func New(pool *pgxpool.Pool, api places.API, spotLimit int) *Scheduler {
	if spotLimit <= 0 {
		spotLimit = DefaultSpotLimit
	}
	return &Scheduler{pool: pool, places: api, spotLimit: spotLimit}
}

// Run runs the jobs once a day until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) {
	timer := time.NewTimer(startDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		result, err := s.RunOnce(ctx)
		switch {
		case err != nil:
			log.Printf("scheduler: %v", err)
		case result.Skipped:
			log.Print("scheduler: skipped, another run is in progress")
		default:
			log.Printf("scheduler: deleted %d expired sessions; refreshed %d spots (%d closed, %d failed)%s",
				result.SessionsDeleted, result.SpotsRefreshed, result.SpotsClosed, result.SpotsFailed, result.note())
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Result summarizes a run.
type Result struct {
	// Skipped is set when another run held the lock, so nothing was done.
	Skipped         bool
	SessionsDeleted int
	SpotsRefreshed  int
	// SpotsClosed counts the spots newly marked as closed.
	SpotsClosed int
	SpotsFailed int
	// QuotaExceeded is set when the run stopped early at the API quota.
	QuotaExceeded bool
}

func (r Result) note() string {
	if r.QuotaExceeded {
		return "; stopped at the API quota"
	}
	return ""
}

// RunOnce runs the jobs now, unless another run is in progress.
func (s *Scheduler) RunOnce(ctx context.Context) (Result, error) {
	// A session-level advisory lock belongs to one connection, so the lock
	// and the unlock must use the same one.
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return Result{}, err
	}
	defer conn.Release()

	var locked bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, int64(advisoryLockKey)).Scan(&locked); err != nil {
		return Result{}, err
	}
	if !locked {
		return Result{Skipped: true}, nil
	}
	defer func() {
		// Unlock even when ctx was cancelled during the run.
		unlockCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if _, err := conn.Exec(unlockCtx, `SELECT pg_advisory_unlock($1)`, int64(advisoryLockKey)); err != nil {
			log.Printf("scheduler: release the lock: %v", err)
		}
	}()

	var result Result
	tag, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= now()`)
	if err != nil {
		return result, fmt.Errorf("delete expired sessions: %w", err)
	}
	result.SessionsDeleted = int(tag.RowsAffected())

	if s.places == nil {
		return result, nil
	}
	if err := s.refreshSpots(ctx, &result); err != nil {
		return result, fmt.Errorf("refresh spots: %w", err)
	}
	return result, nil
}

func (s *Scheduler) refreshSpots(ctx context.Context, result *Result) error {
	// status_checked_at is the time of the last attempt, successful or not.
	// Ordering by it keeps a spot that fails every time from blocking the rest.
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, place_id FROM spots
		WHERE place_id IS NOT NULL
		  AND (location_fetched_at IS NULL OR location_fetched_at < now() - make_interval(secs => $1))
		ORDER BY status_checked_at NULLS FIRST, id
		LIMIT $2`, refreshAfter.Seconds(), s.spotLimit)
	if err != nil {
		return err
	}
	type target struct{ id, placeID string }
	var targets []target
	for rows.Next() {
		var t target
		if err := rows.Scan(&t.id, &t.placeID); err != nil {
			rows.Close()
			return err
		}
		targets = append(targets, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, t := range targets {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		details, err := s.places.Details(ctx, t.placeID)
		if errors.Is(err, places.ErrQuotaExceeded) {
			result.QuotaExceeded = true
			return nil
		}
		if err != nil {
			if errors.Is(err, places.ErrNotFound) {
				log.Printf("scheduler: spot %s: place ID %s is no longer valid; resolve it again by hand", t.id, t.placeID)
			} else {
				log.Printf("scheduler: spot %s: %v", t.id, err)
			}
			result.SpotsFailed++
			if _, err := s.pool.Exec(ctx, `UPDATE spots SET status_checked_at = now() WHERE id = $1`, t.id); err != nil {
				return err
			}
			continue
		}

		// is_closed is only ever set here: a spot closed by a maintainer
		// stays closed even if Google still lists it as operational.
		var newlyClosed bool
		err = s.pool.QueryRow(ctx, `
			UPDATE spots SET
				location = ST_SetSRID(ST_MakePoint($2, $3), 4326)::geography,
				location_fetched_at = now(),
				status_checked_at = now(),
				is_closed = is_closed OR $4
			WHERE id = $1
			RETURNING $4 AND NOT (SELECT is_closed FROM spots WHERE id = $1)`,
			t.id, details.Location.Lng, details.Location.Lat, details.ClosedPermanently(),
		).Scan(&newlyClosed)
		if err != nil {
			return err
		}
		result.SpotsRefreshed++
		if newlyClosed {
			result.SpotsClosed++
		}
	}
	return nil
}
