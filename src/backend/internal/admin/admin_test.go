package admin_test

import (
	"context"
	"encoding/json"
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
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/spots"
)

const audience = "https://map.example/api/admin"

func init() {
	gin.SetMode(gin.TestMode)
}

type env struct {
	t        *testing.T
	pool     *pgxpool.Pool
	provider *oidctest.Provider
	router   *gin.Engine
}

func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{t: t, pool: dbtest.New(t), provider: oidctest.New(t)}
	e.router = e.newRouter(admin.Options{Pool: e.pool, Audience: audience, Issuer: e.provider.Issuer})
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

const validBody = `{"name": "店", "address": "東京都", "location": {"lat": 35.68, "lng": 139.76}, "youtubeId": "abcDEF_-123", "timestamp": 60}`

func TestAuthentication(t *testing.T) {
	e := newEnv(t)

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

	t.Run("the spot update workflow", func(t *testing.T) {
		token := with(func(c map[string]any) {
			c["workflow_ref"] = "YutoMaeda1209/ShikisiMap/.github/workflows/spot_update.yml@refs/heads/main"
		})
		if w := e.do(e.router, "POST", "/api/admin/spots", validBody, token); w.Code != http.StatusOK {
			t.Errorf("status = %d, body = %s", w.Code, w.Body)
		}
	})

	t.Run("not configured", func(t *testing.T) {
		r := e.newRouter(admin.Options{Pool: e.pool, Issuer: e.provider.Issuer})
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

func TestCreateSpot(t *testing.T) {
	e := newEnv(t)

	w, res := e.create(`{"name": " 東京駅 ", "address": "東京都千代田区丸の内", "location": {"lat": 35.6812, "lng": 139.7671},
		"youtubeId": "video000001", "timestamp": 90}`)
	if w.Code != http.StatusCreated || !res.Created || !res.AppearanceAdded {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body)
	}
	s := res.Spot
	if s.Name != "東京駅" || s.Address != "東京都千代田区丸の内" || s.IsClosed || s.Location != (spots.LatLng{Lat: 35.6812, Lng: 139.7671}) {
		t.Errorf("spot = %+v", s)
	}
	if len(s.Appearances) != 1 || s.Appearances[0].YouTubeID != "video000001" || s.Appearances[0].Timestamp != 90 {
		t.Errorf("appearances = %+v", s.Appearances)
	}

	t.Run("same name and address adds only the appearance", func(t *testing.T) {
		// The location and the closure of the request are ignored for an existing spot.
		w, res := e.create(`{"name": "東京駅", "address": "東京都千代田区丸の内", "location": {"lat": 1, "lng": 2},
			"isClosed": true, "youtubeId": "video000002", "timestamp": 0}`)
		if w.Code != http.StatusOK || res.Created || !res.AppearanceAdded || res.Spot.ID != s.ID {
			t.Fatalf("status = %d, body = %s", w.Code, w.Body)
		}
		if len(res.Spot.Appearances) != 2 || res.Spot.IsClosed || res.Spot.Location != s.Location {
			t.Errorf("spot = %+v, want two appearances and the spot otherwise unchanged", res.Spot)
		}
	})

	t.Run("no location is needed for an existing spot", func(t *testing.T) {
		w, res := e.create(`{"name": "東京駅", "address": "東京都千代田区丸の内", "youtubeId": "video000003", "timestamp": 5}`)
		if w.Code != http.StatusOK || res.Created || len(res.Spot.Appearances) != 3 {
			t.Errorf("status = %d, body = %s", w.Code, w.Body)
		}
	})

	t.Run("same appearance again changes nothing", func(t *testing.T) {
		w, res := e.create(`{"name": "東京駅", "address": "東京都千代田区丸の内", "youtubeId": "video000002", "timestamp": 0}`)
		if w.Code != http.StatusOK || res.Created || res.AppearanceAdded || len(res.Spot.Appearances) != 3 {
			t.Errorf("status = %d, body = %s", w.Code, w.Body)
		}
	})

	t.Run("by spot ID", func(t *testing.T) {
		w, res := e.create(`{"spotId": "` + strings.ToUpper(s.ID) + `", "youtubeId": "video000004", "timestamp": 7}`)
		if w.Code != http.StatusOK || res.Created || !res.AppearanceAdded || res.Spot.ID != s.ID || len(res.Spot.Appearances) != 4 {
			t.Errorf("status = %d, body = %s", w.Code, w.Body)
		}
		w, _ = e.create(`{"spotId": "00000000-0000-4000-8000-000000000000", "youtubeId": "video000004", "timestamp": 7}`)
		wantError(t, w, http.StatusUnprocessableEntity, "unknown_spot")
	})

	t.Run("another address is another spot", func(t *testing.T) {
		w, res := e.create(`{"name": "東京駅", "address": "別の住所", "location": {"lat": 35.6812, "lng": 139.7671},
			"isClosed": true, "youtubeId": "video000005", "timestamp": 0}`)
		if w.Code != http.StatusCreated || !res.Created || res.Spot.ID == s.ID || !res.Spot.IsClosed {
			t.Errorf("status = %d, body = %s", w.Code, w.Body)
		}
	})

	t.Run("a new spot needs a location", func(t *testing.T) {
		w, _ := e.create(`{"name": "新しい店", "address": "東京都", "youtubeId": "video000006", "timestamp": 0}`)
		wantError(t, w, http.StatusUnprocessableEntity, "location_required")
	})

	if n := e.count(`SELECT (SELECT count(*) FROM spots) * 10 + (SELECT count(*) FROM appearances)`); n != 25 {
		t.Errorf("spots, appearances = %d, want 2 and 5", n)
	}
}

func TestCreateSpotValidation(t *testing.T) {
	e := newEnv(t)
	base := map[string]any{"name": "店", "address": "東京都", "location": map[string]float64{"lat": 35, "lng": 139}, "youtubeId": "abcDEF_-123", "timestamp": 0}
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
		"bad spot ID":        with("spotId", "1"),
		"latitude too large": with("location", map[string]float64{"lat": 91, "lng": 139}),
		"swapped":            with("location", map[string]float64{"lat": 139, "lng": 35}),
	} {
		if w, _ := e.create(body); w.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: status = %d, body = %s; want 422", name, w.Code, w.Body)
		}
	}
	for name, body := range map[string]string{
		"string timestamp": with("timestamp", "60"),
		"unknown field":    with("placeId", "ChIJ-x"),
		"not json":         "name=x",
	} {
		if w, _ := e.create(body); w.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, body = %s; want 400", name, w.Code, w.Body)
		}
	}
	if n := e.count(`SELECT count(*) FROM spots`); n != 0 {
		t.Errorf("spots = %d", n)
	}

	req := httptest.NewRequest("POST", "/api/admin/spots", strings.NewReader(validBody))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Authorization", "Bearer "+e.token())
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	wantError(t, rec, http.StatusUnsupportedMediaType, "unsupported_media_type")
}

