package admin_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/admin"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/db/dbtest"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/httpx"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/oidctest"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/places"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/spots"
)

const audience = "https://map.example/api/admin"

func init() {
	gin.SetMode(gin.TestMode)
}

type fakePlaces struct {
	search     map[string][]places.Place
	details    map[string]places.Details
	err        error
	searches   []string
	detailsFor []string
}

func (f *fakePlaces) SearchText(_ context.Context, query string) ([]places.Place, error) {
	f.searches = append(f.searches, query)
	if f.err != nil {
		return nil, f.err
	}
	return f.search[query], nil
}

func (f *fakePlaces) Details(_ context.Context, placeID string) (places.Details, error) {
	f.detailsFor = append(f.detailsFor, placeID)
	if f.err != nil {
		return places.Details{}, f.err
	}
	d, ok := f.details[placeID]
	if !ok {
		return places.Details{}, places.ErrNotFound
	}
	return d, nil
}

type env struct {
	t        *testing.T
	pool     *pgxpool.Pool
	provider *oidctest.Provider
	places   *fakePlaces
	router   *gin.Engine
}

func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{t: t, pool: dbtest.New(t), provider: oidctest.New(t), places: &fakePlaces{}}
	e.router = e.newRouter(admin.Options{Pool: e.pool, Places: e.places, Audience: audience, Issuer: e.provider.Issuer})
	return e
}

func (e *env) newRouter(opts admin.Options) *gin.Engine {
	e.t.Helper()
	r, err := httpx.NewEngine()
	if err != nil {
		e.t.Fatal(err)
	}
	admin.Register(r.Group("/api/admin", httpx.RequireJSON()), opts)
	return r
}

// claims returns the claims of a token GitHub issues to the spot request
// workflow on main.
func (e *env) claims() map[string]any {
	return map[string]any{
		"iss": e.provider.Issuer, "aud": audience, "sub": "repo:YutoMaeda1209/ShikisiMap:ref:refs/heads/main",
		"exp": time.Now().Add(5 * time.Minute).Unix(), "iat": time.Now().Unix(),
		"repository":   "YutoMaeda1209/ShikisiMap",
		"workflow_ref": "YutoMaeda1209/ShikisiMap/.github/workflows/spot_request.yml@refs/heads/main",
		"ref":          "refs/heads/main",
	}
}

func (e *env) token() string {
	return e.provider.Sign(e.t, e.claims())
}

func (e *env) do(router *gin.Engine, method, path, body, authorization string) *httptest.ResponseRecorder {
	e.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// call sends an authorized request. GitHub Actions sends no Origin header.
func (e *env) call(method, path, body string) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.do(e.router, method, path, body, "Bearer "+e.token())
}

