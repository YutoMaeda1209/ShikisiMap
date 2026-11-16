package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/config"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/httpx"
	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/spots"
)

const codeDBUnavailable = "db_unavailable"

func newRouter(cfg config.Config, pool *pgxpool.Pool) (*gin.Engine, error) {
	r, err := httpx.NewEngine()
	if err != nil {
		return nil, err
	}

	health := healthHandler(pool)
	// For checks on the host itself. nginx only forwards /api/.
	r.GET("/healthz", health)

	api := r.Group("/api", httpx.RequireSameOrigin(cfg.PublicOrigin), httpx.RequireJSON())
	api.GET("/healthz", health)
	spots.Register(api, pool)

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
