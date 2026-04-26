package directory

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

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
// Applications are real Zitadel applications (OIDC / API / SAML clients
// attached to projects) fetched via ListApplications and overlaid with the
// claim_profiles table for operator-managed claim shaping. The overlay stays
// project-keyed because Zitadel Actions v2 emits custom claims at the
// user-grant level, which is project-scoped.
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

const (
	usersCacheKey           = "users"
	userMetadataCachePrefix = "user_metadata:"
	// MetadataFanoutLimit caps parallel per-user metadata fetches. Sized for
	// makerspace scale (tens of users) — higher would just burn connections
	// without measurable gain.
	MetadataFanoutLimit = 8
)

func userMetadataCacheKey(userID string) string { return userMetadataCachePrefix + userID }

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

	out := make([]models.UserProfile, len(items))
	for i, u := range items {
		out[i] = toUserProfile(u)
	}

	// Overlay per-user metadata in parallel. Errors on individual users are
	// logged but do not fail the whole list — a single broken user must not
	// blank out every card on /users.
	z.applyUserMetadataOverlay(ctx, out)

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
	profile := toUserProfile(*zu)
	if md, ok := z.fetchUserMetadata(ctx, profile.ID); ok {
		applyMetadata(&profile, md)
	}
	return profile, true, nil
}

// applyUserMetadataOverlay fans out ListUserMetadata per user with a bounded
// concurrency limit and merges Title/Team/Location into the corresponding
// UserProfile entry in-place.
func (z *zitadelSource) applyUserMetadataOverlay(ctx context.Context, users []models.UserProfile) {
	if len(users) == 0 {
		return
	}
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(MetadataFanoutLimit)
	for i := range users {
		g.Go(func() error {
			md, ok := z.fetchUserMetadata(gctx, users[i].ID)
			if ok {
				applyMetadata(&users[i], md)
			}
			return nil
		})
	}
	// Ignoring error: goroutines never return one (metadata failures are logged
	// and swallowed inside fetchUserMetadata to preserve partial results).
	_ = g.Wait()
}

// fetchUserMetadata returns the cached-or-fetched metadata list for a user.
// Individual errors are logged and the second return is false — callers
// should treat an errored user as having no metadata rather than failing.
func (z *zitadelSource) fetchUserMetadata(ctx context.Context, userID string) ([]zitadel.UserMetadata, bool) {
	key := userMetadataCacheKey(userID)
	if cached, ok := z.cacheGet(key); ok {
		return cached.([]zitadel.UserMetadata), true
	}
	items, err := paginate(func(p zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserMetadata], error) {
		return z.client.ListUserMetadata(ctx, userID, p)
	})
	if err != nil {
		log.Printf("[DIRECTORY] list metadata for user %s failed (skipping overlay): %v", userID, err)
		return nil, false
	}
	z.cachePut(key, items)
	return items, true
}

// applyMetadata merges well-known keys from Zitadel user metadata into a
// UserProfile. Matching is case-insensitive so admins aren't tripped up by
// "Title" vs "title".
func applyMetadata(p *models.UserProfile, md []zitadel.UserMetadata) {
	for _, m := range md {
		switch strings.ToLower(m.Key) {
		case "title":
			p.Title = m.Value
		case "team":
			p.Team = m.Value
		case "location":
			p.Location = m.Value
		}
	}
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

const (
	applicationsCacheKey     = "applications"
	appsByProjectCachePrefix = "apps_by_project:"
	// AppsFanoutLimit caps parallel ListApplications per project. 4 is enough
	// to mask per-project latency without flooding the Zitadel Management API.
	AppsFanoutLimit = 4
)

func appsByProjectCacheKey(projectID string) string {
	return appsByProjectCachePrefix + projectID
}

// Applications returns real Zitadel applications (OIDC clients, APIs, SAML
// SPs) across every project, overlaid with the claim_profiles table keyed by
// project ID. Apps inherit their project's claim profile since Zitadel
// Actions v2 emits custom claims at the user-grant level, which is
// project-scoped. If a project's ListApplications call fails, that project's
// apps are skipped but the rest of the list is still returned — admins
// shouldn't lose the whole page because one project is broken.
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

	// Fan out per-project ListApplications with bounded concurrency. Each
	// worker writes to its own slot to avoid sharing a mutex on the result.
	// partial flips to true if ANY project's app fetch failed; we still
	// return what we have (admins shouldn't lose the whole list because one
	// project is down) but we DO NOT cache the partial result, so the next
	// call retries the failed projects rather than serving stale gaps for
	// 30s. Per-project caches in fetchProjectApps still persist successful
	// fetches independently.
	perProject := make([][]zitadel.ZitadelApplication, len(projects))
	var partial atomic.Bool
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(AppsFanoutLimit)
	for i, p := range projects {
		g.Go(func() error {
			apps, ok := z.fetchProjectApps(gctx, p.ID)
			if !ok {
				partial.Store(true)
				return nil
			}
			perProject[i] = apps
			return nil
		})
	}
	_ = g.Wait()

	out := make([]models.ApplicationCatalog, 0)
	for i, p := range projects {
		claimName, formatType := overlayClaim(overlay, p.ID)
		for _, app := range perProject[i] {
			out = append(out, models.ApplicationCatalog{
				ID:          app.ID,
				Name:        coalesce(app.Name, app.ID),
				ProjectID:   p.ID,
				Description: "",
				Consumer:    zitadel.HumanizeAppType(app.Type),
				ClaimName:   claimName,
				FormatType:  formatType,
			})
		}
	}
	if !partial.Load() {
		z.cachePut(applicationsCacheKey, out)
	}
	return out, nil
}

// fetchProjectApps returns the cached-or-fetched application list for a
// single project. Errors are logged and the second return is false so the
// per-project fan-out can skip it without aborting the whole Applications call.
func (z *zitadelSource) fetchProjectApps(ctx context.Context, projectID string) ([]zitadel.ZitadelApplication, bool) {
	key := appsByProjectCacheKey(projectID)
	if cached, ok := z.cacheGet(key); ok {
		return cached.([]zitadel.ZitadelApplication), true
	}
	items, err := paginate(func(p zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelApplication], error) {
		return z.client.ListApplications(ctx, projectID, p)
	})
	if err != nil {
		log.Printf("[DIRECTORY] list applications for project %s failed (skipping): %v", projectID, err)
		return nil, false
	}
	z.cachePut(key, items)
	return items, true
}

// overlayClaim resolves the claim_name + format_type pair for a project,
// falling back to the v1 defaults when the operator hasn't created a
// claim_profiles row.
func overlayClaim(overlay map[string]db.ClaimProfileRow, projectID string) (string, string) {
	claimName := "roles"
	formatType := "array"
	if cp, ok := overlay[projectID]; ok {
		if cp.ClaimName != "" {
			claimName = cp.ClaimName
		}
		if cp.FormatType != "" {
			formatType = cp.FormatType
		}
	}
	return claimName, formatType
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
	z.cache.Delete(appsByProjectCacheKey(projectID))
}

func (z *zitadelSource) InvalidateUsers() {
	z.cache.Delete(usersCacheKey)
	// Metadata caches are keyed per-user; drop them too so a newly-set title
	// shows up after an invalidation, not only after TTL.
	z.cache.Range(func(k, _ any) bool {
		if key, ok := k.(string); ok && strings.HasPrefix(key, userMetadataCachePrefix) {
			z.cache.Delete(key)
		}
		return true
	})
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