func (e *env) count(sql string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

func wantError(t *testing.T, w *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if w.Code != status || !strings.Contains(w.Body.String(), `"code":"`+code+`"`) {
		t.Errorf("status = %d, body = %s; want %d %s", w.Code, w.Body, status, code)
	}
}

const validBody = `{"name": "店", "address": "東京都", "youtubeId": "abcDEF_-123", "timestamp": 60, "placeId": "ChIJ-given"}`

func TestAuthentication(t *testing.T) {
	e := newEnv(t)
	e.places.details = map[string]places.Details{"ChIJ-given": {}}

	rejected := func(name string, status int, code, authorization string) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			for _, req := range [][2]string{{"POST", "/api/admin/spots"}, {"PATCH", "/api/admin/spots/00000000-0000-4000-8000-000000000000"}} {
				wantError(t, e.do(e.router, req[0], req[1], validBody, authorization), status, code)
			}
		})
	}
	with := func(change func(c map[string]any)) string {
		c := e.claims()
		change(c)
		return "Bearer " + e.provider.Sign(t, c)
	}

	rejected("no token", 401, "unauthorized", "")
	rejected("not a bearer token", 401, "unauthorized", "Basic dXNlcjpwYXNz")
	rejected("empty bearer token", 401, "unauthorized", "Bearer ")
	rejected("garbage", 401, "unauthorized", "Bearer not.a.jwt")
	rejected("signed by another key", 401, "unauthorized", "Bearer "+e.provider.SignWithOtherKey(t, e.claims()))
	rejected("unsigned (alg none)", 401, "unauthorized",
		"Bearer eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0."+strings.Split(e.token(), ".")[1]+".")
	rejected("expired", 401, "unauthorized", with(func(c map[string]any) { c["exp"] = time.Now().Add(-time.Minute).Unix() }))
	rejected("wrong audience", 401, "unauthorized", with(func(c map[string]any) { c["aud"] = "https://github.com/YutoMaeda1209" }))
	rejected("wrong issuer", 401, "unauthorized", with(func(c map[string]any) { c["iss"] = "https://evil.example" }))

	rejected("another repository", 403, "forbidden", with(func(c map[string]any) { c["repository"] = "evil/ShikisiMap" }))
	rejected("a fork with the same workflow", 403, "forbidden", with(func(c map[string]any) {
		c["repository"] = "evil/ShikisiMap"
		c["workflow_ref"] = "evil/ShikisiMap/.github/workflows/spot_request.yml@refs/heads/main"
	}))
	rejected("another workflow", 403, "forbidden", with(func(c map[string]any) {
		c["workflow_ref"] = "YutoMaeda1209/ShikisiMap/.github/workflows/build_test.yml@refs/heads/main"
	}))
	rejected("another branch", 403, "forbidden", with(func(c map[string]any) { c["ref"] = "refs/heads/feat/evil" }))
	rejected("the workflow of another branch", 403, "forbidden", with(func(c map[string]any) {
		c["workflow_ref"] = "YutoMaeda1209/ShikisiMap/.github/workflows/spot_request.yml@refs/heads/feat/evil"
	}))
	rejected("a pull request", 403, "forbidden", with(func(c map[string]any) { c["ref"] = "refs/pull/1/merge" }))
	rejected("missing claims", 403, "forbidden", with(func(c map[string]any) {
		delete(c, "repository")
		delete(c, "workflow_ref")
		delete(c, "ref")
	}))

	if n := e.count(`SELECT count(*) FROM spots`); n != 0 {
		t.Errorf("spots = %d after the rejected requests", n)
	}

	t.Run("valid token", func(t *testing.T) {
		if w := e.call("POST", "/api/admin/spots", validBody); w.Code != http.StatusCreated {
			t.Errorf("status = %d, body = %s", w.Code, w.Body)
		}
		// The scheme is case-insensitive.
		if w := e.do(e.router, "POST", "/api/admin/spots", validBody, "bearer "+e.token()); w.Code != http.StatusOK {
			t.Errorf("status = %d, body = %s", w.Code, w.Body)
		}
	})

	t.Run("not configured", func(t *testing.T) {
		r := e.newRouter(admin.Options{Pool: e.pool, Places: e.places, Issuer: e.provider.Issuer})
		wantError(t, e.do(r, "POST", "/api/admin/spots", validBody, "Bearer "+e.token()), 503, "admin_unavailable")
	})

	t.Run("a session cookie does not authenticate", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/admin/spots", strings.NewReader(validBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://map.example")
		req.AddCookie(&http.Cookie{Name: "shikisimap_session", Value: "anything"})
		w := httptest.NewRecorder()
		e.router.ServeHTTP(w, req)
		wantError(t, w, 401, "unauthorized")
	})
}

type createResponse struct {
	Spot            spots.Spot `json:"spot"`
	Created         bool       `json:"created"`
	AppearanceAdded bool       `json:"appearanceAdded"`
}

func (e *env) create(body string) (*httptest.ResponseRecorder, createResponse) {
	e.t.Helper()
	w := e.call("POST", "/api/admin/spots", body)
	var res createResponse
	json.Unmarshal(w.Body.Bytes(), &res)
	return w, res
}

