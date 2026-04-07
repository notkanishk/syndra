package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"mkauth/internal/cache"
	"mkauth/internal/db"
	"mkauth/internal/demo"
	"mkauth/internal/models"
)

type roleKey struct {
	projectID string
	roleKey   string
}

func Catalog() models.CatalogResponse {
	return models.CatalogResponse{
		Users:        demo.Users(),
		Projects:     demo.Projects(),
		Applications: demo.Applications(),
	}
}

func ListUsers(ctx context.Context, query string) ([]models.UserListItem, error) {
	var items []models.UserListItem
	query = strings.ToLower(strings.TrimSpace(query))

	for _, user := range demo.Users() {
		if query != "" && !matchesUser(user, query) {
			continue
		}

		roleMap, bundles, err := collectUserRoles(ctx, user.ID)
		if err != nil {
			return nil, err
		}

		keyProjects := make([]string, 0, len(roleMap))
		seenProjects := make(map[string]bool)
		for _, role := range roleMap {
			if seenProjects[role.ProjectName] {
				continue
			}
			seenProjects[role.ProjectName] = true
			keyProjects = append(keyProjects, role.ProjectName)
		}
		sort.Strings(keyProjects)

		items = append(items, models.UserListItem{
			User:               user,
			BundleCount:        len(bundles),
			EffectiveRoleCount: len(roleMap),
			KeyProjects:        keyProjects,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].User.Name < items[j].User.Name
	})

	return items, nil
}

