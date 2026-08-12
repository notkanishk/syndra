package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"syndra/internal/cache"
	"syndra/internal/claims"
	"syndra/internal/db"
	"syndra/internal/directory"
	"syndra/internal/models"
)

type roleKey struct {
	projectID string
	roleKey   string
}

type userRoles struct {
	roleMap map[roleKey]*models.EffectiveRole
	bundles []models.Bundle
}

// collectUserRolesHook is the indirection accessSnapshot calls. Production
// code points it at collectUserRoles; tests override it to count
// invocations. Single-process global — fine because tests run sequentially
// in this package and t.Cleanup restores the original.
var collectUserRolesHook = collectUserRoles

// accessSnapshot is a request-scoped lazy cache for (user → effective roles).
//
// The role-resolution helper collectUserRoles is fast but repeatedly called
// from views that iterate users — ListApplications walks N×M (per user × per
// app), ListProjects walks N×P. The snapshot computes-and-memoises each
// user once per request so cross-view aggregate fan-out collapses to O(N).
//
// The snapshot is NOT process-wide: no mutex, no invalidation, no expiry.
// It lives for the lifetime of one HTTP handler call and is GC'd when the
// request returns.
type accessSnapshot struct {
	ctx   context.Context
	users []models.UserProfile
	roles map[string]userRoles
}

// newAccessSnapshot primes the user list (single directory call) but defers
// role resolution to lazy For() calls.
func newAccessSnapshot(ctx context.Context) (*accessSnapshot, error) {
	users, err := directory.Default.Users(ctx)
	if err != nil {
		return nil, err
	}
	return &accessSnapshot{
		ctx:   ctx,
		users: users,
		roles: make(map[string]userRoles, len(users)),
	}, nil
}

