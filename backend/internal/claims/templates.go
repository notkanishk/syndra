package claims

import "strings"

// Breaking rules: which changes an app acts on, per template. They come from
// docs/research/2026-11-03-app-claims.md, not from a guess.
const (
	// BreaksRenameOrRemove: any role renamed or left out, and any claim that
	// leaves, moves, changes type or lands elsewhere (NetBird, GitLab, Supabase).
	BreaksRenameOrRemove = "rename_or_remove"
	// BreaksAdminsUsers: renaming or leaving out the words `admins` or `users`,
	// plus the claim itself going (Home Assistant).
	BreaksAdminsUsers = "admins_users"
	// BreaksNever: every change is said plainly, none is breaking (Custom,
	// Sign-in only, and an app on its project's default).
	BreaksNever = ""
)

// Where the app reads its claims, for check 1. Empty means Syndra doesn't know.
const (
	ReadsAccessToken = "access_token"
	ReadsIDToken     = "id_token"
	ReadsUserinfo    = "userinfo"
)

// ReachedBy is where claims that land as `lands` arrive, given the app's
// Zitadel settings: the access token only when it is a JWT, the ID token only
// with "User Info inside ID Token" on, userinfo whenever they land there.
func ReachedBy(lands string, jwtAccessToken, idTokenUserinfo bool) map[string]bool {
	return map[string]bool{
		ReadsAccessToken: lands != LandsUserinfo && jwtAccessToken,
		ReadsIDToken:     lands != LandsAccessToken && idTokenUserinfo,
		ReadsUserinfo:    lands != LandsAccessToken,
	}
}

// Template is a starting point that is copied into an app's shape. A saved
// shape records its template's id and version; a template change never
// changes a saved shape.
type Template struct {
	ID          string  `json:"id"`
	Version     int     `json:"version"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Profile     Profile `json:"profile"`
	Reads       string  `json:"reads,omitempty"`
	Breaks      string  `json:"breaking_rule,omitempty"`
	// match is the lowercase fragment of an app's Zitadel name that makes this
	// template Suggested.
	match string
}

const supabaseWhich = "Which one? A `custom:` provider is the custom OIDC one. `GOTRUE_EXTERNAL_KEYCLOAK_*` settings mean the Keycloak one."

func groups(path ...string) []Claim {
	return []Claim{{Path: path, Kind: KindRoles, Format: FormatArray}}
}

// templates is the catalog, in the order the chooser lists it.
var templates = []Template{
	{ID: "netbird", Version: 1, Name: "NetBird", match: "netbird",
		Profile: Profile{Claims: groups("groups"), IncludeNew: false, Lands: LandsBoth},
		Reads:   ReadsAccessToken, Breaks: BreaksRenameOrRemove},
	{ID: "gitlab", Version: 1, Name: "GitLab", match: "gitlab",
		Profile: Profile{Claims: groups("groups"), IncludeNew: true, Lands: LandsUserinfo},
		Reads:   ReadsUserinfo, Breaks: BreaksRenameOrRemove},
	{ID: "supabase-oidc", Version: 1, Name: "Supabase, custom OIDC provider", Description: supabaseWhich, match: "supabase",
		Profile: Profile{Claims: groups("custom_claims", "groups"), IncludeNew: true, Lands: LandsUserinfo},
		Reads:   ReadsIDToken, Breaks: BreaksRenameOrRemove},
	{ID: "supabase-keycloak", Version: 1, Name: "Supabase, Keycloak provider", Description: supabaseWhich,
		Profile: Profile{Claims: groups("groups"), IncludeNew: true, Lands: LandsUserinfo},
		Reads:   ReadsUserinfo, Breaks: BreaksRenameOrRemove},
	{ID: "home-assistant", Version: 1, Name: "Home Assistant", match: "home assistant",
		Profile: Profile{Claims: groups("groups"), IncludeNew: true, Lands: LandsUserinfo},
		Reads:   ReadsIDToken, Breaks: BreaksAdminsUsers},
	{ID: "sign-in-only", Version: 1, Name: "Sign-in only",
		Description: "For Vaultwarden, Kan and Navidrome. Who may sign in is Zitadel's role check.",
		Profile:     Profile{Claims: []Claim{}, Lands: LandsBoth}},
	{ID: "custom", Version: 1, Name: "Custom",
		Description: "Every role, in Syndra's words. For in-house apps: SmartDB, Community Slicer, Inventory, Audio-Dash, Printer Config, Syndra.",
		Profile:     Builtin()},
}

// Templates returns the catalog in chooser order. Copies, so no caller can
// edit the catalog.
func Templates() []Template {
	out := make([]Template, len(templates))
	for i, t := range templates {
		t.Profile = clone(t.Profile)
		out[i] = t
	}
	return out
}

// FindTemplate looks a template up by id.
func FindTemplate(id string) (Template, bool) {
	for _, t := range Templates() {
		if t.ID == id {
			return t, true
		}
	}
	return Template{}, false
}

// Suggest is the template whose name the app's Zitadel name contains.
func Suggest(appName string) (Template, bool) {
	name := strings.ToLower(appName)
	for _, t := range Templates() {
		if t.match != "" && strings.Contains(name, t.match) {
			return t, true
		}
	}
	return Template{}, false
}

// Breaking returns the changes this template's app will act on.
func Breaking(t Template, changes []Change) []Change {
	out := []Change{}
	for _, c := range changes {
		if t.breaks(c) {
			out = append(out, c)
		}
	}
	return out
}

func (t Template) breaks(c Change) bool {
	if t.Breaks == BreaksNever {
		return false
	}
	if c.Role == "" {
		switch c.Kind {
		case ChangeRemove, ChangeMove, ChangeRetype, ChangeLands:
			return true
		}
		return false
	}
	if c.Kind != ChangeRename && c.Kind != ChangeRemove {
		return false
	}
	if t.Breaks == BreaksAdminsUsers {
		return c.From == "admins" || c.From == "users"
	}
	return true
}

// ActionsV1 is what the Actions v1 script `groupsClaim` sent every app: every
// role key flat as `groups`, and `custom_claims` holding the same groups and
// the person's names, in both places. Existing apps are seeded with it as a
// Custom shape, so turning v1 off changes nothing on the day.
func ActionsV1() Profile {
	return Profile{
		Claims: []Claim{
			{Path: []string{"groups"}, Kind: KindRoles, Format: FormatArray},
			{Path: []string{"custom_claims", "groups"}, Kind: KindRoles, Format: FormatArray},
			{Path: []string{"custom_claims", "given_name"}, Kind: KindAttribute, Source: AttrGivenName},
			{Path: []string{"custom_claims", "family_name"}, Kind: KindAttribute, Source: AttrFamilyName},
			{Path: []string{"custom_claims", "name"}, Kind: KindAttribute, Source: AttrName},
		},
		IncludeNew: true,
		Lands:      LandsBoth,
	}
}

func clone(p Profile) Profile {
	out := Profile{IncludeNew: p.IncludeNew, Lands: p.Lands, Claims: make([]Claim, len(p.Claims))}
	for i, c := range p.Claims {
		c.Path = append([]string(nil), c.Path...)
		c.Roles = append([]string(nil), c.Roles...)
		c.Value = append([]byte(nil), c.Value...)
		out.Claims[i] = c
	}
	out.Roles = append([]RoleMap(nil), p.Roles...)
	return out
}
