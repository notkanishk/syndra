// Package claims owns the token shape: the single place that turns a user's
// compiled access facts into the claim map an application receives.
//
// Both the data plane (Zitadel Actions v2 inject) and the operator-facing
// simulator call Shape. That is the whole point of the package existing — a
// preview computed by different code from the token it claims to preview is a
// preview of nothing. Before this package, the simulator emitted
// {iss, sub, aud, <claim_name>: formatted_roles} while the Actions path emitted
// the raw cache map, and the two never agreed.
//
// No database or network access lives here, so the data plane can shape a
// token inside its Redis-read latency budget.
package claims

import (
	"fmt"
	"sort"
	"strings"
)

// Format identifiers accepted for the roles claim. Mirrors the
// ck_claim_profiles_format_type / ck_app_claim_overrides_format_type
// constraints.
const (
	FormatArray          = "array"
	FormatCSV            = "csv"
	FormatSpaceDelimited = "space_delimited"
)

// Attribute sources a profile may project into a token. Every one of these is
// resolvable from the compiled cache entry alone — nothing here requires a
// directory call at token-issue time.
const (
	AttrUserID     = "user_id"
	AttrProjectID  = "project_id"
	AttrEmail      = "email"
	AttrName       = "name"
	AttrTitle      = "title"
	AttrTeam       = "team"
	AttrRoleCount  = "role_count"
	AttrCompiledAt = "compiled_at"
)

// DefaultClaimName / DefaultFormat are what a project emits when no operator
// has ever opened the Token format panel for it.
const (
	DefaultClaimName = "roles"
	DefaultFormat    = FormatArray
)

// Profile is one emitting rule: which claim key carries the roles, in what
// shape, plus any attribute or constant claims that ride along.
//
// ApplicationID empty means "the project default". A non-empty ApplicationID
// is an override belonging to one application.
type Profile struct {
	ProjectID     string `json:"project_id"`
	ApplicationID string `json:"application_id,omitempty"`
	// ApplicationName is display-only; it lets the UI say which app owns a
	// key without a second lookup.
	ApplicationName string `json:"application_name,omitempty"`
	ClaimName       string `json:"claim_name"`
	FormatType      string `json:"format_type"`
	// AttributeClaims maps a claim key to one of the Attr* sources.
	AttributeClaims map[string]string `json:"attribute_claims,omitempty"`
	// StaticClaims maps a claim key to a literal value emitted verbatim.
	StaticClaims map[string]any `json:"static_claims,omitempty"`
}

// Facts is everything known about one (user, project) pair at token time. It
// is exactly what the cache compiler persists, so Shape never needs to widen
// its inputs at the point where latency matters.
type Facts struct {
	Roles      []string `json:"roles"`
	UserID     string   `json:"user_id"`
	ProjectID  string   `json:"project_id"`
	Email      string   `json:"email,omitempty"`
	Name       string   `json:"name,omitempty"`
	Title      string   `json:"title,omitempty"`
	Team       string   `json:"team,omitempty"`
	CompiledAt string   `json:"compiled_at,omitempty"`
}

// FormatRoles renders a role list in the requested shape. An unknown format
// degrades to an array rather than erroring: a token with the roles in the
// wrong shape is recoverable, a token missing them is an outage.
func FormatRoles(roles []string, format string) any {
	if roles == nil {
		roles = []string{}
	}
	switch format {
	case FormatCSV:
		return strings.Join(roles, ",")
	case FormatSpaceDelimited:
		return strings.Join(roles, " ")
	default:
		return roles
	}
}

// Emit renders one profile against one set of facts.
func (p Profile) Emit(f Facts) map[string]any {
	out := make(map[string]any, 1+len(p.AttributeClaims)+len(p.StaticClaims))

	claimName := strings.TrimSpace(p.ClaimName)
	if claimName == "" {
		claimName = DefaultClaimName
	}
	out[claimName] = FormatRoles(f.Roles, p.FormatType)

	for key, source := range p.AttributeClaims {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if v, ok := attributeValue(source, f); ok {
			out[key] = v
		}
	}
	for key, value := range p.StaticClaims {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		out[key] = value
	}
	return out
}

// attributeValue resolves one Attr* source. An unknown source emits nothing
// rather than a null claim — a claim whose value is null reads to an
// application as "the user has no email", which is a different and worse lie
// than the claim being absent.
func attributeValue(source string, f Facts) (any, bool) {
	switch source {
	case AttrUserID:
		return f.UserID, f.UserID != ""
	case AttrProjectID:
		return f.ProjectID, f.ProjectID != ""
	case AttrEmail:
		return f.Email, f.Email != ""
	case AttrName:
		return f.Name, f.Name != ""
	case AttrTitle:
		return f.Title, f.Title != ""
	case AttrTeam:
		return f.Team, f.Team != ""
	case AttrCompiledAt:
		return f.CompiledAt, f.CompiledAt != ""
	case AttrRoleCount:
		return len(f.Roles), true
	default:
		return nil, false
	}
}

// Shape merges every profile that applies to one project into the flat claim
// map the token carries.
//
// A token issued for a project carries the project default AND every
// application override on that project, because the Actions v2 function
// payload does not identify which application the token is for (verified
// against the documented preaccesstoken payload: function, userinfo, user,
// user_metadata, org, user_grants — no client id). Each application reads its
// own key; keys are validated unique so no two can collide.
func Shape(profiles []Profile, f Facts) map[string]any {
	out := map[string]any{}
	for _, p := range sortedProfiles(profiles) {
		for k, v := range p.Emit(f) {
			out[k] = v
		}
	}
	return out
}

