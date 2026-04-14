package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"sort"

	"github.com/jackc/pgx/v5"

	"mkauth/internal/demo"
	"mkauth/internal/models"
	"mkauth/internal/zitadel"
)

// CreateRoleRequest is the input for creating a new role.
type CreateRoleRequest struct {
	ProjectID   string    `json:"project_id"`
	RoleKey     string    `json:"role_key"`
	DisplayName string    `json:"display_name"`
	Description string    `json:"description"`
	Group       string    `json:"group"`
	CloneFrom   *CloneRef `json:"clone_from,omitempty"`
}

// CloneRef identifies the source role to clone metadata from.
type CloneRef struct {
	ProjectID string `json:"project_id"`
	RoleKey   string `json:"role_key"`
}

var roleKeyPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// ErrCloneSourceNotFound is returned when the clone source role cannot be resolved.
var ErrCloneSourceNotFound = errors.New("clone source role not found")

// CreateRole creates a new role, optionally cloning metadata from a source role.
// Persists locally first, then propagates to Zitadel. If Zitadel propagation
// fails, the local row is rolled back to avoid MkAuth tracking a role that
// doesn't exist upstream.
func CreateRole(ctx context.Context, req CreateRoleRequest, createdBy string) (models.Role, error) {
	if !roleKeyPattern.MatchString(req.RoleKey) {
		return models.Role{}, fmt.Errorf("role_key must match [a-zA-Z0-9_-]+")
	}

	// Resolve clone source if provided.
	if req.CloneFrom != nil {
		source, err := resolveRoleMetadata(ctx, req.CloneFrom.ProjectID, req.CloneFrom.RoleKey)
		if err != nil {
			if errors.Is(err, ErrCloneSourceNotFound) {
				return models.Role{}, err
			}
			return models.Role{}, fmt.Errorf("resolve clone source: %w", err)
		}
		if req.DisplayName == "" {
			req.DisplayName = source.DisplayName
		}
		if req.Description == "" {
			req.Description = source.Description
		}
	}

	// 1. Persist locally first — catches duplicates before touching Zitadel.
	var clonedFromProject, clonedFromRole *string
	if req.CloneFrom != nil {
		clonedFromProject = &req.CloneFrom.ProjectID
		clonedFromRole = &req.CloneFrom.RoleKey
	}

	id, err := svcDbCreateRole(ctx, req.ProjectID, req.RoleKey, req.DisplayName, req.Description,
		req.Group, createdBy, clonedFromProject, clonedFromRole)
	if err != nil {
		return models.Role{}, err
	}

	// 2. Propagate to Zitadel. If this fails, roll back the local row.
	if zitadel.MgmtClient != nil {
		if err := zitadel.MgmtClient.AddProjectRole(ctx, req.ProjectID, req.RoleKey, req.DisplayName, req.Group); err != nil {
			// Compensating rollback: remove the local row so state stays consistent.
			if delErr := svcDbDeleteRole(ctx, id); delErr != nil {
				log.Printf("[ROLES] WARNING: failed to rollback local role %s after Zitadel failure: %v", id, delErr)
			}
			return models.Role{}, fmt.Errorf("zitadel role creation failed: %w", err)
		}
		log.Printf("[ROLES] Propagated role %s:%s to Zitadel", req.ProjectID, req.RoleKey)
	} else {
		log.Printf("[ROLES] Skipping Zitadel propagation (local-policy-only mode)")
	}

	// 3. Audit log.
	_ = svcInsertAuditLog(ctx, createdBy, "-", "role.created", id)

	role, err := svcDbGetRole(ctx, req.ProjectID, req.RoleKey)
	if err != nil {
		return models.Role{}, fmt.Errorf("fetch created role: %w", err)
	}
	return role, nil
}

