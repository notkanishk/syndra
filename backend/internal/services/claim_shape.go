package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"syndra/internal/claims"
	"syndra/internal/claimserve"
	"syndra/internal/db"
	"syndra/internal/directory"
	"syndra/internal/models"
)

// Claim shaping — what each application is told about a person (A-14).
//
// A shape belongs to one application (ADR-003). The data plane maps the
// sign-in's application.client_id to the app; an app with no shape of its own
// gets its project's default, and a project with none gets claims.Builtin.
// Every save is an immutable version. The project default is edited on the
// project's page only (one write place); the app page reads it.

// Where an app's effective shape comes from.
const (
	SourceApp            = "app"             // its own saved shape
	SourceProjectDefault = "project_default" // its project's default
	SourceBuiltin        = "builtin"         // nobody set one: claims.Builtin
)

// AppShape and ProjectShapes are what the data plane serves from
// (claimserve); the write side resolves them here and publishes them there.
type (
	AppShape      = claimserve.AppShape
	ProjectShapes = claimserve.ProjectShapes
)

// ResolveProjectShapes is the data plane's read for one project.
func ResolveProjectShapes(ctx context.Context, projectID string) (ProjectShapes, error) {
	def, source, err := projectDefault(ctx, projectID)
	if err != nil {
		return ProjectShapes{}, err
	}
	versions, err := svcLatestClaimShapes(ctx, projectID)
	if err != nil {
		return ProjectShapes{}, err
	}
	out := ProjectShapes{Default: def, DefaultSource: source, ByClient: map[string]AppShape{}}
	for _, v := range versions {
		if v.ClientID == "" {
			continue
		}
		out.ByClient[v.ClientID] = AppShape{ApplicationID: v.ApplicationID, Version: v.Version, TemplateID: v.TemplateID, Profile: v.Profile}
	}
	return out, nil
}

// projectDefault is the project's default shape (stored flat in
// claim_profiles), or Builtin when nobody set one.
func projectDefault(ctx context.Context, projectID string) (claims.Profile, string, error) {
	row, found, err := svcGetClaimProfile(ctx, projectID)
	if err != nil {
		return claims.Profile{}, "", err
	}
	if !found {
		return claims.Builtin(), SourceBuiltin, nil
	}
	return claims.FromFlat(row.ClaimName, row.FormatType, row.AttributeClaims, row.StaticClaims), SourceProjectDefault, nil
}

// ---------------------------------------------------------------------------
// Errors the handlers turn into statuses.

// ErrClaimShapeNotFound is an unknown application or version.
var ErrClaimShapeNotFound = errors.New("not found")

// ClaimShapeInvalid is a shape that can't be saved; Row names the claim row.
type ClaimShapeInvalid struct{ claims.RowError }

// BreakingUnacknowledged is a save the app will act on, sent without
// acknowledge_breaking: true. It carries what breaks and whom it reaches.
type BreakingUnacknowledged struct {
	Breaking []claims.Change `json:"breaking"`
	Reach    *Reach          `json:"reach,omitempty"`
}

func (e *BreakingUnacknowledged) Error() string {
	return fmt.Sprintf("%d changes the app will act on; save again with acknowledge_breaking", len(e.Breaking))
}

// ---------------------------------------------------------------------------
// The app page's read.

// TemplateState is the app's template line: which template it started from,
// what changed since (shape only), and whether the template moved on.
type TemplateState struct {
	ID      string          `json:"id"`
	Version int             `json:"version"`
	Name    string          `json:"name"`
	Changes []claims.Change `json:"changes"`
	Updated bool            `json:"updated"`
}

// AppClaimShape is everything the app's claims part reads.
type AppClaimShape struct {
	Application          models.ApplicationCatalog `json:"application"`
	ClientID             string                    `json:"client_id"`
	Source               string                    `json:"source"`
	Profile              claims.Profile            `json:"profile"`
	Version              *db.ClaimShapeVersion     `json:"version"`
	ProjectDefault       claims.Profile            `json:"project_default"`
	ProjectDefaultSource string                    `json:"project_default_source"`
	ProjectRoles         []models.ProjectRole      `json:"project_roles"`
	Template             *TemplateState            `json:"template"`
	SuggestedTemplate    string                    `json:"suggested_template,omitempty"`
}

