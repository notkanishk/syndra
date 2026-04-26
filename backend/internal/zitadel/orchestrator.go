package zitadel

import (
	"context"
	"fmt"
	"log"
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
	Email       string `json:"email"`
	State       string `json:"state"`
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

	// User metadata (arbitrary admin-managed K/V per user, used for Title/Team/Location overlays)
	ListUserMetadata(ctx context.Context, userID string, p SearchParams) (*SearchResult[UserMetadata], error)

	// Grants (user-role assignments)
	AddUserGrant(ctx context.Context, userID, projectID string, roleKeys []string) error
	UpdateUserGrant(ctx context.Context, userID, grantID string, roleKeys []string) error
	RemoveUserGrant(ctx context.Context, userID, grantID string) error
	ListUserGrants(ctx context.Context, userID string, p SearchParams) (*SearchResult[UserGrant], error)
	ListAllGrants(ctx context.Context, p SearchParams) (*SearchResult[UserGrant], error)
}

// EnforceMappingRules is triggered when a user is modified.
// It cross-references their new roles against our logic engine
// and pushes grants back to Zitadel when the client is live.
func EnforceMappingRules(ctx context.Context, userID, sourceProjectID, sourceRoleKey string) error {
	if MgmtClient == nil {
		log.Println("[ZITADEL] Skipping orchestration: client not initialized (local-policy-only mode).")
		return nil
	}

	rules, err := dbGetActiveMappingRules(ctx)
	if err != nil {
		return fmt.Errorf("failed to query mapping rules: %v", err)
	}

	for _, rule := range rules {
		if rule.SourceProject == sourceProjectID && rule.SourceRole == sourceRoleKey {
			log.Printf("[ZITADEL] Rule matched: propagating %s:%s -> %s:%s for user %s",
				sourceProjectID, sourceRoleKey, rule.TargetProject, rule.TargetRole, userID)

			if err := MgmtClient.AddUserGrant(ctx, userID, rule.TargetProject, []string{rule.TargetRole}); err != nil {
				log.Printf("[ZITADEL ERROR] Grant failed: %v", err)
				// Don't abort — try subsequent rules
			}
		}
	}

	return nil
}

// RevokeMappingRules is the inverse of EnforceMappingRules.
// When a source role is removed, it revokes any derived grants that were
// propagated through mapping rules. Role-aware: if a grant contains multiple
// roles, only the derived role is removed (via UpdateUserGrant); the grant
// is only deleted when it would become empty.
func RevokeMappingRules(ctx context.Context, userID, sourceProjectID, sourceRoleKey string) error {
	if MgmtClient == nil {
		log.Println("[ZITADEL] Skipping revocation: client not initialized (local-policy-only mode).")
		return nil
	}

	rules, err := dbGetActiveMappingRules(ctx)
	if err != nil {
		return fmt.Errorf("failed to query mapping rules: %v", err)
	}

	// Fetch ALL of the user's grants (paginate until exhausted).
	allGrants, err := fetchAllUserGrants(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to list user grants for revocation: %v", err)
	}

	// Index: "projectID:roleKey" -> grant (full object, needed to inspect all roles).
	type grantRef struct {
		grant UserGrant
	}
	grantIndex := make(map[string]*grantRef)
	for _, g := range allGrants {
		ref := &grantRef{grant: g}
		for _, rk := range g.RoleKeys {
			grantIndex[g.ProjectID+":"+rk] = ref
		}
	}

	for _, rule := range rules {
		if rule.SourceProject == sourceProjectID && rule.SourceRole == sourceRoleKey {
			key := rule.TargetProject + ":" + rule.TargetRole
			ref, exists := grantIndex[key]
			if !exists {
				continue
			}

			g := ref.grant
			if len(g.RoleKeys) == 1 {
				// Only role on the grant — delete the entire grant.
				log.Printf("[ZITADEL] Removing grant %s (sole role %s:%s) for user %s",
					g.ID, rule.TargetProject, rule.TargetRole, userID)
				if err := MgmtClient.RemoveUserGrant(ctx, userID, g.ID); err != nil {
					log.Printf("[ZITADEL ERROR] Grant removal failed: %v", err)
				}
			} else {
				// Multiple roles — update the grant to remove only the derived role.
				remaining := make([]string, 0, len(g.RoleKeys)-1)
				for _, rk := range g.RoleKeys {
					if rk != rule.TargetRole {
						remaining = append(remaining, rk)
					}
				}
				log.Printf("[ZITADEL] Updating grant %s: removing role %s, keeping %v for user %s",
					g.ID, rule.TargetRole, remaining, userID)
				if err := MgmtClient.UpdateUserGrant(ctx, userID, g.ID, remaining); err != nil {
					log.Printf("[ZITADEL ERROR] Grant update failed: %v", err)
				}
			}
		}
	}

	return nil
}

// AssignUserToRole performs a single role grant for a user via the Zitadel API.
func AssignUserToRole(ctx context.Context, userID, projectID, roleKey string) error {
	if MgmtClient == nil {
		return fmt.Errorf("zitadel client uninitialized; operating in local-policy-only mode")
	}

	if err := MgmtClient.AddUserGrant(ctx, userID, projectID, []string{roleKey}); err != nil {
		return fmt.Errorf("AddUserGrant failed: %v", err)
	}

	log.Printf("[ZITADEL] Granted %s -> %s to %s via Management API.", projectID, roleKey, userID)
	return nil
}

// fetchAllUserGrants paginates through all grants for a user so that operations
// like revocation never silently miss grants beyond the first page.
func fetchAllUserGrants(ctx context.Context, userID string) ([]UserGrant, error) {
	var all []UserGrant
	offset := 0
	for {
		result, err := MgmtClient.ListUserGrants(ctx, userID, SearchParams{
			Limit:  DefaultSearchLimit,
			Offset: offset,
		})
		if err != nil {
			return nil, err
		}
		all = append(all, result.Items...)
		if len(all) >= result.Total || len(result.Items) == 0 {
			break
		}
		offset += len(result.Items)
	}
	return all, nil
}
