package handlers

import (
	"net/http"
	"strings"
)

// Maximum number of IDs allowed per array in a single lookup request. Bounds
// pathological client requests; legitimate UI batches stay well under this.
const lookupMaxBatchSize = 256

type LookupRoleKey struct {
	ProjectID string `json:"project_id"`
	RoleKey   string `json:"role_key"`
}

type LookupRequest struct {
	UserIDs    []string        `json:"user_ids,omitempty"`
	ProjectIDs []string        `json:"project_ids,omitempty"`
	RoleKeys   []LookupRoleKey `json:"role_keys,omitempty"`
	BundleIDs  []string        `json:"bundle_ids,omitempty"`
}

type ResolvedUser struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}

type ResolvedProject struct {
	Name string `json:"name"`
}

type ResolvedRole struct {
	DisplayName string `json:"display_name"`
}

type ResolvedBundle struct {
	Name string `json:"name"`
}

// LookupResponse maps each requested ID to its resolved metadata. Missing IDs
// are simply absent from the corresponding map (never an error). Top-level
// keys are always present so the client can iterate without nil-checks.
type LookupResponse struct {
	Users    map[string]ResolvedUser    `json:"users"`
	Projects map[string]ResolvedProject `json:"projects"`
	Roles    map[string]ResolvedRole    `json:"roles"`
	Bundles  map[string]ResolvedBundle  `json:"bundles"`
}

// handleLookup resolves a batch of UIDs (users, projects, roles, bundles) to
// their human-readable display names. The frontend uses this to render
// <UserName id=…/>, <ProjectName id=…/> etc. components without leaking raw
// UUIDs to operators. Partial misses are tolerated: an unknown ID is simply
// absent from the response map. Each input array is capped at 256 entries.
func handleLookup(w http.ResponseWriter, r *http.Request) {
	var req LookupRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}

	if len(req.UserIDs) > lookupMaxBatchSize {
		jsonValidationErrorResponse(w, "user_ids exceeds max batch size",
			map[string]string{"user_ids": "max 256"})
		return
	}
	if len(req.ProjectIDs) > lookupMaxBatchSize {
		jsonValidationErrorResponse(w, "project_ids exceeds max batch size",
			map[string]string{"project_ids": "max 256"})
		return
	}
	if len(req.RoleKeys) > lookupMaxBatchSize {
		jsonValidationErrorResponse(w, "role_keys exceeds max batch size",
			map[string]string{"role_keys": "max 256"})
		return
	}
	if len(req.BundleIDs) > lookupMaxBatchSize {
		jsonValidationErrorResponse(w, "bundle_ids exceeds max batch size",
			map[string]string{"bundle_ids": "max 256"})
		return
	}

	ctx := r.Context()
	resp := LookupResponse{
		Users:    map[string]ResolvedUser{},
		Projects: map[string]ResolvedProject{},
		Roles:    map[string]ResolvedRole{},
		Bundles:  map[string]ResolvedBundle{},
	}

	src := directorySource()

	for _, id := range dedupeNonEmpty(req.UserIDs) {
		profile, found, err := src.FindUser(ctx, id)
		if err != nil || !found {
			continue
		}
		resp.Users[id] = ResolvedUser{
			DisplayName: profile.Name,
			Email:       profile.Email,
		}
	}

	for _, id := range dedupeNonEmpty(req.ProjectIDs) {
		project, found, err := src.FindProject(ctx, id)
		if err != nil || !found {
			continue
		}
		resp.Projects[id] = ResolvedProject{Name: project.Name}
	}

	for _, ref := range req.RoleKeys {
		pid := strings.TrimSpace(ref.ProjectID)
		key := strings.TrimSpace(ref.RoleKey)
		if pid == "" || key == "" {
			continue
		}
		role, err := dbGetRole(ctx, pid, key)
		if err != nil {
			continue
		}
		resp.Roles[pid+":"+key] = ResolvedRole{DisplayName: role.DisplayName}
	}

	if len(req.BundleIDs) > 0 {
		bundles, err := dbGetAllBundles(ctx)
		if err == nil {
			byID := make(map[string]string, len(bundles))
			for _, b := range bundles {
				byID[b.ID] = b.Name
			}
			for _, id := range dedupeNonEmpty(req.BundleIDs) {
				if name, ok := byID[id]; ok {
					resp.Bundles[id] = ResolvedBundle{Name: name}
				}
			}
		}
	}

	jsonResponse(w, http.StatusOK, resp)
}

// dedupeNonEmpty returns ids with empty/whitespace entries removed and
// duplicates collapsed, preserving first-seen order.
func dedupeNonEmpty(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
