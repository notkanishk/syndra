package directory

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"mkauth/internal/db"
	"mkauth/internal/models"
	"mkauth/internal/zitadel"
)

// CacheTTL bounds how long list queries against Zitadel are reused. 30s keeps
// pages snappy under N+1-ish view renders (Projects + per-project ListRoles)
// while staying short enough that admin mutations propagate visibly. Mutation
// handlers in the /zitadel admin surface also drop the relevant cache entries
// explicitly via Invalidate*.
var CacheTTL = 30 * time.Second

// zitadelSource reads the directory-of-truth from the Zitadel Management API.
// Applications are synthesized one-per-project and overlaid with the
// claim_profiles table so operators can configure claim shaping per project
// without surfacing Zitadel-native Application objects (OIDC clients) that
// MkAuth doesn't otherwise model.
type zitadelSource struct {
	client zitadel.ZitadelClient

	// listClaimProfiles is injectable so tests can stub the DB.
	listClaimProfiles func(ctx context.Context) ([]db.ClaimProfileRow, error)

	// now is injectable for cache TTL tests.
	now func() time.Time

	cache sync.Map // key -> *cacheEntry
}

type cacheEntry struct {
	val     any
	expires time.Time
}

// NewZitadelSource constructs a Source backed by the given Zitadel client.
// Cache + claim_profiles reader are internal concerns; tests can override via
// the exported fields after construction.
func NewZitadelSource(client zitadel.ZitadelClient) *zitadelSource {
	return &zitadelSource{
		client:            client,
		listClaimProfiles: db.ListClaimProfiles,
		now:               time.Now,
	}
}

// --- cache helpers ----------------------------------------------------------

func (z *zitadelSource) cacheGet(key string) (any, bool) {
	v, ok := z.cache.Load(key)
	if !ok {
		return nil, false
	}
	entry := v.(*cacheEntry)
	if z.now().After(entry.expires) {
		z.cache.Delete(key)
		return nil, false
	}
	return entry.val, true
}

func (z *zitadelSource) cachePut(key string, val any) {
	z.cache.Store(key, &cacheEntry{val: val, expires: z.now().Add(CacheTTL)})
}

// --- Users ------------------------------------------------------------------

const usersCacheKey = "users"

func (z *zitadelSource) Users(ctx context.Context) ([]models.UserProfile, error) {
	if cached, ok := z.cacheGet(usersCacheKey); ok {
		return cached.([]models.UserProfile), nil
	}

	items, err := paginate(func(p zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelUser], error) {
		return z.client.ListUsers(ctx, p)
	})
	if err != nil {
		return nil, fmt.Errorf("directory: list users: %w", err)
	}

	out := make([]models.UserProfile, 0, len(items))
	for _, u := range items {
		out = append(out, toUserProfile(u))
	}
	z.cachePut(usersCacheKey, out)
	return out, nil
}

func (z *zitadelSource) FindUser(ctx context.Context, userID string) (models.UserProfile, bool, error) {
	users, err := z.Users(ctx)
	if err != nil {
		return models.UserProfile{}, false, err
	}
	for _, u := range users {
		if u.ID == userID {
			return u, true, nil
		}
	}
	// Miss on the cached list — try a direct Zitadel lookup in case the user
	// was created since the last cache fill. GetUser does not populate the
	// list cache to avoid hiding deletions.
	zu, err := z.client.GetUser(ctx, userID)
	if err != nil {
		return models.UserProfile{}, false, fmt.Errorf("directory: get user %s: %w", userID, err)
	}
	if zu == nil {
		return models.UserProfile{}, false, nil
	}
	return toUserProfile(*zu), true, nil
}

// --- Projects ---------------------------------------------------------------

const projectsCacheKey = "projects"

func projectRolesCacheKey(projectID string) string {
	return "project_roles:" + projectID
}

