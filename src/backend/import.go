package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/config"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/db"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/places"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/spots"
)

const importUsage = `Usage: shikisimap-backend import-spots [flags] <data.json>

Registers the spots of the legacy data.json in the database, resolving the
place ID of each with Places API. Running it again changes nothing.

Review the result with --dry-run first. To correct it, save the place map
with --write-map, edit it and pass it back with --overrides. A spot whose id
is in the overrides is not searched, so a run with a complete map calls no API.

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
	dryRun := fs.Bool("dry-run", false, "resolve the places and print the result without writing")
	overridesPath := fs.String("overrides", "", "JSON `file` mapping data.json ids to place IDs (null leaves a spot unresolved)")
	writeMapPath := fs.String("write-map", "", "write the resulting id to place ID map to `file`")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("import-spots takes exactly one data.json")
	}

	features, err := readFile(fs.Arg(0), spots.ParseGeoJSON)
	if err != nil {
		return err
	}
	opts := spots.ImportOptions{DryRun: *dryRun}
	if *overridesPath != "" {
		if opts.Overrides, err = readFile(*overridesPath, spots.ParsePlaceMap); err != nil {
			return err
		}
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

	report, importErr := spots.Import(ctx, pool, places.NewClient(cfg.GooglePlacesAPIKey), features, opts)
	if report != nil {
		report.Write(stdout)
		if *writeMapPath != "" {
			if err := writePlaceMap(*writeMapPath, report.PlaceMap); err != nil {
				return errors.Join(importErr, err)
			}
			fmt.Fprintf(stdout, "\nWrote the place map to %s\n", *writeMapPath)
		}
	}
	return importErr
}

func readFile[T any](path string, parse func(io.Reader) (T, error)) (T, error) {
	f, err := os.Open(path)
	if err != nil {
		var zero T
		return zero, err
	}
	defer f.Close()
	v, err := parse(f)
	if err != nil {
		return v, fmt.Errorf("%s: %w", path, err)
	}
	return v, nil
}

func writePlaceMap(path string, m spots.PlaceMap) error {
	// encoding/json sorts map keys, so the file is stable between runs.
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
