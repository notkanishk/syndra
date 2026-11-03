package services

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"syndra/internal/claims"
	"syndra/internal/models"
	"syndra/internal/zitadel"
)

// The explain step (A-11, A-14): one function for the live token and every
// preview, draft included. It returns the token Zitadel receives plus five
// checks, each with a state and the facts its sentence needs:
//
//  1. Zitadel's settings for the app: the project's roles are asserted, and
//     its claims reach where the app reads them (token type, userinfo in the
//     ID token), checked only where the shape lands. Sign-in only: the role
//     check is on.
//  2. The person holds roles in the project.
//  3. Their claims are built (the stored cache entry the token path serves).
//  4. Which shape applies: the app's own (version, template), the project
//     default or the built-in, or a draft.
//  5. The token.
//
// Every check always runs; a failure never hides the rest.

// Check states. Tones: pass lime, none neutral (a fact, nothing to fix), warn
// amber (a broken assumption), fail red, stale an amber ring (not a fresh read).
const (
	CheckPass  = "pass"
	CheckNone  = "none"
	CheckWarn  = "warn"
	CheckFail  = "fail"
	CheckStale = "stale"
)

// Check is one step of the trail.
type Check struct {
	State string         `json:"state"`
	Facts map[string]any `json:"facts"`
}

// Explanation is the trail for one person and one shape.
type Explanation struct {
	UserID string  `json:"user_id"`
	Checks []Check `json:"checks"`
	// Token is what Zitadel receives from Syndra. WouldReceive is set when a
	// fault (claims not built) empties it: what checks 2 to 4 would give.
	Token        *claims.Object `json:"token"`
	WouldReceive *claims.Object `json:"would_receive,omitempty"`
	ReadAt       time.Time      `json:"read_at"`
}

// shapeUnderTest is the shape an explanation runs: the app's effective one or
// a draft of it.
type shapeUnderTest struct {
	profile    claims.Profile
	source     string // SourceApp | SourceProjectDefault | SourceBuiltin
	version    int
	templateID string
	draft      bool
}

// tokenSettingsRead is check 1's facts and whether they are a fresh read.
type tokenSettingsRead struct {
	settings *zitadel.TokenSettings
	readAt   time.Time
	stale    bool
}

var (
	lastSettingsMu sync.Mutex
	lastSettings   = map[string]tokenSettingsRead{}
)

// readTokenSettings reads check 1's facts from Zitadel, falling back to its
// last answer (stale) when it doesn't answer. nil settings: never read.
func readTokenSettings(ctx context.Context, app models.ApplicationCatalog) tokenSettingsRead {
	s, err := svcTokenSettings(ctx, app.ProjectID, app.ID)
	lastSettingsMu.Lock()
	defer lastSettingsMu.Unlock()
	if err == nil && s != nil {
		r := tokenSettingsRead{settings: s, readAt: readAtNow()}
		lastSettings[app.ID] = r
		return r
	}
	last := lastSettings[app.ID]
	last.stale = true
	return last
}

// reachedBy is where a shape's claims arrive, given Zitadel's settings for the
// app: the access token only as a JWT, the ID token only with userinfo inside.
func reachedBy(lands string, st *zitadel.TokenSettings) map[string]bool {
	return map[string]bool{
		claims.ReadsAccessToken: lands != claims.LandsUserinfo && st.AccessTokenType == "JWT",
		claims.ReadsIDToken:     lands != claims.LandsAccessToken && st.IDTokenUserinfoAssertion,
		claims.ReadsUserinfo:    lands != claims.LandsAccessToken,
	}
}

// lastReaches is the apps index's verdict: whether the claims reach where the
// app's template says it reads, from Zitadel's last answer for check 1. nil
// when that was never read here, or the template names no read.
// ponytail: last answer only, so the index never calls Zitadel per app; it
// fills in as app pages are opened. Read fresh if the index must know cold.
func lastReaches(appID string, p claims.Profile, templateID string) *bool {
	tpl, ok := claims.FindTemplate(templateID)
	if !ok || tpl.Reads == "" {
		return nil
	}
	lastSettingsMu.Lock()
	last, seen := lastSettings[appID]
	lastSettingsMu.Unlock()
	if !seen || last.settings == nil {
		return nil
	}
	r := reachedBy(p.Lands, last.settings)[tpl.Reads]
	return &r
}

