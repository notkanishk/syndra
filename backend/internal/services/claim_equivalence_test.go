package services

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"testing"
	"time"

	"syndra/internal/claims"
	"syndra/internal/claimserve"
	"syndra/internal/claimserve/claimservetest"
	"syndra/internal/db"
	"syndra/internal/directory"
	"syndra/internal/models"
)

// equivDir is one project with two apps: aOwn may have a shape of its own,
// aDef never does (it is what every other client gets: the default).
type equivDir struct{ directory.Source }

var equivApps = []models.ApplicationCatalog{
	{ID: "aOwn", Name: "Own", ProjectID: "pEq", ClientID: "cOwn", Created: seedCutoff.Add(time.Hour)},
	{ID: "aDef", Name: "Default", ProjectID: "pEq", ClientID: "cDef", Created: seedCutoff.Add(time.Hour)},
}

func (equivDir) FindApplication(_ context.Context, id string) (models.ApplicationCatalog, bool, error) {
	for _, a := range equivApps {
		if a.ID == id {
			return a, true, nil
		}
	}
	return models.ApplicationCatalog{}, false, nil
}
func (equivDir) Applications(context.Context) ([]models.ApplicationCatalog, error) {
	return equivApps, nil
}
func (equivDir) FindProject(_ context.Context, id string) (models.ProjectCatalog, bool, error) {
	return models.ProjectCatalog{ID: id}, true, nil
}
func (equivDir) RoleKeysForProject(context.Context, string) ([]string, error) { return equivRoles, nil }

// equivUser is the case's person as Zitadel holds them: the directory's read
// in the preview, the payload's user live.
var equivUser models.UserProfile

func (equivDir) FindUser(_ context.Context, id string) (models.UserProfile, bool, error) {
	return equivUser, id == equivUser.ID, nil
}

// randomPerson is u1 as Zitadel holds them, apart from Syndra's facts: a
// machine user, or a person whose names and email Zitadel may lack.
func randomPerson(rng *rand.Rand) models.UserProfile {
	pick := func(s string) string { return []string{"", s}[rng.IntN(2)] }
	if rng.IntN(5) == 0 {
		return models.UserProfile{ID: "u1", Kind: models.UserKindMachine, Username: pick("probe-bot")}
	}
	return models.UserProfile{ID: "u1", Kind: models.UserKindHuman, Username: pick("ada"),
		GivenName: pick("Ada"), FamilyName: pick("Lovelace"), Email: pick("ada@example.edu")}
}

// payloadUser is u as Zitadel's query.User serialises it into the payload:
// snake_case keys, every one omitempty.
func payloadUser(u models.UserProfile) map[string]any {
	m := map[string]any{"id": u.ID, "type": 1, "state": 1, "preferred_login_name": "u1@example.edu"}
	if u.Username != "" {
		m["username"] = u.Username
	}
	if u.IsMachine() {
		m["type"], m["machine"] = 2, map[string]any{"name": "Probe", "access_token_type": 1}
		return m
	}
	h := map[string]any{"display_name": "Not The Name", "preferred_language": "en"}
	for k, v := range map[string]string{"first_name": u.GivenName, "last_name": u.FamilyName, "email": u.Email} {
		if v != "" {
			h[k] = v
		}
	}
	m["human"] = h
	return m
}

var equivRoles = []string{"staff", "admin", "guest", "trained"}

