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

// ZitadelClient is the interface over the Zitadel Management API.
// The live implementation is wired in InitClient once credentials are available.
type ZitadelClient interface {
	AddUserGrant(ctx context.Context, userID, projectID string, roleKeys []string) error
	RemoveUserGrant(ctx context.Context, userID, grantID string) error
	ListUserGrants(ctx context.Context, userID string) ([]UserGrant, error)
	GetUser(ctx context.Context, userID string) (*ZitadelUser, error)
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
