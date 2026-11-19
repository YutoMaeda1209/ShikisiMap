package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"

	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/httpx"
)

const (
	googleIssuer = "https://accounts.google.com"

	// The login flow cookie holds the state, the nonce and the PKCE verifier
	// between the redirect to Google and the callback.
	flowCookieName = "shikisimap_login"
	flowCookiePath = "/api/auth/google"
	flowLifetime   = 10 * time.Minute

	// CodeLoginUnavailable is the error code when Google login is not configured.
	CodeLoginUnavailable = "login_unavailable"

	// MaxDisplayNameLength is the limit of users.display_name, in characters.
	MaxDisplayNameLength = 50
)

// Options configures a Service.
type Options struct {
	Pool *pgxpool.Pool
	// PublicOrigin is the origin of the site, such as "https://map.mendako.dev".
	PublicOrigin string
	// ClientID and ClientSecret identify the Google OAuth client. Login is
	// unavailable while they are empty; sessions keep working.
	ClientID     string
	ClientSecret string
	// Issuer overrides the OpenID Connect issuer. Tests point it at a fake
	// provider; it defaults to Google.
	Issuer string
}

// Service implements login and authentication.
type Service struct {
	pool          *pgxpool.Pool
	publicOrigin  string
	clientID      string
	clientSecret  string
	issuer        string
	secureCookies bool
	httpClient    *http.Client

	mu       sync.Mutex
	provider *oidc.Provider
}

// New returns a Service.
func New(opts Options) *Service {
	issuer := opts.Issuer
	if issuer == "" {
		issuer = googleIssuer
	}
	return &Service{
		pool:          opts.Pool,
		publicOrigin:  opts.PublicOrigin,
		clientID:      opts.ClientID,
		clientSecret:  opts.ClientSecret,
		issuer:        issuer,
		secureCookies: secureOrigin(opts.PublicOrigin),
		httpClient:    &http.Client{Timeout: 15 * time.Second},
	}
}

// Register adds the login and logout endpoints to the API.
func (s *Service) Register(api gin.IRoutes) {
	api.GET("/auth/google/login", s.login)
	api.GET("/auth/google/callback", s.callback)
	api.DELETE("/session", s.RequireUser(), s.logout)
}

// oidcProvider returns the provider metadata, fetching it on first use so
// that the server can start while Google is unreachable.
func (s *Service) oidcProvider() (*oidc.Provider, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.provider != nil {
		return s.provider, nil
	}
	// The provider keeps this context to fetch the signing keys later, so it
	// must not be the context of a request.
	ctx := oidc.ClientContext(context.Background(), s.httpClient)
	provider, err := oidc.NewProvider(ctx, s.issuer)
	if err != nil {
		return nil, fmt.Errorf("fetch OpenID Connect metadata: %w", err)
	}
	s.provider = provider
	return provider, nil
}

func (s *Service) oauthConfig(provider *oidc.Provider) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     s.clientID,
		ClientSecret: s.clientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  s.publicOrigin + "/api/auth/google/callback",
		Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
	}
}

// login handles GET /api/auth/google/login by sending the browser to Google.
func (s *Service) login(c *gin.Context) {
	if s.clientID == "" || s.clientSecret == "" {
		httpx.Error(c, http.StatusServiceUnavailable, CodeLoginUnavailable, "Google login is not configured")
		return
	}
	provider, err := s.oidcProvider()
	if err != nil {
		_ = c.Error(err)
		httpx.Error(c, http.StatusServiceUnavailable, CodeLoginUnavailable, "Google login is temporarily unavailable")
		return
	}

	state, err1 := newToken()
	nonce, err2 := newToken()
	if err := errors.Join(err1, err2); err != nil {
		httpx.InternalError(c, err)
		return
	}
	verifier := oauth2.GenerateVerifier()

	// None of the three values contains a dot (they are base64url).
	s.setCookie(c, flowCookieName, state+"."+nonce+"."+verifier, flowCookiePath, flowLifetime)
	c.Redirect(http.StatusFound, s.oauthConfig(provider).AuthCodeURL(state,
		oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier)))
}