// effective is an app's shape and where it came from.
type effective struct {
	app     models.ApplicationCatalog
	profile claims.Profile
	source  string
	saved   *db.ClaimShapeVersion
	def     claims.Profile
	defSrc  string
}

func resolveApp(ctx context.Context, appID string) (effective, error) {
	app, ok, err := directory.Default.FindApplication(ctx, appID)
	if err != nil {
		return effective{}, err
	}
	if !ok {
		return effective{}, fmt.Errorf("application %q: %w", appID, ErrClaimShapeNotFound)
	}
	def, defSrc, err := projectDefault(ctx, app.ProjectID)
	if err != nil {
		return effective{}, err
	}
	e := effective{app: app, profile: def, source: defSrc, def: def, defSrc: defSrc}
	saved, found, err := svcClaimShapeAt(ctx, appID, 0)
	if err != nil {
		return effective{}, err
	}
	if found {
		e.profile, e.source, e.saved = saved.Profile, SourceApp, &saved
	}
	return e, nil
}

// GetAppClaimShape reads one app's claims part. An app that existed under
// Actions v1 and has no shape yet is seeded first (seeded reports it, so the
// caller publishes it to the data plane).
func GetAppClaimShape(ctx context.Context, appID string) (out AppClaimShape, seeded bool, err error) {
	e, err := resolveApp(ctx, appID)
	if err != nil {
		return AppClaimShape{}, false, err
	}
	if e.saved == nil {
		if seeded, err = seedApp(ctx, e.app); err != nil {
			log.Printf("[CLAIM-SHAPE] seed of %s on read failed: %v", appID, err)
		} else if seeded {
			if e, err = resolveApp(ctx, appID); err != nil {
				return AppClaimShape{}, seeded, err
			}
		}
	}
	out = AppClaimShape{
		Application: e.app, ClientID: e.app.ClientID, Source: e.source, Profile: e.profile, Version: e.saved,
		ProjectDefault: e.def, ProjectDefaultSource: e.defSrc, ProjectRoles: []models.ProjectRole{},
	}
	if p, ok, perr := directory.Default.FindProject(ctx, e.app.ProjectID); perr == nil && ok {
		out.ProjectRoles = p.Roles
	}
	if e.saved != nil && e.saved.TemplateID != "" {
		if t, ok := claims.FindTemplate(e.saved.TemplateID); ok {
			out.Template = &TemplateState{
				ID: t.ID, Version: e.saved.TemplateVersion, Name: t.Name,
				Changes: claims.ShapeOnly(claims.Diff(t.Profile, e.saved.Profile)),
				Updated: t.Version > e.saved.TemplateVersion,
			}
		}
	}
	if t, ok := claims.Suggest(e.app.Name); ok {
		out.SuggestedTemplate = t.ID
	}
	return out, seeded, nil
}

// breakingFor is the changes this app will act on. The rule belongs to the
// app, not to the shape being saved: the first app-specific template among the
// new shape's, the saved one's, and the one its name suggests (so going back
// to a Custom version on NetBird still warns, and so does a seeded NetBird).
// An app with no shape of its own is on a default not made for it, so its
// first shape is a start, never a breaking change.
func breakingFor(e effective, newTemplateID string, changes []claims.Change) []claims.Change {
	if e.saved == nil {
		return []claims.Change{}
	}
	ids := []string{newTemplateID, e.saved.TemplateID}
	if t, ok := claims.Suggest(e.app.Name); ok {
		ids = append(ids, t.ID)
	}
	for _, id := range ids {
		if t, ok := claims.FindTemplate(id); ok && t.Breaks != claims.BreaksNever {
			return claims.Breaking(t, changes)
		}
	}
	return []claims.Change{}
}

// ---------------------------------------------------------------------------
// Saving.

// CopiedFrom records which app's version a shape was copied from. A fact, not
// a link: later changes to that app never reach this one.
type CopiedFrom struct {
	ApplicationID string `json:"application_id"`
	Version       int    `json:"version"`
}

