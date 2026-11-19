package admin

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/httpx"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/spots"
)

const (
	// CodeUnknownSpot: the request names a spot that does not exist.
	CodeUnknownSpot = "unknown_spot"
	// CodeLocationRequired: a new spot needs coordinates.
	CodeLocationRequired = "location_required"

	maxTextLength = 200
)

var youtubeIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{6,20}$`)

// Options configures the admin API.
type Options struct {
	Pool *pgxpool.Pool
	// Audience is the aud the OIDC tokens must have. The API answers 503
	// while it is empty.
	Audience string
	// Issuer overrides the token issuer. Tests point it at a fake provider;
	// it defaults to GitHub Actions.
	Issuer string
}

type handler struct {
	pool *pgxpool.Pool
}

// Register adds the admin endpoints to the group, which must not have the
// Origin check: the callers are not browsers and authenticate with a token.
func Register(group *gin.RouterGroup, opts Options) {
	h := &handler{pool: opts.Pool}
	group.Use(newAuthenticator(opts.Issuer, opts.Audience).middleware())
	group.POST("/spots", h.createSpot)
	group.PATCH("/spots/:id", h.updateSpot)
}

func validText(s string) bool {
	n := utf8.RuneCountInString(s)
	return n > 0 && n <= maxTextLength
}

// createSpot adds an appearance in a video, registering the spot first
// unless it already exists. The spot is the one named by spotId, or else the
// one with the same name and address.
func (h *handler) createSpot(c *gin.Context) {
	var req struct {
		SpotID    string        `json:"spotId"`
		Name      string        `json:"name"`
		Address   string        `json:"address"`
		Location  *spots.LatLng `json:"location"`
		IsClosed  bool          `json:"isClosed"`
		YouTubeID string        `json:"youtubeId"`
		Timestamp int           `json:"timestamp"`
	}
	if !httpx.BindJSON(c, &req) {
		return
	}
	req.Name, req.Address = strings.TrimSpace(req.Name), strings.TrimSpace(req.Address)
	switch {
	case !youtubeIDPattern.MatchString(req.YouTubeID):
		httpx.ValidationError(c, "youtubeId is not a YouTube video ID")
		return
	case req.Timestamp < 0:
		httpx.ValidationError(c, "timestamp must not be negative")
		return
	case req.SpotID != "" && !httpx.IsUUID(req.SpotID):
		httpx.ValidationError(c, "spotId must be a UUID")
		return
	case req.SpotID == "" && (!validText(req.Name) || !validText(req.Address)):
		httpx.ValidationError(c, "name and address must have 1 to 200 characters")
		return
	case req.Location != nil && !req.Location.Valid():
		httpx.ValidationError(c, "location is out of range")
		return
	}
	ctx := c.Request.Context()

	var (
		spotID string
		err    error
	)
	if req.SpotID != "" {
		spotID, err = h.findSpot(ctx, `SELECT id::text FROM spots WHERE id = $1`, req.SpotID)
		if err == nil && spotID == "" {
			httpx.Error(c, http.StatusUnprocessableEntity, CodeUnknownSpot, "no spot has this spotId")
			return
		}
	} else {
		spotID, err = h.findSpot(ctx, `SELECT id::text FROM spots WHERE name = $1 AND address = $2 ORDER BY created_at LIMIT 1`, req.Name, req.Address)
	}
	if err != nil {
		httpx.InternalError(c, err)
		return
	}

	created := false
	if spotID == "" {
		if req.Location == nil {
			httpx.Error(c, http.StatusUnprocessableEntity, CodeLocationRequired, "a new spot needs a location")
			return
		}
		err := h.pool.QueryRow(ctx, `
			INSERT INTO spots (name, address, location, is_closed)
			VALUES ($1, $2, ST_SetSRID(ST_MakePoint($3, $4), 4326)::geography, $5)
			RETURNING id::text`,
			req.Name, req.Address, req.Location.Lng, req.Location.Lat, req.IsClosed).Scan(&spotID)
		if err != nil {
			httpx.InternalError(c, err)
			return
		}
		created = true
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
		// created is false when the spot was already registered.
		"created": created,
		// appearanceAdded is false when the spot already had this scene.
		"appearanceAdded": tag.RowsAffected() == 1,
	})
}

// findSpot returns the ID of the spot the query finds, or "".
func (h *handler) findSpot(ctx context.Context, query string, args ...any) (string, error) {
	var id string
	err := h.pool.QueryRow(ctx, query, args...).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return id, err
}

// updateSpot edits a spot by hand, e.g. to mark it as closed or to correct
// its location. Fields that are not given stay as they are.
func (h *handler) updateSpot(c *gin.Context) {
	const notFound = "no such spot"
	var req struct {
		Name     *string       `json:"name"`
		Address  *string       `json:"address"`
		IsClosed *bool         `json:"isClosed"`
		Location *spots.LatLng `json:"location"`
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
	var lng, lat *float64
	if req.Location != nil {
		if !req.Location.Valid() {
			httpx.ValidationError(c, "location is out of range")
			return
		}
		lng, lat = &req.Location.Lng, &req.Location.Lat
	}

	var updated string
	err := h.pool.QueryRow(c.Request.Context(), `
		UPDATE spots SET
			name = COALESCE($2, name),
			address = COALESCE($3, address),
			is_closed = COALESCE($4, is_closed),
			location = CASE WHEN $5::float8 IS NULL THEN location
			                ELSE ST_SetSRID(ST_MakePoint($5, $6), 4326)::geography END
		WHERE id = $1
		RETURNING id::text`,
		id, req.Name, req.Address, req.IsClosed, lng, lat).Scan(&updated)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		httpx.NotFound(c, notFound)
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