func explain(ctx context.Context, app models.ApplicationCatalog, s shapeUnderTest, settings tokenSettingsRead, userID string) Explanation {
	tpl, _ := claims.FindTemplate(s.templateID)
	signInOnly := len(s.profile.Claims) == 0
	out := Explanation{UserID: userID, ReadAt: readAtNow(), Checks: make([]Check, 5)}

	// 1 · Zitadel's settings for the app.
	c1 := Check{State: CheckPass, Facts: map[string]any{"lands": s.profile.Lands, "reads": tpl.Reads}}
	switch st := settings.settings; {
	case st == nil:
		c1.State, c1.Facts["unread"] = CheckWarn, true
	default:
		c1.Facts["project_role_assertion"] = st.ProjectRoleAssertion
		c1.Facts["project_role_check"] = st.ProjectRoleCheck
		c1.Facts["access_token_type"] = st.AccessTokenType
		c1.Facts["id_token_userinfo_assertion"] = st.IDTokenUserinfoAssertion
		reached := reachedBy(s.profile.Lands, st)
		c1.Facts["reached"] = reached
		switch {
		case signInOnly && !st.ProjectRoleCheck:
			c1.State = CheckWarn
		case signInOnly:
		case !st.ProjectRoleAssertion:
			c1.State = CheckFail
		case tpl.Reads != "" && !reached[tpl.Reads]:
			c1.State = CheckFail
		}
		if settings.stale {
			c1.Facts["read_at"] = settings.readAt
			if c1.State == CheckPass {
				c1.State = CheckStale
			}
		}
	}
	out.Checks[0] = c1

	// 2 and 3 · the roles held, and whether their claims are built.
	stored, built, serr := svcStoredFacts(ctx, userID, app.ProjectID)
	facts := stored
	if !built {
		var cerr error
		if facts, cerr = svcCompileFacts(ctx, userID, app.ProjectID); cerr != nil {
			facts = claims.Facts{UserID: userID, ProjectID: app.ProjectID}
		}
	}
	if facts.UserID == "" {
		facts.UserID = userID
	}
	if facts.ProjectID == "" {
		facts.ProjectID = app.ProjectID
	}
	c2 := Check{State: CheckPass, Facts: map[string]any{"roles": nonNil(facts.Roles)}}
	if len(facts.Roles) == 0 {
		c2.State = CheckNone
	}
	out.Checks[1] = c2

	c3 := Check{State: CheckPass, Facts: map[string]any{"compiled_at": stored.CompiledAt}}
	switch {
	case serr != nil:
		c3.State, c3.Facts["error"] = CheckFail, serr.Error()
	case !built && len(facts.Roles) == 0:
		c3.State = CheckNone // nothing to build
	case !built:
		c3.State = CheckWarn
	}
	out.Checks[2] = c3

	// 4 · which shape applies.
	c4 := Check{State: CheckPass, Facts: map[string]any{
		"source": s.source, "version": s.version, "template_id": s.templateID, "draft": s.draft, "keys": topKeys(s.profile),
	}}
	if s.source != SourceApp && !s.draft {
		if sug, ok := claims.Suggest(app.Name); ok && !sendsAll(s.profile, sug.Profile) {
			c4.State, c4.Facts["suggested_template"] = CheckWarn, sug.ID
		}
	}
	out.Checks[3] = c4

	// 5 · the token.
	shaped := claims.Shape(s.profile, facts)
	c5 := Check{State: CheckPass, Facts: map[string]any{"lands": s.profile.Lands}}
	out.Token = shaped
	switch {
	case signInOnly:
		c5.State = CheckNone
	case c3.State == CheckFail || c3.State == CheckWarn:
		// The token path serves only stored claims: with none it answers
		// degraded, and this is what it would send once they're built.
		out.Token, out.WouldReceive = degradedToken(ctx, app.ProjectID), shaped
		c5.State = CheckFail
	case c1.State == CheckFail:
		c5.State = CheckFail
	}
	out.Checks[4] = c5
	return out
}

// degradedToken is what the token path answers with no stored claims: the
// project's minimal-safe claims, or nothing (fail closed).
func degradedToken(ctx context.Context, projectID string) *claims.Object {
	out := claims.NewObject()
	mode, minimal, err := svcGetClaimFailureMode(ctx, projectID)
	if err != nil || mode != "minimal_safe" {
		return out
	}
	for _, k := range sortedMapKeys(minimal) {
		out.Set(k, minimal[k])
	}
	return out
}

func sortedMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func topKeys(p claims.Profile) []string {
	out, seen := []string{}, map[string]bool{}
	for _, c := range p.Claims {
		if len(c.Path) > 0 && !seen[c.Path[0]] {
			seen[c.Path[0]] = true
			out = append(out, c.Path[0])
		}
	}
	return out
}