func TestCreateSpotBySearch(t *testing.T) {
	e := newEnv(t)
	e.places.search = map[string][]places.Place{
		"東京駅 東京都千代田区丸の内": {{ID: "ChIJ-tokyo", Name: "東京駅 (Google)", Address: "Google の住所", Location: places.LatLng{Lat: 35.6812, Lng: 139.7671}}},
		"カフェ 東京都": {
			{ID: "ChIJ-cafe1", Name: "カフェ 1号店", Address: "住所1", Location: places.LatLng{Lat: 1, Lng: 2}},
			{ID: "ChIJ-cafe2", Name: "カフェ 2号店", Address: "住所2", Location: places.LatLng{Lat: 3, Lng: 4}},
		},
	}

	w, res := e.create(`{"name": " 東京駅 ", "address": "東京都千代田区丸の内", "youtubeId": "video000001", "timestamp": 90}`)
	if w.Code != http.StatusCreated || !res.Created || !res.AppearanceAdded {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body)
	}
	s := res.Spot
	// The name and address are the maintainer's; the place and location are Google's.
	if s.Name != "東京駅" || s.Address != "東京都千代田区丸の内" || *s.PlaceID != "ChIJ-tokyo" || s.IsClosed {
		t.Errorf("spot = %+v", s)
	}
	if s.Location != (places.LatLng{Lat: 35.6812, Lng: 139.7671}) {
		t.Errorf("location = %+v", s.Location)
	}
	if len(s.Appearances) != 1 || s.Appearances[0].YouTubeID != "video000001" || s.Appearances[0].Timestamp != 90 {
		t.Errorf("appearances = %+v", s.Appearances)
	}
	if n := e.count(`SELECT count(*) FROM spots WHERE location_fetched_at > now() - interval '1 minute'`); n != 1 {
		t.Error("location_fetched_at was not set")
	}

	t.Run("same place again adds only the appearance", func(t *testing.T) {
		w, res := e.create(`{"name": "東京駅", "address": "東京都千代田区丸の内", "youtubeId": "video000002", "timestamp": 0, "isClosed": true}`)
		if w.Code != http.StatusOK || res.Created || !res.AppearanceAdded || res.Spot.ID != s.ID {
			t.Fatalf("status = %d, body = %s", w.Code, w.Body)
		}
		if len(res.Spot.Appearances) != 2 || res.Spot.IsClosed {
			t.Errorf("spot = %+v, want two appearances and the spot otherwise unchanged", res.Spot)
		}
		if n := e.count(`SELECT count(*) FROM spots`); n != 1 {
			t.Errorf("spots = %d, want 1", n)
		}
	})

	t.Run("same appearance again changes nothing", func(t *testing.T) {
		w, res := e.create(`{"name": "東京駅", "address": "東京都千代田区丸の内", "youtubeId": "video000002", "timestamp": 0}`)
		if w.Code != http.StatusOK || res.Created || res.AppearanceAdded || len(res.Spot.Appearances) != 2 {
			t.Errorf("status = %d, body = %s", w.Code, w.Body)
		}
	})

	t.Run("several candidates", func(t *testing.T) {
		w, _ := e.create(`{"name": "カフェ", "address": "東京都", "youtubeId": "video000003", "timestamp": 0}`)
		wantError(t, w, http.StatusConflict, "place_not_resolved")
		var body struct {
			Candidates []places.Place `json:"candidates"`
		}
		json.Unmarshal(w.Body.Bytes(), &body)
		if len(body.Candidates) != 2 || body.Candidates[1].ID != "ChIJ-cafe2" || body.Candidates[1].Name != "カフェ 2号店" || body.Candidates[1].Address != "住所2" {
			t.Errorf("body = %s", w.Body)
		}
		if !strings.Contains(w.Body.String(), `"placeId":"ChIJ-cafe1"`) {
			t.Errorf("body = %s, want the candidates' place IDs as placeId", w.Body)
		}
	})

	t.Run("no candidate", func(t *testing.T) {
		w, _ := e.create(`{"name": "どこにもない店", "address": "東京都", "youtubeId": "video000003", "timestamp": 0}`)
		wantError(t, w, http.StatusConflict, "place_not_resolved")
		if !strings.Contains(w.Body.String(), `"candidates":[]`) {
			t.Errorf("body = %s, want an empty candidate list", w.Body)
		}
	})

	if n := e.count(`SELECT (SELECT count(*) FROM spots) * 10 + (SELECT count(*) FROM appearances)`); n != 12 {
		t.Errorf("spots, appearances = %d, want 1 and 2", n)
	}
}

