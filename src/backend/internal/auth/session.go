// Package auth implements Google login and the session-based authentication.
//
// The rest of the backend only learns the users.id of the caller, through
// RequireUser and UserID. That keeps a later move to Firebase Auth local to
// this package.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/httpx"
)

const (
	// SessionCookieName is the cookie that carries the session ID.
	SessionCookieName = "shikisimap_session"
	// The cookie is only sent to the API.
	sessionCookiePath = "/api"

	// A session ends this long after its last use.
	sessionLifetime = 30 * 24 * time.Hour
	// The expiry is pushed back at most this often, to avoid a write on
	// every request.
	sessionRefreshInterval = time.Hour

	// CodeUnauthorized is the error code of a request without a valid session.
	CodeUnauthorized = "unauthorized"

	userIDKey = "auth.userID"
)

// newToken returns 32 random bytes in a URL-safe form.
func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// hashToken returns what is stored for a session ID. The ID itself is never
// stored, so a leaked database does not let anyone take over a session.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// CreateSession starts a session for the user and returns the session ID to
// put in the cookie.
func (s *Service) CreateSession(ctx context.Context, userID string) (string, error) {
	token, err := newToken()
	if err != nil {
		return "", err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO sessions (user_id, token_hash, expires_at)
		VALUES ($1, $2, now() + make_interval(secs => $3))`,
		userID, hashToken(token), sessionLifetime.Seconds())
	if err != nil {
		return "", err
	}
	return token, nil
}

func (s *Service) setCookie(c *gin.Context, name, value, path string, maxAge time.Duration) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		MaxAge:   int(maxAge.Seconds()),
		HttpOnly: true,
		Secure:   s.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Service) clearCookie(c *gin.Context, name, path string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Path:     path,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearSessionCookie makes the browser forget the session ID.
func (s *Service) ClearSessionCookie(c *gin.Context) {
	s.clearCookie(c, SessionCookieName, sessionCookiePath)
}

// authenticate returns the users.id the request is authenticated as, or ""
// when it carries no valid session.
func (s *Service) authenticate(c *gin.Context) (string, error) {
	token, err := c.Cookie(SessionCookieName)
	if err != nil || token == "" {
		return "", nil
	}

	var (
		sessionID, userID string
		stale             bool
	)
	err = s.pool.QueryRow(c.Request.Context(), `
		SELECT id, user_id, last_used_at < now() - make_interval(secs => $2)
		FROM sessions WHERE token_hash = $1 AND expires_at > now()`,
		hashToken(token), sessionRefreshInterval.Seconds(),
	).Scan(&sessionID, &userID, &stale)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	if stale {
		if _, err := s.pool.Exec(c.Request.Context(), `
			UPDATE sessions SET last_used_at = now(), expires_at = now() + make_interval(secs => $2)
			WHERE id = $1`, sessionID, sessionLifetime.Seconds()); err != nil {
			return "", err
		}
		// The cookie has to live as long as the session.
		s.setCookie(c, SessionCookieName, token, sessionCookiePath, sessionLifetime)
	}
	return userID, nil
}

// RequireUser rejects requests without a valid session and makes the user
// available through UserID.
func (s *Service) RequireUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := s.authenticate(c)
		if err != nil {
			httpx.InternalError(c, err)
			return
		}
		if userID == "" {
			httpx.Error(c, http.StatusUnauthorized, CodeUnauthorized, "login required")
			return
		}
		c.Set(userIDKey, userID)
		c.Next()
	}
}

// UserID returns the users.id of the caller. It must only be called in
// handlers behind RequireUser.
func UserID(c *gin.Context) string {
	return c.GetString(userIDKey)
}

// logout handles DELETE /api/session.
func (s *Service) logout(c *gin.Context) {
	token, _ := c.Cookie(SessionCookieName)
	if _, err := s.pool.Exec(c.Request.Context(),
		`DELETE FROM sessions WHERE token_hash = $1`, hashToken(token)); err != nil {
		httpx.InternalError(c, err)
		return
	}
	s.ClearSessionCookie(c)
	c.Status(http.StatusNoContent)
}

// secureOrigin reports whether cookies for the origin can be marked Secure.
func secureOrigin(origin string) bool {
	return strings.HasPrefix(origin, "https://")
}
