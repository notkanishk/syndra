package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"sort"
	"syndra/internal/textsort"
	"time"

	"github.com/jackc/pgx/v5"

	"syndra/internal/db"
	"syndra/internal/directory"
	"syndra/internal/extcall"
	"syndra/internal/models"
	"syndra/internal/zitadel"
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
// It is the one way a role is made, from Access › Roles and from Zitadel ›
// Projects alike: writeRole records it, sends it, and reads it back before
// undoing anything.
func CreateRole(ctx context.Context, req CreateRoleRequest, createdBy string) (models.Role, error) {
	if !roleKeyPattern.MatchString(req.RoleKey) {
		return models.Role{}, ErrInvalidRoleKey
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

	if req.DisplayName == "" {
		req.DisplayName = req.RoleKey // Zitadel and the roles table both need one
	}

	var clonedFromProject, clonedFromRole *string
	if req.CloneFrom != nil {
		clonedFromProject = &req.CloneFrom.ProjectID
		clonedFromRole = &req.CloneFrom.RoleKey
	}

	var id string
	err := writeRole(ctx, roleWrite{
		actor: createdBy, action: auditRoleCreated, projectID: req.ProjectID, roleKey: req.RoleKey,
		details: map[string]any{"display_name": req.DisplayName, "group": req.Group},
		// The local row first also catches a duplicate before Zitadel is touched.
		local: func(ctx context.Context) (err error) {
			id, err = svcDbCreateRole(ctx, req.ProjectID, req.RoleKey, req.DisplayName, req.Description,
				req.Group, createdBy, clonedFromProject, clonedFromRole)
			return err
		},
		undo: func(ctx context.Context) error { return svcDbDeleteRole(ctx, id) },
		send: func(ctx context.Context, c zitadel.ZitadelClient) error {
			return c.AddProjectRole(ctx, req.ProjectID, req.RoleKey, req.DisplayName, req.Group)
		},
		landed: func(r *zitadel.ProjectRoleResult) bool {
			return r != nil && r.DisplayName == req.DisplayName && r.Group == req.Group
		},
	})
	if err != nil {
		return models.Role{}, err
	}

	role, err := svcDbGetRole(ctx, req.ProjectID, req.RoleKey)
	if err != nil {
		return models.Role{}, fmt.Errorf("fetch created role: %w", err)
	}
	return role, nil
}

// UpdateRole renames a role or changes its group, in Zitadel and in Syndra's
// record of it (when there is one: a role made in Zitadel directly has none).
func UpdateRole(ctx context.Context, projectID, roleKey, displayName, group, actor string) error {
	if displayName == "" {
		displayName = roleKey
	}
	var before *models.Role
	return writeRole(ctx, roleWrite{
		actor: actor, action: auditRoleUpdated, projectID: projectID, roleKey: roleKey,
		details: map[string]any{"display_name": displayName, "group": group},
		local: func(ctx context.Context) error {
			old, err := svcDbGetRole(ctx, projectID, roleKey)
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			if err != nil {
				return err
			}
			before = &old
			return svcDbUpdateRole(ctx, projectID, roleKey, displayName, group)
		},
		undo: func(ctx context.Context) error {
			if before == nil {
				return nil
			}
			return svcDbUpdateRole(ctx, projectID, roleKey, before.DisplayName, before.Group)
		},
		send: func(ctx context.Context, c zitadel.ZitadelClient) error {
			return c.UpdateProjectRole(ctx, projectID, roleKey, displayName, group)
		},
		landed: func(r *zitadel.ProjectRoleResult) bool {
			return r != nil && r.DisplayName == displayName && r.Group == group
		},
	})
}

// RoleInUseError refuses a delete while Syndra itself still gives the role.
// Deleting it in Zitadel then would leave Syndra sending a role that no longer
// exists, so the giving is undone first, where each removal is traced.
type RoleInUseError struct {
	Given, Bundles, Rules int
}

func (e *RoleInUseError) Error() string {
	return fmt.Sprintf("syndra still gives this role: %d people, %d bundles, %d rules", e.Given, e.Bundles, e.Rules)
}

// RoleHeldError refuses a delete that would take the role from people Zitadel
// shows holding it, until the caller confirms exactly that many. Unknown means
// Syndra has no complete, current read of who holds it, so it cannot say how
// many would lose it and does not guess.
type RoleHeldError struct {
	Holders []models.UserProfile
	Unknown bool
}

func (e *RoleHeldError) Error() string {
	if e.Unknown {
		return "syndra cannot tell who holds this role in zitadel"
	}
	return fmt.Sprintf("%d people hold this role in zitadel", len(e.Holders))
}

// DeleteRole deletes a role in Zitadel and Syndra's record of it.
//
// Zitadel takes a deleted role from everyone who holds it, at once, with no
// trace of its own. So this never strips silently: it refuses while Syndra
// gives the role (RoleInUseError), and while anyone holds it in Zitadel unless
// confirmHolders names exactly how many (RoleHeldError). The check runs inside
// the trace's transaction, under the access lock, so nobody is given the role
// between the count and the write.
func DeleteRole(ctx context.Context, projectID, roleKey, actor string, confirmHolders int) error {
	var before *models.Role
	details := map[string]any{}
	return writeRole(ctx, roleWrite{
		actor: actor, action: auditRoleDeleted, projectID: projectID, roleKey: roleKey,
		details: details,
		local: func(ctx context.Context) error {
			holders, err := roleDeleteGuard(ctx, projectID, roleKey, confirmHolders)
			if err != nil {
				return err
			}
			ids := make([]string, 0, len(holders))
			for _, h := range holders {
				ids = append(ids, h.ID)
			}
			details["holders_lost"] = ids
			old, err := svcDbGetRole(ctx, projectID, roleKey)
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			if err != nil {
				return err
			}
			before = &old
			return svcDbDeleteRole(ctx, old.ID)
		},
		undo: func(ctx context.Context) error {
			if before == nil {
				return nil
			}
			// ponytail: the row comes back with a new id and created_at; nothing references roles.id.
			_, err := svcDbCreateRole(ctx, before.ProjectID, before.RoleKey, before.DisplayName, before.Description,
				before.Group, before.CreatedBy, nonEmpty(before.ClonedFromProject), nonEmpty(before.ClonedFromRole))
			return err
		},
		send: func(ctx context.Context, c zitadel.ZitadelClient) error {
			return c.DeleteProjectRole(ctx, projectID, roleKey)
		},
		landed: func(r *zitadel.ProjectRoleResult) bool { return r == nil },
	})
}

// roleDeleteGuard returns who would lose the role, alphabetically, or why the
// delete may not go ahead.
func roleDeleteGuard(ctx context.Context, projectID, roleKey string, confirmHolders int) ([]models.UserProfile, error) {
	usage, err := svcDbGetRoleUsageCounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("load role usage: %w", err)
	}
	view, err := roleMembersFor(ctx, projectID, roleKey)
	if err != nil {
		return nil, fmt.Errorf("read who holds the role: %w", err)
	}
	u := usage[projectID+":"+roleKey]
	if len(view.Members) > 0 || u.BundleCount > 0 || u.RuleCount > 0 {
		return nil, &RoleInUseError{Given: len(view.Members), Bundles: u.BundleCount, Rules: u.RuleCount}
	}
	if view.ObservedOnly == nil || !view.Observation.Current || view.Observation.Truncated {
		return nil, &RoleHeldError{Unknown: true}
	}
	// Everyone Zitadel shows holding it counts, a holder whose removal is on its
	// way too (owner, 10 Nov 2026): they hold it until Zitadel shows it gone,
	// and deleting the role under them is what this confirmation is for.
	holders := make([]models.UserProfile, 0, len(view.ObservedOnly))
	for _, m := range view.ObservedOnly {
		holders = append(holders, m.User)
	}
	textsort.ByName(holders, func(u models.UserProfile) string { return u.Name }, nil)
	if len(holders) != confirmHolders {
		return nil, &RoleHeldError{Holders: holders}
	}
	return holders, nil
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Audit actions for role writes. role.not_changed is the second row of a
// write Zitadel refused: Syndra undid its own record and says so.
// role.change_unknown is the second row of a write whose answer was lost and
// whose read-back failed: Syndra kept its record and says it can't tell.
const (
	auditRoleCreated       = "role.created"
	auditRoleUpdated       = "role.updated"
	auditRoleDeleted       = "role.deleted"
	auditRoleNotChanged    = "role.not_changed"
	auditRoleChangeUnknown = "role.change_unknown"
	auditRoleNotRestored   = "role.not_restored" // a refused write whose undo failed too
	auditNoTarget          = "-"
)

// ErrInvalidRoleKey is a role key Zitadel and Syndra would not agree on.
var ErrInvalidRoleKey = errors.New("role_key must match [a-zA-Z0-9_-]+")

// ErrRoleNotChangedInZitadel wraps a Zitadel refusal the read-back confirmed:
// nothing changed there, and Syndra undid its own record.
var ErrRoleNotChangedInZitadel = errors.New("zitadel did not make the change, so syndra undid its record")

// ErrRoleRecordNotRestored is a refusal Zitadel made final (so nothing changed
// there) whose undo of Syndra's own record failed too: Syndra's list of roles
// may show a change Zitadel does not hold. Never "Syndra put its record back".
var ErrRoleRecordNotRestored = errors.New("zitadel did not make the change, and syndra could not undo its record")

// RoleRecordNotRestoredError carries the reference an operator can give
// whoever runs the server: the target of the role.not_restored audit row.
type RoleRecordNotRestoredError struct{ Reference string }

func (e *RoleRecordNotRestoredError) Error() string {
	return ErrRoleRecordNotRestored.Error() + " (" + e.Reference + ")"
}
func (e *RoleRecordNotRestoredError) Is(t error) bool { return t == ErrRoleRecordNotRestored }

// ErrRoleChangeUnknown is a write whose answer was lost (a timeout, a broken
// connection, a 5xx) and whose read-back failed too. Zitadel may have made
// the change: a delete may already have taken the role from every holder. So
// Syndra keeps its record rather than undo something that may have happened.
var ErrRoleChangeUnknown = errors.New("zitadel didn't answer and the read-back failed, so syndra can't tell whether the change was made; it kept its record")

var (
	svcDbUpdateRole          = db.UpdateRole
	svcInsertAuditLogDetails = db.InsertAuditLogDetails
	roleMembersFor           = RoleMembers
)

// roleWrite is one write to a project role. local and undo change Syndra's
// record; send changes Zitadel; landed judges the read-back (nil when Zitadel
// has no such role): is Zitadel now as asked?
type roleWrite struct {
	actor, action, projectID, roleKey string
	details                           map[string]any
	local, undo                       func(context.Context) error
	send                              func(context.Context, zitadel.ZitadelClient) error
	landed                            func(*zitadel.ProjectRoleResult) bool
}

// writeRole is the one pipe for a project role, from any screen:
//
//  1. the trace, before Zitadel is touched: Syndra's record and its audit row
//     in one transaction under the access lock. No trace, no write.
//  2. the call.
//  3. on any error, a read-back before anything is undone: a 503 or a timeout
//     can hide a write that landed, and undoing that would orphan the role.
//  4. only when Zitadel is known not to hold the change, the undo, traced as
//     role.not_changed in its own transaction: the read-back shows it absent,
//     or Zitadel answered a refusal (a final 4xx). When the answer was lost
//     AND the read-back failed, nobody knows: the record stays, the audit
//     says role.change_unknown, and the caller gets ErrRoleChangeUnknown.
//
// The directory's cached roles for the project are dropped either way, so
// every picker sees what Zitadel now holds.
func writeRole(ctx context.Context, w roleWrite) error {
	if err := svcInTxLockingAccess(ctx, func(ctx context.Context) error {
		if err := w.local(ctx); err != nil {
			return err
		}
		return svcInsertAuditLogDetails(ctx, w.actor, auditNoTarget, w.action, w.projectID+"/"+w.roleKey, w.details)
	}); err != nil {
		return err
	}
	defer directory.Default.InvalidateProject(w.projectID)

	client := zitadel.MgmtClient
	if client == nil {
		log.Printf("[ROLES] %s %s:%s recorded locally only (no Zitadel client)", w.action, w.projectID, w.roleKey)
		return nil
	}

	sendErr := w.send(ctx, client)
	if sendErr == nil {
		return nil
	}
	// Detached: a request that timed out must still read back and undo.
	dctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	found, readErr := readBackRole(dctx, client, w.projectID, w.roleKey)
	if readErr == nil && w.landed(found) {
		log.Printf("[ROLES] %s %s:%s answered %v, but the read-back shows it landed", w.action, w.projectID, w.roleKey, sendErr)
		return nil
	}
	why := map[string]any{"action": w.action, "error": sendErr.Error()}
	if readErr != nil {
		why["read_back_error"] = readErr.Error()
		if answerLost(sendErr) {
			if err := svcInsertAuditLogDetails(dctx, w.actor, auditNoTarget, auditRoleChangeUnknown, w.projectID+"/"+w.roleKey, why); err != nil {
				log.Printf("[ROLES] WARNING: could not audit the unknown outcome of %s %s:%s: %v", w.action, w.projectID, w.roleKey, err)
			}
			return fmt.Errorf("%w: %v", ErrRoleChangeUnknown, sendErr)
		}
	}
	if err := svcInTxLockingAccess(dctx, func(ctx context.Context) error {
		if err := w.undo(ctx); err != nil {
			return err
		}
		return svcInsertAuditLogDetails(ctx, w.actor, auditNoTarget, auditRoleNotChanged, w.projectID+"/"+w.roleKey, why)
	}); err != nil {
		// The undo's own transaction rolled back, audit row included, so the
		// failure gets its own row. Never say "put its record back".
		log.Printf("[ROLES] WARNING: could not undo %s %s:%s after Zitadel refused it: %v", w.action, w.projectID, w.roleKey, err)
		why["undo_error"] = err.Error()
		ref := w.projectID + "/" + w.roleKey
		if aerr := svcInsertAuditLogDetails(dctx, w.actor, auditNoTarget, auditRoleNotRestored, ref, why); aerr != nil {
			log.Printf("[ROLES] WARNING: could not audit the failed undo of %s %s:%s: %v", w.action, w.projectID, w.roleKey, aerr)
		}
		return fmt.Errorf("%w: %v", &RoleRecordNotRestoredError{Reference: ref}, sendErr)
	}
	return fmt.Errorf("%w: %v", ErrRoleNotChangedInZitadel, sendErr)
}

// answerLost is whether err leaves Zitadel's state unknown: no answer at all,
// or a status that means "try again" (408, 429, 5xx). Any other status is
// Zitadel's final word that it did not do it.
func answerLost(err error) bool {
	var se *zitadel.StatusError
	return !errors.As(err, &se) || extcall.RetryableStatus(se.Code)
}

// readBackRole is the role as Zitadel holds it now, or nil when it has none.
// ponytail: first page only (500 roles); page on when a project has more.
func readBackRole(ctx context.Context, c zitadel.ZitadelClient, projectID, roleKey string) (*zitadel.ProjectRoleResult, error) {
	res, err := c.ListProjectRoles(ctx, projectID, zitadel.SearchParams{Limit: zitadel.DefaultSearchLimit})
	if err != nil {
		return nil, err
	}
	for i := range res.Items {
		if res.Items[i].Key == roleKey {
			return &res.Items[i], nil
		}
	}
	return nil, nil
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

	// Try the directory source (live Zitadel or demo fallback).
	proj, ok, dirErr := directory.Default.FindProject(ctx, projectID)
	if dirErr != nil {
		return roleMetadata{}, fmt.Errorf("lookup project %s in directory: %w", projectID, dirErr)
	}
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

type catalogEntry struct {
	projectID   string
	roleKey     string
	displayName string
	description string
	group       string
	clonedFromP string
	clonedFromR string
	source      string
}

// collectCatalogEntries is the one definition of "a role the catalogue knows":
// the union of local, directory and referenced roles, keyed "projectID:roleKey".
// GlobalRoleCatalog enriches it; Insights reads it bare, so neither can mean a
// different set by "every role".
func collectCatalogEntries(ctx context.Context) (map[string]*catalogEntry, []models.ProjectCatalog, error) {
	seen := make(map[string]*catalogEntry) // key: "projectID:roleKey"

	// 1. Local DB roles (highest priority source label).
	localRoles, err := svcDbGetAllLocalRoles(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("load local roles: %w", err)
	}
	for _, r := range localRoles {
		key := r.ProjectID + ":" + r.RoleKey
		seen[key] = &catalogEntry{
			projectID: r.ProjectID, roleKey: r.RoleKey,
			displayName: r.DisplayName, description: r.Description,
			group:       r.Group,
			clonedFromP: r.ClonedFromProject, clonedFromR: r.ClonedFromRole,
			source: "syndra",
		}
	}

	// 2. Directory-source roles. Source tag is "zitadel" in live mode or "demo"
	// when the directory is falling back to the local catalog — the UI
	// distinguishes discovered directory roles from operator-created
	// ("syndra") ones and from persisted references.
	dirProjects, err := directory.Default.Projects(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("load projects from directory: %w", err)
	}
	dirSource := directory.Default.Tag()
	for _, proj := range dirProjects {
		for _, role := range proj.Roles {
			key := proj.ID + ":" + role.Key
			if _, exists := seen[key]; !exists {
				seen[key] = &catalogEntry{
					projectID: proj.ID, roleKey: role.Key,
					displayName: role.Label, description: role.Description,
					group:  role.Group,
					source: dirSource,
				}
			}
		}
	}

	// 3. Referenced roles from DB (bundle_roles, mapping_rules, direct_role_grants).
	refs, err := svcDbGetAllReferencedRoleKeys(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("load referenced role keys: %w", err)
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

	return seen, dirProjects, nil
}

// GlobalRoleCatalog builds a consolidated view of all roles across all sources.
func GlobalRoleCatalog(ctx context.Context) ([]models.CatalogRole, error) {
	seen, dirProjects, err := collectCatalogEntries(ctx)
	if err != nil {
		return nil, err
	}

	// Load usage counts.
	usageCounts, err := svcDbGetRoleUsageCounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("load role usage counts: %w", err)
	}
	// Both facts from ONE pass. What a role was GIVEN to and what Zitadel
	// confirms of it are different questions and the surface needs both — but
	// they are two readings of one walk over the directory, not two walks.
	// Asked separately they built the snapshot twice per request.
	facts, err := svcRoleHolderFacts(ctx)
	if err != nil {
		return nil, fmt.Errorf("count holders: %w", err)
	}

	// Roles that wait for you (N14 B): the catalogue carries the mark, so
	// every picker and before-line reads it from the one read.
	waitRows, err := svcListRoleWaits(ctx)
	if err != nil {
		return nil, fmt.Errorf("load roles that wait: %w", err)
	}
	waits := make(map[string]bool, len(waitRows))
	for _, w := range waitRows {
		waits[w.ProjectID+":"+w.RoleKey] = true
	}

	// Pre-index directory project names so building the catalog doesn't fan
	// out another ProjectName lookup per role.
	projectNames := make(map[string]string, len(dirProjects))
	for _, p := range dirProjects {
		projectNames[p.ID] = p.Name
	}

	// Build catalog.
	catalog := make([]models.CatalogRole, 0, len(seen))
	for key, entry := range seen {
		usage := usageCounts[key]
		assignedUsers := facts.Given[key]
		projectName, ok := projectNames[entry.projectID]
		if !ok {
			// Referenced role pointing at a project not currently visible in
			// the directory — render the ID rather than the empty string so
			// operators see which project needs attention.
			projectName = entry.projectID
		}

		cr := models.CatalogRole{
			ProjectID:         entry.projectID,
			ProjectName:       projectName,
			RoleKey:           entry.roleKey,
			DisplayName:       entry.displayName,
			Description:       entry.description,
			Group:             entry.group,
			ClonedFromProject: entry.clonedFromP,
			ClonedFromRole:    entry.clonedFromR,
			WaitsForSend:      waits[key],
			BundleCount:       usage.BundleCount,
			RuleCount:         usage.RuleCount,
			AssignedUserCount: assignedUsers,
			// Unused means nobody holds it by any account — Zitadel's included.
			// A role Syndra never gave but Zitadel shows held is drift, not unused.
			IsUnused:    usage.BundleCount+usage.RuleCount == 0 && assignedUsers == 0 && facts.NobodyObserved(key),
			Source:      entry.source,
			Observation: facts.Basis,
		}
		if facts.Confirmed != nil {
			confirmed := facts.Confirmed[key]
			cr.ConfirmedUserCount = &confirmed
		}
		if facts.Observed != nil {
			observed := facts.Observed[key]
			cr.ObservedUserCount = &observed
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
