// Package claims owns the token shape: the single place that turns a user's
// compiled access facts into the claims one application receives.
//
// Both the data plane (Zitadel Actions v2 inject) and every operator-facing
// preview call Shape. That is the whole point of the package existing — a
// preview computed by different code from the token it claims to preview is a
// preview of nothing.
//
// A shape belongs to one application (ADR-003). An application with none uses
// its project's default, and a project with none uses Builtin. Shape never
// sees more than one profile.
//
// No database or network access lives here, so the data plane can shape a
// token inside its Redis-read latency budget.
package claims

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Format identifiers accepted for a roles claim. Mirrors the
// ck_claim_profiles_format_type constraint. Templates pin FormatArray: no app
// Syndra has a template for reads roles as text.
const (
	FormatArray          = "array"
	FormatCSV            = "csv"
	FormatSpaceDelimited = "space_delimited"
)

// Claim kinds: what a row carries.
const (
	KindRoles     = "roles"     // the roles this app sees, in its words
	KindAttribute = "attribute" // one Attr* fact about the person
	KindStatic    = "static"    // the same JSON value for everyone
	KindIfRole    = "if_role"   // true if they hold any of Roles, else false
	KindBundles   = "bundles"   // the names of the bundles they hold
	KindRoleEnds  = "role_ends" // {word: "YYYY-MM-DD"} for roles that end
)

// Where a profile's claims land. Zitadel makes "ID token and userinfo" one
// choice: Syndra answers Pre Userinfo, and the ID token carries that only when
// the app has "User Info inside ID Token" on.
const (
	LandsBoth        = "both"
	LandsAccessToken = "access_token"
	LandsUserinfo    = "userinfo"
)

// The two Actions v2 functions Syndra answers.
const (
	FunctionPreAccessToken = "function/preaccesstoken"
	FunctionPreUserinfo    = "function/preuserinfo"
)

// Attribute sources a row may project. Every one is resolvable from the
// compiled cache entry alone — nothing here needs a directory call at
// token-issue time.
const (
	AttrUserID           = "user_id"
	AttrProjectID        = "project_id"
	AttrEmail            = "email"
	AttrName             = "name"
	AttrGivenName        = "given_name"
	AttrFamilyName       = "family_name"
	AttrUsername         = "username"
	AttrIsServiceAccount = "is_service_account"
	AttrTitle            = "title"
	AttrTeam             = "team"
	AttrRoleCount        = "role_count"
	AttrCompiledAt       = "compiled_at"
)

// DefaultClaimName / DefaultFormat are the built-in shape's roles row.
const (
	DefaultClaimName = "roles"
	DefaultFormat    = FormatArray
)

// RoleMap is one project role as this app sees it: included or left out, and
// the word it is sent as (As empty = Syndra's key).
type RoleMap struct {
	Key      string `json:"key"`
	As       string `json:"as,omitempty"`
	Included bool   `json:"included"`
}

// Claim is one row of a shape. Path builds nested objects: ["custom_claims",
// "groups"] is {"custom_claims": {"groups": …}}. A dot inside one segment stays
// a dot, so "syndra.laser.roles" is one flat key.
type Claim struct {
	Path   []string        `json:"path"`
	Kind   string          `json:"kind"`
	Source string          `json:"source,omitempty"` // attribute: an Attr* constant
	Value  json.RawMessage `json:"value,omitempty"`  // static: the literal, typed
	Roles  []string        `json:"roles,omitempty"`  // if_role: Syndra role keys
	Format string          `json:"format,omitempty"` // roles: array | csv | space_delimited
}

// Profile is one application's shape (or a project's default): ordered claim
// rows, which project roles it sees and in what words, and where it lands.
//
// A role not named in Roles is "added later": included only when IncludeNew.
type Profile struct {
	Claims     []Claim   `json:"claims"`
	Roles      []RoleMap `json:"roles,omitempty"`
	IncludeNew bool      `json:"include_new"`
	Lands      string    `json:"lands"`
}

// Builtin is what an app gets when neither it nor its project has a shape:
// every role in Syndra's words, as a list, in both places. Never nothing.
func Builtin() Profile {
	return Profile{
		Claims:     []Claim{{Path: []string{DefaultClaimName}, Kind: KindRoles, Format: DefaultFormat}},
		IncludeNew: true,
		Lands:      LandsBoth,
	}
}

