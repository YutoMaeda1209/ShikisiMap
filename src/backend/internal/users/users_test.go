package users_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/apitest"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/auth"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/users"
)

func newEnv(t *testing.T) *apitest.Env {
	e := apitest.New(t)
	users.Register(e.API, e.Pool, e.Auth)
	return e
}

func TestMeRequiresLogin(t *testing.T) {
	e := newEnv(t)
	for _, req := range []apitest.Request{
		{Method: "GET", Path: "/api/me"},
		{Method: "PATCH", Path: "/api/me", Body: `{"displayName": "x"}`},
		{Method: "DELETE", Path: "/api/me"},
	} {
		e.WantError(e.Do(req), http.StatusUnauthorized, "unauthorized")
	}
}

func TestGetMe(t *testing.T) {
	e := newEnv(t)
	id, cookie := e.NewUser("tetra")
	e.NewUser("other")

	w := e.Do(apitest.Request{Method: "GET", Path: "/api/me", Cookie: cookie})
	want := `{"id":"` + id + `","email":"tetra@example.com","displayName":"tetra","profilePublic":false}`
	if w.Code != 200 || w.Body.String() != want {
		t.Errorf("status = %d, body = %s; want %s", w.Code, w.Body, want)
	}
	// Nothing internal, such as google_sub, leaks.
	if strings.Contains(w.Body.String(), "sub") {
		t.Errorf("body = %s", w.Body)
	}
}

func TestPatchMe(t *testing.T) {
	e := newEnv(t)
	_, cookie := e.NewUser("tetra")
	_, otherCookie := e.NewUser("other")

	patch := func(body string) (int, users.Me, string) {
		t.Helper()
		w := e.Do(apitest.Request{Method: "PATCH", Path: "/api/me", Body: body, Cookie: cookie})
		var me users.Me
		json.Unmarshal(w.Body.Bytes(), &me)
		return w.Code, me, w.Body.String()
	}

	if code, me, body := patch(`{"displayName": "  新しい名前  "}`); code != 200 || me.DisplayName != "新しい名前" || me.ProfilePublic {
		t.Errorf("rename: %d %s", code, body)
	}
	if code, me, body := patch(`{"profilePublic": true}`); code != 200 || me.DisplayName != "新しい名前" || !me.ProfilePublic {
		t.Errorf("publish: %d %s", code, body)
	}
	if code, me, body := patch(`{"displayName": "両方", "profilePublic": false}`); code != 200 || me.DisplayName != "両方" || me.ProfilePublic {
		t.Errorf("both: %d %s", code, body)
	}
	if code, me, body := patch(`{}`); code != 200 || me.DisplayName != "両方" {
		t.Errorf("empty patch: %d %s", code, body)
	}
	if code, me, _ := patch(`{"displayName": "` + strings.Repeat("あ", 50) + `"}`); code != 200 || len([]rune(me.DisplayName)) != 50 {
		t.Errorf("50 characters: %d", code)
	}

	for name, body := range map[string]string{
		"empty name":      `{"displayName": ""}`,
		"blank name":      `{"displayName": "   "}`,
		"too long":        `{"displayName": "` + strings.Repeat("あ", 51) + `"}`,
		"control char":    `{"displayName": "a\nb"}`,
		"null name":       `{"displayName": null}`,
		"number name":     `{"displayName": 1}`,
		"string public":   `{"profilePublic": "true"}`,
		"null public":     `{"profilePublic": null}`,
		"invalid with ok": `{"displayName": "ok", "profilePublic": 1}`,
	} {
		w := e.Do(apitest.Request{Method: "PATCH", Path: "/api/me", Body: body, Cookie: cookie})
		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: status = %d, body = %s; want 422", name, w.Code, w.Body)
		}
	}
	for name, body := range map[string]string{
		"unknown field": `{"email": "evil@example.com"}`,
		"not json":      `displayName=x`,
		"array":         `[]`,
		"trailing data": `{} {}`,
	} {
		w := e.Do(apitest.Request{Method: "PATCH", Path: "/api/me", Body: body, Cookie: cookie})
		e.WantError(w, http.StatusBadRequest, "invalid_request")
		_ = name
	}

	// Rejected requests changed nothing, and nobody else was touched.
	if n := e.Count(`SELECT count(*) FROM users WHERE display_name = $1 AND NOT profile_public AND email = 'tetra@example.com'`, strings.Repeat("あ", 50)); n != 1 {
		t.Error("the user is not in the expected state")
	}
	w := e.Do(apitest.Request{Method: "GET", Path: "/api/me", Cookie: otherCookie})
	if !strings.Contains(w.Body.String(), `"displayName":"other"`) {
		t.Errorf("the other user changed: %s", w.Body)
	}
}

