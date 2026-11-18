package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/config"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/db"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/spots"
)

const importUsage = `Usage: shikisimap-backend import-spots [flags] <data.json>

Registers the spots of the legacy data.json in the database, each with its
id, coordinates and closure as they are in the file. Spots that are already
in the database are left unchanged, so running it again changes nothing.

Flags:
`

// runImport implements the import-spots subcommand.
func runImport(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("import-spots", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprint(stderr, importUsage)
		fs.PrintDefaults()
	}
	dryRun := fs.Bool("dry-run", false, "print what would be imported without writing")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("import-spots takes exactly one data.json")
	}

	f, err := os.Open(fs.Arg(0))
	if err != nil {
		return err
	}
	features, err := spots.ParseGeoJSON(f)
	f.Close()
	if err != nil {
		return fmt.Errorf("%s: %w", fs.Arg(0), err)
	}

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

	report, err := spots.Import(ctx, pool, features, *dryRun)
	if err != nil {
		return err
	}
	report.Write(stdout)
	return nil
}
