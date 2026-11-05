package zitadel

import (
	"context"
	"time"
)

// UserGrant represents a Zitadel user grant (role assignment to a project).
type UserGrant struct {
	ID        string   `json:"id"`
	UserID    string   `json:"userId"`
	ProjectID string   `json:"projectId"`
	RoleKeys  []string `json:"roleKeys"`
}

// ZitadelUser represents a Zitadel user profile.
type ZitadelUser struct {
	ID          string `json:"id"`
	Username    string `json:"userName"`
	DisplayName string `json:"displayName"`
	GivenName   string `json:"-"`
	FamilyName  string `json:"-"`
	Email       string `json:"email"`
	State       string `json:"state"`
	// Machine is true for a machine user (the search item carries `machine`
	// where a person's carries `human`).
	Machine bool `json:"-"`
	// Created is when Zitadel created the account (details.creationDate). Zero
	// when the response carried none or an unreadable one.
	Created time.Time `json:"-"`
}

// ProjectRoleResult represents a role definition within a Zitadel project.
type ProjectRoleResult struct {
	Key         string `json:"key"`
	DisplayName string `json:"displayName"`
	Group       string `json:"group"`
}

// ZitadelProject represents a Zitadel project summary.
type ZitadelProject struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
}

// SearchParams controls pagination for Zitadel _search endpoints.
type SearchParams struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// DefaultSearchLimit is the default page size for _search calls. Zitadel's
// own default is 100; we use 500 to reduce round-trips at makerspace scale.
const DefaultSearchLimit = 500

// SearchResult wraps a paginated result set with total count metadata.
type SearchResult[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
}

// ZitadelClient is the interface over the Zitadel Management API.
// The live implementation is wired in InitClient once credentials are available.
type ZitadelClient interface {
	// Users
	GetUser(ctx context.Context, userID string) (*ZitadelUser, error)
	ListUsers(ctx context.Context, p SearchParams) (*SearchResult[ZitadelUser], error)

	// Projects & Roles
	ListProjects(ctx context.Context, p SearchParams) (*SearchResult[ZitadelProject], error)
	AddProjectRole(ctx context.Context, projectID, roleKey, displayName, group string) error
	ListProjectRoles(ctx context.Context, projectID string, p SearchParams) (*SearchResult[ProjectRoleResult], error)
	UpdateProjectRole(ctx context.Context, projectID, roleKey, displayName, group string) error
	DeleteProjectRole(ctx context.Context, projectID, roleKey string) error

	// Applications (OIDC/API/SAML clients attached to a project)
	ListApplications(ctx context.Context, projectID string, p SearchParams) (*SearchResult[ZitadelApplication], error)
	// TokenSettings reads the app and project settings that decide where its
	// claims land (read-only; check 1 of the claims explain step).
	TokenSettings(ctx context.Context, projectID, appID string) (*TokenSettings, error)

	// User metadata (arbitrary admin-managed K/V per user, used for Title/Team overlays)
	ListUserMetadata(ctx context.Context, userID string, p SearchParams) (*SearchResult[UserMetadata], error)

	// Grants (user-role assignments)
	AddUserGrant(ctx context.Context, userID, projectID string, roleKeys []string) error
	UpdateUserGrant(ctx context.Context, userID, grantID string, roleKeys []string) error
	RemoveUserGrant(ctx context.Context, userID, grantID string) error
	ListUserGrants(ctx context.Context, userID string, p SearchParams) (*SearchResult[UserGrant], error)
	ListAllGrants(ctx context.Context, p SearchParams) (*SearchResult[UserGrant], error)

	// Events — Zitadel's own record of who did something, which no other read
	// here can answer. The sweep compares grant SETS and therefore cannot name
	// an actor; this can, because Zitadel is event-sourced.
	GrantOriginByID(ctx context.Context, grantID string) (*GrantOrigin, error)
	// SearchEvents reads events by type and time, for the operator's reasoned
	// "Look up sign-ins". Never called by a sweep or a page load.
	SearchEvents(ctx context.Context, q EventSearch) ([]LoggedEvent, bool, error)
}
