// Package scheduler runs the periodic job that deletes expired sessions.
package scheduler

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	interval = 24 * time.Hour
	// The first run waits a little so that it does not slow down the start.
	startDelay = time.Minute

	// advisoryLockKey identifies the job among PostgreSQL advisory locks.
	// The value is arbitrary but must not change.
	advisoryLockKey = 7411_2026_0001
)

// Scheduler runs the periodic job.
type Scheduler struct {
	pool *pgxpool.Pool
}

// New returns a Scheduler.
func New(pool *pgxpool.Pool) *Scheduler {
	return &Scheduler{pool: pool}
}

// Run runs the job once a day until ctx is cancelled.
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
			log.Printf("scheduler: deleted %d expired sessions", result.SessionsDeleted)
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
}

// RunOnce runs the job now, unless another run is in progress.
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

	tag, err := conn.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= now()`)
	if err != nil {
		return Result{}, fmt.Errorf("delete expired sessions: %w", err)
	}
	return Result{SessionsDeleted: int(tag.RowsAffected())}, nil
}
