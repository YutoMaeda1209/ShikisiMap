package spots

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/httpx"
)

// Feature is a spot of the legacy data.json (a GeoJSON feature).
type Feature struct {
	ID        string
	Name      string
	Address   string
	IsClosed  bool
	YouTubeID string
	Timestamp int
	Location  LatLng
}

// ParseGeoJSON reads the legacy data.json.
func ParseGeoJSON(r io.Reader) ([]Feature, error) {
	var doc struct {
		Features []struct {
			ID         string `json:"id"`
			Properties struct {
				Name      string `json:"name"`
				Address   string `json:"address"`
				IsClosed  bool   `json:"isClosed"`
				YouTubeID string `json:"youtubeId"`
				Timestamp int    `json:"timestamp"`
			} `json:"properties"`
			Geometry struct {
				Type        string    `json:"type"`
				Coordinates []float64 `json:"coordinates"`
			} `json:"geometry"`
		} `json:"features"`
	}
	if err := json.NewDecoder(r).Decode(&doc); err != nil {
		return nil, fmt.Errorf("parse GeoJSON: %w", err)
	}

	seen := make(map[string]bool, len(doc.Features))
	features := make([]Feature, 0, len(doc.Features))
	for i, f := range doc.Features {
		p := f.Properties
		id := strings.ToLower(f.ID)
		switch {
		case !httpx.IsUUID(id):
			return nil, fmt.Errorf("feature %d (%q): id %q is not a UUID", i, p.Name, f.ID)
		case seen[id]:
			return nil, fmt.Errorf("feature %d (%q): id %s is used twice", i, p.Name, id)
		case p.Name == "" || p.Address == "" || p.YouTubeID == "":
			return nil, fmt.Errorf("feature %d (%s): name, address and youtubeId are required", i, id)
		case p.Timestamp < 0:
			return nil, fmt.Errorf("feature %d (%s): timestamp is negative", i, id)
		case f.Geometry.Type != "Point" || len(f.Geometry.Coordinates) != 2:
			return nil, fmt.Errorf("feature %d (%s): geometry must be a Point with [lng, lat]", i, id)
		}
		// GeoJSON orders coordinates as [longitude, latitude].
		location := LatLng{Lat: f.Geometry.Coordinates[1], Lng: f.Geometry.Coordinates[0]}
		if !location.Valid() {
			return nil, fmt.Errorf("feature %d (%s): coordinates are out of range", i, id)
		}
		seen[id] = true
		features = append(features, Feature{
			ID: id, Name: p.Name, Address: p.Address, IsClosed: p.IsClosed,
			YouTubeID: p.YouTubeID, Timestamp: p.Timestamp, Location: location,
		})
	}
	return features, nil
}

// ImportReport describes what Import did, or would do in a dry run.
type ImportReport struct {
	DryRun bool
	Total  int
	// Added are the features that were not in the database yet.
	Added []Feature
	// Existing are the features whose spot was already there. They are left
	// as they are, so that the edits made since the first import are kept.
	Existing []Feature
}

// Import registers the features of the legacy data.json as spots: each one
// becomes a spot with the feature's id, and one appearance.
//
// A spot that already exists is not changed, so running it again changes
// nothing. The coordinates and the closure are taken from data.json as they
// are; after the import they are maintained by hand.
func Import(ctx context.Context, pool *pgxpool.Pool, features []Feature, dryRun bool) (*ImportReport, error) {
	ids := make([]string, len(features))
	for i, f := range features {
		ids[i] = f.ID
	}
	rows, err := pool.Query(ctx, `SELECT id::text FROM spots WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	existing := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		existing[id] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	report := &ImportReport{DryRun: dryRun, Total: len(features)}
	for _, f := range features {
		if existing[f.ID] {
			report.Existing = append(report.Existing, f)
		} else {
			report.Added = append(report.Added, f)
		}
	}
	if dryRun || len(report.Added) == 0 {
		return report, nil
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	for _, f := range report.Added {
		// clock_timestamp() advances within the transaction, so the spots are
		// listed in the order of data.json.
		if _, err := tx.Exec(ctx, `
			INSERT INTO spots (id, name, address, location, is_closed, created_at)
			VALUES ($1, $2, $3, ST_SetSRID(ST_MakePoint($4, $5), 4326)::geography, $6, clock_timestamp())`,
			f.ID, f.Name, f.Address, f.Location.Lng, f.Location.Lat, f.IsClosed); err != nil {
			return nil, fmt.Errorf("insert spot %s: %w", f.ID, err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO appearances (spot_id, youtube_id, "timestamp") VALUES ($1, $2, $3)`,
			f.ID, f.YouTubeID, f.Timestamp); err != nil {
			return nil, fmt.Errorf("insert appearance of %s: %w", f.ID, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return report, nil
}

// Write prints the report.
func (r *ImportReport) Write(w io.Writer) {
	if r.DryRun {
		fmt.Fprintln(w, "DRY RUN: nothing was written.")
	}
	verb := "added"
	if r.DryRun {
		verb = "to add"
	}
	closed := 0
	for _, f := range r.Added {
		if f.IsClosed {
			closed++
		}
	}
	fmt.Fprintf(w, "%d features: %d %s (%d of them closed), %d already in the database and left unchanged\n",
		r.Total, len(r.Added), verb, closed, len(r.Existing))
}
