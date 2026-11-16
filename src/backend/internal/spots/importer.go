package spots

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/httpx"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/places"
)

// Feature is a spot of the legacy data.json (a GeoJSON feature).
type Feature struct {
	ID        string
	Name      string
	Address   string
	IsClosed  bool
	YouTubeID string
	Timestamp int
	Location  places.LatLng
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
		seen[id] = true
		features = append(features, Feature{
			ID: id, Name: p.Name, Address: p.Address, IsClosed: p.IsClosed,
			YouTubeID: p.YouTubeID, Timestamp: p.Timestamp,
			// GeoJSON orders coordinates as [longitude, latitude].
			Location: places.LatLng{Lat: f.Geometry.Coordinates[1], Lng: f.Geometry.Coordinates[0]},
		})
	}
	return features, nil
}

// PlaceMap maps the id of a data.json feature to its place ID. A nil value
// means "leave the place unresolved without searching".
type PlaceMap map[string]*string

// ParsePlaceMap reads a PlaceMap from JSON such as {"<id>": "<place id>", "<id>": null}.
func ParsePlaceMap(r io.Reader) (PlaceMap, error) {
	var raw map[string]*string
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return nil, fmt.Errorf("parse place map: %w", err)
	}
	m := make(PlaceMap, len(raw))
	for id, placeID := range raw {
		if !httpx.IsUUID(id) {
			return nil, fmt.Errorf("place map: key %q is not a UUID", id)
		}
		if placeID != nil && *placeID == "" {
			placeID = nil
		}
		m[strings.ToLower(id)] = placeID
	}
	return m, nil
}

// Where a place ID came from.
const (
	SourceSearch   = "search"
	SourceOverride = "override"
	SourceDatabase = "database"
)

// ImportOptions controls Import.
type ImportOptions struct {
	// DryRun resolves the places and builds the report without writing.
	DryRun bool
	// Overrides gives the place ID of some features, which are then not searched.
	Overrides PlaceMap
}

// Resolved is a feature whose place ID is known.
type Resolved struct {
	Feature Feature
	PlaceID string
	Source  string
}

// Unresolved is a feature that is registered without a place ID.
type Unresolved struct {
	Feature Feature
	Reason  string
	// Candidates are the search results when there were several.
	Candidates []places.Place
}

// Merge is a group of features that resolved to the same place. They become
// one spot, which keeps the id of the first feature, with one appearance each.
type Merge struct {
	PlaceID string
	Kept    Feature
	Merged  []Feature
}

// ImportReport describes what Import did, or would do in a dry run.
type ImportReport struct {
	DryRun     bool
	Total      int
	Resolved   []Resolved
	Unresolved []Unresolved
	Merges     []Merge
	// NotSearched are the features left out because the search quota ran
	// out. When it is not empty, nothing is written.
	NotSearched []Feature
	// PlaceMap holds the outcome for every feature that was settled. Passing
	// it back as ImportOptions.Overrides repeats the import without searching.
	PlaceMap PlaceMap
	// Spots and Appearances count the rows the import results in.
	Spots, Appearances int
}

// ErrSearchQuota is returned by a non-dry run that could not search every
// feature. The report is still returned so that the progress can be saved.
var ErrSearchQuota = errors.New("the search quota ran out before every spot was resolved; nothing was written")