// For returns the cached (roleMap, bundles) for userID, computing-and-
// caching on first access.
func (s *accessSnapshot) For(userID string) (map[roleKey]*models.EffectiveRole, []models.Bundle, error) {
	if r, ok := s.roles[userID]; ok {
		return r.roleMap, r.bundles, nil
	}
	roleMap, bundles, err := collectUserRolesHook(s.ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	s.roles[userID] = userRoles{roleMap: roleMap, bundles: bundles}
	return roleMap, bundles, nil
}

// Users returns the list primed at construction. Cheap accessor so view
// functions don't re-fetch the directory.
func (s *accessSnapshot) Users() []models.UserProfile {
	return s.users
}

func Catalog(ctx context.Context) (models.CatalogResponse, error) {
	users, err := directory.Default.Users(ctx)
	if err != nil {
		return models.CatalogResponse{}, err
	}
	projects, err := directory.Default.Projects(ctx)
	if err != nil {
		return models.CatalogResponse{}, err
	}
	apps, err := directory.Default.Applications(ctx)
	if err != nil {
		return models.CatalogResponse{}, err
	}
	return models.CatalogResponse{
		Users:        users,
		Projects:     projects,
		Applications: apps,
	}, nil
}

func ListUsers(ctx context.Context, query string) ([]models.UserListItem, error) {
	snap, err := newAccessSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	attention, err := loadAttention(ctx)
	if err != nil {
		return nil, err
	}
	return listUsersFromSnapshot(snap, query, attention)
}

// attentionIndex is the "needs attention" side of the People index, loaded once
// per request rather than per row: three cheap whole-table reads instead of
// three queries times N people.
type attentionIndex struct {
	expiring    map[string]int
	soonest     map[string]time.Time
	requests    map[string]int
	unexplained map[string]int
}

func loadAttention(ctx context.Context) (attentionIndex, error) {
	idx := attentionIndex{
		expiring:    map[string]int{},
		soonest:     map[string]time.Time{},
		requests:    map[string]int{},
		unexplained: map[string]int{},
	}

	grants, err := svcGetExpiringDirectGrants(ctx, reviewHorizon)
	if err != nil {
		return idx, fmt.Errorf("load expiring grants: %w", err)
	}
	for _, g := range grants {
		idx.expiring[g.UserID]++
		if g.ExpiresAt == nil {
			continue
		}
		if cur, ok := idx.soonest[g.UserID]; !ok || g.ExpiresAt.Before(cur) {
			idx.soonest[g.UserID] = *g.ExpiresAt
		}
	}

	requests, err := svcGetAccessRequests(ctx, "pending")
	if err != nil {
		return idx, fmt.Errorf("load pending requests: %w", err)
	}
	for _, r := range requests {
		idx.requests[r.RequesterID]++
	}

	// Drift is operator-only data; a failure here must not blank the whole
	// index. It is one column of one screen, and "we couldn't count" is better
	// rendered as zero than as a 500 on the People page.
	drift, err := svcGetPendingDriftItems(ctx)
	if err == nil {
		for _, d := range drift {
			idx.unexplained[d.UserID]++
		}
	}

	return idx, nil
}

func listUsersFromSnapshot(snap *accessSnapshot, query string, attention attentionIndex) ([]models.UserListItem, error) {
	query = strings.ToLower(strings.TrimSpace(query))
	items := make([]models.UserListItem, 0, len(snap.Users()))

	for _, user := range snap.Users() {
		roleMap, bundles, err := snap.For(user.ID)
		if err != nil {
			return nil, err
		}

		// Search matches role keys as well as names and emails: "who has
		// `trained` in the laser lab" gets typed here before anyone thinks to
		// go to Roles, and a search that silently ignores it looks broken.
		if query != "" && !matchesUser(user, query) && !holdsMatchingRole(roleMap, query) {
			continue
		}

		keyProjects := make([]string, 0, len(roleMap))
		keyProjectIDs := make([]string, 0, len(roleMap))
		seenProjects := make(map[string]bool)
		for _, role := range roleMap {
			if seenProjects[role.ProjectID] {
				continue
			}
			seenProjects[role.ProjectID] = true
			keyProjects = append(keyProjects, role.ProjectName)
			keyProjectIDs = append(keyProjectIDs, role.ProjectID)
		}
		sort.Strings(keyProjects)
		sort.Strings(keyProjectIDs)

		bundleNames := make([]string, 0, len(bundles))
		bundleVersions := make(map[string]int, len(bundles))
		for _, b := range bundles {
			bundleNames = append(bundleNames, b.Name)
			bundleVersions[b.Name] = b.PinnedVersion
		}
		sort.Strings(bundleNames)

		item := models.UserListItem{
			User:               user,
			BundleCount:        len(bundles),
			BundleNames:        bundleNames,
			BundleVersions:     bundleVersions,
			EffectiveRoleCount: len(roleMap),
			ProjectCount:       len(keyProjects),
			KeyProjects:        keyProjects,
			KeyProjectIDs:      keyProjectIDs,
			ExpiringCount:      attention.expiring[user.ID],
			OpenRequestCount:   attention.requests[user.ID],
			UnexplainedCount:   attention.unexplained[user.ID],
		}
		if when, ok := attention.soonest[user.ID]; ok {
			soonest := when
			item.SoonestExpiry = &soonest
		}
		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].User.Name < items[j].User.Name
	})

	return items, nil
}

// holdsMatchingRole reports whether any role this person holds matches the
// query by key or display name.
func holdsMatchingRole(roleMap map[roleKey]*models.EffectiveRole, query string) bool {
	for key := range roleMap {
		if strings.Contains(strings.ToLower(key.roleKey), query) {
			return true
		}
	}
	return false
}

// allowanceBand builds the third access band for a subject.
//
// Every allowance ever recorded, not only the ones in force. The question a
// surface asks here is "what has been decided about this person", and an answer
// showing only what still applies would erase every suspension that ended —
// which is precisely the history this layer keeps attached to the person rather
// than erasing into an absence.
func allowanceBand(ctx context.Context, userID string) ([]models.AllowanceBand, error) {
	rows, err := svcAllowancesForSubject(ctx, userID)
	if err != nil {
		return []models.AllowanceBand{}, err
	}
	now := time.Now()
	out := make([]models.AllowanceBand, 0, len(rows))
	for _, a := range rows {
		band := models.AllowanceBand{
			ID: a.ID, Target: a.Target, Field: a.Field, Value: a.Value,
			Direction: a.Direction, ActorID: a.ActorID, Reason: a.Reason,
			// Derived here rather than read from a column, so "in force" cannot
			// be stale while the date it depends on passes.
			InForce:   a.InForce(now),
			ReviewDue: a.ReviewDue(now),
			CreatedAt: a.CreatedAt.UTC().Format(time.RFC3339),
		}
		switch {
		case a.LiftedAt != nil:
			band.Ended, band.EndedBy = a.LiftedAt.UTC().Format(time.RFC3339), a.LiftedBy
		case a.ExpiresAt != nil && !a.ExpiresAt.After(now):
			// Lapsed is not lifted. The row keeps `lifted_at` NULL until the
			// sweep records it, so an allowance that ran out and one somebody
			// ended stay distinguishable forever — and the band says which.
			band.Ended, band.EndedBy = a.ExpiresAt.UTC().Format(time.RFC3339), "the expiry date"
		}
		out = append(out, band)
	}
	return out, nil
}

