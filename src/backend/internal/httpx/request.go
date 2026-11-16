package httpx

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
)

// Error codes for invalid input.
const (
	// CodeInvalidRequest: the body is not the JSON the endpoint expects.
	CodeInvalidRequest = "invalid_request"
	// CodeValidationFailed: the body is well-formed but a value is not acceptable.
	CodeValidationFailed = "validation_failed"
)

const maxBodyBytes = 1 << 20

// BindJSON decodes the request body into v. On failure it writes a 400
// response and returns false.
//
// Unknown fields are rejected so that a typo in a field name does not pass
// silently as "field not given".
func BindJSON(c *gin.Context, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		Error(c, http.StatusBadRequest, CodeInvalidRequest, "invalid JSON body: "+err.Error())
		return false
	}
	if _, err := dec.Token(); err != io.EOF {
		Error(c, http.StatusBadRequest, CodeInvalidRequest, "invalid JSON body: unexpected data after the value")
		return false
	}
	return true
}

// ValidationError writes a 422 response.
func ValidationError(c *gin.Context, message string) {
	Error(c, http.StatusUnprocessableEntity, CodeValidationFailed, message)
}

// NotFound writes a 404 response.
func NotFound(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, CodeNotFound, message)
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// IsUUID reports whether s is a UUID in the canonical text form. Handlers
// check IDs with it before passing them to PostgreSQL, which would otherwise
// fail the query on a malformed one.
func IsUUID(s string) bool {
	return uuidPattern.MatchString(s)
}
