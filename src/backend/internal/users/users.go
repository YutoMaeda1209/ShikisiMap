// Package users serves the account settings and the public profiles.
package users

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/auth"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/httpx"
)

// Me is what a user sees of their own account.
type Me struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	DisplayName   string `json:"displayName"`
	ProfilePublic bool   `json:"profilePublic"`
}

// Profile is what everyone sees of a user whose profile is public. It has
// neither the dates nor the memos of the visits.
type Profile struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	// VisitedSpotIDs are the spots the user visited at least once.
	VisitedSpotIDs []string `json:"visitedSpotIds"`
}

type handler struct {
	pool *pgxpool.Pool
	auth *auth.Service
}

// Register adds the endpoints to the API.
func Register(api gin.IRoutes, pool *pgxpool.Pool, authSvc *auth.Service) {
	h := &handler{pool: pool, auth: authSvc}
	api.GET("/me", authSvc.RequireUser(), h.getMe)
	api.PATCH("/me", authSvc.RequireUser(), h.patchMe)
	api.DELETE("/me", authSvc.RequireUser(), h.deleteMe)
	api.GET("/users/:id", h.getProfile)
}

const meColumns = `id, email, display_name, profile_public`

func scanMe(row pgx.Row) (Me, error) {
	var me Me
	err := row.Scan(&me.ID, &me.Email, &me.DisplayName, &me.ProfilePublic)
	return me, err
}

func (h *handler) getMe(c *gin.Context) {
	me, err := scanMe(h.pool.QueryRow(c.Request.Context(),
		`SELECT `+meColumns+` FROM users WHERE id = $1`, auth.UserID(c)))
	if err != nil {
		httpx.InternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, me)
}

func (h *handler) patchMe(c *gin.Context) {
	// Raw messages tell "not given" from a value. null is not a value of either field.
	var req struct {
		DisplayName   json.RawMessage `json:"displayName"`
		ProfilePublic json.RawMessage `json:"profilePublic"`
	}
	if !httpx.BindJSON(c, &req) {
		return
	}

	var displayName *string
	if req.DisplayName != nil {
		var name string
		if err := json.Unmarshal(req.DisplayName, &name); err != nil || string(req.DisplayName) == "null" {
			httpx.ValidationError(c, "displayName must be a string")
			return
		}
		name = strings.TrimSpace(name)
		if n := utf8.RuneCountInString(name); n == 0 || n > auth.MaxDisplayNameLength {
			httpx.ValidationError(c, "displayName must have 1 to 50 characters")
			return
		}
		if strings.ContainsFunc(name, isControl) {
			httpx.ValidationError(c, "displayName must not contain control characters")
			return
		}
		displayName = &name
	}
	var profilePublic *bool
	if req.ProfilePublic != nil {
		var public bool
		if err := json.Unmarshal(req.ProfilePublic, &public); err != nil || string(req.ProfilePublic) == "null" {
			httpx.ValidationError(c, "profilePublic must be a boolean")
			return
		}
		profilePublic = &public
	}

	me, err := scanMe(h.pool.QueryRow(c.Request.Context(), `
		UPDATE users SET
			display_name = COALESCE($2, display_name),
			profile_public = COALESCE($3, profile_public)
		WHERE id = $1
		RETURNING `+meColumns, auth.UserID(c), displayName, profilePublic))
	if err != nil {
		httpx.InternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, me)
}

func isControl(r rune) bool {
	return r < 0x20 || r == 0x7f
}

// deleteMe deletes the account. The visits and every session of the user go
// with it through ON DELETE CASCADE.
func (h *handler) deleteMe(c *gin.Context) {
	if _, err := h.pool.Exec(c.Request.Context(), `DELETE FROM users WHERE id = $1`, auth.UserID(c)); err != nil {
		httpx.InternalError(c, err)
		return
	}
	h.auth.ClearSessionCookie(c)
	c.Status(http.StatusNoContent)
}

// getProfile serves a public profile. A private profile looks exactly like a
// user that does not exist.
func (h *handler) getProfile(c *gin.Context) {
	const notFound = "no such user, or the profile is not public"
	id := c.Param("id")
	if !httpx.IsUUID(id) {
		httpx.NotFound(c, notFound)
		return
	}

	profile := Profile{}
	err := h.pool.QueryRow(c.Request.Context(), `
		SELECT u.id, u.display_name,
		       COALESCE((
		           SELECT array_agg(v.spot_id::text ORDER BY v.first_visit, v.spot_id)
		           FROM (SELECT spot_id, min(visited_at) AS first_visit
		                 FROM visits WHERE user_id = u.id GROUP BY spot_id) v
		       ), '{}')
		FROM users u WHERE u.id = $1 AND u.profile_public`, id,
	).Scan(&profile.ID, &profile.DisplayName, &profile.VisitedSpotIDs)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.NotFound(c, notFound)
		return
	}
	if err != nil {
		httpx.InternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, profile)
}
