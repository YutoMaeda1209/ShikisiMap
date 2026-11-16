package places

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeAPI records the last request and answers with a fixed response.
type fakeAPI struct {
	status int
	body   string

	method, path, query, key, fieldMask, reqBody string
}

func (f *fakeAPI) client(t *testing.T) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		f.method, f.path, f.query = r.Method, r.URL.Path, r.URL.RawQuery
		f.key, f.fieldMask, f.reqBody = r.Header.Get("X-Goog-Api-Key"), r.Header.Get("X-Goog-FieldMask"), string(b)
		w.WriteHeader(f.status)
		io.WriteString(w, f.body)
	}))
	t.Cleanup(srv.Close)
	return NewClient("test-key").WithBaseURL(srv.URL)
}

func TestSearchText(t *testing.T) {
	f := &fakeAPI{status: 200, body: `{"places": [
		{"id": "ChIJ1", "displayName": {"text": "東京駅", "languageCode": "ja"},
		 "formattedAddress": "日本、東京都千代田区丸の内１丁目", "location": {"latitude": 35.6812, "longitude": 139.7671}},
		{"id": "ChIJ2", "displayName": {"text": "別の店"}, "formattedAddress": "住所", "location": {"latitude": 1, "longitude": 2}}
	]}`}
	got, err := f.client(t).SearchText(context.Background(), "東京駅 東京都千代田区")
	if err != nil {
		t.Fatal(err)
	}

	want := []Place{
		{ID: "ChIJ1", Name: "東京駅", Address: "日本、東京都千代田区丸の内１丁目", Location: LatLng{Lat: 35.6812, Lng: 139.7671}},
		{ID: "ChIJ2", Name: "別の店", Address: "住所", Location: LatLng{Lat: 1, Lng: 2}},
	}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("places = %+v, want %+v", got, want)
	}

	if f.method != "POST" || f.path != "/v1/places:searchText" {
		t.Errorf("request = %s %s", f.method, f.path)
	}
	if f.key != "test-key" {
		t.Errorf("X-Goog-Api-Key = %q", f.key)
	}
	if f.fieldMask != "places.id,places.displayName,places.formattedAddress,places.location" {
		t.Errorf("X-Goog-FieldMask = %q", f.fieldMask)
	}
	var body map[string]string
	if err := json.Unmarshal([]byte(f.reqBody), &body); err != nil {
		t.Fatal(err)
	}
	if body["textQuery"] != "東京駅 東京都千代田区" || body["languageCode"] != "ja" {
		t.Errorf("body = %v", body)
	}
}

func TestSearchTextNoResult(t *testing.T) {
	// The API omits "places" when nothing matches.
	f := &fakeAPI{status: 200, body: `{}`}
	got, err := f.client(t).SearchText(context.Background(), "nowhere")
	if err != nil || len(got) != 0 {
		t.Errorf("places = %v, err = %v; want none", got, err)
	}
}

func TestDetails(t *testing.T) {
	f := &fakeAPI{status: 200, body: `{"location": {"latitude": 35.5, "longitude": 139.5}, "businessStatus": "CLOSED_PERMANENTLY"}`}
	got, err := f.client(t).Details(context.Background(), "ChIJ/odd id")
	if err != nil {
		t.Fatal(err)
	}
	if got.Location != (LatLng{Lat: 35.5, Lng: 139.5}) || !got.ClosedPermanently() {
		t.Errorf("details = %+v", got)
	}
	if f.method != "GET" || f.path != "/v1/places/ChIJ/odd id" {
		t.Errorf("request = %s %s", f.method, f.path)
	}
	if f.fieldMask != "location,businessStatus" {
		t.Errorf("X-Goog-FieldMask = %q", f.fieldMask)
	}
}

func TestDetailsWithoutBusinessStatus(t *testing.T) {
	// Places that are not businesses, such as parks, have no businessStatus.
	f := &fakeAPI{status: 200, body: `{"location": {"latitude": 1, "longitude": 2}}`}
	got, err := f.client(t).Details(context.Background(), "ChIJ1")
	if err != nil || got.ClosedPermanently() {
		t.Errorf("details = %+v, err = %v", got, err)
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"not found", 404, `{"error": {"message": "Place ID is no longer valid"}}`, ErrNotFound},
		{"invalid id", 400, `{"error": {"message": "Not a valid Place ID"}}`, ErrNotFound},
		{"quota", 429, `{"error": {"message": "Quota exceeded"}}`, ErrQuotaExceeded},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeAPI{status: tc.status, body: tc.body}
			_, err := f.client(t).Details(context.Background(), "ChIJ1")
			if !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}

	t.Run("search quota", func(t *testing.T) {
		f := &fakeAPI{status: 429, body: `{}`}
		_, err := f.client(t).SearchText(context.Background(), "q")
		if !errors.Is(err, ErrQuotaExceeded) {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("search bad request is not ErrNotFound", func(t *testing.T) {
		f := &fakeAPI{status: 400, body: `{"error": {"message": "bad"}}`}
		_, err := f.client(t).SearchText(context.Background(), "q")
		if err == nil || errors.Is(err, ErrNotFound) {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("forbidden", func(t *testing.T) {
		f := &fakeAPI{status: 403, body: `{"error": {"message": "API key not valid"}}`}
		_, err := f.client(t).Details(context.Background(), "ChIJ1")
		if err == nil || errors.Is(err, ErrNotFound) || errors.Is(err, ErrQuotaExceeded) {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("missing location", func(t *testing.T) {
		f := &fakeAPI{status: 200, body: `{}`}
		if _, err := f.client(t).Details(context.Background(), "ChIJ1"); err == nil {
			t.Error("err = nil")
		}
	})
	t.Run("no key", func(t *testing.T) {
		c := NewClient("")
		if _, err := c.SearchText(context.Background(), "q"); !errors.Is(err, ErrNotConfigured) {
			t.Errorf("err = %v", err)
		}
		if c.Configured() {
			t.Error("Configured() = true")
		}
	})
}