func TestCreateSpotWithPlaceID(t *testing.T) {
	e := newEnv(t)
	e.places.details = map[string]places.Details{
		"ChIJ-cafe2":  {Location: places.LatLng{Lat: 3, Lng: 4}, BusinessStatus: "OPERATIONAL"},
		"ChIJ-closed": {Location: places.LatLng{Lat: 5, Lng: 6}, BusinessStatus: "CLOSED_PERMANENTLY"},
	}

	w, res := e.create(`{"name": "カフェ", "address": "東京都", "youtubeId": "video000003", "timestamp": 5, "placeId": "ChIJ-cafe2"}`)
	if w.Code != http.StatusCreated || !res.Created || *res.Spot.PlaceID != "ChIJ-cafe2" || res.Spot.Location != (places.LatLng{Lat: 3, Lng: 4}) {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body)
	}
	// The place was given, so nothing is searched.
	if len(e.places.searches) != 0 || len(e.places.detailsFor) != 1 {
		t.Errorf("searches = %v, details = %v", e.places.searches, e.places.detailsFor)
	}

	t.Run("existing place needs no API call", func(t *testing.T) {
		e.places.detailsFor = nil
		w, res := e.create(`{"name": "カフェ", "address": "東京都", "youtubeId": "video000004", "timestamp": 0, "placeId": "ChIJ-cafe2"}`)
		if w.Code != http.StatusOK || res.Created || len(res.Spot.Appearances) != 2 || len(e.places.detailsFor) != 0 {
			t.Errorf("status = %d, body = %s, details = %v", w.Code, w.Body, e.places.detailsFor)
		}
	})

	t.Run("closed according to Google", func(t *testing.T) {
		_, res := e.create(`{"name": "閉業店", "address": "東京都", "youtubeId": "video000005", "timestamp": 0, "placeId": "ChIJ-closed"}`)
		if !res.Created || !res.Spot.IsClosed {
			t.Errorf("spot = %+v", res.Spot)
		}
	})

	t.Run("closed according to the request", func(t *testing.T) {
		e.places.details["ChIJ-open"] = places.Details{BusinessStatus: "OPERATIONAL"}
		_, res := e.create(`{"name": "閉業店2", "address": "東京都", "youtubeId": "video000006", "timestamp": 0, "placeId": "ChIJ-open", "isClosed": true}`)
		if !res.Created || !res.Spot.IsClosed {
			t.Errorf("spot = %+v", res.Spot)
		}
	})

	t.Run("unknown place ID", func(t *testing.T) {
		before := e.count(`SELECT count(*) FROM spots`)
		w, _ := e.create(`{"name": "店", "address": "東京都", "youtubeId": "video000007", "timestamp": 0, "placeId": "ChIJ-typo"}`)
		wantError(t, w, http.StatusUnprocessableEntity, "validation_failed")
		if e.count(`SELECT count(*) FROM spots`) != before {
			t.Error("a spot was created")
		}
	})
}

func TestCreateSpotValidation(t *testing.T) {
	e := newEnv(t)
	base := map[string]any{"name": "店", "address": "東京都", "youtubeId": "abcDEF_-123", "timestamp": 0, "placeId": "ChIJ-given"}
	with := func(key string, value any) string {
		m := map[string]any{}
		for k, v := range base {
			m[k] = v
		}
		if value == nil {
			delete(m, key)
		} else {
			m[key] = value
		}
		b, _ := json.Marshal(m)
		return string(b)
	}
	for name, body := range map[string]string{
		"no name":            with("name", nil),
		"blank name":         with("name", "  "),
		"long name":          with("name", strings.Repeat("あ", 201)),
		"no address":         with("address", nil),
		"no video":           with("youtubeId", nil),
		"video URL":          with("youtubeId", "https://youtu.be/abcDEF_-123"),
		"negative timestamp": with("timestamp", -1),
		"long place":         with("placeId", strings.Repeat("x", 201)),
	} {
		if w, _ := e.create(body); w.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: status = %d, body = %s; want 422", name, w.Code, w.Body)
		}
	}
	for name, body := range map[string]string{
		"string timestamp": with("timestamp", "60"),
		"unknown field":    with("id", "00000000-0000-4000-8000-000000000000"),
		"not json":         "name=x",
	} {
		if w, _ := e.create(body); w.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, body = %s; want 400", name, w.Code, w.Body)
		}
	}
	req := httptest.NewRequest("POST", "/api/admin/spots", strings.NewReader(validBody))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Authorization", "Bearer "+e.token())
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	wantError(t, rec, http.StatusUnsupportedMediaType, "unsupported_media_type")
}

func TestCreateSpotWhenPlacesFails(t *testing.T) {
	e := newEnv(t)
	search := `{"name": "店", "address": "東京都", "youtubeId": "video000001", "timestamp": 0}`

	e.places.err = fmt.Errorf("%w: daily limit", places.ErrQuotaExceeded)
	w, _ := e.create(search)
	wantError(t, w, http.StatusServiceUnavailable, "places_unavailable")
	if !strings.Contains(w.Body.String(), "quota") {
		t.Errorf("body = %s, want it to mention the quota", w.Body)
	}
	w, _ = e.create(validBody)
	wantError(t, w, http.StatusServiceUnavailable, "places_unavailable")

	e.places.err = errors.New("secret upstream detail")
	w, _ = e.create(search)
	wantError(t, w, http.StatusServiceUnavailable, "places_unavailable")
	if strings.Contains(w.Body.String(), "secret") {
		t.Errorf("body = %s exposes the upstream error", w.Body)
	}

	t.Run("no API key", func(t *testing.T) {
		r := e.newRouter(admin.Options{Pool: e.pool, Audience: audience, Issuer: e.provider.Issuer})
		for _, body := range []string{search, validBody} {
			wantError(t, e.do(r, "POST", "/api/admin/spots", body, "Bearer "+e.token()), http.StatusServiceUnavailable, "places_unavailable")
		}
	})
	if n := e.count(`SELECT count(*) FROM spots`); n != 0 {
		t.Errorf("spots = %d", n)
	}
}

