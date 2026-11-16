// Package spots serves the spots and their appearances in videos.
package spots

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/httpx"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/places"
)

// Appearance is a scene of a video in which a spot appears.
type Appearance struct {
	ID        string `json:"id"`
	YouTubeID string `json:"youtubeId"`
	// Timestamp is the position in the video, in seconds.
	Timestamp int `json:"timestamp"`
}

// Spot is a place together with its appearances.
type Spot struct {
	ID       string        `json:"id"`
	PlaceID  *string       `json:"placeId"`
	Name     string        `json:"name"`
	Address  string        `json:"address"`
	Location places.LatLng `json:"location"`
	IsClosed bool          `json:"isClosed"`
	// VisitorCount is the number of users who recorded a visit.
	VisitorCount int `json:"visitorCount"`
	// Appearances are ordered from the most recently registered one.
	Appearances []Appearance `json:"appearances"`
}

const selectSpots = `
SELECT s.id, s.place_id, s.name, s.address,
       ST_Y(s.location::geometry), ST_X(s.location::geometry),
       s.is_closed,
       (SELECT count(DISTINCT v.user_id) FROM visits v WHERE v.spot_id = s.id),
       COALESCE((
           SELECT json_agg(json_build_object('id', a.id, 'youtubeId', a.youtube_id, 'timestamp', a."timestamp")
                           ORDER BY a.created_at DESC, a.youtube_id, a."timestamp")
           FROM appearances a WHERE a.spot_id = s.id
       ), '[]'::json)
FROM spots s`

// Querier is satisfied by a pool and by a transaction.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func scanSpots(rows pgx.Rows) ([]Spot, error) {
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Spot, error) {
		var s Spot
		err := row.Scan(&s.ID, &s.PlaceID, &s.Name, &s.Address,
			&s.Location.Lat, &s.Location.Lng, &s.IsClosed, &s.VisitorCount, &s.Appearances)
		return s, err
	})
}

// List returns every spot in the order they were registered.
func List(ctx context.Context, q Querier) ([]Spot, error) {
	rows, err := q.Query(ctx, selectSpots+" ORDER BY s.created_at, s.id")
	if err != nil {
		return nil, err
	}
	return scanSpots(rows)
}

// Get returns the spot with the ID, or nil when there is none.
func Get(ctx context.Context, q Querier, id string) (*Spot, error) {
	rows, err := q.Query(ctx, selectSpots+" WHERE s.id = $1", id)
	if err != nil {
		return nil, err
	}
	found, err := scanSpots(rows)
	if err != nil || len(found) == 0 {
		return nil, err
	}
	return &found[0], nil
}

// Register adds the public endpoints to the API.
func Register(api gin.IRoutes, pool *pgxpool.Pool) {
	api.GET("/spots", func(c *gin.Context) {
		list, err := List(c.Request.Context(), pool)
		if err != nil {
			httpx.InternalError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"spots": list})
	})
}
