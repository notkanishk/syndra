package zitadel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// The world is a saved copy of one Zitadel's users, projects, roles,
// applications and grants, so staging can run the real directory, holdings,
// drift and planner code against production's shape with no Zitadel behind it.
//
// Production writes it nightly with syndra-world-export, which only reads.
// Staging loads it with ZITADEL_FAKE_WORLD and never beside ZITADEL_DOMAIN:
// grant and role writes there change the file and nothing else.

// WorldVersion is the only file version this build reads.
const WorldVersion = 1

// World is the file. Field names are the file format; change them with v.
type World struct {
	V        int            `json:"v"`
	Users    []WorldUser    `json:"users"`
	Projects []WorldProject `json:"projects"`
	Grants   []UserGrant    `json:"grants"`
}

type WorldUser struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"displayName"`
	GivenName   string    `json:"givenName,omitempty"`
	FamilyName  string    `json:"familyName,omitempty"`
	Email       string    `json:"email,omitempty"`
	State       string    `json:"state"`
	Type        string    `json:"type"` // "human" or "machine"
	Created     time.Time `json:"created,omitzero"`
	// Metadata holds only the keys the directory reads (title, team).
	Metadata []UserMetadata `json:"metadata,omitempty"`
}

type WorldProject struct {
	ID    string              `json:"id"`
	Name  string              `json:"name"`
	State string              `json:"state"`
	Roles []ProjectRoleResult `json:"roles"`
	Apps  []WorldApp          `json:"apps"`
}

type WorldApp struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	State    string    `json:"state"`
	Type     string    `json:"type"`
	ClientID string    `json:"clientId,omitempty"`
	Created  time.Time `json:"created,omitzero"`
	// Token is nil when the export could not read it; TokenSettings then
	// answers not found.
	Token *TokenSettings `json:"token,omitempty"`
}

// worldMetadataKeys are the metadata keys copied into the world: the ones
// directory.applyMetadata reads. Anything else stays in production.
var worldMetadataKeys = []string{"title", "team"}

// ErrWorldBesideZitadel refuses a start with both ZITADEL_FAKE_WORLD and
// ZITADEL_DOMAIN set: the stand-in must never run beside a real Zitadel.
var ErrWorldBesideZitadel = errors.New("ZITADEL_FAKE_WORLD and ZITADEL_DOMAIN are both set; the staging stand-in never runs beside a real Zitadel, so unset one")

// WorldClient is a ZitadelClient answering from a World held in memory.
// Every write saves the file before it returns.
type WorldClient struct {
	mu    sync.Mutex
	path  string
	world World
}

// NewWorldClient loads the world at path.
func NewWorldClient(path string) (*WorldClient, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read world: %w", err)
	}
	var w World
	if err := json.Unmarshal(b, &w); err != nil {
		return nil, fmt.Errorf("decode world %s: %w", path, err)
	}
	if w.V != WorldVersion {
		return nil, fmt.Errorf("world %s is version %d; this build reads %d", path, w.V, WorldVersion)
	}
	return &WorldClient{path: path, world: w}, nil
}

// IsStandIn says MgmtClient is the staging stand-in, not a real Zitadel.
func IsStandIn() bool {
	_, ok := MgmtClient.(*WorldClient)
	return ok
}

// Counts is for the start-up log line.
func (c *WorldClient) Counts() (users, projects, grants int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.world.Users), len(c.world.Projects), len(c.world.Grants)
}

// save writes the world beside itself and renames it into place, so a crash
// leaves the old file or the new one, never half of one. Called with mu held.
func (c *WorldClient) save() error {
	b, err := json.MarshalIndent(c.world, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(c.path), ".world-*.json")
	if err != nil {
		return fmt.Errorf("save world: %w", err)
	}
	defer os.Remove(tmp.Name()) // no-op once renamed
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return fmt.Errorf("save world: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("save world: %w", err)
	}
	if err := os.Rename(tmp.Name(), c.path); err != nil {
		return fmt.Errorf("save world: %w", err)
	}
	return nil
}

func notFound(what string) error {
	return &StatusError{Code: http.StatusNotFound, Message: what + " not found"}
}

// page answers one SearchParams page of items, as Zitadel does.
func page[T any](items []T, p SearchParams) *SearchResult[T] {
	limit := p.Limit
	if limit <= 0 {
		limit = DefaultSearchLimit
	}
	start := min(max(p.Offset, 0), len(items))
	end := min(start+limit, len(items))
	return &SearchResult[T]{Items: slices.Clone(items[start:end]), Total: len(items)}
}

// --- lookups, called with mu held ---

func (c *WorldClient) user(id string) *WorldUser {
	for i := range c.world.Users {
		if c.world.Users[i].ID == id {
			return &c.world.Users[i]
		}
	}
	return nil
}

func (c *WorldClient) project(id string) *WorldProject {
	for i := range c.world.Projects {
		if c.world.Projects[i].ID == id {
			return &c.world.Projects[i]
		}
	}
	return nil
}

func (c *WorldClient) grantIndex(userID, grantID string) int {
	return slices.IndexFunc(c.world.Grants, func(g UserGrant) bool { return g.ID == grantID && g.UserID == userID })
}

func roleIndex(p *WorldProject, key string) int {
	return slices.IndexFunc(p.Roles, func(r ProjectRoleResult) bool { return r.Key == key })
}

func (u WorldUser) zitadelUser() ZitadelUser {
	return ZitadelUser{
		ID: u.ID, Username: u.Username, DisplayName: u.DisplayName,
		GivenName: u.GivenName, FamilyName: u.FamilyName, Email: u.Email,
		State: u.State, Machine: u.Type == "machine", Created: u.Created,
	}
}

// --- reads ---

func (c *WorldClient) GetUser(_ context.Context, userID string) (*ZitadelUser, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	u := c.user(userID)
	if u == nil {
		return nil, notFound("user")
	}
	zu := u.zitadelUser()
	return &zu, nil
}

func (c *WorldClient) ListUsers(_ context.Context, p SearchParams) (*SearchResult[ZitadelUser], error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]ZitadelUser, len(c.world.Users))
	for i, u := range c.world.Users {
		out[i] = u.zitadelUser()
	}
	return page(out, p), nil
}

func (c *WorldClient) ListProjects(_ context.Context, p SearchParams) (*SearchResult[ZitadelProject], error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]ZitadelProject, len(c.world.Projects))
	for i, pr := range c.world.Projects {
		out[i] = ZitadelProject{ID: pr.ID, Name: pr.Name, State: pr.State}
	}
	return page(out, p), nil
}

func (c *WorldClient) ListProjectRoles(_ context.Context, projectID string, p SearchParams) (*SearchResult[ProjectRoleResult], error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var roles []ProjectRoleResult
	if pr := c.project(projectID); pr != nil {
		roles = pr.Roles
	}
	return page(roles, p), nil
}

func (c *WorldClient) ListApplications(_ context.Context, projectID string, p SearchParams) (*SearchResult[ZitadelApplication], error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []ZitadelApplication
	if pr := c.project(projectID); pr != nil {
		for _, a := range pr.Apps {
			out = append(out, ZitadelApplication{ID: a.ID, Name: a.Name, State: a.State, Type: a.Type, ClientID: a.ClientID, Created: a.Created})
		}
	}
	return page(out, p), nil
}

func (c *WorldClient) TokenSettings(_ context.Context, projectID, appID string) (*TokenSettings, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if pr := c.project(projectID); pr != nil {
		for _, a := range pr.Apps {
			if a.ID == appID && a.Token != nil {
				ts := *a.Token
				return &ts, nil
			}
		}
	}
	return nil, notFound("app")
}

func (c *WorldClient) ListUserMetadata(_ context.Context, userID string, p SearchParams) (*SearchResult[UserMetadata], error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var md []UserMetadata
	if u := c.user(userID); u != nil {
		md = u.Metadata
	}
	return page(md, p), nil
}

func (c *WorldClient) ListUserGrants(_ context.Context, userID string, p SearchParams) (*SearchResult[UserGrant], error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []UserGrant
	for _, g := range c.world.Grants {
		if g.UserID == userID {
			out = append(out, cloneGrant(g))
		}
	}
	return page(out, p), nil
}

func (c *WorldClient) ListAllGrants(_ context.Context, p SearchParams) (*SearchResult[UserGrant], error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]UserGrant, len(c.world.Grants))
	for i, g := range c.world.Grants {
		out[i] = cloneGrant(g)
	}
	return page(out, p), nil
}

func cloneGrant(g UserGrant) UserGrant {
	g.RoleKeys = slices.Clone(g.RoleKeys)
	return g
}

// The world has no event log. nil is the real client's "the log has nothing
// for this grant", which every caller already handles.
func (c *WorldClient) GrantOriginByID(context.Context, string) (*GrantOrigin, error) {
	return nil, nil
}

func (c *WorldClient) SearchEvents(context.Context, EventSearch) ([]LoggedEvent, bool, error) {
	return nil, false, nil
}

// --- writes: each changes the world and saves it, or changes nothing ---

// commit saves, and on a failed save puts the world back as it was.
func (c *WorldClient) commit(before World) error {
	if err := c.save(); err != nil {
		c.world = before
		return err
	}
	return nil
}

func (c *WorldClient) snapshot() World {
	w := c.world
	w.Projects = slices.Clone(w.Projects)
	for i := range w.Projects {
		w.Projects[i].Roles = slices.Clone(w.Projects[i].Roles)
	}
	w.Grants = slices.Clone(w.Grants)
	for i := range w.Grants {
		w.Grants[i] = cloneGrant(w.Grants[i])
	}
	return w
}

// checkRoles is Zitadel's refusal of a role the project does not define.
func checkRoles(pr *WorldProject, keys []string) error {
	for _, k := range keys {
		if roleIndex(pr, k) < 0 {
			return &StatusError{Code: http.StatusBadRequest, Message: "role " + k + " not found on project"}
		}
	}
	return nil
}

func (c *WorldClient) AddUserGrant(_ context.Context, userID, projectID string, roleKeys []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.user(userID) == nil {
		return notFound("user")
	}
	pr := c.project(projectID)
	if pr == nil {
		return notFound("project")
	}
	if err := checkRoles(pr, roleKeys); err != nil {
		return err
	}
	// Zitadel holds one grant per user and project; a second is AlreadyExists.
	if slices.ContainsFunc(c.world.Grants, func(g UserGrant) bool { return g.UserID == userID && g.ProjectID == projectID }) {
		return &StatusError{Code: http.StatusConflict, Message: "user grant already exists"}
	}
	before := c.snapshot()
	c.world.Grants = append(c.world.Grants, UserGrant{
		ID: c.newGrantID(), UserID: userID, ProjectID: projectID, RoleKeys: slices.Clone(roleKeys),
	})
	return c.commit(before)
}

// newGrantID is a made-up numeric id like Zitadel's, unused in this world.
func (c *WorldClient) newGrantID() string {
	for {
		id := strconv.FormatUint(rand.Uint64()>>1, 10)
		if !slices.ContainsFunc(c.world.Grants, func(g UserGrant) bool { return g.ID == id }) {
			return id
		}
	}
}

func (c *WorldClient) UpdateUserGrant(_ context.Context, userID, grantID string, roleKeys []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	i := c.grantIndex(userID, grantID)
	if i < 0 {
		return notFound("user grant")
	}
	if pr := c.project(c.world.Grants[i].ProjectID); pr != nil {
		if err := checkRoles(pr, roleKeys); err != nil {
			return err
		}
	}
	before := c.snapshot()
	c.world.Grants[i].RoleKeys = slices.Clone(roleKeys)
	return c.commit(before)
}

func (c *WorldClient) RemoveUserGrant(_ context.Context, userID, grantID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	i := c.grantIndex(userID, grantID)
	if i < 0 {
		return notFound("user grant")
	}
	before := c.snapshot()
	c.world.Grants = slices.Delete(c.world.Grants, i, i+1)
	return c.commit(before)
}

func (c *WorldClient) AddProjectRole(_ context.Context, projectID, roleKey, displayName, group string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	pr := c.project(projectID)
	if pr == nil {
		return notFound("project")
	}
	if roleIndex(pr, roleKey) >= 0 {
		return &StatusError{Code: http.StatusConflict, Message: "project role already exists"}
	}
	before := c.snapshot()
	pr.Roles = append(pr.Roles, ProjectRoleResult{Key: roleKey, DisplayName: displayName, Group: group})
	return c.commit(before)
}

func (c *WorldClient) UpdateProjectRole(_ context.Context, projectID, roleKey, displayName, group string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	pr := c.project(projectID)
	if pr == nil {
		return notFound("project")
	}
	i := roleIndex(pr, roleKey)
	if i < 0 {
		return notFound("project role")
	}
	before := c.snapshot()
	pr.Roles[i].DisplayName, pr.Roles[i].Group = displayName, group
	return c.commit(before)
}

// DeleteProjectRole also takes the role out of every grant, as Zitadel does.
func (c *WorldClient) DeleteProjectRole(_ context.Context, projectID, roleKey string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	pr := c.project(projectID)
	if pr == nil {
		return notFound("project")
	}
	i := roleIndex(pr, roleKey)
	if i < 0 {
		return notFound("project role")
	}
	before := c.snapshot()
	pr.Roles = slices.Delete(pr.Roles, i, i+1)
	for j := range c.world.Grants {
		if c.world.Grants[j].ProjectID == projectID {
			c.world.Grants[j].RoleKeys = slices.DeleteFunc(c.world.Grants[j].RoleKeys, func(k string) bool { return k == roleKey })
		}
	}
	return c.commit(before)
}

// --- export ---

// WorldReader is every read the export makes, and nothing else: the export
// takes this, so it cannot call a write on the live client it is handed.
type WorldReader interface {
	ListUsers(ctx context.Context, p SearchParams) (*SearchResult[ZitadelUser], error)
	ListUserMetadata(ctx context.Context, userID string, p SearchParams) (*SearchResult[UserMetadata], error)
	ListProjects(ctx context.Context, p SearchParams) (*SearchResult[ZitadelProject], error)
	ListProjectRoles(ctx context.Context, projectID string, p SearchParams) (*SearchResult[ProjectRoleResult], error)
	ListApplications(ctx context.Context, projectID string, p SearchParams) (*SearchResult[ZitadelApplication], error)
	TokenSettings(ctx context.Context, projectID, appID string) (*TokenSettings, error)
	ListAllGrants(ctx context.Context, p SearchParams) (*SearchResult[UserGrant], error)
}

// exportPageSize is the export's page size; a var so a test can page by one.
var exportPageSize = DefaultSearchLimit

// readAll pages fetch to the end. It stops on the reported total or an empty
// page, and refuses to spin past 10 000 pages.
func readAll[T any](fetch func(SearchParams) (*SearchResult[T], error)) ([]T, error) {
	var all []T
	for range 10_000 {
		res, err := fetch(SearchParams{Limit: exportPageSize, Offset: len(all)})
		if err != nil {
			return nil, err
		}
		all = append(all, res.Items...)
		if len(all) >= res.Total || len(res.Items) == 0 {
			return all, nil
		}
	}
	return nil, errors.New("export: paging did not end")
}

// ExportWorld reads a whole world from r. Every list is paged to its end; a
// failed read fails the export, so a half world is never written. A token
// setting that cannot be read is left out (TokenSettings then says not found).
func ExportWorld(ctx context.Context, r WorldReader) (*World, error) {
	w := &World{V: WorldVersion, Users: []WorldUser{}, Projects: []WorldProject{}}

	users, err := readAll(func(p SearchParams) (*SearchResult[ZitadelUser], error) { return r.ListUsers(ctx, p) })
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	for _, u := range users {
		md, err := readAll(func(p SearchParams) (*SearchResult[UserMetadata], error) { return r.ListUserMetadata(ctx, u.ID, p) })
		if err != nil {
			return nil, fmt.Errorf("list metadata of user %s: %w", u.ID, err)
		}
		wu := WorldUser{
			ID: u.ID, Username: u.Username, DisplayName: u.DisplayName, GivenName: u.GivenName,
			FamilyName: u.FamilyName, Email: u.Email, State: u.State, Type: "human", Created: u.Created,
		}
		if u.Machine {
			wu.Type = "machine"
		}
		for _, m := range md {
			if slices.Contains(worldMetadataKeys, strings.ToLower(m.Key)) {
				wu.Metadata = append(wu.Metadata, m)
			}
		}
		w.Users = append(w.Users, wu)
	}

	projects, err := readAll(func(p SearchParams) (*SearchResult[ZitadelProject], error) { return r.ListProjects(ctx, p) })
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	for _, pr := range projects {
		roles, err := readAll(func(p SearchParams) (*SearchResult[ProjectRoleResult], error) {
			return r.ListProjectRoles(ctx, pr.ID, p)
		})
		if err != nil {
			return nil, fmt.Errorf("list roles of project %s: %w", pr.ID, err)
		}
		apps, err := readAll(func(p SearchParams) (*SearchResult[ZitadelApplication], error) {
			return r.ListApplications(ctx, pr.ID, p)
		})
		if err != nil {
			return nil, fmt.Errorf("list apps of project %s: %w", pr.ID, err)
		}
		wp := WorldProject{ID: pr.ID, Name: pr.Name, State: pr.State, Roles: roles, Apps: []WorldApp{}}
		if wp.Roles == nil {
			wp.Roles = []ProjectRoleResult{}
		}
		for _, a := range apps {
			wa := WorldApp{ID: a.ID, Name: a.Name, State: a.State, Type: a.Type, ClientID: a.ClientID, Created: a.Created}
			if ts, err := r.TokenSettings(ctx, pr.ID, a.ID); err == nil {
				wa.Token = ts
			}
			wp.Apps = append(wp.Apps, wa)
		}
		w.Projects = append(w.Projects, wp)
	}

	if w.Grants, err = readAll(func(p SearchParams) (*SearchResult[UserGrant], error) { return r.ListAllGrants(ctx, p) }); err != nil {
		return nil, fmt.Errorf("list grants: %w", err)
	}
	if w.Grants == nil {
		w.Grants = []UserGrant{}
	}
	return w, nil
}
