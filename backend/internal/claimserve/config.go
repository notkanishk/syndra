// Package claimserve is the claims data plane's answer: what one Actions v2
// call gets, from memory and one bounded Redis read (ADR-004).
//
// Two tiers. Config (every project's shapes and failure mode) is an immutable,
// versioned snapshot in an atomic.Pointer, reloaded when claims:gen moves. The
// write side writes it through (Publish) and never lets it expire. Facts are
// one HGET facts:{u} {p} with a real deadline; a last-known-good copy in
// process answers while Redis is out, within the project's staleness bound;
// then the failure mode. No Postgres, no Zitadel: nothing on the request path
// can block past FactsTimeout.
//
// It imports only internal/claims and internal/claimtel, so the data plane can
// move to its own container (assessment P4) as packaging, not a rewrite.
package claimserve

import (
	"encoding/json"
	"sort"
	"time"

	"syndra/internal/claims"
)

// Redis keys. The write side and the data plane share these and nothing else.
const (
	GenKey      = "claims:gen"      // INCR on every config write; the loader polls it
	ProjectsKey = "claims:projects" // the set of projects with published config
	// MetaField is the field every facts hash carries beside its projects: when
	// the person was last compiled. Its presence also says the person is on the
	// hash layout, so their old mapping:* keys are gone.
	MetaField = "_compiled_at"
	// KeyringKey holds the claims target's signing keys as the write side read
	// them from Zitadel (Keyring JSON, no TTL). The key is in Redis because the
	// data plane may run in its own container (P4); Redis is internal-only.
	KeyringKey = "claims:keyring"
)

// PreviousKeyKeep is how long the key Zitadel signed with before a rotation
// keeps verifying: calls Zitadel signed just before it swapped still arrive.
const PreviousKeyKeep = 15 * time.Minute

// Keyring is the signing keys read from Zitadel (S5, P3). Zitadel signs with
// one key and swaps it with no grace period, so the data plane follows it
// here; the env key stays the bootstrap and the fallback. Never logged.
type Keyring struct {
	Current       string `json:"current"`
	Previous      string `json:"previous,omitempty"`
	PreviousUntil int64  `json:"previous_until,omitempty"` // unix seconds
}

// Next is the keyring after Zitadel was read holding key: unchanged when it is
// the current one, else key becomes current and the old current is kept
// PreviousKeyKeep.
func (k Keyring) Next(key string, now time.Time) Keyring {
	if key == "" || key == k.Current {
		return k
	}
	next := Keyring{Current: key}
	if k.Current != "" {
		next.Previous, next.PreviousUntil = k.Current, now.Add(PreviousKeyKeep).Unix()
	}
	return next
}

// SlotKey is one key the data plane verifies with, and its telemetry slot.
type SlotKey struct {
	Slot, Key string
}

// Keys is the keyring's live keys, current first.
func (k Keyring) Keys(now time.Time) []SlotKey {
	var out []SlotKey
	if k.Current != "" {
		out = append(out, SlotKey{Slot: "current", Key: k.Current})
	}
	if k.Previous != "" && now.Unix() < k.PreviousUntil {
		out = append(out, SlotKey{Slot: "previous", Key: k.Previous})
	}
	return out
}

// ShapesKey holds one project's shapes (ProjectShapes JSON), no TTL.
func ShapesKey(projectID string) string { return "claim_shapes:" + projectID }

// ModeKey holds one project's failure mode (Mode JSON), no TTL.
func ModeKey(projectID string) string { return "claim_mode:" + projectID }

// FactsKey is one person's facts hash: a field per project, Facts JSON.
func FactsKey(userID string) string { return "facts:" + userID }

// LegacyFactsKey is the per-(person, project) key the compiler wrote before
// the hash layout (S1b). Read only when the hash has no field, and only until
// the first warm after the upgrade has moved everyone.
// ponytail: delete with its two readers once every deployment has warmed once.
func LegacyFactsKey(userID, projectID string) string {
	return LegacyFactsPrefix + userID + ":" + projectID
}

