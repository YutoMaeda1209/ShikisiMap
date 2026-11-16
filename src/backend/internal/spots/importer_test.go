package spots_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/db/dbtest"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/places"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/spots"
)

// fakePlaces answers searches from a table keyed by the feature name.
type fakePlaces struct {
	results map[string][]places.Place
	// quota is the number of searches allowed; negative means unlimited.
	quota    int
	searches []string
}

func (f *fakePlaces) SearchText(_ context.Context, query string) ([]places.Place, error) {
	if f.quota == 0 {
		return nil, places.ErrQuotaExceeded
	}
	f.quota--
	f.searches = append(f.searches, query)
	name, _, _ := strings.Cut(query, " ")
	return f.results[name], nil
}

func (f *fakePlaces) Details(context.Context, string) (places.Details, error) {
	return places.Details{}, errors.New("unexpected Details call")
}

func place(id string) places.Place {
	return places.Place{ID: id, Name: "g-" + id, Address: "g-address", Location: places.LatLng{Lat: 10, Lng: 20}}
}

func featureID(n int) string {
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", n)
}

func feature(n int, name string) spots.Feature {
	return spots.Feature{
		ID: featureID(n), Name: name, Address: "住所" + name, YouTubeID: "video-" + name, Timestamp: n,
		Location: places.LatLng{Lat: float64(n), Lng: float64(n) + 100},
	}
}

func ptr(s string) *string { return &s }

