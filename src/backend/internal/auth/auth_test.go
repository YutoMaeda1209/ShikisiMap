package auth_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/auth"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/db/dbtest"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/httpx"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/oidctest"
)

const (
	origin   = "https://map.example"
	clientID = "client-id"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type env struct {
	t        *testing.T
	pool     *pgxpool.Pool
	provider *oidctest.Provider
	svc      *auth.Service
	router   *gin.Engine
	// claims are what the fake provider puts in the ID token. The nonce is
	// filled in from the login request.
	claims map[string]any
	// tokenRequests counts the calls to the token endpoint.
	tokenRequests int
}

func newEnv(t *testing.T, opts auth.Options) *env {
	t.Helper()
	e := &env{t: t, pool: dbtest.New(t), provider: oidctest.New(t)}
	e.claims = map[string]any{
		"iss": e.provider.Issuer, "aud": clientID, "sub": "google-sub-1",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
		"email": "tetra@example.com", "name": "てとら",
	}

	opts.Pool = e.pool
	opts.Issuer = e.provider.Issuer
	if opts.PublicOrigin == "" {
		opts.PublicOrigin = origin
	}
	e.svc = auth.New(opts)

	r, err := httpx.NewEngine()
	if err != nil {
		t.Fatal(err)
	}
	api := r.Group("/api", httpx.RequireSameOrigin(opts.PublicOrigin), httpx.RequireJSON())
	e.svc.Register(api)
	api.GET("/whoami", e.svc.RequireUser(), func(c *gin.Context) {
		c.String(http.StatusOK, auth.UserID(c))
	})
	e.router = r
	return e
}

func configured() auth.Options {
	return auth.Options{ClientID: clientID, ClientSecret: "client-secret"}
}

func (e *env) do(method, target string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	e.t.Helper()
	req := httptest.NewRequest(method, target, nil)
	if method != http.MethodGet {
		req.Header.Set("Origin", origin)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	return w
}

func cookie(w *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range w.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func (e *env) count(query string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

// startLogin calls the login endpoint and returns the flow cookie and the
// parameters sent to the provider.
func (e *env) startLogin() (*http.Cookie, url.Values) {
	e.t.Helper()
	w := e.do(http.MethodGet, "/api/auth/google/login")
	if w.Code != http.StatusFound {
		e.t.Fatalf("login: status = %d, body = %s", w.Code, w.Body)
	}
	loc, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		e.t.Fatal(err)
	}
	if got := loc.Scheme + "://" + loc.Host + loc.Path; got != e.provider.Issuer+"/authorize" {
		e.t.Errorf("redirected to %s", got)
	}
	flow := cookie(w, "shikisimap_login")
	if flow == nil {
		e.t.Fatal("login set no flow cookie")
	}

	// The provider checks PKCE and echoes the nonce, like Google does.
	params := loc.Query()
	e.provider.Token = func(w http.ResponseWriter, r *http.Request) {
		e.tokenRequests++
		sum := sha256.Sum256([]byte(r.PostFormValue("code_verifier")))
		if base64.RawURLEncoding.EncodeToString(sum[:]) != params.Get("code_challenge") || r.PostFormValue("code") != "good-code" {
			http.Error(w, `{"error": "invalid_grant"}`, http.StatusBadRequest)
			return
		}
		claims := map[string]any{"nonce": params.Get("nonce")}
		for k, v := range e.claims {
			claims[k] = v
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "access", "token_type": "Bearer", "expires_in": 3600,
			"id_token": e.provider.Sign(e.t, claims),
		})
	}
	return flow, params
}

func (e *env) callback(flow *http.Cookie, query string) *httptest.ResponseRecorder {
	e.t.Helper()
	var cookies []*http.Cookie
	if flow != nil {
		cookies = append(cookies, flow)
	}
	w := e.do(http.MethodGet, "/api/auth/google/callback?"+query, cookies...)
	if w.Code != http.StatusFound {
		e.t.Fatalf("callback: status = %d, body = %s", w.Code, w.Body)
	}
	return w
}

// login runs the whole flow and returns the session cookie.
func (e *env) login() *http.Cookie {
	e.t.Helper()
	flow, params := e.startLogin()
	w := e.callback(flow, "code=good-code&state="+params.Get("state"))
	if loc := w.Header().Get("Location"); loc != origin+"/" {
		e.t.Fatalf("callback redirected to %s", loc)
	}
	session := cookie(w, auth.SessionCookieName)
	if session == nil {
		e.t.Fatal("callback set no session cookie")
	}
	return session
}

func TestLoginRedirect(t *testing.T) {
	e := newEnv(t, configured())
	flow, params := e.startLogin()

	if params.Get("client_id") != clientID || params.Get("response_type") != "code" {
		t.Errorf("params = %v", params)
	}
	if params.Get("redirect_uri") != origin+"/api/auth/google/callback" {
		t.Errorf("redirect_uri = %s", params.Get("redirect_uri"))
	}
	if params.Get("scope") != "openid email profile" {
		t.Errorf("scope = %q", params.Get("scope"))
	}
	if params.Get("code_challenge_method") != "S256" || len(params.Get("code_challenge")) < 43 {
		t.Errorf("PKCE params = %v", params)
	}
	if len(params.Get("state")) < 43 || len(params.Get("nonce")) < 43 {
		t.Errorf("state = %q, nonce = %q; want 32 random bytes each", params.Get("state"), params.Get("nonce"))
	}

	if !flow.HttpOnly || !flow.Secure || flow.SameSite != http.SameSiteLaxMode || flow.Path != "/api/auth/google" || flow.MaxAge != 600 {
		t.Errorf("flow cookie = %+v", flow)
	}
	// The verifier must stay out of the URL.
	verifier := flow.Value[strings.LastIndex(flow.Value, ".")+1:]
	if strings.Contains(e.do(http.MethodGet, "/api/auth/google/login").Header().Get("Location"), verifier) {
		t.Error("the redirect URL contains the PKCE verifier")
	}

	_, params2 := e.startLogin()
	if params2.Get("state") == params.Get("state") {
		t.Error("two logins got the same state")
	}
}

func TestLoginCreatesUserAndSession(t *testing.T) {
	e := newEnv(t, configured())
	session := e.login()

	if !session.HttpOnly || !session.Secure || session.SameSite != http.SameSiteLaxMode || session.Path != "/api" {
		t.Errorf("session cookie = %+v", session)
	}
	if session.MaxAge != 30*24*60*60 {
		t.Errorf("session cookie Max-Age = %d, want 30 days", session.MaxAge)
	}
	if raw, err := base64.RawURLEncoding.DecodeString(session.Value); err != nil || len(raw) != 32 {
		t.Errorf("session ID %q is not 32 random bytes", session.Value)
	}

	var userID, email, name, tokenHash string
	var public bool
	err := e.pool.QueryRow(context.Background(), `
		SELECT u.id, u.email, u.display_name, u.profile_public, s.token_hash
		FROM users u JOIN sessions s ON s.user_id = u.id WHERE u.google_sub = 'google-sub-1'`,
	).Scan(&userID, &email, &name, &public, &tokenHash)
	if err != nil {
		t.Fatal(err)
	}
	if email != "tetra@example.com" || name != "てとら" || public {
		t.Errorf("user = %s, %s, public %v", email, name, public)
	}
	if tokenHash == session.Value || strings.Contains(tokenHash, session.Value) || len(tokenHash) != 64 {
		t.Errorf("stored token hash %q must be a SHA-256 of the session ID", tokenHash)
	}

	w := e.do(http.MethodGet, "/api/whoami", session)
	if w.Code != 200 || w.Body.String() != userID {
		t.Errorf("whoami = %d %s, want %s", w.Code, w.Body, userID)
	}
}

func TestSecondLoginReusesUser(t *testing.T) {
	e := newEnv(t, configured())
	first := e.login()
	if _, err := e.pool.Exec(context.Background(), `UPDATE users SET display_name = '編集済み'`); err != nil {
		t.Fatal(err)
	}

	e.claims["email"] = "new@example.com"
	e.claims["name"] = "Googleの新しい名前"
	second := e.login()

	if first.Value == second.Value {
		t.Error("the second login reused the session ID")
	}
	if n := e.count(`SELECT count(*) FROM users`); n != 1 {
		t.Errorf("users = %d, want 1", n)
	}
	if n := e.count(`SELECT count(*) FROM users WHERE email = 'new@example.com' AND display_name = '編集済み'`); n != 1 {
		t.Error("want the email updated and the edited display name kept")
	}
	// Both devices stay logged in.
	for _, s := range []*http.Cookie{first, second} {
		if w := e.do(http.MethodGet, "/api/whoami", s); w.Code != 200 {
			t.Errorf("whoami = %d", w.Code)
		}
	}

	e.claims["sub"] = "google-sub-2"
	e.login()
	if n := e.count(`SELECT count(*) FROM users`); n != 2 {
		t.Errorf("users = %d after another account logged in, want 2", n)
	}
}

func TestInitialDisplayName(t *testing.T) {
	tests := []struct {
		name, email, want string
	}{
		{"", "tetra@example.com", "tetra"},
		{"  ", "", "ユーザー"},
		{strings.Repeat("あ", 80), "a@example.com", strings.Repeat("あ", 50)},
	}
	for i, tc := range tests {
		e := newEnv(t, configured())
		e.claims["name"], e.claims["email"] = tc.name, tc.email
		e.login()
		var got string
		if err := e.pool.QueryRow(context.Background(), `SELECT display_name FROM users`).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Errorf("case %d: display name = %q, want %q", i, got, tc.want)
		}
	}
}

func TestCallbackRejects(t *testing.T) {
	type setup struct {
		query    func(params url.Values) string
		noCookie bool
		prepare  func(e *env)
	}
	good := func(p url.Values) string { return "code=good-code&state=" + p.Get("state") }
	tests := map[string]setup{
		"wrong state":      {query: func(url.Values) string { return "code=good-code&state=forged" }},
		"missing state":    {query: func(url.Values) string { return "code=good-code" }},
		"missing code":     {query: func(p url.Values) string { return "state=" + p.Get("state") }},
		"denied by user":   {query: func(p url.Values) string { return "error=access_denied&state=" + p.Get("state") }},
		"no flow cookie":   {query: good, noCookie: true},
		"wrong code":       {query: func(p url.Values) string { return "code=bad-code&state=" + p.Get("state") }},
		"wrong audience":   {query: good, prepare: func(e *env) { e.claims["aud"] = "another-client" }},
		"wrong issuer":     {query: good, prepare: func(e *env) { e.claims["iss"] = "https://evil.example" }},
		"expired ID token": {query: good, prepare: func(e *env) { e.claims["exp"] = time.Now().Add(-time.Hour).Unix() }},
		"wrong nonce":      {query: good, prepare: func(e *env) { e.claims["nonce"] = "forged" }},
		"no sub":           {query: good, prepare: func(e *env) { delete(e.claims, "sub") }},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, configured())
			flow, params := e.startLogin()
			if tc.prepare != nil {
				tc.prepare(e)
			}
			if tc.noCookie {
				flow = nil
			}
			w := e.callback(flow, tc.query(params))

			if loc := w.Header().Get("Location"); loc != origin+"/?login=failed" {
				t.Errorf("redirected to %s", loc)
			}
			if cookie(w, auth.SessionCookieName) != nil {
				t.Error("a session cookie was set")
			}
			if n := e.count(`SELECT (SELECT count(*) FROM users) + (SELECT count(*) FROM sessions)`); n != 0 {
				t.Errorf("%d users or sessions were created", n)
			}
			if c := cookie(w, "shikisimap_login"); c == nil || c.MaxAge >= 0 {
				t.Errorf("flow cookie = %+v, want it cleared", c)
			}
		})
	}

	t.Run("ID token signed by another key", func(t *testing.T) {
		e := newEnv(t, configured())
		flow, params := e.startLogin()
		e.provider.Token = func(w http.ResponseWriter, _ *http.Request) {
			claims := map[string]any{"nonce": params.Get("nonce")}
			for k, v := range e.claims {
				claims[k] = v
			}
			json.NewEncoder(w).Encode(map[string]any{
				"access_token": "access", "token_type": "Bearer",
				"id_token": e.provider.SignWithOtherKey(t, claims),
			})
		}
		w := e.callback(flow, "code=good-code&state="+params.Get("state"))
		if loc := w.Header().Get("Location"); loc != origin+"/?login=failed" || e.count(`SELECT count(*) FROM users`) != 0 {
			t.Errorf("redirected to %s", loc)
		}
	})

	t.Run("flow cookie cannot be replayed", func(t *testing.T) {
		e := newEnv(t, configured())
		flow, params := e.startLogin()
		w := e.callback(flow, "code=good-code&state="+params.Get("state"))
		if c := cookie(w, "shikisimap_login"); c == nil || c.MaxAge >= 0 {
			t.Errorf("flow cookie = %+v after a successful login, want it cleared", c)
		}
	})
}

