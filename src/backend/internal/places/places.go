// Package places is a client for Places API (New).
//
// It requests only the fields the backend stores, because the price of a
// request depends on the fields in its field mask.
package places

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const defaultBaseURL = "https://places.googleapis.com"

const (
	searchFieldMask  = "places.id,places.displayName,places.formattedAddress,places.location"
	detailsFieldMask = "location,businessStatus"
)

// BusinessStatusClosedPermanently is the businessStatus of a place that has
// closed for good.
const BusinessStatusClosedPermanently = "CLOSED_PERMANENTLY"

var (
	// ErrNotConfigured is returned when no API key is set.
	ErrNotConfigured = errors.New("places: GOOGLE_PLACES_API_KEY is not set")
	// ErrNotFound is returned when a place ID does not exist (any more).
	ErrNotFound = errors.New("places: place not found")
	// ErrQuotaExceeded is returned when the request quota is used up. Callers
	// should stop sending requests until the quota resets.
	ErrQuotaExceeded = errors.New("places: quota exceeded")
)

// LatLng is a WGS 84 coordinate.
type LatLng struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// Place is a result of a text search.
type Place struct {
	ID       string `json:"placeId"`
	Name     string `json:"name"`
	Address  string `json:"address"`
	Location LatLng `json:"location"`
}

// Details is the current state of a place.
type Details struct {
	Location       LatLng
	BusinessStatus string
}

// ClosedPermanently reports whether the place has closed for good.
func (d Details) ClosedPermanently() bool {
	return d.BusinessStatus == BusinessStatusClosedPermanently
}

// API is what the rest of the backend needs from Places API.
type API interface {
	// SearchText returns the places matching a free-form query such as
	// "name address". The result is empty when nothing matches.
	SearchText(ctx context.Context, query string) ([]Place, error)
	// Details returns the location and the business status of a place.
	Details(ctx context.Context, placeID string) (Details, error)
}

// Client calls Places API (New) over HTTP.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

var _ API = (*Client)(nil)

// NewClient returns a client. With an empty apiKey, every call fails with
// ErrNotConfigured.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// WithBaseURL returns a copy of the client that sends requests to baseURL.
// Tests use it to talk to a fake server.
func (c *Client) WithBaseURL(baseURL string) *Client {
	clone := *c
	clone.baseURL = baseURL
	return &clone
}

// Configured reports whether an API key is set.
func (c *Client) Configured() bool {
	return c.apiKey != ""
}

type apiLatLng struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

func (l *apiLatLng) toLatLng() (LatLng, error) {
	if l == nil {
		return LatLng{}, errors.New("places: response has no location")
	}
	return LatLng{Lat: l.Latitude, Lng: l.Longitude}, nil
}

// SearchText implements API.
func (c *Client) SearchText(ctx context.Context, query string) ([]Place, error) {
	reqBody, err := json.Marshal(map[string]string{
		"textQuery":    query,
		"languageCode": "ja",
		"regionCode":   "JP",
	})
	if err != nil {
		return nil, err
	}

	var res struct {
		Places []struct {
			ID          string `json:"id"`
			DisplayName struct {
				Text string `json:"text"`
			} `json:"displayName"`
			FormattedAddress string     `json:"formattedAddress"`
			Location         *apiLatLng `json:"location"`
		} `json:"places"`
	}
	if err := c.do(ctx, http.MethodPost, "/v1/places:searchText", searchFieldMask, reqBody, &res); err != nil {
		return nil, err
	}

	result := make([]Place, 0, len(res.Places))
	for _, p := range res.Places {
		loc, err := p.Location.toLatLng()
		if err != nil {
			return nil, err
		}
		result = append(result, Place{
			ID:       p.ID,
			Name:     p.DisplayName.Text,
			Address:  p.FormattedAddress,
			Location: loc,
		})
	}
	return result, nil
}

// Details implements API.
func (c *Client) Details(ctx context.Context, placeID string) (Details, error) {
	var res struct {
		Location       *apiLatLng `json:"location"`
		BusinessStatus string     `json:"businessStatus"`
	}
	path := "/v1/places/" + url.PathEscape(placeID) + "?languageCode=ja"
	if err := c.do(ctx, http.MethodGet, path, detailsFieldMask, nil, &res); err != nil {
		return Details{}, err
	}
	loc, err := res.Location.toLatLng()
	if err != nil {
		return Details{}, err
	}
	return Details{Location: loc, BusinessStatus: res.BusinessStatus}, nil
}

func (c *Client) do(ctx context.Context, method, path, fieldMask string, body []byte, out any) error {
	if c.apiKey == "" {
		return ErrNotConfigured
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("X-Goog-Api-Key", c.apiKey)
	req.Header.Set("X-Goog-FieldMask", fieldMask)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		// The URL in the error has no secrets: the key travels in a header.
		return fmt.Errorf("places: %w", err)
	}
	defer res.Body.Close()

	data, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("places: read response: %w", err)
	}

	switch {
	case res.StatusCode == http.StatusOK:
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("places: decode response: %w", err)
		}
		return nil
	case res.StatusCode == http.StatusTooManyRequests:
		return fmt.Errorf("%w: %s", ErrQuotaExceeded, apiMessage(data))
	// An ID that never existed is a 400, one that no longer exists is a 404.
	case method == http.MethodGet && (res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusBadRequest):
		return fmt.Errorf("%w: %s", ErrNotFound, apiMessage(data))
	default:
		return fmt.Errorf("places: HTTP %d: %s", res.StatusCode, apiMessage(data))
	}
}

// apiMessage extracts the message from a Google API error body.
func apiMessage(data []byte) string {
	var e struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(data, &e) == nil && e.Error.Message != "" {
		return e.Error.Message
	}
	if len(data) > 200 {
		data = data[:200]
	}
	return string(data)
}