// callback handles GET /api/auth/google/callback, where Google sends the
// browser back. The browser is navigating, so the outcome is a redirect to
// the site rather than a JSON response.
func (s *Service) callback(c *gin.Context) {
	userID, err := s.completeLogin(c)
	// The flow cookie is single-use whatever the outcome.
	s.clearCookie(c, flowCookieName, flowCookiePath)
	if err != nil {
		log.Printf("auth: login failed: %v", err)
		c.Redirect(http.StatusFound, s.publicOrigin+"/?login=failed")
		return
	}

	// A new session ID on every login.
	token, err := s.CreateSession(c.Request.Context(), userID)
	if err != nil {
		log.Printf("auth: create session: %v", err)
		c.Redirect(http.StatusFound, s.publicOrigin+"/?login=failed")
		return
	}
	s.setCookie(c, SessionCookieName, token, sessionCookiePath, sessionLifetime)
	c.Redirect(http.StatusFound, s.publicOrigin+"/")
}

// completeLogin verifies the callback and returns the users.id of the user,
// creating the user on the first login.
func (s *Service) completeLogin(c *gin.Context) (string, error) {
	if s.clientID == "" || s.clientSecret == "" {
		return "", errors.New("Google login is not configured")
	}
	if e := c.Query("error"); e != "" {
		return "", fmt.Errorf("Google returned %q", e)
	}

	flow, err := c.Cookie(flowCookieName)
	if err != nil {
		return "", errors.New("no login flow cookie")
	}
	parts := strings.Split(flow, ".")
	if len(parts) != 3 {
		return "", errors.New("malformed login flow cookie")
	}
	state, nonce, verifier := parts[0], parts[1], parts[2]
	if subtle.ConstantTimeCompare([]byte(c.Query("state")), []byte(state)) != 1 {
		return "", errors.New("state mismatch")
	}
	code := c.Query("code")
	if code == "" {
		return "", errors.New("no authorization code")
	}

	provider, err := s.oidcProvider()
	if err != nil {
		return "", err
	}
	ctx := oidc.ClientContext(c.Request.Context(), s.httpClient)
	token, err := s.oauthConfig(provider).Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return "", fmt.Errorf("exchange the code: %w", err)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return "", errors.New("the token response has no id_token")
	}
	// Verify checks the signature, the issuer, the audience and the expiry.
	idToken, err := provider.Verifier(&oidc.Config{ClientID: s.clientID}).Verify(ctx, rawIDToken)
	if err != nil {
		return "", fmt.Errorf("verify the ID token: %w", err)
	}
	if subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(nonce)) != 1 {
		return "", errors.New("nonce mismatch")
	}

	var claims struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return "", fmt.Errorf("read the ID token claims: %w", err)
	}
	if idToken.Subject == "" {
		return "", errors.New("the ID token has no sub")
	}

	// The display name is only set on creation: the user may have edited it.
	var userID string
	err = s.pool.QueryRow(c.Request.Context(), `
		INSERT INTO users (google_sub, email, display_name) VALUES ($1, $2, $3)
		ON CONFLICT (google_sub) DO UPDATE SET email = EXCLUDED.email
		RETURNING id`,
		idToken.Subject, claims.Email, initialDisplayName(claims.Name, claims.Email),
	).Scan(&userID)
	if err != nil {
		return "", fmt.Errorf("find or create the user: %w", err)
	}
	return userID, nil
}

// initialDisplayName picks the display name of a new user: the Google
// account name, or the local part of the email address without one.
func initialDisplayName(name, email string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name, _, _ = strings.Cut(email, "@")
	}
	if name == "" {
		name = "ユーザー"
	}
	if utf8.RuneCountInString(name) > MaxDisplayNameLength {
		name = string([]rune(name)[:MaxDisplayNameLength])
	}
	return name
}