func TestLoginUnavailableWithoutCredentials(t *testing.T) {
	e := newEnv(t, auth.Options{})
	w := e.do(http.MethodGet, "/api/auth/google/login")
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), `"code":"login_unavailable"`) {
		t.Errorf("status = %d, body = %s", w.Code, w.Body)
	}
	w = e.callback(nil, "code=x&state=y")
	if loc := w.Header().Get("Location"); loc != origin+"/?login=failed" {
		t.Errorf("redirected to %s", loc)
	}
}

func TestCookiesAreNotSecureOverHTTP(t *testing.T) {
	opts := configured()
	opts.PublicOrigin = "http://localhost:5173"
	e := newEnv(t, opts)
	w := e.do(http.MethodGet, "/api/auth/google/login")
	if c := cookie(w, "shikisimap_login"); c == nil || c.Secure {
		t.Errorf("flow cookie = %+v, want it without Secure for local development", c)
	}
}

func TestRequireUser(t *testing.T) {
	e := newEnv(t, configured())
	session := e.login()
	ctx := context.Background()

	unauthorized := func(name string, cookies ...*http.Cookie) {
		t.Helper()
		w := e.do(http.MethodGet, "/api/whoami", cookies...)
		if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), `"code":"unauthorized"`) {
			t.Errorf("%s: status = %d, body = %s", name, w.Code, w.Body)
		}
	}
	unauthorized("no cookie")
	unauthorized("unknown session", &http.Cookie{Name: auth.SessionCookieName, Value: "unknown"})
	unauthorized("empty session", &http.Cookie{Name: auth.SessionCookieName, Value: ""})
	// The hash itself must not work as a session ID.
	var hash string
	if err := e.pool.QueryRow(ctx, `SELECT token_hash FROM sessions`).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	unauthorized("token hash", &http.Cookie{Name: auth.SessionCookieName, Value: hash})

	t.Run("fresh session is not written to", func(t *testing.T) {
		var before, after time.Time
		e.pool.QueryRow(ctx, `SELECT last_used_at FROM sessions`).Scan(&before)
		w := e.do(http.MethodGet, "/api/whoami", session)
		e.pool.QueryRow(ctx, `SELECT last_used_at FROM sessions`).Scan(&after)
		if w.Code != 200 || !before.Equal(after) {
			t.Errorf("status = %d, last_used_at %v -> %v", w.Code, before, after)
		}
		if cookie(w, auth.SessionCookieName) != nil {
			t.Error("the cookie was reissued")
		}
	})

	t.Run("stale session is extended", func(t *testing.T) {
		if _, err := e.pool.Exec(ctx, `
			UPDATE sessions SET last_used_at = now() - interval '2 hours', expires_at = now() + interval '1 day'`); err != nil {
			t.Fatal(err)
		}
		w := e.do(http.MethodGet, "/api/whoami", session)
		if w.Code != 200 {
			t.Fatalf("status = %d", w.Code)
		}
		if n := e.count(`
			SELECT count(*) FROM sessions
			WHERE last_used_at > now() - interval '1 minute'
			  AND expires_at BETWEEN now() + interval '29 days 23 hours' AND now() + interval '30 days 1 hour'`); n != 1 {
			t.Error("want last_used_at = now and expires_at = now + 30 days")
		}
		c := cookie(w, auth.SessionCookieName)
		if c == nil || c.Value != session.Value || c.MaxAge != 30*24*60*60 || !c.HttpOnly || c.Path != "/api" {
			t.Errorf("reissued cookie = %+v", c)
		}
	})

	t.Run("expired session is rejected", func(t *testing.T) {
		if _, err := e.pool.Exec(ctx, `UPDATE sessions SET expires_at = now() - interval '1 second'`); err != nil {
			t.Fatal(err)
		}
		unauthorized("expired", session)
	})
}

