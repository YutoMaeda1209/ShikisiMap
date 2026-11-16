package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/httpx"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/places"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/spots"
)

const (
	// CodePlaceNotResolved: the search found no place or several. The
	// response lists the candidates; the caller retries with a placeId.
	CodePlaceNotResolved = "place_not_resolved"
	// CodePlacesUnavailable: Places API cannot be used right now.
	CodePlacesUnavailable = "places_unavailable"
	// CodePlaceInUse: another spot already has the place ID.
	CodePlaceInUse = "place_in_use"

	maxTextLength   = 200
	uniqueViolation = "23505"
)

var youtubeIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{6,20}$`)

// Options configures the admin API.
type Options struct {
	Pool *pgxpool.Pool
	// Places resolves the places. It is nil when no API key is configured.
	Places places.API
	// Audience is the aud the OIDC tokens must have. The API answers 503
	// while it is empty.
	Audience string
	// Issuer overrides the token issuer. Tests point it at a fake provider;
	// it defaults to GitHub Actions.
	Issuer string
}

type handler struct {
	pool   *pgxpool.Pool
	places places.API
}

// Register adds the admin endpoints to the group, which must not have the
// Origin check: the callers are not browsers and authenticate with a token.
func Register(group *gin.RouterGroup, opts Options) {
	h := &handler{pool: opts.Pool, places: opts.Places}
	group.Use(newAuthenticator(opts.Issuer, opts.Audience).middleware())
	group.POST("/spots", h.createSpot)
	group.PATCH("/spots/:id", h.updateSpot)
}

func validText(s string) bool {
	n := utf8.RuneCountInString(s)
	return n > 0 && n <= maxTextLength
}

// placesError answers a failed Places API call.
func placesError(c *gin.Context, err error) {
	_ = c.Error(err)
	message := "Places API failed"
	switch {
	case errors.Is(err, places.ErrQuotaExceeded):
		message = "the Places API quota is used up; try again tomorrow"
	case errors.Is(err, places.ErrNotConfigured):
		message = "Places API is not configured"
	}
	httpx.Error(c, http.StatusServiceUnavailable, CodePlacesUnavailable, message)
}

// createSpot registers a spot with one appearance. When a spot with the
// same place already exists, only the appearance is added to it.
func (h *handler) createSpot(c *gin.Context) {
	var req struct {
		Name      string `json:"name"`
		Address   string `json:"address"`
		YouTubeID string `json:"youtubeId"`
		Timestamp int    `json:"timestamp"`
		PlaceID   string `json:"placeId"`
		IsClosed  bool   `json:"isClosed"`
	}
	if !httpx.BindJSON(c, &req) {
		return
	}
	req.Name, req.Address, req.PlaceID = strings.TrimSpace(req.Name), strings.TrimSpace(req.Address), strings.TrimSpace(req.PlaceID)
	switch {
	case !validText(req.Name) || !validText(req.Address):
		httpx.ValidationError(c, "name and address must have 1 to 200 characters")
		return
	case !youtubeIDPattern.MatchString(req.YouTubeID):
		httpx.ValidationError(c, "youtubeId is not a YouTube video ID")
		return
	case req.Timestamp < 0:
		httpx.ValidationError(c, "timestamp must not be negative")
		return
	case utf8.RuneCountInString(req.PlaceID) > maxTextLength:
		httpx.ValidationError(c, "placeId is too long")
		return
	}
	ctx := c.Request.Context()

	// Without a place ID from the caller, the place has to be unambiguous.
	placeID := req.PlaceID
	var location *places.LatLng
	if placeID == "" {
		if h.places == nil {
			placesError(c, places.ErrNotConfigured)
			return
		}
		found, err := h.places.SearchText(ctx, req.Name+" "+req.Address)
		if err != nil {
			placesError(c, err)
			return
		}
		if len(found) != 1 {
			if found == nil {
				// An empty list rather than null for the caller to iterate.
				found = []places.Place{}
			}
			message := "no place matches the name and address"
			if len(found) > 1 {
				message = "several places match the name and address"
			}
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{
				"error":      httpx.ErrorDetail{Code: CodePlaceNotResolved, Message: message},
				"candidates": found,
			})
			return
		}
		placeID, location = found[0].ID, &found[0].Location
	}

	spotID, err := h.spotIDByPlace(c, placeID)
	if err != nil {
		httpx.InternalError(c, err)
		return
	}
	created := false
	if spotID == "" {
		isClosed := req.IsClosed
		if location == nil {
			// The caller chose the place; its location still comes from Google.
			if h.places == nil {
				placesError(c, places.ErrNotConfigured)
				return
			}
			details, err := h.places.Details(ctx, placeID)
			if errors.Is(err, places.ErrNotFound) {
				httpx.ValidationError(c, "placeId is not a valid place ID")
				return
			}
			if err != nil {
				placesError(c, err)
				return
			}
			location = &details.Location
			isClosed = isClosed || details.ClosedPermanently()
		}
		// ON CONFLICT covers a concurrent request registering the same place.
		err := h.pool.QueryRow(ctx, `
			INSERT INTO spots (place_id, name, address, location, location_fetched_at, is_closed)
			VALUES ($1, $2, $3, ST_SetSRID(ST_MakePoint($4, $5), 4326)::geography, now(), $6)
			ON CONFLICT (place_id) DO NOTHING
			RETURNING id::text`,
			placeID, req.Name, req.Address, location.Lng, location.Lat, isClosed).Scan(&spotID)
		switch {
		case err == nil:
			created = true
		case errors.Is(err, pgx.ErrNoRows):
			if spotID, err = h.spotIDByPlace(c, placeID); err != nil || spotID == "" {
				httpx.InternalError(c, errors.Join(err, errors.New("the spot vanished after a conflict")))
				return
			}
		default:
			httpx.InternalError(c, err)
			return
		}
	}

	tag, err := h.pool.Exec(ctx, `
		INSERT INTO appearances (spot_id, youtube_id, "timestamp") VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING`, spotID, req.YouTubeID, req.Timestamp)
	if err != nil {
		httpx.InternalError(c, err)
		return
	}

	spot, err := spots.Get(ctx, h.pool, spotID)
	if err != nil || spot == nil {
		httpx.InternalError(c, errors.Join(err, errors.New("the spot vanished")))
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	c.JSON(status, gin.H{
		"spot": spot,
		// created is false when the place was already registered.
		"created": created,
		// appearanceAdded is false when the spot already had this scene.
		"appearanceAdded": tag.RowsAffected() == 1,
	})
}

// spotIDByPlace returns the ID of the spot with the place ID, or "".
func (h *handler) spotIDByPlace(c *gin.Context, placeID string) (string, error) {
	var id string
	err := h.pool.QueryRow(c.Request.Context(), `SELECT id::text FROM spots WHERE place_id = $1`, placeID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return id, err
}

// updateSpot edits a spot. Fields that are not given stay as they are.
func (h *handler) updateSpot(c *gin.Context) {
	const notFound = "no such spot"
	var req struct {
		Name     *string `json:"name"`
		Address  *string `json:"address"`
		IsClosed *bool   `json:"isClosed"`
		// PlaceID is a string, or null to mark the spot as unresolved.
		PlaceID json.RawMessage `json:"placeId"`
	}
	if !httpx.BindJSON(c, &req) {
		return
	}
	id := c.Param("id")
	if !httpx.IsUUID(id) {
		httpx.NotFound(c, notFound)
		return
	}
	for _, text := range []*string{req.Name, req.Address} {
		if text == nil {
			continue
		}
		if *text = strings.TrimSpace(*text); !validText(*text) {
			httpx.ValidationError(c, "name and address must have 1 to 200 characters")
			return
		}
	}
	var placeID *string
	placeGiven := req.PlaceID != nil
	if placeGiven && string(req.PlaceID) != "null" {
		var s string
		if err := json.Unmarshal(req.PlaceID, &s); err != nil || !validText(strings.TrimSpace(s)) {
			httpx.ValidationError(c, "placeId must be a place ID or null")
			return
		}
		s = strings.TrimSpace(s)
		placeID = &s
	}

	// When the place changes, the stored location belongs to the old place.
	// Clearing location_fetched_at makes the scheduled job fetch the new one.
	var updated string
	err := h.pool.QueryRow(c.Request.Context(), `
		UPDATE spots SET
			name = COALESCE($2, name),
			address = COALESCE($3, address),
			is_closed = COALESCE($4, is_closed),
			place_id = CASE WHEN $5 THEN $6 ELSE place_id END,
			location_fetched_at = CASE WHEN $5 AND place_id IS DISTINCT FROM $6 THEN NULL ELSE location_fetched_at END,
			status_checked_at = CASE WHEN $5 AND place_id IS DISTINCT FROM $6 THEN NULL ELSE status_checked_at END
		WHERE id = $1
		RETURNING id::text`,
		id, req.Name, req.Address, req.IsClosed, placeGiven, placeID).Scan(&updated)
	var pgErr *pgconn.PgError
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		httpx.NotFound(c, notFound)
		return
	case errors.As(err, &pgErr) && pgErr.Code == uniqueViolation:
		httpx.Error(c, http.StatusConflict, CodePlaceInUse, "another spot already has this place ID")
		return
	case err != nil:
		httpx.InternalError(c, err)
		return
	}

	spot, err := spots.Get(c.Request.Context(), h.pool, updated)
	if err != nil || spot == nil {
		httpx.InternalError(c, errors.Join(err, errors.New("the spot vanished")))
		return
	}
	c.JSON(http.StatusOK, spot)
}
