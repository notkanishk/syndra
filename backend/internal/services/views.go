package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

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

func collectUserRoles(ctx context.Context, userID string) (map[roleKey]*models.EffectiveRole, []models.Bundle, error) {
	roleMap := make(map[roleKey]*models.EffectiveRole)

	for _, grant := range demo.BaseGrants(userID) {
		key := roleKey{projectID: grant.ProjectID, roleKey: grant.RoleKey}
		upsertRole(roleMap, key, true, models.RoleReason{
			Kind:        "direct",
			Description: "Direct Zitadel grant",
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