func ExplainUserAccess(ctx context.Context, userID string) (models.UserAccessView, error) {
	user, ok, err := directory.Default.FindUser(ctx, userID)
	if err != nil {
		return models.UserAccessView{}, err
	}
	if !ok {
		return models.UserAccessView{}, fmt.Errorf("user %q not found in directory", userID)
	}

	roleMap, bundles, err := collectUserRoles(ctx, userID)
	if err != nil {
		return models.UserAccessView{}, err
	}

	projectBuckets := make(map[string]*models.ProjectAccessView)
	for _, role := range roleMap {
		bucket := projectBuckets[role.ProjectID]
		if bucket == nil {
			bucket = &models.ProjectAccessView{
				ProjectID:         role.ProjectID,
				ProjectName:       role.ProjectName,
				SourceRoles:       []models.EffectiveRole{},
				DerivedRoles:      []models.EffectiveRole{},
				EffectiveRoleKeys: []string{},
			}
			projectBuckets[role.ProjectID] = bucket
		}

		if role.IsSource {
			bucket.SourceRoles = append(bucket.SourceRoles, *role)
		} else {
			bucket.DerivedRoles = append(bucket.DerivedRoles, *role)
		}
		bucket.EffectiveRoleKeys = append(bucket.EffectiveRoleKeys, role.RoleKey)
	}

	projects := make([]models.ProjectAccessView, 0, len(projectBuckets))
	for _, bucket := range projectBuckets {
		if bucket.SourceRoles == nil {
			bucket.SourceRoles = []models.EffectiveRole{}
		}
		if bucket.DerivedRoles == nil {
			bucket.DerivedRoles = []models.EffectiveRole{}
		}
		if bucket.EffectiveRoleKeys == nil {
			bucket.EffectiveRoleKeys = []string{}
		}
		sort.Slice(bucket.SourceRoles, func(i, j int) bool {
			return bucket.SourceRoles[i].RoleKey < bucket.SourceRoles[j].RoleKey
		})
		sort.Slice(bucket.DerivedRoles, func(i, j int) bool {
			return bucket.DerivedRoles[i].RoleKey < bucket.DerivedRoles[j].RoleKey
		})
		sort.Strings(bucket.EffectiveRoleKeys)
		projects = append(projects, *bucket)
	}
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].ProjectName < projects[j].ProjectName
	})

	// The third band. Read failure is non-fatal and says so rather than
	// silently returning an access view with no carve-outs in it: an empty band
	// and an unread band look identical to a surface, and one of them means
	// "this person is suspended from something and we could not tell you".
	allowances, allowanceErr := allowanceBand(ctx, userID)

	hints := make([]string, 0, 2)
	if allowanceErr != nil {
		hints = append(hints, "Carve-outs could not be read, so this view may not show every restriction in force.")
	}
	if len(bundles) == 0 {
		hints = append(hints, "No Syndra bundle is assigned yet, so this user depends entirely on direct platform grants.")
	}
	if len(roleMap) >= 5 {
		hints = append(hints, "This user spans several systems; review whether every downstream permission is still required.")
	}

	return models.UserAccessView{
		Allowances:   allowances,
		User:         user,
		Bundles:      ensureBundles(bundles),
		Projects:     projects,
		CleanupHints: hints,
	}, nil
}

func ListApplications(ctx context.Context) ([]models.ApplicationView, error) {
	snap, err := newAccessSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	return listApplicationsFromSnapshot(snap)
}

func listApplicationsFromSnapshot(snap *accessSnapshot) ([]models.ApplicationView, error) {
	apps, err := directory.Default.Applications(snap.ctx)
	if err != nil {
		return nil, err
	}
	views := make([]models.ApplicationView, 0, len(apps))

	for _, app := range apps {
		assignedCount := 0
		for _, user := range snap.Users() {
			roleMap, _, err := snap.For(user.ID)
			if err != nil {
				return nil, err
			}
			if hasProjectRole(roleMap, app.ProjectID) {
				assignedCount++
			}
		}

		consumedRoles, err := directory.Default.RoleKeysForProject(snap.ctx, app.ProjectID)
		if err != nil {
			return nil, err
		}

		views = append(views, models.ApplicationView{
			Application:       app,
			ConsumedRoles:     consumedRoles,
			AssignedUserCount: assignedCount,
		})
	}

	return views, nil
}