// LegacyFactsPrefix starts every old per-project facts key.
const LegacyFactsPrefix = "mapping:"

// AppShape is one app's saved shape, as the data plane needs it.
type AppShape struct {
	ApplicationID string         `json:"application_id"`
	Version       int            `json:"version"`
	TemplateID    string         `json:"template_id,omitempty"`
	Profile       claims.Profile `json:"profile"`
}

// ProjectShapes is everything the data plane needs to shape a token for one
// project: the default, and each app's shape by client id.
type ProjectShapes struct {
	Default       claims.Profile      `json:"default"`
	DefaultSource string              `json:"default_source"`
	ByClient      map[string]AppShape `json:"by_client"`
}

// For picks the shape for a sign-in's client id: the app's own, else the
// project default. ok is false when the client has no shape of its own.
func (ps ProjectShapes) For(clientID string) (AppShape, bool) {
	if s, ok := ps.ByClient[clientID]; ok && clientID != "" {
		return s, true
	}
	return AppShape{Profile: ps.Default}, false
}

// Failure modes.
const (
	FailClosed  = "fail_closed"
	MinimalSafe = "minimal_safe"
)

// DefaultLKGMaxAge is how old a last-known-good answer may be when a project
// sets no bound. "Serve what we knew six hours ago" is a policy choice about
// revocations, so it is per project (claim_profiles.claim_lkg_max_age_seconds).
// ponytail: 6h until the owner confirms it.
const DefaultLKGMaxAge = 6 * time.Hour

// Mode is a project's answer when its facts can't be served.
type Mode struct {
	Mode              string         `json:"mode"`
	MinimalSafeClaims map[string]any `json:"minimal_safe_claims"`
	// LKGMaxAgeS bounds a last-known-good answer; 0 is DefaultLKGMaxAge.
	LKGMaxAgeS int `json:"lkg_max_age_s,omitempty"`
}

// LKGMaxAge is the project's staleness bound.
func (m Mode) LKGMaxAge() time.Duration {
	if m.LKGMaxAgeS > 0 {
		return time.Duration(m.LKGMaxAgeS) * time.Second
	}
	return DefaultLKGMaxAge
}

// Claims is the failure mode's answer: the minimal-safe set, key-sorted, or
// nothing (fail closed, and any mode this code does not know).
func (m Mode) Claims() []Claim {
	if m.Mode != MinimalSafe {
		return []Claim{}
	}
	keys := make([]string, 0, len(m.MinimalSafeClaims))
	for k := range m.MinimalSafeClaims {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]Claim, 0, len(keys))
	for _, k := range keys {
		out = append(out, Claim{Key: k, Value: m.MinimalSafeClaims[k]})
	}
	return out
}

// Project is one project's published config.
type Project struct {
	Shapes ProjectShapes `json:"shapes"`
	Mode   Mode          `json:"mode"`
}

// Config is the immutable snapshot the data plane answers from. Never
// modified after it is stored; a reload builds a new one.
type Config struct {
	Gen      int64
	BuiltAt  time.Time
	Projects map[string]Project
	// ClientProject maps each client with a shape of its own to its project
	// (invariant I2: the client's own project is the token's).
	ClientProject map[string]string
	// Keys is the signing keyring, empty until the write side has read it.
	Keys Keyring
}

// Claim is one entry in the Actions v2 append_claims list.
type Claim struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

// Envelope turns a shaped token into the append_claims list, in row order.
func Envelope(o *claims.Object) []Claim {
	out := make([]Claim, 0, len(o.Keys()))
	for _, k := range o.Keys() {
		v, _ := o.Get(k)
		out = append(out, Claim{Key: k, Value: v})
	}
	return out
}

// encode is the one encoding both sides compare and store.
func encode(v any) (string, error) {
	b, err := json.Marshal(v)
	return string(b), err
}