// resolveRoleMetadata looks up role metadata from local DB, then demo catalog.
// Returns ErrCloneSourceNotFound when the role doesn't exist in any source.
// Returns a wrapped DB error if the local lookup fails for reasons other than
// "row not found" — this prevents silently falling back to demo data when
// the database is actually unhealthy.
func resolveRoleMetadata(ctx context.Context, projectID, roleKey string) (roleMetadata, error) {
	// Try local DB first.
	r, err := svcDbGetRole(ctx, projectID, roleKey)
	if err == nil {
		return roleMetadata{DisplayName: r.DisplayName, Description: r.Description}, nil
	}
	// Only fall through to demo catalog if the role genuinely doesn't exist.
	// Real DB errors (connection failure, query error) must surface immediately.
	if !errors.Is(err, pgx.ErrNoRows) {
		return roleMetadata{}, fmt.Errorf("lookup role %s:%s: %w", projectID, roleKey, err)
	}

	// Try demo catalog.
	proj, ok := demo.FindProject(projectID)
	if ok {
		for _, role := range proj.Roles {
			if role.Key == roleKey {
				return roleMetadata{DisplayName: role.Label, Description: role.Description}, nil
			}
		}
	}

	return roleMetadata{}, ErrCloneSourceNotFound
}

type roleMetadata struct {
	DisplayName string
	Description string
}

// GlobalRoleCatalog builds a consolidated view of all roles across all sources.
func GlobalRoleCatalog(ctx context.Context) ([]models.CatalogRole, error) {
	// Collect roles from all sources into a deduplicated map.
	type catalogEntry struct {
		projectID   string
		roleKey     string
		displayName string
		description string
		source      string
	}
	seen := make(map[string]*catalogEntry) // key: "projectID:roleKey"

	// 1. Local DB roles (highest priority source label).
	localRoles, err := svcDbGetAllLocalRoles(ctx)
	if err != nil {
		return nil, fmt.Errorf("load local roles: %w", err)
	}
	for _, r := range localRoles {
		key := r.ProjectID + ":" + r.RoleKey
		seen[key] = &catalogEntry{
			projectID: r.ProjectID, roleKey: r.RoleKey,
			displayName: r.DisplayName, description: r.Description,
			source: "mkauth",
		}
	}

	// 2. Demo catalog roles.
	for _, proj := range demo.Projects() {
		for _, role := range proj.Roles {
			key := proj.ID + ":" + role.Key
			if _, exists := seen[key]; !exists {
				seen[key] = &catalogEntry{
					projectID: proj.ID, roleKey: role.Key,
					displayName: role.Label, description: role.Description,
					source: "demo",
				}
			}
		}
	}

	// 3. Referenced roles from DB (bundle_roles, mapping_rules, direct_role_grants).
	refs, err := svcDbGetAllReferencedRoleKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("load referenced role keys: %w", err)
	}
	for _, ref := range refs {
		key := ref[0] + ":" + ref[1]
		if _, exists := seen[key]; !exists {
			seen[key] = &catalogEntry{
				projectID: ref[0], roleKey: ref[1],
				displayName: ref[1], description: "",
				source: "referenced",
			}
		}
	}

	// Load usage counts.
	usageCounts, err := svcDbGetRoleUsageCounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("load role usage counts: %w", err)
	}
	userCounts, err := svcDbGetAssignedUserCounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("load assigned user counts: %w", err)
	}

	// Build catalog.
	catalog := make([]models.CatalogRole, 0, len(seen))
	for key, entry := range seen {
		usage := usageCounts[key]
		assignedUsers := userCounts[key]
		projectName := demo.ProjectName(entry.projectID)

		cr := models.CatalogRole{
			ProjectID:         entry.projectID,
			ProjectName:       projectName,
			RoleKey:           entry.roleKey,
			DisplayName:       entry.displayName,
			Description:       entry.description,
			BundleCount:       usage.BundleCount,
			RuleCount:         usage.RuleCount,
			AssignedUserCount: assignedUsers,
			IsUnused:          usage.BundleCount+usage.RuleCount == 0 && assignedUsers == 0,
			Source:            entry.source,
			DisplayLabel:      projectName + ": " + entry.displayName,
		}
		catalog = append(catalog, cr)
	}

	sort.Slice(catalog, func(i, j int) bool {
		if catalog[i].ProjectName != catalog[j].ProjectName {
			return catalog[i].ProjectName < catalog[j].ProjectName
		}
		return catalog[i].RoleKey < catalog[j].RoleKey
	})

	return catalog, nil
}