func TestUpdateSpot(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	var id, otherID string
	if err := e.pool.QueryRow(ctx, `
		INSERT INTO spots (name, address, location) VALUES ('店A', '住所A', 'POINT(2 1)') RETURNING id`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if err := e.pool.QueryRow(ctx, `
		INSERT INTO spots (name, address, location) VALUES ('店B', '住所B', 'POINT(4 3)') RETURNING id`).Scan(&otherID); err != nil {
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

	if w, s := patch(`{"isClosed": true}`); w.Code != 200 || !s.IsClosed || s.Name != "店A" || s.Location != (spots.LatLng{Lat: 1, Lng: 2}) {
		t.Errorf("close: %d %s", w.Code, w.Body)
	}
	if w, s := patch(`{"isClosed": false}`); w.Code != 200 || s.IsClosed {
		t.Errorf("reopen: %d %s", w.Code, w.Body)
	}
	if w, s := patch(`{"location": {"lat": 35.5, "lng": 139.25}}`); w.Code != 200 || s.Location != (spots.LatLng{Lat: 35.5, Lng: 139.25}) || s.Name != "店A" {
		t.Errorf("move: %d %s", w.Code, w.Body)
	}
	if w, s := patch(`{"name": " 新しい名前 ", "address": "新しい住所"}`); w.Code != 200 || s.Name != "新しい名前" || s.Address != "新しい住所" || s.Location.Lat != 35.5 {
		t.Errorf("rename: %d %s", w.Code, w.Body)
	}
	if w, s := patch(`{}`); w.Code != 200 || s.Name != "新しい名前" {
		t.Errorf("empty patch: %d %s", w.Code, w.Body)
	}

	for name, body := range map[string]string{
		"blank name":       `{"name": " "}`,
		"long address":     `{"address": "` + strings.Repeat("あ", 201) + `"}`,
		"out of range":     `{"location": {"lat": -91, "lng": 0}}`,
		"longitude beyond": `{"location": {"lat": 0, "lng": 181}}`,
	} {
		if w, _ := patch(body); w.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: status = %d, body = %s; want 422", name, w.Code, w.Body)
		}
	}
	for name, body := range map[string]string{
		"place ID":      `{"placeId": "ChIJ-x"}`,
		"string closed": `{"isClosed": "true"}`,
	} {
		if w, _ := patch(body); w.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d", name, w.Code)
		}
	}

	wantError(t, e.call("PATCH", "/api/admin/spots/00000000-0000-4000-8000-000000000000", `{"name": "x"}`), 404, "not_found")
	wantError(t, e.call("PATCH", "/api/admin/spots/abc", `{"name": "x"}`), 404, "not_found")

	if n := e.count(`SELECT count(*) FROM spots WHERE id = $1 AND name = '店B' AND NOT is_closed AND ST_Y(location::geometry) = 3`, otherID); n != 1 {
		t.Error("the other spot changed")
	}
}