func TestLogout(t *testing.T) {
	e := newEnv(t, configured())
	session := e.login()
	other := e.login()

	w := e.do(http.MethodDelete, "/api/session", session)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body)
	}
	if c := cookie(w, auth.SessionCookieName); c == nil || c.MaxAge >= 0 || c.Path != "/api" {
		t.Errorf("cookie = %+v, want it cleared", c)
	}
	if w := e.do(http.MethodGet, "/api/whoami", session); w.Code != http.StatusUnauthorized {
		t.Errorf("the session still works after logout: %d", w.Code)
	}
	// Only the current session ends.
	if w := e.do(http.MethodGet, "/api/whoami", other); w.Code != 200 {
		t.Errorf("the other session stopped working: %d", w.Code)
	}

	if w := e.do(http.MethodDelete, "/api/session"); w.Code != http.StatusUnauthorized {
		t.Errorf("logout without a session = %d, want 401", w.Code)
	}

	// Logout is protected against cross-site requests.
	req := httptest.NewRequest(http.MethodDelete, "/api/session", nil)
	req.Header.Set("Origin", "https://evil.example")
	req.AddCookie(other)
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("cross-site logout = %d, want 403", rec.Code)
	}
}

func TestCreateSession(t *testing.T) {
	e := newEnv(t, configured())
	var userID string
	if err := e.pool.QueryRow(context.Background(), `
		INSERT INTO users (google_sub, email, display_name) VALUES ('s', 'e', 'n') RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	token, err := e.svc.CreateSession(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	w := e.do(http.MethodGet, "/api/whoami", &http.Cookie{Name: auth.SessionCookieName, Value: token})
	if w.Code != 200 || w.Body.String() != userID {
		t.Errorf("whoami = %d %s", w.Code, w.Body)
	}
}