func randomProfile(rng *rand.Rand) claims.Profile {
	ts := claims.Templates()
	p := ts[rng.IntN(len(ts))].Profile
	if rng.IntN(4) == 0 {
		p = claims.ActionsV1() // the seeded shape: the names, as v1 built them
	}
	p.Lands = []string{claims.LandsBoth, claims.LandsAccessToken, claims.LandsUserinfo}[rng.IntN(3)]
	p.IncludeNew = rng.IntN(2) == 0
	p.Roles = nil
	for _, r := range equivRoles {
		if rng.IntN(2) == 0 {
			p.Roles = append(p.Roles, claims.RoleMap{Key: r, As: []string{"", "w-" + r}[rng.IntN(2)], Included: rng.IntN(3) > 0})
		}
	}
	extra := []claims.Claim{
		{Path: []string{"roles_csv"}, Kind: claims.KindRoles, Format: claims.FormatCSV},
		{Path: []string{"x", "team"}, Kind: claims.KindAttribute, Source: claims.AttrTeam},
		{Path: []string{"x", "username"}, Kind: claims.KindAttribute, Source: claims.AttrUsername},
		{Path: []string{"x", "email"}, Kind: claims.KindAttribute, Source: claims.AttrEmail},
		{Path: []string{"is_admin"}, Kind: claims.KindIfRole, Roles: []string{"admin"}},
		{Path: []string{"ends"}, Kind: claims.KindRoleEnds},
		{Path: []string{"bundles"}, Kind: claims.KindBundles},
		{Path: []string{"tenant"}, Kind: claims.KindStatic, Value: json.RawMessage(`{"a": [1, "b"]}`)},
	}
	p.Claims = append([]claims.Claim(nil), p.Claims...)
	for _, c := range extra {
		if rng.IntN(2) == 0 {
			p.Claims = append(p.Claims, c)
		}
	}
	return p
}

func randomFacts(rng *rand.Rand, today time.Time) claims.Facts {
	f := claims.Facts{UserID: "u1", ProjectID: "pEq", Team: []string{"", "Fabrication"}[rng.IntN(2)], Email: "u1@example.edu"}
	if rng.IntN(2) == 0 { // the cached copy, which Zitadel's live identity overrides
		f.GivenName, f.FamilyName, f.Name, f.Username = "Old", "Name", "Display Name", "old"
	}
	for _, r := range equivRoles {
		if rng.IntN(2) == 0 {
			f.Roles = append(f.Roles, r)
			if rng.IntN(2) == 0 {
				if f.RoleEnds == nil {
					f.RoleEnds = map[string]string{}
				}
				// Past or future, never today's edge: the two sides read the
				// clock a moment apart.
				days := rng.IntN(400) + 2
				if rng.IntN(2) == 0 {
					days = -days
				}
				f.RoleEnds[r] = today.AddDate(0, 0, days).Format(time.DateOnly)
			}
		}
	}
	if rng.IntN(2) == 0 {
		f.Bundles = []string{"Makers"}
	}
	f.IsServiceAccount = rng.IntN(4) == 0
	return f
}

// randomGrants is Zitadel's grants for u1: some of the project's roles and one
// only Zitadel knows, in any order, split over one or two rows (Zitadel may
// send a row per role), and a grant in another project that must not count.
func randomGrants(rng *rand.Rand) []db.ObservedGrant {
	var roles []string
	for _, r := range append([]string{"zitadel-only"}, equivRoles...) {
		if rng.IntN(2) == 0 {
			roles = append(roles, r)
		}
	}
	rng.Shuffle(len(roles), func(i, j int) { roles[i], roles[j] = roles[j], roles[i] })
	cut := rng.IntN(len(roles) + 1)
	return []db.ObservedGrant{
		{UserID: "u1", ProjectID: "pEq", RoleKeys: roles[:cut]},
		{UserID: "u1", ProjectID: "pOther", RoleKeys: []string{"admin"}},
		{UserID: "u1", ProjectID: "pEq", RoleKeys: roles[cut:]},
	}
}