func ExplainUserAccess(ctx context.Context, userID string) (models.UserAccessView, error) {
	user, ok := demo.FindUser(userID)
	if !ok {
		return models.UserAccessView{}, fmt.Errorf("user %q not found in demo catalog", userID)
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
				ProjectID:   role.ProjectID,
				ProjectName: role.ProjectName,
				SourceRoles: []models.EffectiveRole{},
				DerivedRoles: []models.EffectiveRole{},
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

	hints := make([]string, 0, 2)
	if len(bundles) == 0 {
		hints = append(hints, "No MkAuth bundle is assigned yet, so this user depends entirely on direct platform grants.")
	}
	if len(roleMap) >= 5 {
		hints = append(hints, "This user spans several systems; review whether every downstream permission is still required.")
	}

	return models.UserAccessView{
		User:         user,
		Bundles:      ensureBundles(bundles),
		Projects:     projects,
		CleanupHints: hints,
	}, nil
}

func ListApplications(ctx context.Context) ([]models.ApplicationView, error) {
	users := demo.Users()
	apps := demo.Applications()
	views := make([]models.ApplicationView, 0, len(apps))

	for _, app := range apps {
		assignedCount := 0
		for _, user := range users {
			roleMap, _, err := collectUserRoles(ctx, user.ID)
			if err != nil {
				return nil, err
			}
			if hasProjectRole(roleMap, app.ProjectID) {
				assignedCount++
			}
		}

		views = append(views, models.ApplicationView{
			Application:       app,
			ConsumedRoles:     demo.RoleKeysForProject(app.ProjectID),
			AssignedUserCount: assignedCount,
		})
	}

	return views, nil
}

func SimulateApplication(ctx context.Context, appID, userID string) (models.ApplicationSimulation, error) {
	app, ok := demo.FindApplication(appID)
	if !ok {
		return models.ApplicationSimulation{}, fmt.Errorf("application %q not found", appID)
	}

	user, ok := demo.FindUser(userID)
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

	var claims map[string]interface{}
	if err := json.Unmarshal([]byte(val), &claims); err != nil {
		return models.ApplicationSimulation{}, err
	}

	rawRoles := readRoles(claims["roles"])
	sort.Strings(rawRoles)

	payload := map[string]interface{}{
		"iss":     "https://auth.makerspace.local",
		"sub":     user.ID,
		"aud":     app.Name,
		"source":  "mkauth-demo-simulator",
		"project": app.ProjectID,
	}
	payload[app.ClaimName] = formatRoles(rawRoles, app.FormatType)

	return models.ApplicationSimulation{
		Application:  app,
		User:         user,
		RawRoles:     rawRoles,
		CustomClaims: payload,
	}, nil
}

func ListProjects(ctx context.Context) ([]models.ProjectSummary, error) {
	projectSummaries := make([]models.ProjectSummary, 0, len(demo.Projects()))
	allUsers := demo.Users()
	rules, err := db.GetActiveMappingRules(ctx)
	if err != nil {
		return nil, err
	}
	bundles, err := db.GetAllBundles(ctx)
	if err != nil {
		return nil, err
	}

	for _, project := range demo.Projects() {
		memberCount := 0
		sampleMembers := []string{}
		activeRoleSet := make(map[string]bool)
		for _, user := range allUsers {
			roleMap, _, err := collectUserRoles(ctx, user.ID)
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
			roles, err := db.GetRolesForBundle(ctx, bundle.ID)
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
	roles, err := db.GetRolesForBundle(ctx, bundleID)
	if err != nil {
		return models.BundleImpact{}, err
	}

	impactedUsers := []models.UserProfile{}
	for _, user := range demo.Users() {
		bundles, err := db.GetBundlesForUser(ctx, user.ID)
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
	return db.GetDirectGrantsForUser(ctx, userID, true)
}

func Governance(ctx context.Context) (models.GovernanceSummary, error) {
	requests, err := db.GetAccessRequests(ctx, "pending")
	if err != nil {
		return models.GovernanceSummary{}, err
	}

	expiring, err := db.GetExpiringDirectGrants(ctx, 14*24*time.Hour)
	if err != nil {
		return models.GovernanceSummary{}, err
	}

	cleanupHints := []string{}
	bundles, err := db.GetAllBundles(ctx)
	if err != nil {
		return models.GovernanceSummary{}, err
	}
	for _, bundle := range bundles {
		impact, err := BundleImpact(ctx, bundle.ID)
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

	return models.GovernanceSummary{
		PendingRequests: requests,
		ExpiringGrants:  expiring,
		CleanupHints:    cleanupHints,
	}, nil
}

func Topology(ctx context.Context) (models.TopologyGraph, error) {
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

	for _, project := range demo.Projects() {
		projectCatalog[project.ID] = project
		roleCatalog[project.ID] = make(map[string]models.ProjectRole, len(project.Roles))
		ensureProjectNode(project.ID)

		for _, role := range project.Roles {
			roleCatalog[project.ID][role.Key] = role
			ensureRoleNode(project.ID, role.Key)
		}
	}

	appViews, err := ListApplications(ctx)
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

	bundles, err := db.GetAllBundles(ctx)
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

		roles, err := db.GetRolesForBundle(ctx, bundle.ID)
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

	rules, err := db.GetActiveMappingRules(ctx)
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
			Meta: map[string]string{
				"version": fmt.Sprintf("%d", rule.Version),
			},
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

	directGrants, err := db.GetDirectGrantsForUser(ctx, userID, false)
	if err != nil {
		return nil, nil, err
	}
	for _, grant := range directGrants {
		key := roleKey{projectID: grant.ProjectID, roleKey: grant.RoleKey}
		description := "Direct Zitadel grant"
		if grant.Reason != "" {
			description = grant.Reason
		}
		upsertRole(roleMap, key, true, models.RoleReason{
			Kind:        "direct",
			Description: description,
		})
	}

	bundles, err := db.GetBundlesForUser(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	for _, bundle := range bundles {
		roles, err := db.GetRolesForBundle(ctx, bundle.ID)
		if err != nil {
			return nil, nil, err
		}
		for _, role := range roles {
			key := roleKey{projectID: role.ProjectID, roleKey: role.RoleKey}
			upsertRole(roleMap, key, true, models.RoleReason{
				Kind:        "bundle",
				Description: fmt.Sprintf("Granted by bundle %s", bundle.Name),
				BundleID:    bundle.ID,
				BundleName:  bundle.Name,
			})
		}
	}

	rules, err := db.GetActiveMappingRules(ctx)
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
			if upsertRole(roleMap, targetKey, false, models.RoleReason{
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

func upsertRole(roleMap map[roleKey]*models.EffectiveRole, key roleKey, isSource bool, reason models.RoleReason) bool {
	current := roleMap[key]
	if current == nil {
		current = &models.EffectiveRole{
			ProjectID:   key.projectID,
			ProjectName: demo.ProjectName(key.projectID),
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

func readRoles(value interface{}) []string {
	raw, ok := value.([]interface{})
	if !ok {
		return nil
	}

	roles := make([]string, 0, len(raw))
	for _, item := range raw {
		role, ok := item.(string)
		if ok {
			roles = append(roles, role)
		}
	}
	return roles
}

func formatRoles(roles []string, formatType string) interface{} {
	switch formatType {
	case "csv":
		return strings.Join(roles, ",")
	case "space_delimited":
		return strings.Join(roles, " ")
	default:
		return roles
	}
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