func SimulateApplication(ctx context.Context, appID, userID string) (models.ApplicationSimulation, error) {
	app, ok, err := directory.Default.FindApplication(ctx, appID)
	if err != nil {
		return models.ApplicationSimulation{}, err
	}
	if !ok {
		return models.ApplicationSimulation{}, fmt.Errorf("application %q not found", appID)
	}

	user, ok, err := directory.Default.FindUser(ctx, userID)
	if err != nil {
		return models.ApplicationSimulation{}, err
	}
	if !ok {
		return models.ApplicationSimulation{}, fmt.Errorf("user %q not found", userID)
	}

	if err := cache.CompileUserCache(ctx, userID, app.ProjectID); err != nil {
		return models.ApplicationSimulation{}, err
	}

	cacheKey := fmt.Sprintf("mapping:%s:%s", userID, app.ProjectID)
	val, err := db.Redis.Get(ctx, cacheKey).Result()
	if err != nil {
		return models.ApplicationSimulation{}, err
	}

	var facts claims.Facts
	if err := json.Unmarshal([]byte(val), &facts); err != nil {
		return models.ApplicationSimulation{}, err
	}
	if facts.ProjectID == "" {
		facts.ProjectID = app.ProjectID
	}
	if facts.UserID == "" {
		facts.UserID = user.ID
	}

	rawRoles := append([]string(nil), facts.Roles...)
	sort.Strings(rawRoles)
	facts.Roles = rawRoles

	// The preview is the data plane. Same profile resolution, same shaper, same
	// facts — the only difference is that nothing is signed. Previously this
	// function invented an {iss, sub, aud, source, project} envelope that the
	// Actions v2 path never emitted, so the screen operators used to debug
	// "my app isn't seeing the roles it expects" was showing them a token that
	// did not exist.
	profiles, err := ResolveClaimProfiles(ctx, app.ProjectID)
	if err != nil {
		return models.ApplicationSimulation{}, err
	}
	payload := claims.Shape(profiles, facts)

	// Which of those keys does THIS application actually read? Everything else
	// in the map belongs to a sibling application on the same project and is
	// carried because Zitadel does not tell us which app the token is for.
	own := ownedKeys(profiles, appID)

	return models.ApplicationSimulation{
		Application:  app,
		User:         user,
		RawRoles:     rawRoles,
		CustomClaims: payload,
		OwnedClaims:  own,
		ClaimOwners:  emittedKeys(profiles),
	}, nil
}

// ownedKeys lists the claim keys the given application reads: its own override
// if it has one, otherwise the project default.
func ownedKeys(profiles []claims.Profile, applicationID string) []string {
	for _, p := range profiles {
		if p.ApplicationID == applicationID {
			return p.Keys()
		}
	}
	for _, p := range profiles {
		if p.ApplicationID == "" {
			return p.Keys()
		}
	}
	return []string{}
}

func ListProjects(ctx context.Context) ([]models.ProjectSummary, error) {
	snap, err := newAccessSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	return listProjectsFromSnapshot(snap)
}