// The equivalence property (assessment §4h; CLAUDE.md: "a preview computed by
// different code from the token it previews is a preview of nothing"). For
// random shapes, facts, functions and client ids, what the data plane's
// snapshot answers, after the write side's resolve, Publish, a Redis round
// trip and a reload, is exactly what the operator's preview shows for that
// app: the same claims in the same order, or nothing where the shape does
// not land. The roles are Zitadel's on both sides (the payload's live, the
// observed grants' in the preview), whatever Syndra's facts say; so is the
// identity (the payload's user live, the directory's read in the preview),
// whatever Syndra's cached copy says; and a person with no facts is answered
// over the roles and identity alone on both.
func TestSnapshotAnswerEqualsThePreview(t *testing.T) {
	s := stubClaimShapeDeps(t)
	directory.Default = equivDir{}
	ctx := context.Background()
	rng := rand.New(rand.NewPCG(3, 11))
	today := time.Now().UTC()
	functions := []string{claims.FunctionPreAccessToken, claims.FunctionPreUserinfo, "function/somethingnew"}
	clients := []struct{ client, app string }{{"cOwn", "aOwn"}, {"cDef", "aDef"}, {"", "aDef"}, {"cElse", "aDef"}}

	compared, landed := 0, 0
	for i := 0; i < 400; i++ {
		s.versions, s.defaults = map[string][]db.ClaimShapeVersion{}, map[string]db.ClaimProfileRow{}
		if rng.IntN(2) == 0 {
			s.defaults["pEq"] = db.ClaimProfileRow{ProjectID: "pEq", ClaimName: "d-roles", FormatType: claims.FormatSpaceDelimited,
				AttributeClaims: map[string]string{"d.email": claims.AttrEmail}, StaticClaims: map[string]any{"d.static": "x"}}
		}
		if rng.IntN(3) > 0 {
			s.versions["aOwn"] = []db.ClaimShapeVersion{{ApplicationID: "aOwn", ProjectID: "pEq", ClientID: "cOwn", Version: 1, Profile: randomProfile(rng)}}
		}
		facts := randomFacts(rng, today)
		s.stored = map[string]claims.Facts{"u1:pEq": facts}
		// Zitadel's grants, independent of Syndra's facts (a role given in
		// Zitadel directly, or one of Syndra's still queued): the same rows
		// are the live payload's user_grants and the preview's observed read.
		grants := randomGrants(rng)
		s.observed = map[string][]db.ObservedGrant{"u1": grants}
		// Zitadel's user, drawn apart from the facts too: the directory's read
		// in the preview, the payload's user (query.User's keys) live. The
		// live side decodes the payload exactly as the door does.
		equivUser = randomPerson(rng)
		var rows []map[string]any
		for _, g := range grants {
			rows = append(rows, map[string]any{"project_id": g.ProjectID, "roles": g.RoleKeys})
		}
		body, _ := json.Marshal(map[string]any{"user": payloadUser(equivUser), "user_grants": rows})
		req, bad := claimserve.Decode(body)
		if bad != "" {
			t.Fatalf("case %d: the payload decoded %s", i, bad)
		}
		listed := req.Roles("pEq")
		built := rng.IntN(4) > 0 // else no facts at all: answered over the roles alone
		if !built {
			delete(s.stored, "u1:pEq")
		}

		// The write side, as a save publishes it, and the data plane reading it.
		ps, err := ResolveProjectShapes(ctx, "pEq")
		if err != nil {
			t.Fatal(err)
		}
		r := claimservetest.New()
		if _, err := claimserve.Publish(ctx, r, map[string]claimserve.Project{"pEq": {Shapes: ps}}); err != nil {
			t.Fatal(err)
		}
		if built {
			raw, _ := json.Marshal(facts)
			r.SetField(claimserve.FactsKey("u1"), "pEq", string(raw))
		}
		plane := claimserve.New(r, nil)
		if err := plane.Poll(ctx); err != nil {
			t.Fatal(err)
		}

		for _, c := range clients {
			function := functions[rng.IntN(len(functions))]
			got := plane.Answer(ctx, "u1", "pEq", c.client, function, listed, req.User.Identity)

			pv, err := PreviewAppClaimShape(ctx, c.app, PreviewRequest{UserIDs: []string{"u1"}})
			if err != nil {
				t.Fatal(err)
			}
			e, _ := resolveApp(ctx, c.app)
			want := []claimserve.Claim{}
			if e.profile.LandsIn(function) {
				want = claimserve.Envelope(pv.People[0].Now.Token)
				landed++
			}
			gj, _ := json.Marshal(got.Claims)
			wj, _ := json.Marshal(want)
			if string(gj) != string(wj) {
				t.Fatalf("case %d, client %q, %s:\nsnapshot %s\npreview  %s\nfacts %+v", i, c.client, function, gj, wj, facts)
			}
			compared++
		}
	}
	t.Logf("measured: %d random (shape, facts, function, client) cases equal, %d of them landing", compared, landed)
	if landed < compared/3 {
		t.Fatalf("only %d of %d cases landed: the property is barely exercised", landed, compared)
	}
}
