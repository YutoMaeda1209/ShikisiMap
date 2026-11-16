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