// SaveClaimShapeRequest is the wire shape of a save.
type SaveClaimShapeRequest struct {
	Profile             claims.Profile `json:"profile"`
	TemplateID          string         `json:"template_id"`
	TemplateVersion     int            `json:"template_version"`
	CopiedFrom          *CopiedFrom    `json:"copied_from"`
	Note                string         `json:"note"`
	AcknowledgeBreaking bool           `json:"acknowledge_breaking"`
}

// SaveResult is the saved version and what it changed.
type SaveResult struct {
	Version  db.ClaimShapeVersion `json:"version"`
	Changes  []claims.Change      `json:"changes"`
	Breaking []claims.Change      `json:"breaking"`
}

// SaveAppClaimShape validates and saves the next version of an app's shape.
// A change the app will act on (per its template) needs AcknowledgeBreaking.
// restoredFrom is set by RestoreAppClaimShape only.
func SaveAppClaimShape(ctx context.Context, appID, actor string, req SaveClaimShapeRequest, restoredFrom int) (SaveResult, error) {
	e, err := resolveApp(ctx, appID)
	if err != nil {
		return SaveResult{}, err
	}
	p := claims.Normalize(req.Profile)
	if err := validateForProject(ctx, e.app.ProjectID, p); err != nil {
		return SaveResult{}, err
	}
	if req.TemplateID != "" {
		t, ok := claims.FindTemplate(req.TemplateID)
		if !ok {
			return SaveResult{}, &ClaimShapeInvalid{claims.RowError{Row: -1, Msg: fmt.Sprintf("unknown template %q", req.TemplateID)}}
		}
		if req.TemplateVersion == 0 {
			req.TemplateVersion = t.Version
		}
	}

	changes := claims.Diff(e.profile, p)
	breaking := breakingFor(e, req.TemplateID, changes)
	if len(breaking) > 0 && !req.AcknowledgeBreaking {
		return SaveResult{}, &BreakingUnacknowledged{Breaking: breaking, Reach: reachOf(ctx, e.app.ProjectID, breaking)}
	}

	v := db.ClaimShapeVersion{
		ApplicationID: appID, ProjectID: e.app.ProjectID, ClientID: e.app.ClientID, Profile: p,
		TemplateID: req.TemplateID, TemplateVersion: req.TemplateVersion, RestoredFrom: restoredFrom,
		Note: strings.TrimSpace(req.Note), SavedBy: actor,
	}
	if req.CopiedFrom != nil {
		v.CopiedFromApp, v.CopiedFromVersion = req.CopiedFrom.ApplicationID, req.CopiedFrom.Version
	}
	saved, _, err := svcInsertClaimShape(ctx, v, false)
	if err != nil {
		return SaveResult{}, err
	}
	return SaveResult{Version: saved, Changes: changes, Breaking: breaking}, nil
}

// RestoreAppClaimShape is "go back": it saves version n again as a new one.
func RestoreAppClaimShape(ctx context.Context, appID, actor string, n int, acknowledge bool) (SaveResult, error) {
	old, ok, err := svcClaimShapeAt(ctx, appID, n)
	if err != nil {
		return SaveResult{}, err
	}
	if !ok || n <= 0 {
		return SaveResult{}, fmt.Errorf("version %d of %q: %w", n, appID, ErrClaimShapeNotFound)
	}
	return SaveAppClaimShape(ctx, appID, actor, SaveClaimShapeRequest{
		Profile: old.Profile, TemplateID: old.TemplateID, TemplateVersion: old.TemplateVersion,
		AcknowledgeBreaking: acknowledge,
	}, n)
}

