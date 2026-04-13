package handlers

import "context"

// contextKey is an unexported type for context keys scoped to this package,
// preventing collisions with keys from other packages.
type contextKey int

const adminUserIDKey contextKey = iota

// getAdminUserID retrieves the Zitadel user ID of the acting admin from the
// request context. Returns an empty string in local-dev (API-key) mode.
func getAdminUserID(ctx context.Context) string {
	v, _ := ctx.Value(adminUserIDKey).(string)
	return v
}

// withAdminUserID stores the acting admin's Zitadel user ID in the context.
func withAdminUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, adminUserIDKey, userID)
}