// Effective picks the profile that applies to one app: its own, else its
// project's default, else Builtin.
func Effective(app, projectDefault *Profile) Profile {
	switch {
	case app != nil:
		return *app
	case projectDefault != nil:
		return *projectDefault
	default:
		return Builtin()
	}
}

// FromFlat converts the project-default storage (one roles key plus attribute
// and static maps) into a profile. Map rows are key-sorted so the order is
// stable; the roles row comes first.
func FromFlat(claimName, format string, attrs map[string]string, statics map[string]any) Profile {
	if strings.TrimSpace(claimName) == "" {
		claimName = DefaultClaimName
	}
	if format == "" {
		format = DefaultFormat
	}
	p := Profile{
		Claims:     []Claim{{Path: []string{strings.TrimSpace(claimName)}, Kind: KindRoles, Format: format}},
		IncludeNew: true,
		Lands:      LandsBoth,
	}
	for _, k := range sortedKeys(attrs) {
		p.Claims = append(p.Claims, Claim{Path: []string{k}, Kind: KindAttribute, Source: attrs[k]})
	}
	for _, k := range sortedKeys(statics) {
		raw, err := json.Marshal(statics[k])
		if err != nil {
			continue
		}
		p.Claims = append(p.Claims, Claim{Path: []string{k}, Kind: KindStatic, Value: raw})
	}
	return p
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
	GivenName  string   `json:"given_name,omitempty"`
	FamilyName string   `json:"family_name,omitempty"`
	Username   string   `json:"username,omitempty"`
	Title      string   `json:"title,omitempty"`
	Team       string   `json:"team,omitempty"`
	CompiledAt string   `json:"compiled_at,omitempty"`
	// Bundles are the names (not ids, so a rename reaches the app) of the
	// bundles that give the person a role in this project.
	Bundles []string `json:"bundles,omitempty"`
	// RoleEnds maps a role key to the ISO date it ends. A role held with no
	// end (from any source) is absent.
	RoleEnds         map[string]string `json:"role_ends,omitempty"`
	IsServiceAccount bool              `json:"is_service_account,omitempty"`
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

// LandsIn reports whether this profile answers the given Actions v2 function.
// An unnamed function is answered: a payload without one predates the field.
func (p Profile) LandsIn(function string) bool {
	switch function {
	case FunctionPreAccessToken:
		return p.Lands != LandsUserinfo
	case FunctionPreUserinfo:
		return p.Lands != LandsAccessToken
	default:
		return true
	}
}

// Words is the roles this app sees, in its words: filtered by inclusion,
// renamed, then deduplicated (two roles may share a word). Mapped roles come
// in the map's order, roles added later after them in held order.
func (p Profile) Words(held []string) []string {
	out := []string{}
	seen := map[string]bool{}
	add := func(w string) {
		if !seen[w] {
			seen[w] = true
			out = append(out, w)
		}
	}
	holds := make(map[string]bool, len(held))
	for _, r := range held {
		holds[r] = true
	}
	mapped := make(map[string]bool, len(p.Roles))
	for _, m := range p.Roles {
		mapped[m.Key] = true
		if m.Included && holds[m.Key] {
			add(word(m))
		}
	}
	if p.IncludeNew {
		for _, r := range held {
			if !mapped[r] {
				add(r)
			}
		}
	}
	return out
}

func word(m RoleMap) string {
	if as := strings.TrimSpace(m.As); as != "" {
		return as
	}
	return m.Key
}

// role resolves one role key: whether this app sees it, and its word.
func (p Profile) role(key string) (bool, string) {
	for _, m := range p.Roles {
		if m.Key == key {
			return m.Included, word(m)
		}
	}
	return p.IncludeNew, key
}

// Shape renders one profile against one set of facts, rows in order.
func Shape(p Profile, f Facts) *Object {
	out := NewObject()
	for _, c := range p.Claims {
		if v, ok := c.value(p, f); ok && len(c.Path) > 0 {
			place(out, c.Path, v)
		}
	}
	return out
}

func (c Claim) value(p Profile, f Facts) (any, bool) {
	switch c.Kind {
	case KindRoles:
		// Always sent, [] when nothing survives: Supabase keeps old groups
		// when the claim is missing.
		return FormatRoles(p.Words(f.Roles), c.Format), true
	case KindAttribute:
		return attributeValue(c.Source, f)
	case KindStatic:
		if len(c.Value) == 0 {
			return nil, false
		}
		return c.Value, true
	case KindIfRole:
		// A missing claim reads differently from false, so it is never left out.
		for _, r := range f.Roles {
			for _, want := range c.Roles {
				if r == want {
					return true, true
				}
			}
		}
		return false, true
	case KindBundles:
		if f.Bundles == nil {
			return []string{}, true
		}
		return f.Bundles, true
	case KindRoleEnds:
		ends := NewObject()
		for _, r := range f.Roles {
			date, ok := f.RoleEnds[r]
			if !ok {
				continue
			}
			if included, w := p.role(r); included {
				if prev, had := ends.Get(w); !had || prev.(string) < date {
					ends.Set(w, date)
				}
			}
		}
		return ends, true
	default:
		return nil, false
	}
}

// place sets a leaf at path, merging shared parents. A parent already holding
// a non-object is left alone (Validate refuses that shape at save time).
func place(root *Object, path []string, v any) {
	o := root
	for _, seg := range path[:len(path)-1] {
		cur, ok := o.Get(seg)
		if !ok {
			child := NewObject()
			o.Set(seg, child)
			o = child
			continue
		}
		child, isObj := cur.(*Object)
		if !isObj {
			return
		}
		o = child
	}
	leaf := path[len(path)-1]
	if _, taken := o.Get(leaf); taken {
		return
	}
	o.Set(leaf, v)
}

// attributeValue resolves one Attr* source. An unknown or empty source emits
// nothing rather than a null claim — a null reads to an app as "the user has
// no email", a different and worse lie than the claim being absent.
func attributeValue(source string, f Facts) (any, bool) {
	text := func(s string) (any, bool) { return s, s != "" }
	switch source {
	case AttrUserID:
		return text(f.UserID)
	case AttrProjectID:
		return text(f.ProjectID)
	case AttrEmail:
		return text(f.Email)
	case AttrName:
		return text(f.Name)
	case AttrGivenName:
		return text(f.GivenName)
	case AttrFamilyName:
		return text(f.FamilyName)
	case AttrUsername:
		return text(f.Username)
	case AttrTitle:
		return text(f.Title)
	case AttrTeam:
		return text(f.Team)
	case AttrCompiledAt:
		return text(f.CompiledAt)
	case AttrRoleCount:
		return len(f.Roles), true
	case AttrIsServiceAccount:
		return f.IsServiceAccount, true
	default:
		return nil, false
	}
}

// Attributes lists every selectable attribute source, for the UI's picker.
func Attributes() []string {
	return []string{
		AttrGivenName, AttrFamilyName, AttrName, AttrUsername, AttrEmail, AttrIsServiceAccount,
		AttrTitle, AttrTeam, AttrRoleCount, AttrCompiledAt, AttrUserID, AttrProjectID,
	}
}

// KnownAttribute reports whether source is one of the Attr* constants.
func KnownAttribute(source string) bool {
	for _, a := range Attributes() {
		if a == source {
			return true
		}
	}
	return false
}

// Normalize fills the defaults a client may leave out (lands both, a roles row
// as a list) and trims names, so Validate and Diff see one spelling.
func Normalize(p Profile) Profile {
	out := Profile{IncludeNew: p.IncludeNew, Lands: strings.TrimSpace(p.Lands), Claims: make([]Claim, len(p.Claims))}
	if out.Lands == "" {
		out.Lands = LandsBoth
	}
	for i, c := range p.Claims {
		c.Path = append([]string(nil), c.Path...)
		for j := range c.Path {
			c.Path[j] = strings.TrimSpace(c.Path[j])
		}
		if c.Kind == KindRoles && c.Format == "" {
			c.Format = FormatArray
		}
		out.Claims[i] = c
	}
	for _, m := range p.Roles {
		m.Key, m.As = strings.TrimSpace(m.Key), strings.TrimSpace(m.As)
		out.Roles = append(out.Roles, m)
	}
	return out
}

// RowError is a validation failure on one claim row (Row >= 0) or on the
// profile as a whole (Row == -1), so the UI can put it under its row.
type RowError struct {
	Row int    `json:"row"`
	Msg string `json:"message"`
}

func (e RowError) Error() string {
	if e.Row < 0 {
		return e.Msg
	}
	return fmt.Sprintf("claim %d: %s", e.Row+1, e.Msg)
}

const maxDepth = 4

// Validate checks a normalized profile: known kinds and sources, valid keys per
// path segment, typed static values, no repeated key, and no leaf whose path is
// another row's parent.
func Validate(p Profile) error {
	switch p.Lands {
	case LandsBoth, LandsAccessToken, LandsUserinfo:
	default:
		return RowError{-1, fmt.Sprintf("lands must be both, access_token or userinfo (got %q)", p.Lands)}
	}
	seenRole := map[string]bool{}
	for _, m := range p.Roles {
		if m.Key == "" {
			return RowError{-1, "a role needs its key"}
		}
		if seenRole[m.Key] {
			return RowError{-1, fmt.Sprintf("role %q is listed twice", m.Key)}
		}
		seenRole[m.Key] = true
		if len(m.As) > 128 || strings.ContainsFunc(m.As, func(r rune) bool { return r < 0x20 }) {
			return RowError{-1, fmt.Sprintf("the word for %q is not a usable value", m.Key)}
		}
	}

	paths := make([]string, len(p.Claims))
	for i, c := range p.Claims {
		if len(c.Path) == 0 || len(c.Path) > maxDepth {
			return RowError{i, fmt.Sprintf("a claim needs a name, at most %d levels deep", maxDepth)}
		}
		for _, seg := range c.Path {
			if !validClaimKey(seg) {
				return RowError{i, fmt.Sprintf("%q is not a valid claim name", seg)}
			}
		}
		if err := validateKind(c); err != "" {
			return RowError{i, err}
		}
		paths[i] = strings.Join(c.Path, "\x00")
	}
	for i := range paths {
		for j := range paths {
			if i == j {
				continue
			}
			if paths[i] == paths[j] && j < i {
				return RowError{i, fmt.Sprintf("`%s` is already sent by another row", strings.Join(p.Claims[i].Path, " › "))}
			}
			if strings.HasPrefix(paths[j], paths[i]+"\x00") {
				parent := p.Claims[i].Path
				child := p.Claims[j].Path[len(parent)]
				return RowError{i, fmt.Sprintf("`%s` is already an object here, holding `%s`.", parent[len(parent)-1], child)}
			}
		}
	}
	return nil
}

func validateKind(c Claim) string {
	switch c.Kind {
	case KindRoles:
		switch c.Format {
		case FormatArray, FormatCSV, FormatSpaceDelimited:
		default:
			return fmt.Sprintf("format must be array, csv or space_delimited (got %q)", c.Format)
		}
	case KindAttribute:
		if !KnownAttribute(c.Source) {
			return fmt.Sprintf("unknown source %q", c.Source)
		}
	case KindStatic:
		v := bytes.TrimSpace(c.Value)
		if len(v) == 0 || !json.Valid(v) || v[0] == '{' || bytes.Equal(v, []byte("null")) {
			return "a fixed value must be text, a number, yes/no or a list"
		}
	case KindIfRole:
		if len(c.Roles) == 0 {
			return "choose at least one role"
		}
	case KindBundles, KindRoleEnds:
	default:
		return fmt.Sprintf("unknown kind %q", c.Kind)
	}
	return ""
}

// validClaimKey allows one segment: the dotted namespaces the design uses
// ("syndra.laser.roles") plus the plain snake/kebab keys apps tend to expect.
// Rejecting the rest early keeps a malformed key out of a signed token, where
// it would be an app-side parse failure nobody can trace back here.
func validClaimKey(key string) bool {
	if key == "" || len(key) > 128 {
		return false
	}
	for i, r := range key {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '_' || r == '-' || r == '/' || r == ':':
		case r == '.':
			if i == 0 || i == len(key)-1 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		if strings.TrimSpace(k) != "" {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// Object is a JSON object that keeps insertion order, so a token's claims come
// out in the order the rows were added.
type Object struct {
	keys []string
	vals map[string]any
}

// NewObject returns an empty ordered object.
func NewObject() *Object { return &Object{vals: map[string]any{}} }

// Set adds or replaces a key, keeping its first position.
func (o *Object) Set(k string, v any) {
	if _, ok := o.vals[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.vals[k] = v
}

// Get returns a key's value.
func (o *Object) Get(k string) (any, bool) {
	v, ok := o.vals[k]
	return v, ok
}

// Keys returns the keys in order.
func (o *Object) Keys() []string { return o.keys }

// MarshalJSON writes the keys in order.
func (o *Object) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		b.Write(kb)
		b.WriteByte(':')
		vb, err := json.Marshal(o.vals[k])
		if err != nil {
			return nil, err
		}
		b.Write(vb)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}
