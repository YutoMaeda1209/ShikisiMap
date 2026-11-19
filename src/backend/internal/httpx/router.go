package httpx

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// NewEngine returns a router whose own responses (unknown route, wrong method
// and panic) use the common error format.
func NewEngine() (*gin.Engine, error) {
	r := gin.New()
	r.Use(gin.Logger(), gin.CustomRecovery(func(c *gin.Context, _ any) {
		Error(c, http.StatusInternalServerError, CodeInternal, "internal server error")
	}))

	// The backend is only reached through nginx on the same host.
	if err := r.SetTrustedProxies([]string{"127.0.0.1", "::1"}); err != nil {
		return nil, fmt.Errorf("set trusted proxies: %w", err)
	}

	r.HandleMethodNotAllowed = true
	r.NoRoute(func(c *gin.Context) {
		Error(c, http.StatusNotFound, CodeNotFound, "no such endpoint")
	})
	r.NoMethod(func(c *gin.Context) {
		Error(c, http.StatusMethodNotAllowed, CodeMethodNotAllowed, "method not allowed")
	})
	return r, nil
}