func listProjectsFromSnapshot(snap *accessSnapshot) ([]models.ProjectSummary, error) {
	projects, err := directory.Default.Projects(snap.ctx)
	if err != nil {
		return nil, err
	}
	rules, err := svcGetActiveMappingRules(snap.ctx)
	if err != nil {
		return nil, err
	}
	bundles, err := svcGetAllBundles(snap.ctx)
	if err != nil {
		return nil, err
	}

	projectSummaries := make([]models.ProjectSummary, 0, len(projects))
	for _, project := range projects {
		memberCount := 0
		sampleMembers := []string{}
		activeRoleSet := make(map[string]bool)
		for _, user := range snap.Users() {
			roleMap, _, err := snap.For(user.ID)
			if err != nil {
				return nil, err
			}
			for _, role := range roleMap {
				if role.ProjectID != project.ID {
					continue
				}
				activeRoleSet[role.RoleKey] = true
			}
			if hasProjectRole(roleMap, project.ID) {
				memberCount++
				if len(sampleMembers) < 3 {
					sampleMembers = append(sampleMembers, user.Name)
				}
			}
		}

		bundleCount := 0
		for _, bundle := range bundles {
			roles, err := svcGetRolesForBundle(snap.ctx, bundle.ID)
			if err != nil {
				return nil, err
			}
			for _, role := range roles {
				if role.ProjectID == project.ID {
					bundleCount++
					break
				}
			}
		}

		ruleInCount := 0
		ruleOutCount := 0
		for _, rule := range rules {
			if rule.TargetProject == project.ID {
				ruleInCount++
			}
			if rule.SourceProject == project.ID {
				ruleOutCount++
			}
		}

		activeRoleKeys := make([]string, 0, len(activeRoleSet))
		for roleKey := range activeRoleSet {
			activeRoleKeys = append(activeRoleKeys, roleKey)
		}
		sort.Strings(activeRoleKeys)

		projectSummaries = append(projectSummaries, models.ProjectSummary{
			Project:        project,
			MemberCount:    memberCount,
			BundleCount:    bundleCount,
			RuleInCount:    ruleInCount,
			RuleOutCount:   ruleOutCount,
			ActiveRoleKeys: activeRoleKeys,
			SampleMembers:  sampleMembers,
		})
	}

	sort.Slice(projectSummaries, func(i, j int) bool {
		return projectSummaries[i].Project.Name < projectSummaries[j].Project.Name
	})

	return projectSummaries, nil
}

func BundleImpact(ctx context.Context, bundleID string) (models.BundleImpact, error) {
	snap, err := newAccessSnapshot(ctx)
	if err != nil {
		return models.BundleImpact{}, err
	}
	return bundleImpactFromSnapshot(snap, bundleID)
}

func bundleImpactFromSnapshot(snap *accessSnapshot, bundleID string) (models.BundleImpact, error) {
	roles, err := svcGetRolesForBundle(snap.ctx, bundleID)
	if err != nil {
		return models.BundleImpact{}, err
	}

	impactedUsers := []models.UserProfile{}
	for _, user := range snap.Users() {
		bundles, err := svcGetBundlesForUser(snap.ctx, user.ID)
		if err != nil {
			return models.BundleImpact{}, err
		}
		for _, bundle := range bundles {
			if bundle.ID == bundleID {
				impactedUsers = append(impactedUsers, user)
				break
			}
		}
	}

	return models.BundleImpact{
		BundleID:  bundleID,
		RoleCount: len(roles),
		Users:     impactedUsers,
	}, nil
}

func UserDirectGrants(ctx context.Context, userID string) ([]models.DirectGrant, error) {
	return svcGetDirectGrantsForUser(ctx, userID, true)
}

// AllDirectGrants returns every active Syndra-direct grant. Active means
// expires_at is NULL or in the future — expired rows are excluded so the
// reconciliation handler compares like-for-like with what Zitadel currently
// surfaces. Backs GET /api/v1/reconciliation/grants.
func AllDirectGrants(ctx context.Context) ([]models.DirectGrant, error) {
	return svcGetAllDirectGrants(ctx, false)
}

func Governance(ctx context.Context) (models.GovernanceSummary, error) {
	snap, err := newAccessSnapshot(ctx)
	if err != nil {
		return models.GovernanceSummary{}, err
	}
	return governanceFromSnapshot(snap)
}