// validateForProject runs the shaper's checks, then refuses role keys the
// project doesn't have: a copied row whose role isn't here stops the save.
func validateForProject(ctx context.Context, projectID string, p claims.Profile) error {
	if err := claims.Validate(p); err != nil {
		var re claims.RowError
		if errors.As(err, &re) {
			return &ClaimShapeInvalid{re}
		}
		return err
	}
	keys, err := directory.Default.RoleKeysForProject(ctx, projectID)
	if err != nil {
		return err
	}
	have := map[string]bool{}
	for _, k := range keys {
		have[k] = true
	}
	for _, m := range p.Roles {
		if !have[m.Key] {
			return &ClaimShapeInvalid{claims.RowError{Row: -1, Msg: fmt.Sprintf("role %q isn't in this project", m.Key)}}
		}
	}
	for i, c := range p.Claims {
		for _, r := range c.Roles {
			if !have[r] {
				return &ClaimShapeInvalid{claims.RowError{Row: i, Msg: fmt.Sprintf("role %q isn't in this project", r)}}
			}
		}
	}
	return nil
}

// Reach is whom a breaking change reaches, from what Zitadel was last seen to
// hold: per role key, and everyone holding any role in the project.
type Reach struct {
	ByRole map[string]int `json:"by_role"`
	People int            `json:"people"`
}

// reachOf counts holders for the before-line ("12 people hold Staff"). Nil
// when Zitadel has never been read: a count of zero would be a guess.
var reachOf = func(ctx context.Context, projectID string, changes []claims.Change) *Reach {
	if len(changes) == 0 {
		return nil
	}
	snap, err := newAccessSnapshot(ctx)
	if err != nil {
		return nil
	}
	if basis, err := snap.Basis(); err != nil || basis.ReadAt == nil {
		return nil
	}
	r := &Reach{ByRole: map[string]int{}}
	prefix := projectID + ":"
	for _, u := range snap.Users() {
		observed, err := snap.Observed(u.ID)
		if err != nil {
			return nil
		}
		any := false
		for key := range observed {
			if role, ok := strings.CutPrefix(key, prefix); ok {
				any = true
				r.ByRole[role]++
			}
		}
		if any {
			r.People++
		}
	}
	return r
}

// ---------------------------------------------------------------------------
// History.

// HistoryEntry is one saved version and what it changed from the one before.
type HistoryEntry struct {
	db.ClaimShapeVersion
	Changes []claims.Change `json:"changes"`
}

