package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/admin"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/auth"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/config"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/httpx"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/spots"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/users"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/visits"
)

const codeDBUnavailable = "db_unavailable"

// newRouter builds the HTTP API.
func newRouter(cfg config.Config, pool *pgxpool.Pool) (*gin.Engine, error) {
	r, err := httpx.NewEngine()
	if err != nil {
		return nil, err
	}

	health := healthHandler(pool)
	// For checks on the host itself. nginx only forwards /api/.
	r.GET("/healthz", health)

	// The API the site's frontend calls, authenticated by the session cookie.
	api := r.Group("/api", httpx.RequireSameOrigin(cfg.PublicOrigin), httpx.RequireJSON())
	api.GET("/healthz", health)

	authSvc := auth.New(auth.Options{
		Pool:         pool,
		PublicOrigin: cfg.PublicOrigin,
		ClientID:     cfg.GoogleOAuthClientID,
		ClientSecret: cfg.GoogleOAuthClientSecret,
	})
	authSvc.Register(api)
	spots.Register(api, pool)
	users.Register(api, pool, authSvc)
	visits.Register(api, pool, authSvc)

	// The admin API is called from GitHub Actions with an OIDC token instead
	// of a cookie, so the Origin check does not apply to it.
	admin.Register(r.Group("/api/admin", httpx.RequireJSON()), admin.Options{
		Pool:     pool,
		Audience: cfg.AdminOIDCAudience,
	})

	return r, nil
}

func healthHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := pool.Ping(c.Request.Context()); err != nil {
			_ = c.Error(err)
			httpx.Error(c, http.StatusServiceUnavailable, codeDBUnavailable, "the database is unavailable")
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}