func governanceFromSnapshot(snap *accessSnapshot) (models.GovernanceSummary, error) {
	requests, err := svcGetAccessRequests(snap.ctx, "pending")
	if err != nil {
		return models.GovernanceSummary{}, err
	}

	expiring, err := svcGetExpiringDirectGrants(snap.ctx, expiryHorizon)
	if err != nil {
		return models.GovernanceSummary{}, err
	}

	cleanupHints := []string{}
	bundles, err := svcGetAllBundles(snap.ctx)
	if err != nil {
		return models.GovernanceSummary{}, err
	}
	for _, bundle := range bundles {
		impact, err := bundleImpactFromSnapshot(snap, bundle.ID)
		if err != nil {
			return models.GovernanceSummary{}, err
		}
		if len(impact.Users) == 0 {
			cleanupHints = append(cleanupHints, fmt.Sprintf("Bundle %q is unused and can be reviewed for cleanup.", bundle.Name))
		}
	}
	if len(requests) == 0 {
		cleanupHints = append(cleanupHints, "No pending requests right now, so approvals are caught up.")
	}

	if requests == nil {
		requests = []models.AccessRequest{}
	}
	if expiring == nil {
		expiring = []models.DirectGrant{}
	}

	// Outbox depth + reachability so the UI can render the amber "N changes
	// awaiting Zitadel" callout and gate "Resume now". A count error is
	// non-fatal — the rest of the summary is still useful, so degrade to 0.
	pendingCount, err := svcCountPendingPropagations(snap.ctx)
	if err != nil {
		pendingCount = 0
	}

	// Drift pending-triage count + top-N preview for the dashboard callout. A
	// count/lookup error is non-fatal — the rest of the summary is still useful,
	// so degrade to 0/empty (mirrors the pending-propagation degrade above).
	driftCount, err := svcCountPendingDrift(snap.ctx)
	if err != nil {
		driftCount = 0
	}
	topDrift, err := svcGetTopDrift(snap.ctx, 3)
	if err != nil {
		topDrift = nil
	}
	if topDrift == nil {
		topDrift = []models.DriftItem{}
	}

	// The targets Syndra cannot vouch for. Non-fatal like the two counts above,
	// and for the same reason — but note what degrading to empty means here: it
	// reports "nothing is unreadable" when the read itself failed. That is the
	// same false quiet this field exists to break, so the failure is logged
	// rather than swallowed silently.
	unreconciled, err := svcGetUnreconciledTargets(snap.ctx)
	if err != nil {
		log.Printf("[GOVERNANCE] could not read unreconciled targets: %v (degrading to none)", err)
		unreconciled = nil
	}
	if unreconciled == nil {
		unreconciled = []models.UnreconciledTarget{}
	}

	return models.GovernanceSummary{
		PendingRequests: requests,
		ExpiringGrants:  expiring,
		CleanupHints:    cleanupHints,
		PendingPropagation: models.PendingPropagationSummary{
			Count:            pendingCount,
			ZitadelReachable: svcZitadelReachable(snap.ctx),
		},
		Drift:               models.DriftSummary{Count: driftCount, Top: topDrift},
		UnreconciledTargets: unreconciled,
	}, nil
}

func Topology(ctx context.Context) (models.TopologyGraph, error) {
	snap, err := newAccessSnapshot(ctx)
	if err != nil {
		return models.TopologyGraph{
			Nodes: []models.TopologyNode{},
			Edges: []models.TopologyEdge{},
		}, err
	}
	return topologyFromSnapshot(snap)
}

