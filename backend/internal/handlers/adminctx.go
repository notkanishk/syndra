package handlers

import (
	"context"

	"mkauth/internal/auth"
)

// contextKey is an unexported type for context keys scoped to this package,
// preventing collisions with keys from other packages.
type contextKey int

const principalCtxKey contextKey = iota

// withPrincipal returns a copy of ctx that carries the validated auth principal.
// withUserAuth stashes the principal once per request; withOperatorAuth reads
// it back via principalFromContext to avoid re-parsing the JWT.
func withPrincipal(ctx context.Context, p *auth.Principal) context.Context {
	return context.WithValue(ctx, principalCtxKey, p)
}

// principalFromContext returns the principal stashed by withUserAuth.
// Returns nil if no principal is in context (dev-mode API-key path).
func principalFromContext(ctx context.Context) *auth.Principal {
	p, _ := ctx.Value(principalCtxKey).(*auth.Principal)
	return p
}

// getAdminUserID returns the authenticated admin user ID for the request, or
// the empty string when none is present (dev-mode API-key auth). Reads from
// the principal stashed by withUserAuth — single source of truth for request
// identity.
func getAdminUserID(ctx context.Context) string {
	if p := principalFromContext(ctx); p != nil {
		return p.Subject
	}
	return ""
}
