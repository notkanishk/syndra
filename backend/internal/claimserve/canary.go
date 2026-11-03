package claimserve

import (
	"encoding/json"

	"syndra/internal/claims"
)

// The L1 canary (claims-delivery-integrity S6, assessment §4d). A fixed
// person, project and client that exist only in Redis: the write side
// publishes the project like any other (Publish), the compiler writes the
// person's facts like any other (cache.RefreshUser), and the probe asks the
// public URL for their claims. None of the three is in Zitadel or Postgres,
// so no screen lists them as an app or a person; the reconciler and the
// snapshot's project count leave the project out, and telemetry counts the
// call apart (claimtel.Call.Probe).
const (
	CanaryUserID    = "syndra-canary"
	CanaryProjectID = "syndra-canary"
	CanaryClientID  = "syndra-canary"
	canaryRole      = "probe"
)

var canaryProfile = claims.Profile{
	Claims: []claims.Claim{
		{Path: []string{"syndra_canary"}, Kind: claims.KindRoles, Format: claims.FormatArray},
		{Path: []string{"syndra_canary_user"}, Kind: claims.KindAttribute, Source: claims.AttrUserID},
	},
	IncludeNew: true,
	Lands:      claims.LandsBoth,
}

// CanaryProject is the canary's published config: one app with its own
// shape, failing closed, so a canary answered by any rung but served reads
// as wrong claims.
func CanaryProject() Project {
	return Project{
		Shapes: ProjectShapes{
			Default: canaryProfile, DefaultSource: "canary",
			ByClient: map[string]AppShape{CanaryClientID: {ApplicationID: CanaryClientID, Version: 1, Profile: canaryProfile}},
		},
		Mode: Mode{Mode: FailClosed},
	}
}

// CanaryFacts is what the compiler writes for the canary.
func CanaryFacts() claims.Facts {
	return claims.Facts{Roles: []string{canaryRole}, UserID: CanaryUserID, ProjectID: CanaryProjectID}
}

// CanaryPayload is the body L1 posts: Zitadel's preaccesstoken shape for the
// canary.
func CanaryPayload() []byte {
	b, _ := json.Marshal(map[string]any{
		"function":    claims.FunctionPreAccessToken,
		"user":        map[string]string{"id": CanaryUserID},
		"user_grants": []map[string]string{{"project_id": CanaryProjectID}},
		"application": map[string]string{"client_id": CanaryClientID},
	})
	return b
}

// CanaryExpected is the exact append_claims the canary must get. Written out,
// not computed: a shaper change that alters it fails TestCanaryExpected
// rather than moving the probe's expectation with it.
const CanaryExpected = `[{"key":"syndra_canary","value":["probe"]},{"key":"syndra_canary_user","value":"syndra-canary"}]`

// WithoutCanary is cfg with the canary project left out: what the reconciler
// checks, since the canary is no app Zitadel knows.
func WithoutCanary(cfg *Config) *Config {
	if cfg == nil {
		return nil
	}
	if _, ok := cfg.Projects[CanaryProjectID]; !ok {
		return cfg
	}
	out := *cfg
	out.Projects = make(map[string]Project, len(cfg.Projects))
	for id, p := range cfg.Projects {
		if id != CanaryProjectID {
			out.Projects[id] = p
		}
	}
	out.ClientProject = make(map[string]string, len(cfg.ClientProject))
	for c, p := range cfg.ClientProject {
		if p != CanaryProjectID {
			out.ClientProject[c] = p
		}
	}
	return &out
}