// Import registers the features of the legacy data.json as spots.
//
// Each feature becomes a spot with the feature's id and one appearance.
// Features that resolve to the same place ID are merged into one spot.
// Running it again with the same input leaves the database unchanged.
func Import(ctx context.Context, pool *pgxpool.Pool, api places.API, features []Feature, opts ImportOptions) (*ImportReport, error) {
	ids := make([]string, len(features))
	for i, f := range features {
		ids[i] = f.ID
	}
	existing, err := existingPlaceIDs(ctx, pool, ids)
	if err != nil {
		return nil, err
	}

	report := &ImportReport{DryRun: opts.DryRun, Total: len(features), PlaceMap: PlaceMap{}}
	type entry struct {
		feature Feature
		placeID *string
		// location is where the spot is put when its location is (re)set.
		location   places.LatLng
		fromGoogle bool
	}
	var entries []entry
	quotaExceeded := false

	for _, f := range features {
		e := entry{feature: f, location: f.Location}
		override, overridden := opts.Overrides[f.ID]
		known, inDB := existing[f.ID]

		switch {
		case overridden && override != nil:
			e.placeID = override
			report.Resolved = append(report.Resolved, Resolved{f, *override, SourceOverride})
		case overridden:
			report.Unresolved = append(report.Unresolved, Unresolved{Feature: f, Reason: "left unresolved by the overrides"})
		case inDB && known != nil:
			e.placeID = known
			report.Resolved = append(report.Resolved, Resolved{f, *known, SourceDatabase})
		case quotaExceeded:
			report.NotSearched = append(report.NotSearched, f)
			continue
		default:
			found, err := api.SearchText(ctx, f.Name+" "+f.Address)
			if errors.Is(err, places.ErrQuotaExceeded) {
				quotaExceeded = true
				report.NotSearched = append(report.NotSearched, f)
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("search %q: %w", f.Name, err)
			}
			switch len(found) {
			case 1:
				e.placeID = &found[0].ID
				e.location = found[0].Location
				e.fromGoogle = true
				report.Resolved = append(report.Resolved, Resolved{f, found[0].ID, SourceSearch})
			case 0:
				report.Unresolved = append(report.Unresolved, Unresolved{Feature: f, Reason: "no place found"})
			default:
				report.Unresolved = append(report.Unresolved, Unresolved{
					Feature: f, Reason: fmt.Sprintf("%d candidates", len(found)), Candidates: found,
				})
			}
		}
		report.PlaceMap[f.ID] = e.placeID
		entries = append(entries, e)
	}

	// Group the features by place. Unresolved ones each form their own group.
	type group struct {
		kept   entry
		merged []entry
	}
	var groups []*group
	byPlace := map[string]*group{}
	for _, e := range entries {
		if e.placeID != nil {
			if g, ok := byPlace[*e.placeID]; ok {
				g.merged = append(g.merged, e)
				g.kept.feature.IsClosed = g.kept.feature.IsClosed || e.feature.IsClosed
				continue
			}
		}
		g := &group{kept: e}
		groups = append(groups, g)
		if e.placeID != nil {
			byPlace[*e.placeID] = g
		}
	}
	for _, g := range groups {
		if len(g.merged) == 0 {
			continue
		}
		m := Merge{PlaceID: *g.kept.placeID, Kept: g.kept.feature}
		for _, e := range g.merged {
			m.Merged = append(m.Merged, e.feature)
		}
		report.Merges = append(report.Merges, m)
	}
	report.Spots = len(groups)
	report.Appearances = len(entries)

	if opts.DryRun {
		return report, nil
	}
	if len(report.NotSearched) > 0 {
		return report, ErrSearchQuota
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// place_id is unique. Clearing it first lets the places move between the
	// imported spots in any order.
	if _, err := tx.Exec(ctx, `UPDATE spots SET place_id = NULL WHERE id = ANY($1)`, ids); err != nil {
		return nil, err
	}

	now := time.Now()
	for _, g := range groups {
		e := g.kept
		oldPlaceID, existed := existing[e.feature.ID]
		// A location fetched from Google earlier stays as long as the spot
		// still points at the same place.
		keepLocation := existed && !e.fromGoogle && equalPtr(oldPlaceID, e.placeID)
		var fetchedAt *time.Time
		if e.fromGoogle {
			fetchedAt = &now
		}
		_, err := tx.Exec(ctx, `
			-- clock_timestamp() advances within the transaction, so the spots
			-- are listed in the order of data.json.
			INSERT INTO spots (id, place_id, name, address, location, location_fetched_at, is_closed, created_at)
			VALUES ($1, $2, $3, $4, ST_SetSRID(ST_MakePoint($5, $6), 4326)::geography, $7, $8, clock_timestamp())
			ON CONFLICT (id) DO UPDATE SET
				place_id = EXCLUDED.place_id,
				name = EXCLUDED.name,
				address = EXCLUDED.address,
				location = CASE WHEN $9 THEN spots.location ELSE EXCLUDED.location END,
				location_fetched_at = CASE WHEN $9 THEN spots.location_fetched_at ELSE EXCLUDED.location_fetched_at END,
				-- A closure found by the scheduled job is not undone by an older data.json.
				is_closed = spots.is_closed OR EXCLUDED.is_closed`,
			e.feature.ID, e.placeID, e.feature.Name, e.feature.Address,
			e.location.Lng, e.location.Lat, fetchedAt, e.feature.IsClosed, keepLocation)
		if err != nil {
			return nil, fmt.Errorf("upsert spot %s: %w", e.feature.ID, err)
		}
	}

	for _, g := range groups {
		// A spot merged into another one may exist from an earlier run.
		for _, e := range g.merged {
			if err := mergeSpot(ctx, tx, e.feature.ID, g.kept.feature.ID); err != nil {
				return nil, fmt.Errorf("merge spot %s into %s: %w", e.feature.ID, g.kept.feature.ID, err)
			}
		}
		for _, e := range append([]entry{g.kept}, g.merged...) {
			f := e.feature
			// The appearance may sit on another imported spot from an earlier run.
			if _, err := tx.Exec(ctx, `
				DELETE FROM appearances
				WHERE youtube_id = $1 AND "timestamp" = $2 AND spot_id <> $3 AND spot_id = ANY($4)`,
				f.YouTubeID, f.Timestamp, g.kept.feature.ID, ids); err != nil {
				return nil, err
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO appearances (spot_id, youtube_id, "timestamp") VALUES ($1, $2, $3)
				ON CONFLICT DO NOTHING`,
				g.kept.feature.ID, f.YouTubeID, f.Timestamp); err != nil {
				return nil, fmt.Errorf("insert appearance of %s: %w", f.ID, err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return report, nil
}

func equalPtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// existingPlaceIDs returns the place ID (nil when unresolved) of those of the
// spots that exist.
func existingPlaceIDs(ctx context.Context, pool *pgxpool.Pool, ids []string) (map[string]*string, error) {
	rows, err := pool.Query(ctx, `SELECT id::text, place_id FROM spots WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]*string{}
	for rows.Next() {
		var id string
		var placeID *string
		if err := rows.Scan(&id, &placeID); err != nil {
			return nil, err
		}
		result[id] = placeID
	}
	return result, rows.Err()
}

// mergeSpot deletes the spot from, moving its visits to the spot into.
func mergeSpot(ctx context.Context, tx pgx.Tx, from, into string) error {
	// A visit that the user also recorded on the target at the same time
	// would break the unique constraint; that one is dropped.
	if _, err := tx.Exec(ctx, `
		UPDATE visits v SET spot_id = $2
		WHERE v.spot_id = $1 AND NOT EXISTS (
			SELECT 1 FROM visits o
			WHERE o.user_id = v.user_id AND o.spot_id = $2 AND o.visited_at = v.visited_at)`,
		from, into); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM visits WHERE spot_id = $1`, from); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `DELETE FROM spots WHERE id = $1`, from)
	return err
}

// Write prints the report for the maintainer who reviews the import.
func (r *ImportReport) Write(w io.Writer) {
	if r.DryRun {
		fmt.Fprintln(w, "DRY RUN: nothing was written.")
	}
	bySource := map[string]int{}
	for _, res := range r.Resolved {
		bySource[res.Source]++
	}
	fmt.Fprintf(w, "%d features: %d resolved (%d by search, %d by overrides, %d already in the database), %d unresolved, %d not searched\n",
		r.Total, len(r.Resolved), bySource[SourceSearch], bySource[SourceOverride], bySource[SourceDatabase],
		len(r.Unresolved), len(r.NotSearched))
	fmt.Fprintf(w, "Result: %d spots, %d appearances\n", r.Spots, r.Appearances)

	if len(r.Unresolved) > 0 {
		fmt.Fprintf(w, "\nUnresolved (registered without a place ID, at the data.json coordinates):\n")
		for _, u := range r.Unresolved {
			fmt.Fprintf(w, "  %s  %s (%s): %s\n", u.Feature.ID, u.Feature.Name, u.Feature.Address, u.Reason)
			for _, c := range u.Candidates {
				fmt.Fprintf(w, "      candidate %s  %s (%s)\n", c.ID, c.Name, c.Address)
			}
		}
	}
	if len(r.Merges) > 0 {
		fmt.Fprintf(w, "\nMerged (same place ID; check that they really are the same place):\n")
		for _, m := range r.Merges {
			fmt.Fprintf(w, "  %s\n", m.PlaceID)
			fmt.Fprintf(w, "      kept   %s  %s (%s)\n", m.Kept.ID, m.Kept.Name, m.Kept.Address)
			for _, f := range m.Merged {
				fmt.Fprintf(w, "      merged %s  %s (%s)\n", f.ID, f.Name, f.Address)
			}
		}
	}
	if len(r.NotSearched) > 0 {
		fmt.Fprintf(w, "\nNot searched (the search quota ran out):\n")
		for _, f := range r.NotSearched {
			fmt.Fprintf(w, "  %s  %s\n", f.ID, f.Name)
		}
	}
}