func (z *zitadelSource) Projects(ctx context.Context) ([]models.ProjectCatalog, error) {
	if cached, ok := z.cacheGet(projectsCacheKey); ok {
		return cached.([]models.ProjectCatalog), nil
	}

	items, err := paginate(func(p zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelProject], error) {
		return z.client.ListProjects(ctx, p)
	})
	if err != nil {
		return nil, fmt.Errorf("directory: list projects: %w", err)
	}

	out := make([]models.ProjectCatalog, 0, len(items))
	for _, p := range items {
		roles, err := z.roles(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, models.ProjectCatalog{
			ID:          p.ID,
			Name:        p.Name,
			Kind:        "zitadel",
			Description: "",
			Roles:       roles,
		})
	}
	z.cachePut(projectsCacheKey, out)
	return out, nil
}

func (z *zitadelSource) roles(ctx context.Context, projectID string) ([]models.ProjectRole, error) {
	key := projectRolesCacheKey(projectID)
	if cached, ok := z.cacheGet(key); ok {
		return cached.([]models.ProjectRole), nil
	}

	items, err := paginate(func(p zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ProjectRoleResult], error) {
		return z.client.ListProjectRoles(ctx, projectID, p)
	})
	if err != nil {
		return nil, fmt.Errorf("directory: list roles for project %s: %w", projectID, err)
	}

	roles := make([]models.ProjectRole, 0, len(items))
	for _, r := range items {
		roles = append(roles, models.ProjectRole{
			Key:         r.Key,
			Label:       coalesce(r.DisplayName, r.Key),
			Description: r.Group,
		})
	}
	z.cachePut(key, roles)
	return roles, nil
}

func (z *zitadelSource) FindProject(ctx context.Context, projectID string) (models.ProjectCatalog, bool, error) {
	projects, err := z.Projects(ctx)
	if err != nil {
		return models.ProjectCatalog{}, false, err
	}
	for _, p := range projects {
		if p.ID == projectID {
			return p, true, nil
		}
	}
	return models.ProjectCatalog{}, false, nil
}

func (z *zitadelSource) RoleKeysForProject(ctx context.Context, projectID string) ([]string, error) {
	roles, err := z.roles(ctx, projectID)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(roles))
	for _, r := range roles {
		keys = append(keys, r.Key)
	}
	return keys, nil
}

func (z *zitadelSource) ProjectName(ctx context.Context, projectID string) (string, error) {
	p, ok, err := z.FindProject(ctx, projectID)
	if err != nil {
		return "", err
	}
	if !ok {
		// Match demo.ProjectName's fallback: unknown ID renders as itself so
		// audit rows and topology placeholders don't collapse to empty.
		return projectID, nil
	}
	return p.Name, nil
}

// --- Applications -----------------------------------------------------------

const applicationsCacheKey = "applications"

func (z *zitadelSource) Applications(ctx context.Context) ([]models.ApplicationCatalog, error) {
	if cached, ok := z.cacheGet(applicationsCacheKey); ok {
		return cached.([]models.ApplicationCatalog), nil
	}

	projects, err := z.Projects(ctx)
	if err != nil {
		return nil, err
	}

	overlay := map[string]db.ClaimProfileRow{}
	rows, err := z.listClaimProfiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("directory: load claim profiles overlay: %w", err)
	}
	for _, r := range rows {
		overlay[r.ProjectID] = r
	}

	out := make([]models.ApplicationCatalog, 0, len(projects))
	for _, p := range projects {
		claimName := "roles"
		formatType := "array"
		if cp, ok := overlay[p.ID]; ok {
			if cp.ClaimName != "" {
				claimName = cp.ClaimName
			}
			if cp.FormatType != "" {
				formatType = cp.FormatType
			}
		}
		out = append(out, models.ApplicationCatalog{
			ID:          p.ID,
			Name:        p.Name,
			ProjectID:   p.ID,
			Description: "",
			Consumer:    "",
			ClaimName:   claimName,
			FormatType:  formatType,
		})
	}
	z.cachePut(applicationsCacheKey, out)
	return out, nil
}

func (z *zitadelSource) FindApplication(ctx context.Context, appID string) (models.ApplicationCatalog, bool, error) {
	apps, err := z.Applications(ctx)
	if err != nil {
		return models.ApplicationCatalog{}, false, err
	}
	for _, a := range apps {
		if a.ID == appID {
			return a, true, nil
		}
	}
	return models.ApplicationCatalog{}, false, nil
}

