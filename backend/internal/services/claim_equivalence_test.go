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

var equivRoles = []string{"staff", "admin", "guest", "trained"}

func randomProfile(rng *rand.Rand) claims.Profile {
	ts := claims.Templates()
	p := ts[rng.IntN(len(ts))].Profile
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

// The equivalence property (assessment §4h; CLAUDE.md: "a preview computed by
// different code from the token it previews is a preview of nothing"). For
// random shapes, facts, functions and client ids, what the data plane's
// snapshot answers, after the write side's resolve, Publish, a Redis round
// trip and a reload, is exactly what the operator's preview shows for that
// app: the same claims in the same order, or nothing where the shape does
// not land.
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
		s.stored["u1:pEq"] = facts

		// The write side, as a save publishes it, and the data plane reading it.
		ps, err := ResolveProjectShapes(ctx, "pEq")
		if err != nil {
			t.Fatal(err)
		}
		r := claimservetest.New()
		if _, err := claimserve.Publish(ctx, r, map[string]claimserve.Project{"pEq": {Shapes: ps}}); err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(facts)
		r.SetField(claimserve.FactsKey("u1"), "pEq", string(raw))
		plane := claimserve.New(r, nil)
		if err := plane.Poll(ctx); err != nil {
			t.Fatal(err)
		}

		for _, c := range clients {
			function := functions[rng.IntN(len(functions))]
			got := plane.Answer(ctx, "u1", "pEq", c.client, function)

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
