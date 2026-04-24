package seed

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"mkauth/internal/cache"
	"mkauth/internal/db"
	"mkauth/internal/demo"
	"mkauth/internal/zitadel"
)

func EnsureDemoData(ctx context.Context) error {
	if !demoEnabled() {
		if liveDirectoryActive() {
			log.Println("[SEED] Live Zitadel directory active; skipping demo seed. Set MKAUTH_SEED_DEMO=true to force-enable.")
		} else {
			log.Println("[SEED] Demo seed disabled via MKAUTH_SEED_DEMO.")
		}
		return nil
	}

	log.Println("[SEED] Ensuring demo data exists for the current v1 experience...")

	if err := seedBundles(ctx); err != nil {
		return err
	}
	if err := seedDirectGrants(ctx); err != nil {
		return err
	}
	if err := seedRules(ctx); err != nil {
		return err
	}
	if err := seedClaimProfiles(ctx); err != nil {
		return err
	}
	if err := seedAssignments(ctx); err != nil {
		return err
	}
	if err := seedAuditLogs(ctx); err != nil {
		return err
	}
	if err := seedAccessRequests(ctx); err != nil {
		return err
	}

	projectIDs := make([]string, 0, len(demo.Applications()))
	for _, app := range demo.Applications() {
		projectIDs = append(projectIDs, app.ProjectID)
	}
	for _, user := range demo.Users() {
		cache.RebuildUserCache(ctx, user.ID, projectIDs)
	}

	return nil
}

// demoEnabled decides whether EnsureDemoData runs. The explicit env override
// wins in both directions so operators can force-enable in live mode (useful
// for integration tests against a staging Zitadel) or force-disable in demo
// mode (useful for reproducing empty-DB behavior locally). When unset, the
// default is "off when the live directory is active, on otherwise" —
// exactly what a freshly-configured production deployment wants.
//
// Keying off the post-Init client state (rather than raw env presence) keeps
// this decision aligned with directory.Init: if ZITADEL_MACHINE_KEY_PATH is
// set but unreadable, InitClient fails, MgmtClient stays nil, the directory
// falls back to demoSource — and the seed must also run so the app doesn't
// end up serving the demo catalog on top of an empty DB.
func demoEnabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("MKAUTH_SEED_DEMO")))
	switch value {
	case "1", "true", "yes":
		return true
	case "0", "false", "no":
		return false
	}
	return !liveDirectoryActive()
}

// liveDirectoryActive reports whether the live Zitadel Management client is
// ready. This is the exact signal directory.Init uses to select the source,
// so seed gating and directory mode can never disagree.
//
// MUST be called after zitadel.InitClient() has run. main.go enforces this
// ordering. Tests that call EnsureDemoData directly without InitClient see
// MgmtClient == nil, which correctly treats them as demo mode.
func liveDirectoryActive() bool {
	return zitadel.MgmtClient != nil
}

func seedBundles(ctx context.Context) error {
	type bundleSeed struct {
		Name        string
		Description string
		Roles       [][2]string
	}

	bundles := []bundleSeed{
		{
			Name:        "Student Access",
			Description: "Common member access for active student makers.",
			Roles:       [][2]string{{"printing", "member"}, {"wiki", "member"}},
		},
		{
			Name:        "Staff Onboarding",
			Description: "Starter bundle for staff and front-desk operators.",
			Roles:       [][2]string{{"platform", "support"}, {"doors", "staff_entry"}, {"wiki", "editor"}},
		},
		{
			Name:        "Prototyping Mentor",
			Description: "Cross-lab mentor access spanning documentation and equipment.",
			Roles:       [][2]string{{"printing", "calibrator"}, {"laser", "mentor"}, {"wiki", "editor"}},
		},
	}

	for _, bundle := range bundles {
		var bundleID string
		err := db.PG.QueryRow(ctx, `
			INSERT INTO bundles (name, description)
			VALUES ($1, $2)
			ON CONFLICT (name) DO UPDATE SET description = EXCLUDED.description
			RETURNING id
		`, bundle.Name, bundle.Description).Scan(&bundleID)
		if err != nil {
			return err
		}

		for _, role := range bundle.Roles {
			if _, err := db.PG.Exec(ctx, `
				INSERT INTO bundle_roles (bundle_id, zitadel_project_id, zitadel_role_key)
				VALUES ($1, $2, $3)
				ON CONFLICT DO NOTHING
			`, bundleID, role[0], role[1]); err != nil {
				return err
			}
		}
	}

	return nil
}

