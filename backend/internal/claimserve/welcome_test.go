package claimserve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"syndra/internal/claims"
	"syndra/internal/claimserve/claimservetest"
	"syndra/internal/claimtel"
)

// The welcome switch (assessment SC4 option b): a brand-new member's first
// token, which names no project yet, carries the welcome roles for the app's
// own project, shaped by the app's own shape. Everything else stays no_grants.

var appShape = claims.Profile{
	Claims: []claims.Claim{
		{Path: []string{"app_roles"}, Kind: claims.KindRoles, Format: claims.FormatCSV},
		{Path: []string{"bundles"}, Kind: claims.KindBundles},
	},
	IncludeNew: true,
	Lands:      claims.LandsBoth,
}

var welcomeOn = Welcome{
	On: true,
	Facts: map[string]claims.Facts{
		"p1": {ProjectID: "p1", Roles: []string{"maker", "visitor"}, Bundles: []string{"Welcome"}},
	},
	Clients: map[string]string{"cApp": "p1", "cDefault": "p1", "cOther": "p2"},
}

// welcomePlane is p1 (cApp with its own shape) and p2 published, the welcome
// switch as w, and a clock at noon.
func welcomePlane(t *testing.T, w Welcome) (*Door, *claimservetest.Redis, *clock) {
	t.Helper()
	ctx := context.Background()
	r := claimservetest.New()
	if _, err := Publish(ctx, r, map[string]Project{
		"p1": {Shapes: ProjectShapes{Default: claims.Builtin(), DefaultSource: "builtin", ByClient: map[string]AppShape{"cApp": {ApplicationID: "a1", Version: 1, Profile: appShape}}}},
		"p2": {Shapes: ProjectShapes{Default: claims.Builtin(), DefaultSource: "builtin"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := PublishWelcome(ctx, r, w); err != nil {
		t.Fatal(err)
	}
	c := &clock{t: time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)}
	p := New(r, c.now)
	if err := p.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	return &Door{Plane: func() *Plane { return p }}, r, c
}

// signIn is a payload with no user_grants, as Zitadel sends a first sign-in.
func signIn(client string, created time.Time, kind string) Request {
	user := map[string]any{"id": "u1", "creation_date": created.Format(time.RFC3339Nano)}
	switch kind {
	case "human":
		user["human"] = map[string]any{"email": "new@example.edu"}
	case "machine":
		user["machine"] = map[string]any{"name": "ingest"}
	}
	body, _ := json.Marshal(map[string]any{"function": pre, "user": user, "application": map[string]any{"client_id": client}})
	req, fault := Decode(body)
	if fault != "" {
		panic(fmt.Sprintf("decode: %s", fault))
	}
	return req
}

func claimOf(a Answer, key string) any {
	for _, c := range a.Claims {
		if c.Key == key {
			return c.Value
		}
	}
	return nil
}

func TestWelcome_NewMemberGetsTheWelcomeRolesShapedByTheApp(t *testing.T) {
	d, _, c := welcomePlane(t, welcomeOn)
	ans := d.Answer(context.Background(), signIn("cApp", c.now().Add(-2*time.Minute), "human"))
	if ans.Outcome != claimtel.ServedWelcome || ans.ProjectID != "p1" || !ans.ClientKnown || ans.Gen == 0 {
		t.Fatalf("answer: %+v", ans)
	}
	if got := claimOf(ans, "app_roles"); got != "maker,visitor" {
		t.Fatalf("the app's shape (csv, its own claim name) must apply: %#v", ans.Claims)
	}
	// Equivalence with the preview: the same shaper over the same facts.
	want := Envelope(claims.ShapeAt(appShape, claims.Facts{UserID: "u1", ProjectID: "p1", Roles: []string{"maker", "visitor"}, Bundles: []string{"Welcome"}}, c.now()))
	if !reflect.DeepEqual(ans.Claims, want) {
		t.Fatalf("welcome answer differs from the preview over the same facts:\n got %#v\nwant %#v", ans.Claims, want)
	}

	// An app on the project default gets the default's shape.
	def := d.Answer(context.Background(), signIn("cDefault", c.now(), "human"))
	if def.Outcome != claimtel.ServedWelcome || def.ClientKnown || claimOf(def, claims.DefaultClaimName) == nil {
		t.Fatalf("default-shaped app: %+v", def)
	}
}

func TestWelcome_NotApplied(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name  string
		w     Welcome
		setup func(r *claimservetest.Redis)
		req   func(now time.Time) Request
	}{
		{"switch off", Welcome{Facts: welcomeOn.Facts, Clients: welcomeOn.Clients}, nil,
			func(now time.Time) Request { return signIn("cApp", now, "human") }},
		{"older than ten minutes", welcomeOn, nil,
			func(now time.Time) Request { return signIn("cApp", now.Add(-11*time.Minute), "human") }},
		{"no creation date", welcomeOn, nil,
			func(now time.Time) Request { return signIn("cApp", time.Time{}, "human") }},
		{"a machine user", welcomeOn, nil,
			func(now time.Time) Request { return signIn("cApp", now, "machine") }},
		{"neither human nor machine", welcomeOn, nil,
			func(now time.Time) Request { return signIn("cApp", now, "") }},
		{"already has facts in the project", welcomeOn,
			func(r *claimservetest.Redis) { r.SetField(FactsKey("u1"), "p1", facts()) },
			func(now time.Time) Request { return signIn("cApp", now, "human") }},
		{"still on the old facts key", welcomeOn,
			func(r *claimservetest.Redis) { r.Set(LegacyFactsKey("u1", "p1"), facts("admin")) },
			func(now time.Time) Request { return signIn("cApp", now, "human") }},
		{"Redis cannot say", welcomeOn,
			func(r *claimservetest.Redis) { r.Fail(errors.New("down")) },
			func(now time.Time) Request { return signIn("cApp", now, "human") }},
		{"an app of another project (never cross-project)", welcomeOn, nil,
			func(now time.Time) Request { return signIn("cOther", now, "human") }},
		{"an unknown app", welcomeOn, nil,
			func(now time.Time) Request { return signIn("cStranger", now, "human") }},
		{"nothing the bundle gives without approval", Welcome{On: true}, nil,
			func(now time.Time) Request { return signIn("cApp", now, "human") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, r, c := welcomePlane(t, tc.w)
			if tc.setup != nil {
				tc.setup(r)
			}
			ans := d.Answer(ctx, tc.req(c.now()))
			if ans.Outcome != claimtel.NoGrants || len(ans.Claims) != 0 || ans.ProjectID != "" {
				t.Fatalf("want no_grants and nothing sent, got %+v", ans)
			}
		})
	}
}

func TestWelcome_ShapeThatLandsElsewhereIsNotLanding(t *testing.T) {
	d, r, c := welcomePlane(t, welcomeOn)
	shape := appShape
	shape.Lands = claims.LandsUserinfo
	if _, err := Publish(context.Background(), r, map[string]Project{"p1": {Shapes: ProjectShapes{Default: claims.Builtin(), DefaultSource: "builtin", ByClient: map[string]AppShape{"cApp": {Profile: shape}}}}}); err != nil {
		t.Fatal(err)
	}
	c.add(2 * ReloadAtLeast)
	if err := d.Plane().Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	ans := d.Answer(context.Background(), signIn("cApp", c.now(), "human"))
	if ans.Outcome != claimtel.NotLanding || len(ans.Claims) != 0 {
		t.Fatalf("got %+v", ans)
	}
}

// Write-through: the switch reaches a running plane within a poll, and an
// unchanged publish does not move claims:gen.
func TestPublishWelcome_WritesThroughAndMovesTheGenerationOnlyOnChange(t *testing.T) {
	ctx := context.Background()
	d, r, c := welcomePlane(t, Welcome{})
	p := d.Plane()
	if p.Config().Welcome.On {
		t.Fatal("off was published")
	}
	gen := p.Config().Gen
	if err := PublishWelcome(ctx, r, welcomeOn); err != nil {
		t.Fatal(err)
	}
	if err := p.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	if cfg := p.Config(); cfg.Gen == gen || !cfg.Welcome.On || cfg.Welcome.Clients["cApp"] != "p1" {
		t.Fatalf("not written through: gen %d→%d %+v", gen, cfg.Gen, cfg.Welcome)
	}
	if ans := d.Answer(ctx, signIn("cApp", c.now(), "human")); ans.Outcome != claimtel.ServedWelcome {
		t.Fatalf("after the switch: %+v", ans)
	}
	incr := r.Count("INCR")
	if err := PublishWelcome(ctx, r, welcomeOn); err != nil {
		t.Fatal(err)
	}
	if r.Count("INCR") != incr {
		t.Fatal("an unchanged publish moved claims:gen")
	}
}

func TestWelcome_UnreadableIsOff(t *testing.T) {
	ctx := context.Background()
	d, r, c := welcomePlane(t, welcomeOn)
	r.Set(WelcomeKey, `{"on":true,"facts":"broken"}`)
	c.add(2 * ReloadAtLeast)
	if err := d.Plane().Poll(ctx); err != nil {
		t.Fatal(err)
	}
	if d.Plane().Config().Welcome.On {
		t.Fatal("half a switch read as on")
	}
}

func TestDecode_UserCreationAndKind(t *testing.T) {
	at := time.Date(2026, 11, 3, 11, 58, 0, 123, time.UTC)
	req := signIn("c", at, "human")
	if !req.User.CreatedAt.Equal(at) || !req.User.Human {
		t.Fatalf("human: %+v", req.User)
	}
	if signIn("c", at, "machine").User.Human {
		t.Fatal("a machine user read as a person")
	}
	both, _ := Decode([]byte(`{"user":{"id":"u","human":{},"machine":{}}}`))
	if both.User.Human {
		t.Fatal("a block with both read as a person")
	}
	odd, fault := Decode([]byte(`{"user":{"id":"u","creation_date":17,"human":"x"}}`))
	if fault != "" || !odd.User.CreatedAt.IsZero() || odd.User.Human {
		t.Fatalf("an odd user block must read as unknown, never fail the call: %q %+v", fault, odd.User)
	}
}