func TestUpdateSpot(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	var id, otherID string
	if err := e.pool.QueryRow(ctx, `
		INSERT INTO spots (place_id, name, address, location, location_fetched_at, status_checked_at)
		VALUES ('ChIJ-a', '店A', '住所A', 'POINT(2 1)', now(), now()) RETURNING id`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if err := e.pool.QueryRow(ctx, `
		INSERT INTO spots (place_id, name, address, location) VALUES ('ChIJ-b', '店B', '住所B', 'POINT(4 3)') RETURNING id`).Scan(&otherID); err != nil {
		t.Fatal(err)
	}
	path := "/api/admin/spots/" + id

	patch := func(body string) (*httptest.ResponseRecorder, spots.Spot) {
		t.Helper()
		w := e.call("PATCH", path, body)
		var s spots.Spot
		json.Unmarshal(w.Body.Bytes(), &s)
		return w, s
	}
	fetched := func() bool {
		return e.count(`SELECT count(*) FROM spots WHERE id = $1 AND location_fetched_at IS NOT NULL`, id) == 1
	}

	if w, s := patch(`{"isClosed": true}`); w.Code != 200 || !s.IsClosed || s.Name != "店A" || *s.PlaceID != "ChIJ-a" {
		t.Errorf("close: %d %s", w.Code, w.Body)
	}
	if w, s := patch(`{"name": " 新しい名前 ", "address": "新しい住所", "isClosed": false}`); w.Code != 200 || s.IsClosed || s.Name != "新しい名前" || s.Address != "新しい住所" {
		t.Errorf("rename: %d %s", w.Code, w.Body)
	}
	if w, s := patch(`{}`); w.Code != 200 || s.Name != "新しい名前" {
		t.Errorf("empty patch: %d %s", w.Code, w.Body)
	}
	if w, s := patch(`{"placeId": "ChIJ-a"}`); w.Code != 200 || *s.PlaceID != "ChIJ-a" || !fetched() {
		t.Errorf("same place: %d %s; the location must stay fetched", w.Code, w.Body)
	}

	if w, s := patch(`{"placeId": "ChIJ-new"}`); w.Code != 200 || *s.PlaceID != "ChIJ-new" || fetched() {
		t.Errorf("new place: %d %s; the location must be marked for refetching", w.Code, w.Body)
	}
	if w, s := patch(`{"placeId": null}`); w.Code != 200 || s.PlaceID != nil {
		t.Errorf("unresolve: %d %s", w.Code, w.Body)
	}

	w, _ := patch(`{"placeId": "ChIJ-b"}`)
	wantError(t, w, http.StatusConflict, "place_in_use")

	for name, body := range map[string]string{
		"blank name":   `{"name": " "}`,
		"long address": `{"address": "` + strings.Repeat("あ", 201) + `"}`,
		"empty place":  `{"placeId": ""}`,
		"number place": `{"placeId": 1}`,
	} {
		if w, _ := patch(body); w.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: status = %d, body = %s; want 422", name, w.Code, w.Body)
		}
	}
	if w, _ := patch(`{"location": {"lat": 0, "lng": 0}}`); w.Code != http.StatusBadRequest {
		t.Errorf("unknown field: status = %d", w.Code)
	}

	wantError(t, e.call("PATCH", "/api/admin/spots/00000000-0000-4000-8000-000000000000", `{"name": "x"}`), 404, "not_found")
	wantError(t, e.call("PATCH", "/api/admin/spots/abc", `{"name": "x"}`), 404, "not_found")

	if n := e.count(`SELECT count(*) FROM spots WHERE id = $1 AND name = '店B' AND place_id = 'ChIJ-b'`, otherID); n != 1 {
		t.Error("the other spot changed")
	}
}
