package spots_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/db/dbtest"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/httpx"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/spots"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func exec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func getSpots(t *testing.T, pool *pgxpool.Pool) (int, string) {
	t.Helper()
	r, err := httpx.NewEngine()
	if err != nil {
		t.Fatal(err)
	}
	spots.Register(r.Group("/api"), pool)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/spots", nil))
	return w.Code, w.Body.String()
}

func TestListEmpty(t *testing.T) {
	pool := dbtest.New(t)
	code, body := getSpots(t, pool)
	// An empty list, not null, so that clients can iterate without a check.
	if code != 200 || body != `{"spots":[]}` {
		t.Errorf("status = %d, body = %s", code, body)
	}
}

func TestList(t *testing.T) {
	pool := dbtest.New(t)
	const (
		spotA = "aaaaaaaa-0000-4000-8000-000000000001"
		spotB = "bbbbbbbb-0000-4000-8000-000000000002"
		spotC = "cccccccc-0000-4000-8000-000000000003"
	)
	exec(t, pool, `
		INSERT INTO spots (id, name, address, location, is_closed, created_at) VALUES
		($1, '店A', '東京都A', 'POINT(139.7671 35.6812)', false, now() - interval '3 hours'),
		($2, '店B', '東京都B', 'POINT(127.5 26.25)',     true,  now() - interval '2 hours'),
		($3, '店C', '東京都C', 'POINT(0 0)',             false, now() - interval '1 hour')`,
		spotA, spotB, spotC)
	exec(t, pool, `
		INSERT INTO appearances (spot_id, youtube_id, "timestamp", created_at) VALUES
		($1, 'old-video', 10, now() - interval '2 days'),
		($1, 'new-video', 0,  now() - interval '1 day'),
		($2, 'b-video',   89, now())`, spotA, spotB)
	exec(t, pool, `
		INSERT INTO users (id, google_sub, email, display_name) VALUES
		('11111111-0000-4000-8000-000000000001', 'u1', 'u1@example.com', 'u1'),
		('22222222-0000-4000-8000-000000000002', 'u2', 'u2@example.com', 'u2')`)
	// u1 visited A twice and u2 once: two visitors, not three.
	exec(t, pool, `
		INSERT INTO visits (user_id, spot_id, visited_at) VALUES
		('11111111-0000-4000-8000-000000000001', $1, now() - interval '1 day'),
		('11111111-0000-4000-8000-000000000001', $1, now()),
		('22222222-0000-4000-8000-000000000002', $1, now()),
		('22222222-0000-4000-8000-000000000002', $2, now())`, spotA, spotB)

	code, body := getSpots(t, pool)
	if code != 200 {
		t.Fatalf("status = %d, body = %s", code, body)
	}
	var got struct {
		Spots []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Address  string `json:"address"`
			Location struct {
				Lat float64 `json:"lat"`
				Lng float64 `json:"lng"`
			} `json:"location"`
			IsClosed     bool `json:"isClosed"`
			VisitorCount int  `json:"visitorCount"`
			Appearances  []struct {
				ID        string `json:"id"`
				YouTubeID string `json:"youtubeId"`
				Timestamp int    `json:"timestamp"`
			} `json:"appearances"`
		} `json:"spots"`
	}
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Spots) != 3 {
		t.Fatalf("spots = %d, want 3: %s", len(got.Spots), body)
	}

	a, b, c := got.Spots[0], got.Spots[1], got.Spots[2]
	if a.ID != spotA || b.ID != spotB || c.ID != spotC {
		t.Errorf("order = %s, %s, %s; want registration order", a.ID, b.ID, c.ID)
	}
	if a.Name != "店A" || a.Address != "東京都A" || a.IsClosed {
		t.Errorf("spot A = %+v", a)
	}
	if a.Location.Lat != 35.6812 || a.Location.Lng != 139.7671 {
		t.Errorf("spot A location = %+v", a.Location)
	}
	if a.VisitorCount != 2 || b.VisitorCount != 1 || c.VisitorCount != 0 {
		t.Errorf("visitor counts = %d, %d, %d; want 2, 1, 0", a.VisitorCount, b.VisitorCount, c.VisitorCount)
	}
	if len(a.Appearances) != 2 || a.Appearances[0].YouTubeID != "new-video" || a.Appearances[1].YouTubeID != "old-video" {
		t.Errorf("spot A appearances = %+v, want the newest first", a.Appearances)
	}
	if a.Appearances[1].Timestamp != 10 || !httpx.IsUUID(a.Appearances[0].ID) {
		t.Errorf("spot A appearances = %+v", a.Appearances)
	}
	if !b.IsClosed || len(b.Appearances) != 1 || b.Appearances[0].Timestamp != 89 {
		t.Errorf("spot B = %+v", b)
	}
	if c.Appearances == nil || len(c.Appearances) != 0 {
		t.Errorf("spot C appearances = %v, want an empty list", c.Appearances)
	}
}

func TestGet(t *testing.T) {
	pool := dbtest.New(t)
	const id = "aaaaaaaa-0000-4000-8000-000000000001"
	exec(t, pool, `INSERT INTO spots (id, name, address, location) VALUES ($1, '店', '住所', 'POINT(1 2)')`, id)

	got, err := spots.Get(context.Background(), pool, id)
	if err != nil || got == nil || got.Name != "店" || got.Location.Lat != 2 {
		t.Errorf("Get = %+v, %v", got, err)
	}
	got, err = spots.Get(context.Background(), pool, "bbbbbbbb-0000-4000-8000-000000000002")
	if err != nil || got != nil {
		t.Errorf("Get(unknown) = %+v, %v; want nil", got, err)
	}
}

func TestListHasNoGoogleFields(t *testing.T) {
	pool := dbtest.New(t)
	exec(t, pool, `INSERT INTO spots (name, address, location) VALUES ('店', '住所', 'POINT(1 2)')`)
	_, body := getSpots(t, pool)
	if strings.Contains(body, "placeId") {
		t.Errorf("body = %s", body)
	}
}
