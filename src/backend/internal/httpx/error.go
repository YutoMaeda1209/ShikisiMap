// Package httpx holds what every HTTP handler shares: the error response
// format, the common middleware and the base router.
package httpx

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Error codes shared across the API. Packages add their own codes for errors
// specific to them.
const (
	CodeNotFound             = "not_found"
	CodeMethodNotAllowed     = "method_not_allowed"
	CodeForbiddenOrigin      = "forbidden_origin"
	CodeUnsupportedMediaType = "unsupported_media_type"
	CodeInternal             = "internal_error"
)

// ErrorResponse is the body of every error response:
//
//	{"error": {"code": "not_found", "message": "..."}}
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail describes an error. Code is a stable identifier for clients to
// branch on, and Message is a description for developers.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error writes an error response and stops the remaining handlers.
func Error(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, ErrorResponse{
		Error: ErrorDetail{Code: code, Message: message},
	})
}

// InternalError records err and writes a 500 response that does not expose it.
func InternalError(c *gin.Context, err error) {
	_ = c.Error(err)
	Error(c, http.StatusInternalServerError, CodeInternal, "internal server error")
}