func TestDeleteMe(t *testing.T) {
	e := newEnv(t)
	id, cookie := e.NewUser("tetra")
	_, secondDevice := e.NewUser("unused")
	e.Exec(`UPDATE sessions SET user_id = $1`, id) // both sessions belong to tetra
	otherID, otherCookie := e.NewUser("other")
	spot := e.NewSpot("spot")
	e.Exec(`INSERT INTO visits (user_id, spot_id) VALUES ($1, $3), ($2, $3)`, id, otherID, spot)

	w := e.Do(apitest.Request{Method: "DELETE", Path: "/api/me", Cookie: cookie})
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body)
	}
	cleared := false
	for _, c := range w.Result().Cookies() {
		cleared = cleared || (c.Name == auth.SessionCookieName && c.MaxAge < 0)
	}
	if !cleared {
		t.Error("the session cookie was not cleared")
	}

	if n := e.Count(`SELECT count(*) FROM users WHERE id = $1`, id); n != 0 {
		t.Error("the user remains")
	}
	if n := e.Count(`SELECT count(*) FROM visits`); n != 1 {
		t.Errorf("visits = %d, want only the other user's", n)
	}
	if n := e.Count(`SELECT count(*) FROM sessions`); n != 1 {
		t.Errorf("sessions = %d, want only the other user's", n)
	}
	if n := e.Count(`SELECT count(*) FROM spots`); n != 1 {
		t.Error("the spot was deleted")
	}
	// Every session of the deleted user stops working.
	for _, c := range []*http.Cookie{cookie, secondDevice} {
		e.WantError(e.Do(apitest.Request{Method: "GET", Path: "/api/me", Cookie: c}), http.StatusUnauthorized, "unauthorized")
	}
	if w := e.Do(apitest.Request{Method: "GET", Path: "/api/me", Cookie: otherCookie}); w.Code != 200 {
		t.Errorf("the other user: %d", w.Code)
	}
}

func TestPublicProfile(t *testing.T) {
	e := newEnv(t)
	id, _ := e.NewUser("tetra")
	privateID, _ := e.NewUser("private")
	spotA, spotB, spotC := e.NewSpot("A"), e.NewSpot("B"), e.NewSpot("C")
	e.Exec(`UPDATE users SET profile_public = true, display_name = 'てとら' WHERE id = $1`, id)
	// B first, then A twice. C is visited by the private user only.
	e.Exec(`INSERT INTO visits (user_id, spot_id, visited_at, memo) VALUES
		($1, $3, '2026-01-01T00:00:00Z', '秘密のメモ'),
		($1, $2, '2026-02-01T00:00:00Z', NULL),
		($1, $2, '2026-03-01T00:00:00Z', NULL),
		($5, $4, '2026-01-01T00:00:00Z', NULL)`, id, spotA, spotB, spotC, privateID)

	w := e.Do(apitest.Request{Method: "GET", Path: "/api/users/" + id})
	want := `{"id":"` + id + `","displayName":"てとら","visitedSpotIds":["` + spotB + `","` + spotA + `"]}`
	if w.Code != 200 || w.Body.String() != want {
		t.Errorf("status = %d, body = %s; want %s", w.Code, w.Body, want)
	}

	notFound := func(name, path string) {
		t.Helper()
		w := e.Do(apitest.Request{Method: "GET", Path: path})
		e.WantError(w, http.StatusNotFound, "not_found")
		if strings.Contains(w.Body.String(), "private") {
			t.Errorf("%s: body = %s", name, w.Body)
		}
	}
	notFound("private profile", "/api/users/"+privateID)
	notFound("unknown user", "/api/users/00000000-0000-4000-8000-000000000000")
	notFound("malformed id", "/api/users/not-a-uuid")
	// A private profile and a missing user are indistinguishable.
	a := e.Do(apitest.Request{Method: "GET", Path: "/api/users/" + privateID}).Body.String()
	b := e.Do(apitest.Request{Method: "GET", Path: "/api/users/00000000-0000-4000-8000-000000000000"}).Body.String()
	if a != b {
		t.Errorf("private: %s, unknown: %s", a, b)
	}

	t.Run("no visits", func(t *testing.T) {
		e.Exec(`UPDATE users SET profile_public = true WHERE id = $1`, privateID)
		e.Exec(`DELETE FROM visits WHERE user_id = $1`, privateID)
		w := e.Do(apitest.Request{Method: "GET", Path: "/api/users/" + privateID})
		if !strings.Contains(w.Body.String(), `"visitedSpotIds":[]`) {
			t.Errorf("body = %s, want an empty list", w.Body)
		}
	})
}
