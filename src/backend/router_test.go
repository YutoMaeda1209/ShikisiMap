package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/config"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/db/dbtest"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestHealth(t *testing.T) {
	pool := dbtest.New(t)
	r, err := newRouter(config.Config{PublicOrigin: "https://map.example"}, pool)
	if err != nil {
		t.Fatal(err)
	}

	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w
	}

	for _, path := range []string{"/healthz", "/api/healthz"} {
		t.Run(path, func(t *testing.T) {
			w := get(path)
			if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != `{"status":"ok"}` {
				t.Errorf("status = %d, body = %s", w.Code, w.Body)
			}
		})
	}

	// Runs last: it closes the pool.
	t.Run("database down", func(t *testing.T) {
		pool.Close()
		for _, path := range []string{"/healthz", "/api/healthz"} {
			w := get(path)
			if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), `"code":"db_unavailable"`) {
				t.Errorf("%s: status = %d, body = %s", path, w.Code, w.Body)
			}
		}
	})
}

// TestRoutes checks that every endpoint of the spec is registered, by
// telling a 404 or 405 of the router apart from the answer of a handler.
func TestRoutes(t *testing.T) {
	pool := dbtest.New(t)
	r, err := newRouter(config.Config{PublicOrigin: "https://map.example"}, pool)
	if err != nil {
		t.Fatal(err)
	}
	const id = "00000000-0000-4000-8000-000000000000"
	routes := []struct {
		method, path string
		want         int
	}{
		{"GET", "/api/spots", 200},
		{"GET", "/api/auth/google/login", 503}, // not configured in this test
		{"GET", "/api/auth/google/callback", 302},
		{"DELETE", "/api/session", 401},
		{"GET", "/api/me", 401},
		{"PATCH", "/api/me", 401},
		{"DELETE", "/api/me", 401},
		{"GET", "/api/me/visits", 401},
		{"POST", "/api/me/visits", 401},
		{"PATCH", "/api/me/visits/" + id, 401},
		{"DELETE", "/api/me/visits/" + id, 401},
		{"GET", "/api/users/" + id, 404},
	}
	for _, rt := range routes {
		body := strings.NewReader("")
		if rt.method == "POST" || rt.method == "PATCH" {
			body = strings.NewReader("{}")
		}
		req := httptest.NewRequest(rt.method, rt.path, body)
		req.Header.Set("Origin", "https://map.example")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != rt.want {
			t.Errorf("%s %s = %d, want %d (body %s)", rt.method, rt.path, w.Code, rt.want, w.Body)
		}
		if w.Code == 404 && !strings.Contains(w.Body.String(), "no such user") {
			t.Errorf("%s %s is not registered", rt.method, rt.path)
		}

		// Requests that change data are refused from other origins.
		if rt.method != "GET" {
			req := httptest.NewRequest(rt.method, rt.path, strings.NewReader("{}"))
			req.Header.Set("Origin", "https://evil.example")
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusForbidden {
				t.Errorf("cross-site %s %s = %d, want 403", rt.method, rt.path, w.Code)
			}
		}
	}
}
