package visits_test

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/apitest"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/visits"
)

type env struct {
	*apitest.Env
	userID string
	cookie *http.Cookie
	spotA  string
	spotB  string
}

func newEnv(t *testing.T) *env {
	e := apitest.New(t)
	visits.Register(e.API, e.Pool, e.Auth)
	userID, cookie := e.NewUser("tetra")
	return &env{Env: e, userID: userID, cookie: cookie, spotA: e.NewSpot("店A"), spotB: e.NewSpot("店B")}
}

type createResponse struct {
	Visits  []visits.Visit `json:"visits"`
	Skipped int            `json:"skipped"`
}

func (e *env) post(body string) (*httptest.ResponseRecorder, createResponse) {
	e.T.Helper()
	w := e.Do(apitest.Request{Method: "POST", Path: "/api/me/visits", Body: body, Cookie: e.cookie})
	var res createResponse
	json.Unmarshal(w.Body.Bytes(), &res)
	return w, res
}

func (e *env) list(cookie *http.Cookie) []visits.Visit {
	e.T.Helper()
	w := e.Do(apitest.Request{Method: "GET", Path: "/api/me/visits", Cookie: cookie})
	if w.Code != 200 {
		e.T.Fatalf("list: %d %s", w.Code, w.Body)
	}
	var res struct {
		Visits []visits.Visit `json:"visits"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		e.T.Fatal(err)
	}
	return res.Visits
}

func TestRequiresLogin(t *testing.T) {
	e := newEnv(t)
	id := "00000000-0000-4000-8000-000000000000"
	for _, req := range []apitest.Request{
		{Method: "GET", Path: "/api/me/visits"},
		{Method: "POST", Path: "/api/me/visits", Body: `[{"spotId": "` + e.spotA + `"}]`},
		{Method: "PATCH", Path: "/api/me/visits/" + id, Body: `{}`},
		{Method: "DELETE", Path: "/api/me/visits/" + id},
	} {
		e.WantError(e.Do(req), http.StatusUnauthorized, "unauthorized")
	}
	if n := e.Count(`SELECT count(*) FROM visits`); n != 0 {
		t.Errorf("visits = %d", n)
	}
}

func TestCreate(t *testing.T) {
	e := newEnv(t)

	t.Run("one visit with defaults", func(t *testing.T) {
		w, res := e.post(`[{"spotId": "` + e.spotA + `"}]`)
		if w.Code != http.StatusCreated || len(res.Visits) != 1 || res.Skipped != 0 {
			t.Fatalf("status = %d, body = %s", w.Code, w.Body)
		}
		v := res.Visits[0]
		if v.SpotID != e.spotA || v.Memo != nil || time.Since(v.VisitedAt) > time.Minute || v.ID == "" {
			t.Errorf("visit = %+v", v)
		}
	})

	t.Run("several visits with dates and memos", func(t *testing.T) {
		w, res := e.post(`[
			{"spotId": "` + e.spotA + `", "visitedAt": "2026-01-02T12:00:00+09:00", "memo": "おいしかった"},
			{"spotId": "` + e.spotB + `", "visitedAt": "2026-01-03T03:00:00Z", "memo": ""},
			{"spotId": "` + strings.ToUpper(e.spotB) + `", "visitedAt": "2026-01-04T03:00:00Z", "memo": null}]`)
		if w.Code != http.StatusCreated || len(res.Visits) != 3 || res.Skipped != 0 {
			t.Fatalf("status = %d, body = %s", w.Code, w.Body)
		}
		if !res.Visits[0].VisitedAt.Equal(time.Date(2026, 1, 2, 3, 0, 0, 0, time.UTC)) || *res.Visits[0].Memo != "おいしかった" {
			t.Errorf("visit 0 = %+v", res.Visits[0])
		}
		if res.Visits[1].Memo != nil || res.Visits[2].Memo != nil {
			t.Errorf("an empty memo must be stored as null: %s", w.Body)
		}
		// Times are reported in UTC whatever offset they were sent with.
		if !strings.Contains(w.Body.String(), `"visitedAt":"2026-01-02T03:00:00Z"`) {
			t.Errorf("body = %s", w.Body)
		}
	})

	t.Run("duplicates are skipped", func(t *testing.T) {
		w, res := e.post(`[
			{"spotId": "` + e.spotA + `", "visitedAt": "2026-01-02T03:00:00Z", "memo": "別のメモ"},
			{"spotId": "` + e.spotA + `", "visitedAt": "2026-05-05T00:00:00Z"},
			{"spotId": "` + e.spotA + `", "visitedAt": "2026-05-05T09:00:00+09:00"}]`)
		if w.Code != http.StatusCreated || len(res.Visits) != 1 || res.Skipped != 2 {
			t.Fatalf("status = %d, body = %s; want 1 created and 2 skipped", w.Code, w.Body)
		}
		// The existing record keeps its memo.
		if n := e.Count(`SELECT count(*) FROM visits WHERE memo = 'おいしかった'`); n != 1 {
			t.Error("the existing visit was overwritten")
		}
	})

	t.Run("all duplicates", func(t *testing.T) {
		w, res := e.post(`[{"spotId": "` + e.spotA + `", "visitedAt": "2026-01-02T03:00:00Z"}]`)
		if w.Code != http.StatusCreated || res.Skipped != 1 || w.Body.String() != `{"skipped":1,"visits":[]}` {
			t.Errorf("status = %d, body = %s", w.Code, w.Body)
		}
	})

	t.Run("another user may record the same spot and time", func(t *testing.T) {
		_, other := e.NewUser("other")
		w := e.Do(apitest.Request{Method: "POST", Path: "/api/me/visits", Cookie: other,
			Body: `[{"spotId": "` + e.spotA + `", "visitedAt": "2026-01-02T03:00:00Z"}]`})
		if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), `"skipped":0`) {
			t.Errorf("status = %d, body = %s", w.Code, w.Body)
		}
	})
}

func TestCreateRejectsUnknownSpotAtomically(t *testing.T) {
	e := newEnv(t)
	const unknown = "00000000-0000-4000-8000-000000000000"
	w, _ := e.post(`[
		{"spotId": "` + e.spotA + `"},
		{"spotId": "` + unknown + `"},
		{"spotId": "` + e.spotB + `"}]`)
	e.WantError(w, http.StatusUnprocessableEntity, "unknown_spot")
	if !strings.Contains(w.Body.String(), unknown) {
		t.Errorf("body = %s, want it to name the spot", w.Body)
	}
	if n := e.Count(`SELECT count(*) FROM visits`); n != 0 {
		t.Errorf("visits = %d, want none", n)
	}
}

func TestCreateValidation(t *testing.T) {
	e := newEnv(t)
	spot := `"spotId": "` + e.spotA + `"`
	tooMany := "[" + strings.TrimSuffix(strings.Repeat("{"+spot+"},", visits.MaxBatchSize+1), ",") + "]"

	for name, body := range map[string]string{
		"empty array":   `[]`,
		"too many":      tooMany,
		"no spot":       `[{}]`,
		"bad spot id":   `[{"spotId": "1"}]`,
		"sql in spot":   `[{"spotId": "'; DROP TABLE visits; --"}]`,
		"future":        `[{` + spot + `, "visitedAt": "2100-01-01T00:00:00Z"}]`,
		"ancient":       `[{` + spot + `, "visitedAt": "0001-01-01T00:00:00Z"}]`,
		"number memo":   `[{` + spot + `, "memo": 1}]`,
		"long memo":     `[{` + spot + `, "memo": "` + strings.Repeat("あ", visits.MaxMemoLength+1) + `"}]`,
		"second is bad": `[{` + spot + `}, {"spotId": "x"}]`,
	} {
		w, _ := e.post(body)
		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: status = %d, body = %.200s; want 422", name, w.Code, w.Body)
		}
	}
	for name, body := range map[string]string{
		"object":        `{` + spot + `}`,
		"bad date":      `[{` + spot + `, "visitedAt": "yesterday"}]`,
		"date only":     `[{` + spot + `, "visitedAt": "2026-01-01"}]`,
		"unknown field": `[{` + spot + `, "userId": "x"}]`,
		"not json":      `spotId=x`,
	} {
		w, _ := e.post(body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, body = %s; want 400", name, w.Code, w.Body)
		}
	}
	if n := e.Count(`SELECT count(*) FROM visits`); n != 0 {
		t.Errorf("visits = %d, want none", n)
	}

	t.Run("limits are inclusive", func(t *testing.T) {
		w, res := e.post(`[{` + spot + `, "memo": "` + strings.Repeat("あ", visits.MaxMemoLength) + `"}]`)
		if w.Code != http.StatusCreated || len(res.Visits) != 1 {
			t.Errorf("status = %d", w.Code)
		}
		var items []string
		for i := range visits.MaxBatchSize {
			items = append(items, fmt.Sprintf(`{%s, "visitedAt": "2025-01-01T00:00:00Z", "memo": "%d"}`, spot, i))
		}
		items[0] = `{` + spot + `, "visitedAt": "2025-01-01T00:00:00Z"}`
		w, res = e.post("[" + strings.Join(items, ",") + "]")
		// All share one spot and time, so one is created and the rest skipped.
		if w.Code != http.StatusCreated || len(res.Visits) != 1 || res.Skipped != visits.MaxBatchSize-1 {
			t.Errorf("status = %d, created = %d, skipped = %d", w.Code, len(res.Visits), res.Skipped)
		}
	})
}

func TestList(t *testing.T) {
	e := newEnv(t)
	_, other := e.NewUser("other")
	e.post(`[
		{"spotId": "` + e.spotA + `", "visitedAt": "2026-01-01T00:00:00Z", "memo": "古い"},
		{"spotId": "` + e.spotB + `", "visitedAt": "2026-03-01T00:00:00Z"},
		{"spotId": "` + e.spotA + `", "visitedAt": "2026-02-01T00:00:00Z"}]`)
	e.Do(apitest.Request{Method: "POST", Path: "/api/me/visits", Cookie: other,
		Body: `[{"spotId": "` + e.spotA + `", "memo": "他人のメモ"}]`})

	got := e.list(e.cookie)
	if len(got) != 3 {
		t.Fatalf("visits = %d, want the user's own 3", len(got))
	}
	if got[0].SpotID != e.spotB || got[1].SpotID != e.spotA || *got[2].Memo != "古い" {
		t.Errorf("visits = %+v, want the newest first", got)
	}

	if w := e.Do(apitest.Request{Method: "GET", Path: "/api/me/visits", Cookie: other}); strings.Contains(w.Body.String(), "古い") {
		t.Errorf("another user's memo is visible: %s", w.Body)
	}

	_, empty := e.NewUser("empty")
	if w := e.Do(apitest.Request{Method: "GET", Path: "/api/me/visits", Cookie: empty}); w.Body.String() != `{"visits":[]}` {
		t.Errorf("body = %s", w.Body)
	}
}

func TestListCSV(t *testing.T) {
	e := newEnv(t)
	e.Exec(`UPDATE spots SET name = '=店,"B"' WHERE id = $1`, e.spotB)
	e.post(`[
		{"spotId": "` + e.spotA + `", "visitedAt": "2026-01-01T09:00:00+09:00", "memo": "改行\nと,カンマ"},
		{"spotId": "` + e.spotB + `", "visitedAt": "2026-03-01T00:00:00Z", "memo": "=HYPERLINK(\"http://evil\")"}]`)

	w := e.Do(apitest.Request{Method: "GET", Path: "/api/me/visits", Cookie: e.cookie, Header: []string{"Accept", "text/csv"}})
	if w.Code != 200 || w.Header().Get("Content-Type") != "text/csv; charset=utf-8" {
		t.Fatalf("status = %d, Content-Type = %s", w.Code, w.Header().Get("Content-Type"))
	}
	if !strings.HasPrefix(w.Header().Get("Content-Disposition"), "attachment") || w.Header().Get("Vary") != "Accept" {
		t.Errorf("headers = %v", w.Header())
	}
	body := w.Body.String()
	if !strings.HasPrefix(body, "\xEF\xBB\xBF") {
		t.Error("the CSV has no byte order mark")
	}
	records, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(body, "\xEF\xBB\xBF"))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 || strings.Join(records[0], ",") != "id,spot_id,spot_name,visited_at,memo" {
		t.Fatalf("records = %q", records)
	}
	// Cells that would run as a formula are quoted.
	if r := records[1]; r[1] != e.spotB || r[2] != `'=店,"B"` || r[3] != "2026-03-01T00:00:00Z" || r[4] != `'=HYPERLINK("http://evil")` {
		t.Errorf("row 1 = %q", r)
	}
	if r := records[2]; r[2] != "店A" || r[3] != "2026-01-01T00:00:00Z" || r[4] != "改行\nと,カンマ" {
		t.Errorf("row 2 = %q", r)
	}

	for accept, wantCSV := range map[string]bool{
		"":                                   false,
		"*/*":                                false,
		"application/json":                   false,
		"application/json, text/csv":         false,
		"text/csv;q=0.9, */*;q=0.1":          true,
		"TEXT/CSV":                           true,
		"text/html,application/xhtml+xml":    false,
		"text/csv, text/plain;q=0.5":         true,
		"application/json;q=0.1, text/plain": false,
	} {
		w := e.Do(apitest.Request{Method: "GET", Path: "/api/me/visits", Cookie: e.cookie, Header: []string{"Accept", accept}})
		if got := strings.HasPrefix(w.Header().Get("Content-Type"), "text/csv"); got != wantCSV {
			t.Errorf("Accept %q: CSV = %v, want %v", accept, got, wantCSV)
		}
	}

	_, empty := e.NewUser("empty")
	w = e.Do(apitest.Request{Method: "GET", Path: "/api/me/visits", Cookie: empty, Header: []string{"Accept", "text/csv"}})
	if w.Body.String() != "\xEF\xBB\xBFid,spot_id,spot_name,visited_at,memo\n" {
		t.Errorf("empty CSV = %q", w.Body)
	}
}

func TestUpdate(t *testing.T) {
	e := newEnv(t)
	_, res := e.post(`[
		{"spotId": "` + e.spotA + `", "visitedAt": "2026-01-01T00:00:00Z", "memo": "メモ"},
		{"spotId": "` + e.spotA + `", "visitedAt": "2026-02-01T00:00:00Z"}]`)
	id, otherID := res.Visits[0].ID, res.Visits[1].ID
	path := "/api/me/visits/" + id

	patch := func(body string) (*httptest.ResponseRecorder, visits.Visit) {
		t.Helper()
		w := e.Do(apitest.Request{Method: "PATCH", Path: path, Body: body, Cookie: e.cookie})
		var v visits.Visit
		json.Unmarshal(w.Body.Bytes(), &v)
		return w, v
	}

	if w, v := patch(`{"memo": "新しいメモ"}`); w.Code != 200 || *v.Memo != "新しいメモ" || !v.VisitedAt.Equal(res.Visits[0].VisitedAt) || v.SpotID != e.spotA {
		t.Errorf("memo only: %d %s", w.Code, w.Body)
	}
	if w, v := patch(`{"visitedAt": "2026-01-15T00:00:00Z"}`); w.Code != 200 || *v.Memo != "新しいメモ" || v.VisitedAt.Day() != 15 {
		t.Errorf("date only: %d %s", w.Code, w.Body)
	}
	if w, v := patch(`{}`); w.Code != 200 || *v.Memo != "新しいメモ" || v.VisitedAt.Day() != 15 {
		t.Errorf("empty patch: %d %s", w.Code, w.Body)
	}
	if w, v := patch(`{"memo": null}`); w.Code != 200 || v.Memo != nil {
		t.Errorf("clear memo with null: %d %s", w.Code, w.Body)
	}
	patch(`{"memo": "また書く"}`)
	if w, v := patch(`{"memo": "  "}`); w.Code != 200 || v.Memo != nil {
		t.Errorf("clear memo with blank: %d %s", w.Code, w.Body)
	}

	w, _ := patch(`{"visitedAt": "2026-02-01T00:00:00Z"}`)
	e.WantError(w, http.StatusConflict, "duplicate_visit")

	for name, body := range map[string]string{
		"future":    `{"visitedAt": "2100-01-01T00:00:00Z"}`,
		"long memo": `{"memo": "` + strings.Repeat("あ", visits.MaxMemoLength+1) + `"}`,
	} {
		if w, _ := patch(body); w.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: status = %d", name, w.Code)
		}
	}
	// The spot of a visit cannot be changed.
	if w, _ := patch(`{"spotId": "` + e.spotB + `"}`); w.Code != http.StatusBadRequest {
		t.Errorf("spotId: status = %d", w.Code)
	}

	t.Run("not found", func(t *testing.T) {
		_, other := e.NewUser("other")
		for name, req := range map[string]apitest.Request{
			"unknown":        {Path: "/api/me/visits/00000000-0000-4000-8000-000000000000", Cookie: e.cookie},
			"malformed":      {Path: "/api/me/visits/abc", Cookie: e.cookie},
			"another user's": {Path: path, Cookie: other},
		} {
			req.Method, req.Body = "PATCH", `{"memo": "乗っ取り"}`
			w := e.Do(req)
			if w.Code != http.StatusNotFound {
				t.Errorf("%s: status = %d", name, w.Code)
			}
		}
		if n := e.Count(`SELECT count(*) FROM visits WHERE memo = '乗っ取り'`); n != 0 {
			t.Error("a visit was changed")
		}
	})

	if n := e.Count(`SELECT count(*) FROM visits WHERE id = $1 AND visited_at = '2026-02-01T00:00:00Z' AND memo IS NULL`, otherID); n != 1 {
		t.Error("the other visit changed")
	}
}

func TestDelete(t *testing.T) {
	e := newEnv(t)
	_, res := e.post(`[
		{"spotId": "` + e.spotA + `", "visitedAt": "2026-01-01T00:00:00Z"},
		{"spotId": "` + e.spotA + `", "visitedAt": "2026-02-01T00:00:00Z"}]`)
	path := "/api/me/visits/" + res.Visits[0].ID
	_, other := e.NewUser("other")

	for name, req := range map[string]apitest.Request{
		"another user's": {Path: path, Cookie: other},
		"unknown":        {Path: "/api/me/visits/00000000-0000-4000-8000-000000000000", Cookie: e.cookie},
		"malformed":      {Path: "/api/me/visits/abc", Cookie: e.cookie},
	} {
		req.Method = "DELETE"
		if w := e.Do(req); w.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d", name, w.Code)
		}
	}
	if n := e.Count(`SELECT count(*) FROM visits`); n != 2 {
		t.Fatalf("visits = %d, want 2", n)
	}

	if w := e.Do(apitest.Request{Method: "DELETE", Path: path, Cookie: e.cookie}); w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body)
	}
	if got := e.list(e.cookie); len(got) != 1 || got[0].ID != res.Visits[1].ID {
		t.Errorf("visits = %+v", got)
	}
	if w := e.Do(apitest.Request{Method: "DELETE", Path: path, Cookie: e.cookie}); w.Code != http.StatusNotFound {
		t.Errorf("second delete = %d, want 404", w.Code)
	}
}

func TestCrossSiteRequestsAreRejected(t *testing.T) {
	e := newEnv(t)
	w := e.Do(apitest.Request{Method: "POST", Path: "/api/me/visits", Cookie: e.cookie,
		Body: `[{"spotId": "` + e.spotA + `"}]`, Header: []string{"Origin", "https://evil.example"}})
	e.WantError(w, http.StatusForbidden, "forbidden_origin")
	w = e.Do(apitest.Request{Method: "POST", Path: "/api/me/visits", Cookie: e.cookie,
		Body: `[{"spotId": "` + e.spotA + `"}]`, Header: []string{"Content-Type", "text/plain"}})
	e.WantError(w, http.StatusUnsupportedMediaType, "unsupported_media_type")
	if n := e.Count(`SELECT count(*) FROM visits`); n != 0 {
		t.Errorf("visits = %d", n)
	}
}