// AppClaimShapeHistory lists every version, newest first.
func AppClaimShapeHistory(ctx context.Context, appID string) ([]HistoryEntry, error) {
	versions, err := svcClaimShapeHistory(ctx, appID)
	if err != nil {
		return nil, err
	}
	out := make([]HistoryEntry, len(versions))
	for i, v := range versions {
		out[i] = HistoryEntry{ClaimShapeVersion: v, Changes: []claims.Change{}}
		if i+1 < len(versions) {
			out[i].Changes = claims.Diff(versions[i+1].Profile, v.Profile)
		}
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Drafts: from a template, or copied from another app.

// RoleRef names a role key that didn't copy.
type RoleRef struct {
	RoleKey string `json:"role_key"`
	Name    string `json:"name"`
}

// MissingRole is a role-dependent row whose role isn't in this project. The
// row copies with that role dropped and stops the save until fixed.
type MissingRole struct {
	Row int `json:"row"`
	RoleRef
}

// Draft is an unsaved shape to preview and then save. Nothing is written.
type Draft struct {
	Profile         claims.Profile `json:"profile"`
	TemplateID      string         `json:"template_id"`
	TemplateVersion int            `json:"template_version"`
	CopiedFrom      *CopiedFrom    `json:"copied_from"`
	NotCopied       []RoleRef      `json:"not_copied"`
	MissingRoles    []MissingRole  `json:"missing_roles"`
}

// DraftFromRequest names exactly one source.
type DraftFromRequest struct {
	TemplateID    string `json:"template_id"`
	ApplicationID string `json:"application_id"`
}

// DraftFrom makes a draft for appID from a template or from another app's
// saved shape (any project). Role words and inclusions are always the
// operator's: from a template they carry over from what the app gets now;
// from another app they copy only for role keys this project has.
func DraftFrom(ctx context.Context, appID string, req DraftFromRequest) (Draft, error) {
	if (req.TemplateID == "") == (req.ApplicationID == "") {
		return Draft{}, &ClaimShapeInvalid{claims.RowError{Row: -1, Msg: "name exactly one of template_id or application_id"}}
	}
	e, err := resolveApp(ctx, appID)
	if err != nil {
		return Draft{}, err
	}
	project, _, err := directory.Default.FindProject(ctx, e.app.ProjectID)
	if err != nil {
		return Draft{}, err
	}
	here := map[string]bool{}
	for _, r := range project.Roles {
		here[r.Key] = true
	}
	out := Draft{NotCopied: []RoleRef{}, MissingRoles: []MissingRole{}}

	if req.TemplateID != "" {
		t, ok := claims.FindTemplate(req.TemplateID)
		if !ok {
			return Draft{}, fmt.Errorf("template %q: %w", req.TemplateID, ErrClaimShapeNotFound)
		}
		out.Profile, out.TemplateID, out.TemplateVersion = t.Profile, t.ID, t.Version
		cur := claims.Normalize(e.profile)
		for _, r := range project.Roles {
			included, w := roleOf(cur, r.Key)
			m := claims.RoleMap{Key: r.Key, Included: included}
			if w != r.Key {
				m.As = w
			}
			out.Profile.Roles = append(out.Profile.Roles, m)
		}
		return out, nil
	}

	src, ok, err := svcClaimShapeAt(ctx, req.ApplicationID, 0)
	if err != nil {
		return Draft{}, err
	}
	if !ok {
		return Draft{}, fmt.Errorf("application %q has no shape of its own to copy: %w", req.ApplicationID, ErrClaimShapeNotFound)
	}
	names := roleNames(ctx, src.ProjectID)
	out.TemplateID, out.TemplateVersion = src.TemplateID, src.TemplateVersion
	out.CopiedFrom = &CopiedFrom{ApplicationID: src.ApplicationID, Version: src.Version}
	p := src.Profile
	p.Roles = nil
	for _, m := range src.Profile.Roles {
		if here[m.Key] {
			p.Roles = append(p.Roles, m)
		} else {
			out.NotCopied = append(out.NotCopied, RoleRef{RoleKey: m.Key, Name: names[m.Key]})
		}
	}
	p.Claims = make([]claims.Claim, len(src.Profile.Claims))
	for i, c := range src.Profile.Claims {
		kept := []string{}
		for _, r := range c.Roles {
			if here[r] {
				kept = append(kept, r)
			} else {
				out.MissingRoles = append(out.MissingRoles, MissingRole{Row: i, RoleRef: RoleRef{RoleKey: r, Name: names[r]}})
			}
		}
		if c.Kind == claims.KindIfRole {
			c.Roles = kept
		}
		p.Claims[i] = c
	}
	out.Profile = p
	return out, nil
}

// roleOf is whether a profile includes a role key, and its word.
func roleOf(p claims.Profile, key string) (bool, string) {
	for _, m := range p.Roles {
		if m.Key == key {
			if m.As != "" {
				return m.Included, m.As
			}
			return m.Included, m.Key
		}
	}
	return p.IncludeNew, key
}

func roleNames(ctx context.Context, projectID string) map[string]string {
	out := map[string]string{}
	if p, ok, err := directory.Default.FindProject(ctx, projectID); err == nil && ok {
		for _, r := range p.Roles {
			out[r.Key] = r.Label
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// The seed: apps that existed under Actions v1 keep exactly what v1 sent.

// SeedNote is version 1's note for a seeded app.
const SeedNote = "As Actions v1 sent it"

// seedApp gives one app the v1 shape when Zitadel created it before the
// cutoff and it has no shape yet. An override saved under the old per-project
// shaping (app_claim_overrides) rides along, so the key that app read keeps
// coming.
func seedApp(ctx context.Context, app models.ApplicationCatalog) (bool, error) {
	cutoff, err := svcClaimShapeSeedCutoff(ctx)
	if err != nil {
		return false, err
	}
	if !app.Created.IsZero() && !app.Created.Before(cutoff) {
		return false, nil
	}
	p := claims.ActionsV1()
	note := SeedNote
	overrides, err := svcListAppClaimOverrides(ctx)
	if err != nil {
		return false, err
	}
	for _, o := range overrides {
		if o.ApplicationID != app.ID {
			continue
		}
		for _, c := range claims.FromFlat(o.ClaimName, o.FormatType, o.AttributeClaims, o.StaticClaims).Claims {
			next := p
			next.Claims = append(append([]claims.Claim(nil), p.Claims...), c)
			if claims.Validate(next) == nil {
				p = next
				note = SeedNote + ", with its own claims from before"
			}
		}
	}
	_, wrote, err := svcInsertClaimShape(ctx, db.ClaimShapeVersion{
		ApplicationID: app.ID, ProjectID: app.ProjectID, ClientID: app.ClientID, Profile: p,
		TemplateID: "custom", TemplateVersion: 1, Note: note, SavedBy: "system",
	}, true)
	return wrote, err
}

// SeedClaimShapes seeds every app Zitadel lists that qualifies, and returns the
// projects it wrote into so the caller can publish them. It is
// idempotent: run at start and hourly, it catches an app a partial directory
// read missed.
func SeedClaimShapes(ctx context.Context) ([]string, error) {
	apps, err := directory.Default.Applications(ctx)
	if err != nil {
		return nil, err
	}
	touched := map[string]bool{}
	var errs []error
	for _, a := range apps {
		wrote, err := seedApp(ctx, a)
		if err != nil {
			errs = append(errs, fmt.Errorf("seed %s: %w", a.ID, err))
			continue
		}
		if wrote {
			touched[a.ProjectID] = true
		}
	}
	out := make([]string, 0, len(touched))
	for p := range touched {
		out = append(out, p)
	}
	return out, errors.Join(errs...)
}

// ---------------------------------------------------------------------------
// The project default (edited on the project's page, its one write place).

// ProjectClaimShape is the project page's view of its default shape.
type ProjectClaimShape struct {
	ProjectID   string `json:"project_id"`
	ProjectName string `json:"project_name"`
	// Configured is false while the project uses the built-in default.
	Configured bool `json:"configured"`
	// Default is the editable flat form (what PUT …/claim-profile takes);
	// Profile is the same default as claim rows, the form Shape reads.
	Default ProjectDefaultForm `json:"default"`
	Profile claims.Profile     `json:"profile"`
	// Applications says, per app on the project, which shape it gets.
	Applications []ProjectAppShape `json:"applications"`
}

// ProjectDefaultForm is the project default's editable fields.
type ProjectDefaultForm struct {
	ClaimName       string            `json:"claim_name"`
	FormatType      string            `json:"format_type"`
	AttributeClaims map[string]string `json:"attribute_claims"`
	StaticClaims    map[string]any    `json:"static_claims"`
}

// ProjectAppShape is one app's line on the project page.
type ProjectAppShape struct {
	ApplicationID string `json:"application_id"`
	Name          string `json:"name"`
	Source        string `json:"source"`
	Version       int    `json:"version,omitempty"`
	TemplateID    string `json:"template_id,omitempty"`
}

// GetProjectClaimShape builds the project page's view.
func GetProjectClaimShape(ctx context.Context, projectID string) (ProjectClaimShape, error) {
	row, found, err := svcGetClaimProfile(ctx, projectID)
	if err != nil {
		return ProjectClaimShape{}, err
	}
	out := ProjectClaimShape{
		ProjectID: projectID, ProjectName: projectID, Configured: found,
		Default:      ProjectDefaultForm{ClaimName: claims.DefaultClaimName, FormatType: claims.DefaultFormat, AttributeClaims: map[string]string{}, StaticClaims: map[string]any{}},
		Applications: []ProjectAppShape{},
	}
	if found {
		out.Default = ProjectDefaultForm{ClaimName: row.ClaimName, FormatType: row.FormatType, AttributeClaims: row.AttributeClaims, StaticClaims: row.StaticClaims}
	}
	out.Profile = claims.FromFlat(out.Default.ClaimName, out.Default.FormatType, out.Default.AttributeClaims, out.Default.StaticClaims)
	if name, nerr := directory.Default.ProjectName(ctx, projectID); nerr == nil && name != "" {
		out.ProjectName = name
	}

	saved, err := svcLatestClaimShapes(ctx, projectID)
	if err != nil {
		return ProjectClaimShape{}, err
	}
	byApp := map[string]db.ClaimShapeVersion{}
	for _, v := range saved {
		byApp[v.ApplicationID] = v
	}
	apps, err := directory.Default.Applications(ctx)
	if err != nil {
		return ProjectClaimShape{}, err
	}
	defSrc := SourceBuiltin
	if found {
		defSrc = SourceProjectDefault
	}
	for _, a := range apps {
		if a.ProjectID != projectID {
			continue
		}
		line := ProjectAppShape{ApplicationID: a.ID, Name: a.Name, Source: defSrc}
		if v, ok := byApp[a.ID]; ok {
			line.Source, line.Version, line.TemplateID = SourceApp, v.Version, v.TemplateID
		}
		out.Applications = append(out.Applications, line)
	}
	return out, nil
}

// SaveProjectClaimProfile validates and persists a project's default shape.
func SaveProjectClaimProfile(ctx context.Context, projectID string, f ProjectDefaultForm) error {
	p := claims.Normalize(claims.FromFlat(f.ClaimName, f.FormatType, f.AttributeClaims, f.StaticClaims))
	if strings.TrimSpace(f.ClaimName) == "" {
		return &ClaimShapeInvalid{claims.RowError{Row: -1, Msg: "claim_name is required"}}
	}
	if err := claims.Validate(p); err != nil {
		var re claims.RowError
		errors.As(err, &re)
		return &ClaimShapeInvalid{re}
	}
	return svcUpsertClaimProfile(ctx, db.ClaimProfileRow{
		ProjectID: projectID, ClaimName: strings.TrimSpace(f.ClaimName), FormatType: p.Claims[0].Format,
		AttributeClaims: f.AttributeClaims, StaticClaims: f.StaticClaims,
	})
}

// ---------------------------------------------------------------------------
// The explain step lives in claim_explain.go.

// readAtNow is the clock, injectable for tests.
var readAtNow = func() time.Time { return time.Now().UTC() }

// ---------------------------------------------------------------------------
// The apps index's Claims column.

// attachClaimLines gives each app its claims line, with one shape read per
// project. A project whose shapes can't be read leaves its apps without a
// line rather than guessing one.
func attachClaimLines(ctx context.Context, views []models.ApplicationView) {
	type projectShapes struct {
		def    claims.Profile
		src    string
		byApp  map[string]db.ClaimShapeVersion
		failed bool
	}
	read := map[string]*projectShapes{}
	for i := range views {
		app := views[i].Application
		ps, ok := read[app.ProjectID]
		if !ok {
			ps = &projectShapes{byApp: map[string]db.ClaimShapeVersion{}}
			read[app.ProjectID] = ps
			def, src, err := projectDefault(ctx, app.ProjectID)
			versions, verr := svcLatestClaimShapes(ctx, app.ProjectID)
			if err != nil || verr != nil {
				log.Printf("[CLAIM-SHAPE] apps index: shapes for project %s unread: %v %v", app.ProjectID, err, verr)
				ps.failed = true
			}
			ps.def, ps.src = def, src
			for _, v := range versions {
				ps.byApp[v.ApplicationID] = v
			}
		}
		if ps.failed {
			continue
		}
		profile := ps.def
		line := &models.AppClaimsLine{Source: ps.src}
		if v, ok := ps.byApp[app.ID]; ok {
			profile = v.Profile
			line.Source, line.TemplateID, line.CopiedFrom = SourceApp, v.TemplateID, v.CopiedFromApp
			if t, ok := claims.FindTemplate(v.TemplateID); ok {
				line.TemplateChanged = len(claims.ShapeOnly(claims.Diff(t.Profile, v.Profile))) > 0
			}
		}
		line.Keys, line.Lands = topKeys(profile), profile.Lands
		line.Reaches = lastReaches(app.ID, profile, line.TemplateID)
		views[i].Claims = line
	}
}