func (z *zitadelSource) Tag() string { return "zitadel" }

// --- Invalidation -----------------------------------------------------------

func (z *zitadelSource) InvalidateAll() {
	z.cache.Range(func(k, _ any) bool {
		z.cache.Delete(k)
		return true
	})
}

func (z *zitadelSource) InvalidateProject(projectID string) {
	z.cache.Delete(projectRolesCacheKey(projectID))
	z.cache.Delete(projectsCacheKey)
	z.cache.Delete(applicationsCacheKey)
}

func (z *zitadelSource) InvalidateUsers() {
	z.cache.Delete(usersCacheKey)
}

// --- Mapping helpers --------------------------------------------------------

// toUserProfile maps a Zitadel user search result to MkAuth's UserProfile.
// Title/Team/Location are empty in live mode — Zitadel doesn't model them.
// Sync service only reads DisplayName + Email (see sync/internal/worker),
// so the empty fields are presentational-only.
func toUserProfile(u zitadel.ZitadelUser) models.UserProfile {
	name := u.DisplayName
	if name == "" {
		name = u.Username
	}
	if name == "" {
		name = u.ID
	}
	return models.UserProfile{
		ID:       u.ID,
		Name:     name,
		Email:    u.Email,
		Title:    "",
		Team:     "",
		Status:   normalizeUserState(u.State),
		Avatar:   nameToAvatar(name),
		Location: "",
	}
}

// normalizeUserState maps Zitadel's USER_STATE_* enum onto the lowercase tags
// MkAuth's UI uses: "active" | "inactive" | "initial" | "locked" | "deleted".
// Unknown values pass through unchanged for operator visibility.
func normalizeUserState(state string) string {
	s := strings.ToLower(strings.TrimPrefix(state, "USER_STATE_"))
	switch s {
	case "active", "inactive", "initial", "locked", "deleted":
		return s
	case "":
		return "unspecified"
	default:
		return s
	}
}

// nameToAvatar derives two-character initials from a display name. Matches the
// UI's nameToAvatar so server- and client-rendered avatars stay consistent.
func nameToAvatar(name string) string {
	parts := strings.Fields(strings.TrimSpace(name))
	if len(parts) >= 2 {
		return strings.ToUpper(string(parts[0][0]) + string(parts[len(parts)-1][0]))
	}
	if len(parts) == 1 && len(parts[0]) >= 2 {
		return strings.ToUpper(parts[0][:2])
	}
	if len(parts) == 1 {
		return strings.ToUpper(parts[0])
	}
	return ""
}

func coalesce(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// MaxPaginationPages bounds the paginate helper so a bug in the upstream
// total/items reporting can't spin forever. 100 pages * DefaultSearchLimit
// = 50k records, comfortably above makerspace scale. If the upstream still
// claims more remaining after this many pages, we stop and log — surfacing
// something weirder than "slow".
const MaxPaginationPages = 100

// paginate loops a Zitadel _search endpoint until every item is collected.
// Mirrors the pattern in zitadel/orchestrator.go:fetchAllUserGrants so
// directory reads don't silently truncate at the first page for tenants
// with more than DefaultSearchLimit entries.
func paginate[T any](fetch func(zitadel.SearchParams) (*zitadel.SearchResult[T], error)) ([]T, error) {
	var all []T
	offset := 0
	for page := 0; page < MaxPaginationPages; page++ {
		res, err := fetch(zitadel.SearchParams{Limit: zitadel.DefaultSearchLimit, Offset: offset})
		if err != nil {
			return nil, err
		}
		all = append(all, res.Items...)
		// Match the exit condition in zitadel/orchestrator.go:fetchAllUserGrants:
		// stop when we've reached the reported total OR the server returned an
		// empty page. Treating Total=0 as "done after first page" keeps us
		// honest for mocks and test fixtures that omit Total — upstream
		// Zitadel always populates it for the real endpoints.
		if len(all) >= res.Total || len(res.Items) == 0 {
			return all, nil
		}
		offset += len(res.Items)
	}
	return nil, fmt.Errorf("directory: pagination exceeded %d pages; refusing to spin", MaxPaginationPages)
}
