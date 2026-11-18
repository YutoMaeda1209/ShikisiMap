// Package admin implements the admin API, which the spot request workflow
// calls from GitHub Actions.
package admin

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"

	"github.com/YutoMaeda1209/ShikisiMap/src/backend/internal/httpx"
)

const (
	githubIssuer = "https://token.actions.githubusercontent.com"

	// Only these workflows, running from the main branch of this repository,
	// may call the admin API.
	allowedRepository = "YutoMaeda1209/ShikisiMap"
	allowedRef        = "refs/heads/main"

	// Error codes of the admin API.
	CodeAdminUnavailable = "admin_unavailable"
	CodeUnauthorized     = "unauthorized"
	CodeForbidden        = "forbidden"
)

// allowedWorkflowRefs are the workflow_ref claims of the workflows that may
// call the admin API: the spot request flow and the manual spot update.
var allowedWorkflowRefs = map[string]bool{
	allowedRepository + "/.github/workflows/spot_request.yml@" + allowedRef: true,
	allowedRepository + "/.github/workflows/spot_update.yml@" + allowedRef:  true,
}

// authenticator verifies GitHub Actions OIDC tokens.
type authenticator struct {
	issuer     string
	audience   string
	httpClient *http.Client

	mu       sync.Mutex
	verifier *oidc.IDTokenVerifier
}

func newAuthenticator(issuer, audience string) *authenticator {
	if issuer == "" {
		issuer = githubIssuer
	}
	return &authenticator{
		issuer:     issuer,
		audience:   audience,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// tokenVerifier fetches the issuer's metadata on first use, so that the
// server can start while GitHub is unreachable.
func (a *authenticator) tokenVerifier() (*oidc.IDTokenVerifier, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.verifier != nil {
		return a.verifier, nil
	}
	// The verifier keeps this context to fetch the signing keys later, so it
	// must not be the context of a request.
	ctx := oidc.ClientContext(context.Background(), a.httpClient)
	provider, err := oidc.NewProvider(ctx, a.issuer)
	if err != nil {
		return nil, fmt.Errorf("fetch OpenID Connect metadata: %w", err)
	}
	// The verifier checks the signature, iss, aud (the client ID) and exp.
	a.verifier = provider.Verifier(&oidc.Config{ClientID: a.audience})
	return a.verifier, nil
}

func (a *authenticator) middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if a.audience == "" {
			httpx.Error(c, http.StatusServiceUnavailable, CodeAdminUnavailable, "the admin API is not configured")
			return
		}

		scheme, token, _ := strings.Cut(c.GetHeader("Authorization"), " ")
		if !strings.EqualFold(scheme, "Bearer") || token == "" {
			c.Header("WWW-Authenticate", "Bearer")
			httpx.Error(c, http.StatusUnauthorized, CodeUnauthorized, "a GitHub Actions OIDC token is required")
			return
		}

		verifier, err := a.tokenVerifier()
		if err != nil {
			_ = c.Error(err)
			httpx.Error(c, http.StatusServiceUnavailable, CodeAdminUnavailable, "the token issuer is unreachable")
			return
		}
		idToken, err := verifier.Verify(c.Request.Context(), strings.TrimSpace(token))
		if err != nil {
			_ = c.Error(err)
			c.Header("WWW-Authenticate", `Bearer error="invalid_token"`)
			httpx.Error(c, http.StatusUnauthorized, CodeUnauthorized, "the token is not valid")
			return
		}

		var claims struct {
			Repository  string `json:"repository"`
			WorkflowRef string `json:"workflow_ref"`
			Ref         string `json:"ref"`
		}
		if err := idToken.Claims(&claims); err != nil {
			httpx.InternalError(c, err)
			return
		}
		if claims.Repository != allowedRepository || !allowedWorkflowRefs[claims.WorkflowRef] || claims.Ref != allowedRef {
			_ = c.Error(fmt.Errorf("admin: rejected token of %s, workflow %s, ref %s",
				claims.Repository, claims.WorkflowRef, claims.Ref))
			httpx.Error(c, http.StatusForbidden, CodeForbidden, "this workflow may not call the admin API")
			return
		}
		c.Next()
	}
}