// sortedProfiles gives Shape a deterministic merge order — project default
// first, then overrides by application id. Two profiles should never write the
// same key (Conflicts rejects that at save time), but if a row predating
// validation does, the ordering makes the winner predictable instead of
// map-iteration-random.
func sortedProfiles(profiles []Profile) []Profile {
	out := append([]Profile(nil), profiles...)
	sort.SliceStable(out, func(i, j int) bool {
		if (out[i].ApplicationID == "") != (out[j].ApplicationID == "") {
			return out[i].ApplicationID == ""
		}
		return out[i].ApplicationID < out[j].ApplicationID
	})
	return out
}

// Keys lists every claim key a profile emits, sorted. Used by validation and
// by the UI's "this token will carry" summary.
func (p Profile) Keys() []string {
	seen := map[string]bool{}
	claimName := strings.TrimSpace(p.ClaimName)
	if claimName == "" {
		claimName = DefaultClaimName
	}
	seen[claimName] = true
	for k := range p.AttributeClaims {
		if k = strings.TrimSpace(k); k != "" {
			seen[k] = true
		}
	}
	for k := range p.StaticClaims {
		if k = strings.TrimSpace(k); k != "" {
			seen[k] = true
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// validClaimKey allows the dotted namespaces the design uses
// ("syndra.laser.roles") plus the plain snake/kebab keys apps tend to expect.
// Rejecting the rest early keeps a malformed key out of a signed token, where
// it would be an application-side parse failure nobody can trace back here.
func validClaimKey(key string) bool {
	if key == "" || len(key) > 128 {
		return false
	}
	for i, r := range key {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '_' || r == '-' || r == '/' || r == ':':
		case r == '.':
			// A leading or trailing dot produces an empty namespace segment.
			if i == 0 || i == len(key)-1 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// ValidateProfile checks one profile in isolation: format enum, claim key
// syntax, known attribute sources, and no key emitted twice by the same
// profile.
func ValidateProfile(p Profile) error {
	switch p.FormatType {
	case FormatArray, FormatCSV, FormatSpaceDelimited:
	default:
		return fmt.Errorf("format_type must be one of array, csv, space_delimited (got %q)", p.FormatType)
	}

	claimName := strings.TrimSpace(p.ClaimName)
	if claimName == "" {
		return fmt.Errorf("claim_name is required")
	}
	if !validClaimKey(claimName) {
		return fmt.Errorf("claim_name %q is not a valid claim key", claimName)
	}

	seen := map[string]string{claimName: "roles claim"}
	for key, source := range p.AttributeClaims {
		key = strings.TrimSpace(key)
		if !validClaimKey(key) {
			return fmt.Errorf("attribute claim key %q is not a valid claim key", key)
		}
		if !KnownAttribute(source) {
			return fmt.Errorf("attribute claim %q has unknown source %q", key, source)
		}
		if where, dup := seen[key]; dup {
			return fmt.Errorf("claim key %q is already used by the %s", key, where)
		}
		seen[key] = "attribute claims"
	}
	for key := range p.StaticClaims {
		key = strings.TrimSpace(key)
		if !validClaimKey(key) {
			return fmt.Errorf("static claim key %q is not a valid claim key", key)
		}
		if where, dup := seen[key]; dup {
			return fmt.Errorf("claim key %q is already used by the %s", key, where)
		}
		seen[key] = "static claims"
	}
	return nil
}

// KnownAttribute reports whether source is one of the Attr* constants.
func KnownAttribute(source string) bool {
	switch source {
	case AttrUserID, AttrProjectID, AttrEmail, AttrName, AttrTitle, AttrTeam, AttrRoleCount, AttrCompiledAt:
		return true
	default:
		return false
	}
}

// Attributes lists every selectable attribute source, for the UI's dropdown.
func Attributes() []string {
	return []string{AttrUserID, AttrProjectID, AttrEmail, AttrName, AttrTitle, AttrTeam, AttrRoleCount, AttrCompiledAt}
}

// Conflict names one claim key claimed by two profiles on the same project.
type Conflict struct {
	ClaimKey string `json:"claim_key"`
	Owner    string `json:"owner"`
	Other    string `json:"other"`
}

func (c Conflict) Error() string {
	return fmt.Sprintf("claim key %q is already emitted by %s", c.ClaimKey, c.Other)
}

// Conflicts reports claim keys emitted by more than one profile in the set.
// Callers pass the project default plus every override on that project: a
// token is flat, so a duplicate key means one application silently reads
// another's value.
func Conflicts(profiles []Profile) []Conflict {
	owner := map[string]string{}
	var out []Conflict
	for _, p := range sortedProfiles(profiles) {
		label := profileLabel(p)
		for _, key := range p.Keys() {
			if existing, taken := owner[key]; taken {
				out = append(out, Conflict{ClaimKey: key, Owner: label, Other: existing})
				continue
			}
			owner[key] = label
		}
	}
	return out
}

func profileLabel(p Profile) string {
	if p.ApplicationID == "" {
		return "the project default"
	}
	if p.ApplicationName != "" {
		return p.ApplicationName
	}
	return p.ApplicationID
}