func seedRules(ctx context.Context) error {
	rules := [][4]string{
		{"printing", "member", "doors", "3d_lab_pin"},
		{"laser", "mentor", "doors", "laser_room_pin"},
		{"platform", "admin", "wiki", "admin"},
		{"printing", "manager", "finance", "material_checkout"},
	}

	for _, rule := range rules {
		if _, err := db.PG.Exec(ctx, `
			INSERT INTO mapping_rules (
				source_zitadel_project_id,
				source_zitadel_role_key,
				target_zitadel_project_id,
				target_zitadel_role_key
			)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT DO NOTHING
		`, rule[0], rule[1], rule[2], rule[3]); err != nil {
			return err
		}
	}

	return nil
}

func seedDirectGrants(ctx context.Context) error {
	for _, user := range demo.Users() {
		for _, grant := range demo.BaseGrants(user.ID) {
			if _, err := db.UpsertDirectGrant(ctx, user.ID, grant.ProjectID, grant.RoleKey, "seed", "Seeded demo direct grant", nil); err != nil {
				return err
			}
		}
	}

	expiresAt := time.Now().UTC().Add(5 * 24 * time.Hour)
	if _, err := db.UpsertDirectGrant(ctx, "ava_guest", "printing", "member", "seed", "Temporary residency printer access", &expiresAt); err != nil {
		return err
	}
	return nil
}

func seedClaimProfiles(ctx context.Context) error {
	for _, app := range demo.Applications() {
		if _, err := db.PG.Exec(ctx, `
			INSERT INTO claim_profiles (zitadel_project_id, claim_name, format_type)
			VALUES ($1, $2, $3)
			ON CONFLICT (zitadel_project_id) DO UPDATE
			SET claim_name = EXCLUDED.claim_name,
			    format_type = EXCLUDED.format_type
		`, app.ProjectID, app.ClaimName, app.FormatType); err != nil {
			return err
		}
	}

	return nil
}

func seedAssignments(ctx context.Context) error {
	assignments := map[string]string{
		"sam_student": "Student Access",
		"maya_staff":  "Staff Onboarding",
		"leo_mentor":  "Prototyping Mentor",
	}

	for userID, bundleName := range assignments {
		var bundleID string
		if err := db.PG.QueryRow(ctx, `SELECT id FROM bundles WHERE name = $1`, bundleName).Scan(&bundleID); err != nil {
			return err
		}
		if _, err := db.PG.Exec(ctx, `
			INSERT INTO user_bundle_assignments (user_id, bundle_id)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, userID, bundleID); err != nil {
			return err
		}
	}

	return nil
}

func seedAuditLogs(ctx context.Context) error {
	var count int
	if err := db.PG.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	events := [][4]string{
		{"alice.rivera", "sam_student", "bundle.assigned", "Student Access"},
		{"alice.rivera", "-", "mapping_rule.created", "printing:member->doors:3d_lab_pin"},
		{"maya.chen", "leo_mentor", "bundle.assigned", "Prototyping Mentor"},
		{"system", "-", "cache.rebuilt", "demo_seed_v1"},
	}

	for _, event := range events {
		if _, err := db.PG.Exec(ctx, `
			INSERT INTO audit_logs (actor_zitadel_user_id, target_zitadel_user_id, action, resource_id)
			VALUES ($1, $2, $3, $4)
		`, event[0], event[1], event[2], event[3]); err != nil {
			return err
		}
	}

	return nil
}

func seedAccessRequests(ctx context.Context) error {
	requests, err := db.GetAccessRequests(ctx, "")
	if err != nil {
		return err
	}
	if len(requests) > 0 {
		return nil
	}

	durationDays := 21
	if _, err := db.CreateAccessRequest(ctx, "ava_guest", "laser", "trainee", "Needs supervised laser access during the current residency block.", &durationDays); err != nil {
		return err
	}
	if _, err := db.CreateAccessRequest(ctx, "sam_student", "finance", "material_checkout", "Helping with weekend materials desk coverage.", nil); err != nil {
		return err
	}
	return nil
}
