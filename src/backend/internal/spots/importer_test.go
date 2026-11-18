package spots_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/db/dbtest"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/spots"
)

func featureID(n int) string {
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", n)
}

func feature(n int, name string) spots.Feature {
	return spots.Feature{
		ID: featureID(n), Name: name, Address: "住所" + name, YouTubeID: "video-" + name, Timestamp: n,
		Location: spots.LatLng{Lat: float64(n), Lng: float64(n) + 100},
	}
}

// dump renders the spots and appearances so that two states can be compared.
func dump(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT s.id::text, s.name, s.address,
		       ST_Y(s.location::geometry), ST_X(s.location::geometry), s.is_closed,
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
			id, name, address, appearances string
			lat, lng                       float64
			closed                         bool
		)
		if err := rows.Scan(&id, &name, &address, &lat, &lng, &closed, &appearances); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&b, "%s name=%s addr=%s loc=%v,%v closed=%v [%s]\n",
			id[len(id)-2:], name, address, lat, lng, closed, appearances)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func mustImport(t *testing.T, pool *pgxpool.Pool, features []spots.Feature, dryRun bool) *spots.ImportReport {
	t.Helper()
	report, err := spots.Import(context.Background(), pool, features, dryRun)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	return report
}

func TestImport(t *testing.T) {
	pool := dbtest.New(t)
	closed := feature(3, "C")
	closed.IsClosed = true
	// B is in the same building as A: same coordinates, still its own spot.
	sameBuilding := feature(2, "B")
	sameBuilding.Location = feature(1, "A").Location
	features := []spots.Feature{feature(1, "A"), sameBuilding, closed}

	t.Run("dry run writes nothing", func(t *testing.T) {
		report := mustImport(t, pool, features, true)
		if got := dump(t, pool); got != "" {
			t.Errorf("database after the dry run:\n%s", got)
		}
		if len(report.Added) != 3 || len(report.Existing) != 0 {
			t.Errorf("report = %+v", report)
		}
		var out bytes.Buffer
		report.Write(&out)
		if !strings.Contains(out.String(), "DRY RUN") || !strings.Contains(out.String(), "3 features: 3 to add (1 of them closed), 0 already") {
			t.Errorf("report:\n%s", out.String())
		}
	})

	want := "" +
		"01 name=A addr=住所A loc=1,101 closed=false [video-A@1]\n" +
		"02 name=B addr=住所B loc=1,101 closed=false [video-B@2]\n" +
		"03 name=C addr=住所C loc=3,103 closed=true [video-C@3]\n"

	t.Run("import", func(t *testing.T) {
		mustImport(t, pool, features, false)
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
		if strings.Join(order, "") != "ABC" {
			t.Errorf("listed as %v, want the order of data.json", order)
		}
	})

	t.Run("second run changes nothing and keeps the edits", func(t *testing.T) {
		// A maintainer reopened C and moved A after the first import.
		if _, err := pool.Exec(context.Background(), `
			UPDATE spots SET is_closed = false WHERE id = $1;`, featureID(3)); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(), `
			UPDATE spots SET location = 'POINT(139 35)' WHERE id = $1`, featureID(1)); err != nil {
			t.Fatal(err)
		}
		edited := dump(t, pool)

		report := mustImport(t, pool, append(features, feature(4, "D")), false)
		if len(report.Added) != 1 || len(report.Existing) != 3 {
			t.Errorf("report = %+v", report)
		}
		got := dump(t, pool)
		wantAfter := edited + "04 name=D addr=住所D loc=4,104 closed=false [video-D@4]\n"
		if got != wantAfter {
			t.Errorf("database:\n%s\nwant:\n%s", got, wantAfter)
		}
	})
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
		YouTubeID: "abc", Timestamp: 89, Location: spots.LatLng{Lat: 35.664339, Lng: 139.757212},
	}
	if len(got) != 1 || got[0] != want {
		t.Errorf("features = %+v, want %+v", got, want)
	}

	invalid := map[string]string{
		"no id":        strings.Replace(valid, `"id": "AF272CED-d057-42dd-9763-fa5562e6d361",`, "", 1),
		"no name":      strings.Replace(valid, `"name": "店", `, "", 1),
		"bad geometry": strings.Replace(valid, `[139.757212, 35.664339]`, `[139.757212]`, 1),
		"out of range": strings.Replace(valid, `[139.757212, 35.664339]`, `[35.664339, 139.757212]`, 1),
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
