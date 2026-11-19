package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

const origin = "https://map.example"

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestEngine(t *testing.T) *gin.Engine {
	t.Helper()
	r, err := NewEngine()
	if err != nil {
		t.Fatal(err)
	}
	api := r.Group("/api", RequireSameOrigin(origin), RequireJSON())
	ok := func(c *gin.Context) { c.Status(http.StatusNoContent) }
	api.GET("/things", ok)
	api.POST("/things", ok)
	api.PUT("/things/:id", ok)
	api.PATCH("/things/:id", ok)
	api.DELETE("/things/:id", ok)
	api.GET("/panic", func(*gin.Context) { panic("boom") })
	api.GET("/fail", func(c *gin.Context) { InternalError(c, errors.New("secret detail")) })
	return r
}

type request struct {
	method      string
	path        string
	origin      string
	contentType string
	body        string
}

func do(t *testing.T, r http.Handler, req request) *httptest.ResponseRecorder {
	t.Helper()
	hr := httptest.NewRequest(req.method, req.path, strings.NewReader(req.body))
	if req.origin != "" {
		hr.Header.Set("Origin", req.origin)
	}
	if req.contentType != "" {
		hr.Header.Set("Content-Type", req.contentType)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, hr)
	return w
}

// wantError checks the status and that the body is the common error format
// with the given code.
func wantError(t *testing.T, w *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if w.Code != status {
		t.Errorf("status = %d, want %d (body %s)", w.Code, status, w.Body)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var got map[string]map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("body %q is not the error format: %v", w.Body, err)
	}
	if len(got) != 1 || len(got["error"]) != 2 {
		t.Errorf("body = %s, want only error.code and error.message", w.Body)
	}
	if got["error"]["code"] != code {
		t.Errorf("error.code = %q, want %q", got["error"]["code"], code)
	}
	if got["error"]["message"] == "" {
		t.Error("error.message is empty")
	}
}

func TestOriginCheck(t *testing.T) {
	r := newTestEngine(t)

	rejected := []struct {
		name   string
		origin string
	}{
		{"missing", ""},
		{"other site", "https://evil.example"},
		{"other scheme", "http://map.example"},
		{"other port", "https://map.example:8443"},
		{"subdomain", "https://sub.map.example"},
		{"suffix", "https://map.example.evil.example"},
		{"null", "null"},
	}
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		path := "/api/things"
		if method != http.MethodPost {
			path = "/api/things/1"
		}
		for _, tc := range rejected {
			t.Run(method+" "+tc.name, func(t *testing.T) {
				w := do(t, r, request{method: method, path: path, origin: tc.origin, contentType: "application/json", body: "{}"})
				wantError(t, w, http.StatusForbidden, CodeForbiddenOrigin)
			})
		}
		t.Run(method+" same origin", func(t *testing.T) {
			w := do(t, r, request{method: method, path: path, origin: origin, contentType: "application/json", body: "{}"})
			if w.Code != http.StatusNoContent {
				t.Errorf("status = %d, want 204 (body %s)", w.Code, w.Body)
			}
		})
	}

	t.Run("GET needs no Origin", func(t *testing.T) {
		for _, o := range []string{"", "https://evil.example"} {
			w := do(t, r, request{method: http.MethodGet, path: "/api/things", origin: o})
			if w.Code != http.StatusNoContent {
				t.Errorf("Origin %q: status = %d, want 204", o, w.Code)
			}
		}
	})
}

func TestContentTypeCheck(t *testing.T) {
	r := newTestEngine(t)

	tests := []struct {
		name        string
		method      string
		path        string
		contentType string
		body        string
		wantStatus  int
	}{
		{"POST json", http.MethodPost, "/api/things", "application/json", "{}", http.StatusNoContent},
		{"POST json with charset", http.MethodPost, "/api/things", "application/json; charset=utf-8", "{}", http.StatusNoContent},
		{"POST json in upper case", http.MethodPost, "/api/things", "Application/JSON", "{}", http.StatusNoContent},
		{"POST form", http.MethodPost, "/api/things", "application/x-www-form-urlencoded", "a=1", http.StatusUnsupportedMediaType},
		{"POST multipart", http.MethodPost, "/api/things", "multipart/form-data; boundary=x", "x", http.StatusUnsupportedMediaType},
		{"POST text", http.MethodPost, "/api/things", "text/plain", "{}", http.StatusUnsupportedMediaType},
		{"POST json-like", http.MethodPost, "/api/things", "application/jsonx", "{}", http.StatusUnsupportedMediaType},
		{"POST no content type", http.MethodPost, "/api/things", "", "{}", http.StatusUnsupportedMediaType},
		{"POST empty body, no content type", http.MethodPost, "/api/things", "", "", http.StatusUnsupportedMediaType},
		{"PUT text", http.MethodPut, "/api/things/1", "text/plain", "{}", http.StatusUnsupportedMediaType},
		{"PATCH json", http.MethodPatch, "/api/things/1", "application/json", "{}", http.StatusNoContent},
		{"PATCH text", http.MethodPatch, "/api/things/1", "text/plain", "{}", http.StatusUnsupportedMediaType},
		{"DELETE without body", http.MethodDelete, "/api/things/1", "", "", http.StatusNoContent},
		{"DELETE json body", http.MethodDelete, "/api/things/1", "application/json", "{}", http.StatusNoContent},
		{"DELETE text body", http.MethodDelete, "/api/things/1", "text/plain", "{}", http.StatusUnsupportedMediaType},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := do(t, r, request{method: tc.method, path: tc.path, origin: origin, contentType: tc.contentType, body: tc.body})
			if tc.wantStatus == http.StatusNoContent {
				if w.Code != http.StatusNoContent {
					t.Errorf("status = %d, want 204 (body %s)", w.Code, w.Body)
				}
				return
			}
			wantError(t, w, tc.wantStatus, CodeUnsupportedMediaType)
		})
	}

	t.Run("GET needs no Content-Type", func(t *testing.T) {
		w := do(t, r, request{method: http.MethodGet, path: "/api/things", contentType: "text/plain"})
		if w.Code != http.StatusNoContent {
			t.Errorf("status = %d, want 204", w.Code)
		}
	})
}

func TestEngineErrorsUseCommonFormat(t *testing.T) {
	r := newTestEngine(t)

	t.Run("unknown route", func(t *testing.T) {
		w := do(t, r, request{method: http.MethodGet, path: "/api/nothing"})
		wantError(t, w, http.StatusNotFound, CodeNotFound)
	})
	t.Run("unknown route outside the API", func(t *testing.T) {
		w := do(t, r, request{method: http.MethodGet, path: "/"})
		wantError(t, w, http.StatusNotFound, CodeNotFound)
	})
	t.Run("wrong method", func(t *testing.T) {
		w := do(t, r, request{method: http.MethodDelete, path: "/api/things", origin: origin})
		wantError(t, w, http.StatusMethodNotAllowed, CodeMethodNotAllowed)
	})
	t.Run("panic", func(t *testing.T) {
		w := do(t, r, request{method: http.MethodGet, path: "/api/panic"})
		wantError(t, w, http.StatusInternalServerError, CodeInternal)
	})
	t.Run("internal error hides the cause", func(t *testing.T) {
		w := do(t, r, request{method: http.MethodGet, path: "/api/fail"})
		wantError(t, w, http.StatusInternalServerError, CodeInternal)
		if strings.Contains(w.Body.String(), "secret detail") {
			t.Errorf("body %s exposes the internal error", w.Body)
		}
	})
}