// sendsAll is whether p sends every top-level key want does.
func sendsAll(p, want claims.Profile) bool {
	have := map[string]bool{}
	for _, k := range topKeys(p) {
		have[k] = true
	}
	for _, k := range topKeys(want) {
		if !have[k] {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// Preview: the live trail and, for a draft, the trail it would give.

// MaxPreviewPeople is the preview person plus five test people.
const MaxPreviewPeople = 6

// PreviewRequest asks for the trail for some people, optionally for a draft.
type PreviewRequest struct {
	UserIDs    []string        `json:"user_ids"`
	Draft      *claims.Profile `json:"draft"`
	TemplateID string          `json:"template_id"`
}

// PersonPreview is one person's trail now and, with a draft, after saving it.
type PersonPreview struct {
	UserID string       `json:"user_id"`
	Now    Explanation  `json:"now"`
	Draft  *Explanation `json:"draft,omitempty"`
}

// Preview is the dry run. Nothing is written.
type Preview struct {
	People []PersonPreview `json:"people"`
	// For a draft: what it changes against what the app gets now, which of
	// those the app will act on, whom they reach, and the rows the preview had
	// to leave out (a copied role-dependent row with no role yet).
	Changes  []claims.Change `json:"changes"`
	Breaking []claims.Change `json:"breaking"`
	Reach    *Reach          `json:"reach,omitempty"`
	LeftOut  []int           `json:"left_out"`
}

// PreviewAppClaimShape runs the explain step for each person, for the app's
// effective shape and, when given, a draft of it.
func PreviewAppClaimShape(ctx context.Context, appID string, req PreviewRequest) (Preview, error) {
	if len(req.UserIDs) == 0 || len(req.UserIDs) > MaxPreviewPeople {
		return Preview{}, &ClaimShapeInvalid{claims.RowError{Row: -1, Msg: fmt.Sprintf("preview 1 to %d people", MaxPreviewPeople)}}
	}
	e, err := resolveApp(ctx, appID)
	if err != nil {
		return Preview{}, err
	}
	now := shapeUnderTest{profile: e.profile, source: e.source}
	if e.saved != nil {
		now.version, now.templateID = e.saved.Version, e.saved.TemplateID
	}
	out := Preview{People: []PersonPreview{}, Changes: []claims.Change{}, Breaking: []claims.Change{}, LeftOut: []int{}}

	var draft *shapeUnderTest
	if req.Draft != nil {
		p := claims.Normalize(*req.Draft)
		kept := p.Claims[:0:0]
		for i, c := range p.Claims {
			if c.Kind == claims.KindIfRole && len(c.Roles) == 0 {
				out.LeftOut = append(out.LeftOut, i)
				continue
			}
			kept = append(kept, c)
		}
		p.Claims = kept
		if err := validateForProject(ctx, e.app.ProjectID, p); err != nil {
			return Preview{}, err
		}
		tid := req.TemplateID
		if tid == "" {
			tid = now.templateID
		}
		draft = &shapeUnderTest{profile: p, source: SourceApp, version: now.version + 1, templateID: tid, draft: true}
		out.Changes = claims.Diff(e.profile, p)
		out.Breaking = breakingFor(e, req.TemplateID, out.Changes)
		out.Reach = reachOf(ctx, e.app.ProjectID, out.Breaking)
	}

	settings := readTokenSettings(ctx, e.app)
	for _, uid := range req.UserIDs {
		pp := PersonPreview{UserID: uid, Now: explain(ctx, e.app, now, settings, uid)}
		if draft != nil {
			d := explain(ctx, e.app, *draft, settings, uid)
			pp.Draft = &d
		}
		out.People = append(out.People, pp)
	}
	return out, nil
}

// ExplainSignIn is the trail for one captured sign-in, as debug mode records
// it: the shape the data plane used, the facts it read. Computed off the
// latency path.
func ExplainSignIn(ctx context.Context, clientID, userID, projectID string) (Explanation, error) {
	apps, err := svcApplications(ctx)
	if err != nil {
		return Explanation{}, err
	}
	for _, a := range apps {
		if a.ClientID != clientID || a.ProjectID != projectID {
			continue
		}
		e, err := resolveApp(ctx, a.ID)
		if err != nil {
			return Explanation{}, err
		}
		s := shapeUnderTest{profile: e.profile, source: e.source}
		if e.saved != nil {
			s.version, s.templateID = e.saved.Version, e.saved.TemplateID
		}
		return explain(ctx, a, s, readTokenSettings(ctx, a), userID), nil
	}
	return Explanation{}, errors.New("the client isn't an app on this project")
}
