package httpx

import (
	"mime"
	"net/http"

	"github.com/gin-gonic/gin"
)

// changesState reports whether a request with the method may change data.
func changesState(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	return true
}

// RequireSameOrigin rejects requests that change data unless their Origin
// header is publicOrigin. Together with SameSite=Lax cookies, this protects
// the cookie-authenticated API from cross-site request forgery.
//
// Browsers send Origin on every request with these methods, so a request
// without it is rejected too.
func RequireSameOrigin(publicOrigin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if changesState(c.Request.Method) && c.GetHeader("Origin") != publicOrigin {
			Error(c, http.StatusForbidden, CodeForbiddenOrigin, "the request must come from "+publicOrigin)
			return
		}
		c.Next()
	}
}

// RequireJSON rejects requests that change data unless their Content-Type is
// application/json. A request without a body, such as most DELETE requests,
// needs no Content-Type.
func RequireJSON() gin.HandlerFunc {
	return func(c *gin.Context) {
		if changesState(c.Request.Method) && hasBody(c.Request) && !isJSON(c.GetHeader("Content-Type")) {
			Error(c, http.StatusUnsupportedMediaType, CodeUnsupportedMediaType, "Content-Type must be application/json")
			return
		}
		c.Next()
	}
}

func hasBody(r *http.Request) bool {
	if r.Method == http.MethodDelete {
		// ContentLength is -1 when the length is unknown (chunked).
		return r.ContentLength != 0
	}
	// POST, PUT and PATCH carry a body by definition, so an empty one still
	// has to be declared as JSON.
	return true
}

func isJSON(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && mediaType == "application/json"
}
