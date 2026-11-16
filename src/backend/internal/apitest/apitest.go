// Package apitest sets up the API on a test database for handler tests.
package apitest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/auth"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/db/dbtest"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/httpx"
)

// Origin is the public origin the test API is configured with.
const Origin = "https://map.example"

// Env is an API on its own database.
type Env struct {
	T      *testing.T
	Pool   *pgxpool.Pool
	Auth   *auth.Service
	Router *gin.Engine
	// API is the group to register the endpoints under test on. It has the
	// same middleware as in production.
	API *gin.RouterGroup
}

// New creates an Env. The test is skipped without a test database.
func New(t *testing.T) *Env {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pool := dbtest.New(t)
	r, err := httpx.NewEngine()
	if err != nil {
		t.Fatal(err)
	}
	return &Env{
		T:      t,
		Pool:   pool,
		Auth:   auth.New(auth.Options{Pool: pool, PublicOrigin: Origin}),
		Router: r,
		API:    r.Group("/api", httpx.RequireSameOrigin(Origin), httpx.RequireJSON()),
	}
}

// Exec runs a statement and fails the test on error.
func (e *Env) Exec(sql string, args ...any) {
	e.T.Helper()
	if _, err := e.Pool.Exec(context.Background(), sql, args...); err != nil {
		e.T.Fatalf("%s: %v", sql, err)
	}
}

// Scan runs a query returning one row and scans it into dest.
func (e *Env) Scan(sql string, args []any, dest ...any) {
	e.T.Helper()
	if err := e.Pool.QueryRow(context.Background(), sql, args...).Scan(dest...); err != nil {
		e.T.Fatalf("%s: %v", sql, err)
	}
}

// Count runs a query returning one integer.
func (e *Env) Count(sql string, args ...any) int {
	e.T.Helper()
	var n int
	e.Scan(sql, args, &n)
	return n
}

// NewUser creates a user and a session, and returns the user's ID and the
// session cookie.
func (e *Env) NewUser(name string) (string, *http.Cookie) {
	e.T.Helper()
	var id string
	e.Scan(`INSERT INTO users (google_sub, email, display_name) VALUES ($1, $1 || '@example.com', $1) RETURNING id`,
		[]any{name}, &id)
	token, err := e.Auth.CreateSession(context.Background(), id)
	if err != nil {
		e.T.Fatal(err)
	}
	return id, &http.Cookie{Name: auth.SessionCookieName, Value: token}
}

// NewSpot creates a spot and returns its ID.
func (e *Env) NewSpot(name string) string {
	e.T.Helper()
	var id string
	e.Scan(`INSERT INTO spots (name, address, location) VALUES ($1, '住所', 'POINT(139.7 35.6)') RETURNING id`,
		[]any{name}, &id)
	return id
}

// Request describes a request to the test API.
type Request struct {
	Method string
	Path   string
	// Body is sent as JSON when it is not empty.
	Body   string
	Cookie *http.Cookie
	// Header holds extra headers as key, value pairs.
	Header []string
}

// Do sends a request the way the site's own frontend would: with the Origin
// header on requests that change data.
func (e *Env) Do(req Request) *httptest.ResponseRecorder {
	e.T.Helper()
	r := httptest.NewRequest(req.Method, req.Path, strings.NewReader(req.Body))
	if req.Method != http.MethodGet {
		r.Header.Set("Origin", Origin)
	}
	if req.Body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	if req.Cookie != nil {
		r.AddCookie(req.Cookie)
	}
	for i := 0; i+1 < len(req.Header); i += 2 {
		r.Header.Set(req.Header[i], req.Header[i+1])
	}
	w := httptest.NewRecorder()
	e.Router.ServeHTTP(w, r)
	return w
}

// WantError checks that the response is an error with the status and code.
func (e *Env) WantError(w *httptest.ResponseRecorder, status int, code string) {
	e.T.Helper()
	if w.Code != status || !strings.Contains(w.Body.String(), `"code":"`+code+`"`) {
		e.T.Errorf("status = %d, body = %s; want %d %s", w.Code, w.Body, status, code)
	}
}
