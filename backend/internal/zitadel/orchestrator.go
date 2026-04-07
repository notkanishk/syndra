package zitadel

import (
	"context"
	"fmt"
	"log"

	"mkauth/internal/db"

	mgmt_pb "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/management"
)

// EnforceMappingRules is triggered when a user is modified.
// It cross-references their new roles against our logic engine and pushes grants back to Zitadel.
func EnforceMappingRules(ctx context.Context, userID, sourceProjectID, sourceRoleKey string) error {
	if MgmtClient == nil {
		log.Println("Skipping orchestration: Zitadel client is not initialized.")
		return nil
	}

	// 1. Fetch relevant mapping rules pushing outward from this source
	rules, err := db.GetActiveMappingRules(ctx)
	if err != nil {
		return fmt.Errorf("failed to query mapping rules: %v", err)
	}

	for _, rule := range rules {
		if rule.SourceProject == sourceProjectID && rule.SourceRole == sourceRoleKey {
			log.Printf("Rule Match Detected! Enforcing propagation to %s:%s for User %s", rule.TargetProject, rule.TargetRole, userID)
			
			err := AssignUserToRole(ctx, userID, rule.TargetProject, rule.TargetRole)
			if err != nil {
				log.Printf("[ERROR] Failing to execute Zitadel grant: %v", err)
				// We do not break the loop. Attempt subsequent rules.
			}
		}
	}

	return nil
}

// AssignUserToRole performs the raw gRPC/HTTP call to Zitadel adding a UserGrant.
func AssignUserToRole(ctx context.Context, userID, projectID, roleKey string) error {
	if MgmtClient == nil {
		return fmt.Errorf("zitadel client uninitialized")
	}

	req := &mgmt_pb.AddUserGrantRequest{
		UserId:    userID,
		ProjectId: projectID,
		RoleKeys:  []string{roleKey},
	}

	_, err := MgmtClient.AddUserGrant(ctx, req)
	if err != nil {
		return fmt.Errorf("AddUserGrant failed: %v", err)
	}

	log.Printf("Successfully granted %s -> %s to %s via Zitadel API.", projectID, roleKey, userID)
	return nil
}
