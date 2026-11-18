package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/config"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/db"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/scheduler"
)

const shutdownTimeout = 10 * time.Second

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var err error
	switch {
	case len(os.Args) == 1:
		err = serve(ctx)
	case os.Args[1] == "import-spots":
		err = runImport(ctx, os.Args[2:], os.Stdout, os.Stderr)
	default:
		err = fmt.Errorf("unknown command %q; run without arguments to start the server, or with import-spots", os.Args[1])
	}
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		stop()
		log.Fatal(err)
	}
}

// serve runs the HTTP server until ctx is cancelled.
func serve(parent context.Context) error {
	ctx, stop := context.WithCancel(parent)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		return err
	}

	jobs := scheduler.New(pool)
	jobsDone := make(chan struct{})
	go func() {
		defer close(jobsDone)
		jobs.Run(ctx)
	}()
	// The pool must outlive the job, which stops when ctx is cancelled.
	defer func() {
		stop()
		<-jobsDone
	}()

	router, err := newRouter(cfg, pool)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()
	log.Printf("listening on %s", cfg.ListenAddr)

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}

	// Let the requests in progress finish before the pool closes.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