func topologyFromSnapshot(snap *accessSnapshot) (models.TopologyGraph, error) {
	ctx := snap.ctx
	graph := models.TopologyGraph{
		Nodes: []models.TopologyNode{},
		Edges: []models.TopologyEdge{},
	}
	nodeSeen := make(map[string]bool)
	edgeSeen := make(map[string]bool)
	projectCatalog := make(map[string]models.ProjectCatalog)
	roleCatalog := make(map[string]map[string]models.ProjectRole)

	addNode := func(node models.TopologyNode) {
		if nodeSeen[node.ID] {
			return
		}
		nodeSeen[node.ID] = true
		graph.Nodes = append(graph.Nodes, node)
	}
	addEdge := func(edge models.TopologyEdge) {
		if edgeSeen[edge.ID] {
			return
		}
		edgeSeen[edge.ID] = true
		graph.Edges = append(graph.Edges, edge)
	}
	ensureProjectNode := func(projectID string) {
		projectNodeID := "project:" + projectID
		project, ok := projectCatalog[projectID]
		if ok {
			addNode(models.TopologyNode{
				ID:          projectNodeID,
				Label:       project.Name,
				Kind:        "project",
				ProjectID:   project.ID,
				Description: project.Description,
				Meta: map[string]string{
					"kind": project.Kind,
				},
			})
			return
		}

		addNode(models.TopologyNode{
			ID:          projectNodeID,
			Label:       projectID,
			Kind:        "project",
			ProjectID:   projectID,
			Description: "Referenced by persisted rules or bundle grants that do not exist in the seeded demo catalog yet.",
			Meta: map[string]string{
				"kind":   "external",
				"source": "database",
			},
		})
	}
	ensureRoleNode := func(projectID, roleKey string) {
		ensureProjectNode(projectID)

		roleNode := models.TopologyNode{
			ID:          roleNodeID(projectID, roleKey),
			Label:       roleLabel(roleKey),
			Kind:        "role",
			ProjectID:   projectID,
			Description: "Referenced by persisted rules or grants outside the seeded role catalog.",
			Meta: map[string]string{
				"role_key": roleKey,
				"source":   "database",
			},
		}

		if roles, ok := roleCatalog[projectID]; ok {
			if role, ok := roles[roleKey]; ok {
				roleNode.Label = role.Label
				roleNode.Description = role.Description
				roleNode.Meta = map[string]string{
					"role_key": role.Key,
				}
			}
		}

		addNode(roleNode)
		addEdge(models.TopologyEdge{
			ID:     "contains:" + projectID + ":" + roleKey,
			Source: "project:" + projectID,
			Target: roleNode.ID,
			Kind:   "contains",
			Label:  "defines",
		})
	}

	dirProjects, err := directory.Default.Projects(ctx)
	if err != nil {
		return graph, err
	}
	for _, project := range dirProjects {
		projectCatalog[project.ID] = project
		roleCatalog[project.ID] = make(map[string]models.ProjectRole, len(project.Roles))
		ensureProjectNode(project.ID)

		for _, role := range project.Roles {
			roleCatalog[project.ID][role.Key] = role
			ensureRoleNode(project.ID, role.Key)
		}
	}

	appViews, err := listApplicationsFromSnapshot(snap)
	if err != nil {
		return graph, err
	}
	for _, app := range appViews {
		nodeID := "application:" + app.Application.ID
		ensureProjectNode(app.Application.ProjectID)
		addNode(models.TopologyNode{
			ID:          nodeID,
			Label:       app.Application.Name,
			Kind:        "application",
			ProjectID:   app.Application.ProjectID,
			Description: app.Application.Description,
			Meta: map[string]string{
				"claim_name":  app.Application.ClaimName,
				"format_type": app.Application.FormatType,
				"consumer":    app.Application.Consumer,
			},
		})
		addEdge(models.TopologyEdge{
			ID:     "consumes:" + app.Application.ID,
			Source: nodeID,
			Target: "project:" + app.Application.ProjectID,
			Kind:   "application",
			Label:  "consumes",
		})
	}

	bundles, err := svcGetAllBundles(ctx)
	if err != nil {
		return graph, err
	}
	for _, bundle := range bundles {
		bundleID := "bundle:" + bundle.ID
		addNode(models.TopologyNode{
			ID:          bundleID,
			Label:       bundle.Name,
			Kind:        "bundle",
			Description: bundle.Description,
		})

		roles, err := svcGetRolesForBundle(ctx, bundle.ID)
		if err != nil {
			return graph, err
		}
		for _, role := range roles {
			ensureRoleNode(role.ProjectID, role.RoleKey)
			addEdge(models.TopologyEdge{
				ID:     "bundle-role:" + bundle.ID + ":" + role.ProjectID + ":" + role.RoleKey,
				Source: bundleID,
				Target: roleNodeID(role.ProjectID, role.RoleKey),
				Kind:   "bundle",
				Label:  "grants",
			})
		}
	}

	rules, err := svcGetActiveMappingRules(ctx)
	if err != nil {
		return graph, err
	}
	for _, rule := range rules {
		ensureRoleNode(rule.SourceProject, rule.SourceRole)
		ensureRoleNode(rule.TargetProject, rule.TargetRole)
		addEdge(models.TopologyEdge{
			ID:     "rule:" + rule.ID,
			Source: roleNodeID(rule.SourceProject, rule.SourceRole),
			Target: roleNodeID(rule.TargetProject, rule.TargetRole),
			Kind:   "rule",
			Label:  "maps",
		})
	}

	sort.Slice(graph.Nodes, func(i, j int) bool {
		if graph.Nodes[i].Kind == graph.Nodes[j].Kind {
			if graph.Nodes[i].Label == graph.Nodes[j].Label {
				return graph.Nodes[i].ID < graph.Nodes[j].ID
			}
			return graph.Nodes[i].Label < graph.Nodes[j].Label
		}
		return graph.Nodes[i].Kind < graph.Nodes[j].Kind
	})
	sort.Slice(graph.Edges, func(i, j int) bool {
		return graph.Edges[i].ID < graph.Edges[j].ID
	})

	return graph, nil
}

