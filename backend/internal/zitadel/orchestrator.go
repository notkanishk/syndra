package zitadel

import (
	"context"
	"fmt"
	"log"

	"mkauth/internal/db"
)

// ZitadelClient is a minimal interface over the Zitadel Management API.
// The live implementation is wired in InitClient once credentials are available.
type ZitadelClient interface {
	AddUserGrant(ctx context.Context, userID, projectID string, roleKeys []string) error
}

// EnforceMappingRules is triggered when a user is modified.
// It cross-references their new roles against our logic engine
// and pushes grants back to Zitadel when the client is live.
func EnforceMappingRules(ctx context.Context, userID, sourceProjectID, sourceRoleKey string) error {
	if MgmtClient == nil {
		log.Println("ℹ️  Skipping orchestration: Zitadel client is not initialized (local-policy-only mode).")
		return nil
	}

	liveClient, ok := MgmtClient.(ZitadelClient)
	if !ok {
		log.Println("⚠️  MgmtClient does not implement ZitadelClient interface; skipping orchestration.")
		return nil
	}

	// Fetch all mapping rules and apply those matching the triggering role
	rules, err := db.GetActiveMappingRules(ctx)
	if err != nil {
		return fmt.Errorf("failed to query mapping rules: %v", err)
	}

	for _, rule := range rules {
		if rule.SourceProject == sourceProjectID && rule.SourceRole == sourceRoleKey {
			log.Printf("📌 Rule matched: propagating %s:%s → %s:%s for user %s",
				sourceProjectID, sourceRoleKey, rule.TargetProject, rule.TargetRole, userID)

			if err := liveClient.AddUserGrant(ctx, userID, rule.TargetProject, []string{rule.TargetRole}); err != nil {
				log.Printf("[ERROR] Zitadel grant failed: %v", err)
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

	liveClient, ok := MgmtClient.(ZitadelClient)
	if !ok {
		return fmt.Errorf("MgmtClient does not implement ZitadelClient")
	}

	if err := liveClient.AddUserGrant(ctx, userID, projectID, []string{roleKey}); err != nil {
		return fmt.Errorf("AddUserGrant failed: %v", err)
	}

	log.Printf("✅ Granted %s → %s to %s via Zitadel API.", projectID, roleKey, userID)
	return nil
}