// dump renders the spots and appearances so that two states can be compared.
func dump(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT s.id::text, COALESCE(s.place_id, '-'), s.name, s.address,
		       ST_Y(s.location::geometry), ST_X(s.location::geometry),
		       s.location_fetched_at IS NOT NULL, s.is_closed,
		       COALESCE((SELECT string_agg(a.youtube_id || '@' || a."timestamp", ',' ORDER BY a.youtube_id, a."timestamp")
		                 FROM appearances a WHERE a.spot_id = s.id), '')
		FROM spots s ORDER BY s.id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var b strings.Builder
	for rows.Next() {
		var (
			id, placeID, name, address, appearances string
			lat, lng                                float64
			fetched, closed                         bool
		)
		if err := rows.Scan(&id, &placeID, &name, &address, &lat, &lng, &fetched, &closed, &appearances); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&b, "%s place=%s name=%s addr=%s loc=%v,%v fetched=%v closed=%v [%s]\n",
			id[len(id)-2:], placeID, name, address, lat, lng, fetched, closed, appearances)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func mustImport(t *testing.T, pool *pgxpool.Pool, api places.API, features []spots.Feature, opts spots.ImportOptions) *spots.ImportReport {
	t.Helper()
	report, err := spots.Import(context.Background(), pool, api, features, opts)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	return report
}

func TestImport(t *testing.T) {
	pool := dbtest.New(t)
	closed := feature(3, "C")
	closed.IsClosed = true
	features := []spots.Feature{feature(1, "A"), feature(2, "B"), closed, feature(4, "D"), feature(5, "E")}
	api := &fakePlaces{quota: -1, results: map[string][]places.Place{
		"A": {place("pA")},
		"B": {place("pB1"), place("pB2")}, // ambiguous
		"C": {place("pA")},                // same place as A
		"D": {},                           // not found
		"E": {place("pE")},
	}}

	t.Run("dry run writes nothing", func(t *testing.T) {
		report := mustImport(t, pool, api, features, spots.ImportOptions{DryRun: true})
		if got := dump(t, pool); got != "" {
			t.Errorf("database after the dry run:\n%s", got)
		}
		if len(report.Resolved) != 3 || len(report.Unresolved) != 2 || len(report.Merges) != 1 {
			t.Errorf("report = %+v", report)
		}
		if report.Spots != 4 || report.Appearances != 5 {
			t.Errorf("spots = %d, appearances = %d; want 4, 5", report.Spots, report.Appearances)
		}
		if u := report.Unresolved[0]; u.Feature.Name != "B" || len(u.Candidates) != 2 {
			t.Errorf("unresolved[0] = %+v", u)
		}
		m := report.Merges[0]
		if m.PlaceID != "pA" || m.Kept.Name != "A" || len(m.Merged) != 1 || m.Merged[0].Name != "C" {
			t.Errorf("merge = %+v", m)
		}

		var out bytes.Buffer
		report.Write(&out)
		for _, want := range []string{"DRY RUN", "5 features: 3 resolved (3 by search", "2 unresolved", "candidate pB1", "kept   " + featureID(1), "merged " + featureID(3)} {
			if !strings.Contains(out.String(), want) {
				t.Errorf("report lacks %q:\n%s", want, out.String())
			}
		}
	})

	want := "" +
		// A: location from Google, closed because the merged C is closed.
		"01 place=pA name=A addr=住所A loc=10,20 fetched=true closed=true [video-A@1,video-C@3]\n" +
		// B and D: unresolved, at the data.json coordinates.
		"02 place=- name=B addr=住所B loc=2,102 fetched=false closed=false [video-B@2]\n" +
		"04 place=- name=D addr=住所D loc=4,104 fetched=false closed=false [video-D@4]\n" +
		"05 place=pE name=E addr=住所E loc=10,20 fetched=true closed=false [video-E@5]\n"

	var placeMap spots.PlaceMap
	t.Run("import", func(t *testing.T) {
		report := mustImport(t, pool, api, features, spots.ImportOptions{})
		if got := dump(t, pool); got != want {
			t.Errorf("database:\n%s\nwant:\n%s", got, want)
		}
		list, err := spots.List(context.Background(), pool)
		if err != nil {
			t.Fatal(err)
		}
		var order []string
		for _, s := range list {
			order = append(order, s.Name)
		}
		if strings.Join(order, "") != "ABDE" {
			t.Errorf("listed as %v, want the order of data.json", order)
		}
		placeMap = report.PlaceMap
		if len(placeMap) != 5 || placeMap[featureID(2)] != nil || *placeMap[featureID(3)] != "pA" {
			t.Errorf("place map = %v", placeMap)
		}
	})

	t.Run("second run searches only the unresolved and changes nothing", func(t *testing.T) {
		api.searches = nil
		report := mustImport(t, pool, api, features, spots.ImportOptions{})
		if got := dump(t, pool); got != want {
			t.Errorf("database:\n%s\nwant:\n%s", got, want)
		}
		// A and E are known from the database. C was merged away, so it is
		// searched again along with the unresolved B and D.
		if len(api.searches) != 3 {
			t.Errorf("searches = %v", api.searches)
		}
		if len(report.Merges) != 1 {
			t.Errorf("merges = %+v", report.Merges)
		}
	})

	t.Run("run with the saved place map searches nothing", func(t *testing.T) {
		none := &fakePlaces{quota: 0}
		mustImport(t, pool, none, features, spots.ImportOptions{Overrides: placeMap})
		if got := dump(t, pool); got != want {
			t.Errorf("database:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("overrides split a wrong merge and resolve the rest", func(t *testing.T) {
		overrides := spots.PlaceMap{
			featureID(2): ptr("pB2"),
			featureID(3): ptr("pC"),
			featureID(4): nil,
		}
		none := &fakePlaces{quota: 0}
		report := mustImport(t, pool, none, features, spots.ImportOptions{Overrides: overrides})
		got := dump(t, pool)
		wantSplit := "" +
			// A keeps the Google location and, as is_closed is never undone, stays closed.
			"01 place=pA name=A addr=住所A loc=10,20 fetched=true closed=true [video-A@1]\n" +
			// B and C got their place from the overrides; the location stays provisional.
			"02 place=pB2 name=B addr=住所B loc=2,102 fetched=false closed=false [video-B@2]\n" +
			"03 place=pC name=C addr=住所C loc=3,103 fetched=false closed=true [video-C@3]\n" +
			"04 place=- name=D addr=住所D loc=4,104 fetched=false closed=false [video-D@4]\n" +
			"05 place=pE name=E addr=住所E loc=10,20 fetched=true closed=false [video-E@5]\n"
		if got != wantSplit {
			t.Errorf("database:\n%s\nwant:\n%s", got, wantSplit)
		}
		if len(report.Merges) != 0 || len(report.Unresolved) != 1 {
			t.Errorf("report = %+v", report)
		}
	})

	t.Run("overrides merge existing spots and move their visits", func(t *testing.T) {
		ctx := context.Background()
		if _, err := pool.Exec(ctx, `INSERT INTO users (id, google_sub, email, display_name) VALUES ('11111111-0000-4000-8000-000000000001', 'u', 'u@example.com', 'u')`); err != nil {
			t.Fatal(err)
		}
		// The same instant on both spots: only one visit can remain.
		if _, err := pool.Exec(ctx, `
			INSERT INTO visits (user_id, spot_id, visited_at) VALUES
			('11111111-0000-4000-8000-000000000001', $1, '2026-01-01T00:00:00Z'),
			('11111111-0000-4000-8000-000000000001', $2, '2026-01-01T00:00:00Z'),
			('11111111-0000-4000-8000-000000000001', $2, '2026-02-02T00:00:00Z')`,
			featureID(1), featureID(5)); err != nil {
			t.Fatal(err)
		}

		// E moves to the place of A. B and C swap places with each other.
		overrides := spots.PlaceMap{
			featureID(2): ptr("pC"),
			featureID(3): ptr("pB2"),
			featureID(4): nil,
			featureID(5): ptr("pA"),
		}
		mustImport(t, pool, &fakePlaces{quota: 0}, features, spots.ImportOptions{Overrides: overrides})
		wantMerged := "" +
			"01 place=pA name=A addr=住所A loc=10,20 fetched=true closed=true [video-A@1,video-E@5]\n" +
			"02 place=pC name=B addr=住所B loc=2,102 fetched=false closed=false [video-B@2]\n" +
			"03 place=pB2 name=C addr=住所C loc=3,103 fetched=false closed=true [video-C@3]\n" +
			"04 place=- name=D addr=住所D loc=4,104 fetched=false closed=false [video-D@4]\n"
		if got := dump(t, pool); got != wantMerged {
			t.Errorf("database:\n%s\nwant:\n%s", got, wantMerged)
		}
		var visits int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM visits WHERE spot_id = $1`, featureID(1)).Scan(&visits); err != nil {
			t.Fatal(err)
		}
		if visits != 2 {
			t.Errorf("visits on the kept spot = %d, want 2", visits)
		}
	})
}

func TestImportStopsAtQuota(t *testing.T) {
	pool := dbtest.New(t)
	features := []spots.Feature{feature(1, "A"), feature(2, "B"), feature(3, "C")}
	api := &fakePlaces{quota: 1, results: map[string][]places.Place{"A": {place("pA")}, "B": {place("pB")}, "C": {place("pC")}}}

	report, err := spots.Import(context.Background(), pool, api, features, spots.ImportOptions{})
	if !errors.Is(err, spots.ErrSearchQuota) {
		t.Fatalf("err = %v, want ErrSearchQuota", err)
	}
	if got := dump(t, pool); got != "" {
		t.Errorf("database after the failed run:\n%s", got)
	}
	if len(report.NotSearched) != 2 || len(report.PlaceMap) != 1 || *report.PlaceMap[featureID(1)] != "pA" {
		t.Errorf("report = %+v", report)
	}

	// The saved progress lets the next run search only the rest.
	api.quota, api.searches = -1, nil
	mustImport(t, pool, api, features, spots.ImportOptions{Overrides: report.PlaceMap})
	if len(api.searches) != 2 {
		t.Errorf("searches = %v, want only B and C", api.searches)
	}
	if got := dump(t, pool); strings.Count(got, "\n") != 3 || !strings.Contains(got, "place=pA") {
		t.Errorf("database:\n%s", got)
	}
}

func TestParseGeoJSON(t *testing.T) {
	const valid = `{"type": "FeatureCollection", "features": [{
		"type": "Feature", "id": "AF272CED-d057-42dd-9763-fa5562e6d361",
		"properties": {"name": "店", "address": "住所", "isClosed": true, "youtubeId": "abc", "timestamp": 89},
		"geometry": {"type": "Point", "coordinates": [139.757212, 35.664339]}}]}`
	got, err := spots.ParseGeoJSON(strings.NewReader(valid))
	if err != nil {
		t.Fatal(err)
	}
	want := spots.Feature{
		ID: "af272ced-d057-42dd-9763-fa5562e6d361", Name: "店", Address: "住所", IsClosed: true,
		YouTubeID: "abc", Timestamp: 89, Location: places.LatLng{Lat: 35.664339, Lng: 139.757212},
	}
	if len(got) != 1 || got[0] != want {
		t.Errorf("features = %+v, want %+v", got, want)
	}

	invalid := map[string]string{
		"no id":        strings.Replace(valid, `"id": "AF272CED-d057-42dd-9763-fa5562e6d361",`, "", 1),
		"no name":      strings.Replace(valid, `"name": "店", `, "", 1),
		"bad geometry": strings.Replace(valid, `[139.757212, 35.664339]`, `[139.757212]`, 1),
		"negative":     strings.Replace(valid, `89`, `-1`, 1),
		"duplicate id": strings.Replace(valid, `"features": [`, `"features": [`+valid[strings.Index(valid, `[{`)+1:strings.LastIndex(valid, `]}`)]+`,`, 1),
		"not json":     "x",
	}
	for name, doc := range invalid {
		if _, err := spots.ParseGeoJSON(strings.NewReader(doc)); err == nil {
			t.Errorf("%s: err = nil", name)
		}
	}
}

func TestParsePlaceMap(t *testing.T) {
	got, err := spots.ParsePlaceMap(strings.NewReader(`{
		"AF272CED-d057-42dd-9763-fa5562e6d361": "ChIJ1",
		"00000000-0000-4000-8000-000000000001": null,
		"00000000-0000-4000-8000-000000000002": ""}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || *got["af272ced-d057-42dd-9763-fa5562e6d361"] != "ChIJ1" ||
		got["00000000-0000-4000-8000-000000000001"] != nil || got["00000000-0000-4000-8000-000000000002"] != nil {
		t.Errorf("place map = %v", got)
	}
	if _, err := spots.ParsePlaceMap(strings.NewReader(`{"not-a-uuid": "ChIJ1"}`)); err == nil {
		t.Error("err = nil for a key that is not a UUID")
	}
}

func TestParseRealData(t *testing.T) {
	f, err := openDataJSON()
	if err != nil {
		t.Skip(err)
	}
	defer f.Close()
	features, err := spots.ParseGeoJSON(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(features) == 0 {
		t.Error("no features")
	}
}