func collectUserRoles(ctx context.Context, userID string) (map[roleKey]*models.EffectiveRole, []models.Bundle, error) {
	roleMap := make(map[roleKey]*models.EffectiveRole)

	directGrants, err := svcGetDirectGrantsForUser(ctx, userID, false)
	if err != nil {
		return nil, nil, err
	}
	for _, grant := range directGrants {
		key := roleKey{projectID: grant.ProjectID, roleKey: grant.RoleKey}
		description := "Direct Zitadel grant"
		if grant.Reason != "" {
			description = grant.Reason
		}
		upsertRole(ctx, roleMap, key, true, models.RoleReason{
			Kind:        "direct",
			Description: description,
		})
	}

	bundles, err := svcGetBundlesForUser(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	// Resolved through each person's pinned version, not the bundle's working
	// copy: somebody left on v2 holds v2's roles, and reading the working copy
	// here would show them access they do not have.
	byBundle, err := svcGetUserBundleRolesGrouped(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	for _, bundle := range bundles {
		roles := byBundle[bundle.ID]
		for _, role := range roles {
			key := roleKey{projectID: role.ProjectID, roleKey: role.RoleKey}
			upsertRole(ctx, roleMap, key, true, models.RoleReason{
				Kind:        "bundle",
				Description: fmt.Sprintf("Granted by bundle %s v%d", bundle.Name, bundle.PinnedVersion),
				BundleID:    bundle.ID,
				BundleName:  bundle.Name,
			})
		}
	}

	rules, err := svcGetActiveMappingRules(ctx)
	if err != nil {
		return nil, nil, err
	}

	for i := 0; i < len(rules); i++ {
		changed := false
		for _, rule := range rules {
			sourceKey := roleKey{projectID: rule.SourceProject, roleKey: rule.SourceRole}
			if roleMap[sourceKey] == nil {
				continue
			}

			targetKey := roleKey{projectID: rule.TargetProject, roleKey: rule.TargetRole}
			if upsertRole(ctx, roleMap, targetKey, false, models.RoleReason{
				Kind:           "mapping",
				Description:    fmt.Sprintf("Derived from %s:%s", rule.SourceProject, rule.SourceRole),
				TriggerProject: rule.SourceProject,
				TriggerRole:    rule.SourceRole,
			}) {
				changed = true
			}
		}
		if !changed {
			break
		}
	}

	return roleMap, bundles, nil
}

func upsertRole(ctx context.Context, roleMap map[roleKey]*models.EffectiveRole, key roleKey, isSource bool, reason models.RoleReason) bool {
	current := roleMap[key]
	if current == nil {
		// Best-effort project name — never block lineage resolution on a
		// directory lookup failure. Fall back to the raw project ID.
		projectName := key.projectID
		if name, err := directory.Default.ProjectName(ctx, key.projectID); err == nil {
			projectName = name
		}
		current = &models.EffectiveRole{
			ProjectID:   key.projectID,
			ProjectName: projectName,
			RoleKey:     key.roleKey,
			IsSource:    isSource,
		}
		roleMap[key] = current
	}
	if isSource {
		current.IsSource = true
	}

	for _, existing := range current.Reasons {
		if existing.Kind == reason.Kind &&
			existing.Description == reason.Description &&
			existing.BundleID == reason.BundleID &&
			existing.TriggerProject == reason.TriggerProject &&
			existing.TriggerRole == reason.TriggerRole {
			return false
		}
	}

	current.Reasons = append(current.Reasons, reason)
	return true
}

func hasProjectRole(roleMap map[roleKey]*models.EffectiveRole, projectID string) bool {
	for _, role := range roleMap {
		if role.ProjectID == projectID {
			return true
		}
	}
	return false
}

func matchesUser(user models.UserProfile, query string) bool {
	return strings.Contains(strings.ToLower(user.ID), query) ||
		strings.Contains(strings.ToLower(user.Name), query) ||
		strings.Contains(strings.ToLower(user.Email), query) ||
		strings.Contains(strings.ToLower(user.Team), query)
}

func ensureBundles(bundles []models.Bundle) []models.Bundle {
	if bundles == nil {
		return []models.Bundle{}
	}
	return bundles
}

func roleNodeID(projectID, roleKey string) string {
	return "role:" + projectID + ":" + roleKey
}

func roleLabel(roleKey string) string {
	words := strings.Fields(strings.NewReplacer("_", " ", "-", " ").Replace(roleKey))
	for i, word := range words {
		if word == "" {
			continue
		}
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	if len(words) == 0 {
		return roleKey
	}
	return strings.Join(words, " ")
}
