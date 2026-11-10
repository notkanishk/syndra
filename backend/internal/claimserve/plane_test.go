package claimserve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"syndra/internal/claims"
	"syndra/internal/claimserve/claimservetest"
	"syndra/internal/claimtel"
)

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time      { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) add(d time.Duration) { c.mu.Lock(); c.t = c.t.Add(d); c.mu.Unlock() }

func minimalSafe() Mode {
	return Mode{Mode: MinimalSafe, MinimalSafeClaims: map[string]any{"syndra_degraded": true}}
}

// plane is a loaded plane over an in-memory Redis with project p1 published.
func plane(t *testing.T, mode Mode) (*Plane, *claimservetest.Redis, *clock) {
	t.Helper()
	r := claimservetest.New()
	if _, err := Publish(context.Background(), r, map[string]Project{"p1": {
		Shapes: ProjectShapes{Default: claims.Builtin(), DefaultSource: "builtin"},
		Mode:   mode,
	}}); err != nil {
		t.Fatal(err)
	}
	c := &clock{t: time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)}
	p := New(r, c.now)
	if err := p.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	return p, r, c
}

func facts(roles ...string) string {
	b, _ := json.Marshal(claims.Facts{UserID: "u1", ProjectID: "p1", Roles: roles})
	return string(b)
}

func rolesOf(t *testing.T, res Result) []string {
	t.Helper()
	for _, c := range res.Claims {
		if c.Key == "roles" {
			out := []string{}
			switch v := c.Value.(type) {
			case []string:
				out = append(out, v...)
			default:
				t.Fatalf("roles value %#v", c.Value)
			}
			return out
		}
	}
	return nil
}

const pre = claims.FunctionPreAccessToken

// What Zitadel lists for the project in the payload: the token's roles.
var admin, ab = []string{"admin"}, []string{"a", "b"}

// Every rung of the fallback ladder (assessment §4e), each its own outcome.
func TestAnswer_TheLadder(t *testing.T) {
	ctx := context.Background()
	p, r, c := plane(t, minimalSafe())

	// 1. Facts from Redis.
	r.SetField(FactsKey("u1"), "p1", facts("admin"))
	res := p.Answer(ctx, "u1", "p1", "c1", pre, admin, claims.Identity{})
	if res.Outcome != claimtel.Served || len(rolesOf(t, res)) != 1 || res.Gen == 0 {
		t.Fatalf("served: %+v", res)
	}

	// 2. Redis down: the last-known-good copy, within the bound.
	r.Fail(errors.New("connection refused"))
	c.add(DefaultLKGMaxAge - time.Minute)
	if res = p.Answer(ctx, "u1", "p1", "c1", pre, admin, claims.Identity{}); res.Outcome != claimtel.ServedLKG || len(rolesOf(t, res)) != 1 {
		t.Fatalf("within the bound: %+v", res)
	}
	// 3. Past the bound: Zitadel's roles alone, never the minimal-safe set.
	// The built-in shape asks for nothing only facts hold, so the token is
	// whole: served (TestAnswer_IdentityFromThePayload has one that is not).
	c.add(2 * time.Minute)
	res = p.Answer(ctx, "u1", "p1", "c1", pre, admin, claims.Identity{})
	if res.Outcome != claimtel.Served || fmt.Sprint(rolesOf(t, res)) != "[admin]" || len(res.Claims) != 1 || res.Miss {
		t.Fatalf("past the bound: %+v", res)
	}
	// Someone never served before gets no copy at all.
	if res = p.Answer(ctx, "u2", "p1", "c1", pre, admin, claims.Identity{}); res.Outcome != claimtel.Served {
		t.Fatalf("no copy: %+v", res)
	}

	// Redis says none: the roles alone, a repair signal, and the copy is
	// dropped so a later outage cannot serve the dropped facts from memory.
	r.Fail(nil)
	r.SetField(FactsKey("u3"), "p1", facts("admin"))
	p.Answer(ctx, "u3", "p1", "c1", pre, admin, claims.Identity{})
	r.HDel(FactsKey("u3"), "p1")
	if res = p.Answer(ctx, "u3", "p1", "c1", pre, admin, claims.Identity{}); res.Outcome != claimtel.Served || !res.Miss || fmt.Sprint(rolesOf(t, res)) != "[admin]" {
		t.Fatalf("a miss: %+v", res)
	}
	r.Fail(errors.New("down"))
	if res = p.Answer(ctx, "u3", "p1", "c1", pre, admin, claims.Identity{}); res.Outcome == claimtel.ServedLKG {
		t.Fatal("facts Redis said were gone came back from memory")
	}

	// Malformed facts are never served: the project's failure mode answers.
	r.Fail(nil)
	r.SetField(FactsKey("u4"), "p1", "{{")
	if res = p.Answer(ctx, "u4", "p1", "c1", pre, admin, claims.Identity{}); res.Outcome != claimtel.DegradedMalformed || len(res.Claims) != 1 || res.Claims[0].Key != "syndra_degraded" {
		t.Fatalf("malformed: %+v", res)
	}

	// 4. A project nobody published: fail closed, never a guessed shape.
	if res = p.Answer(ctx, "u1", "pUnknown", "c1", pre, admin, claims.Identity{}); res.Outcome != claimtel.DegradedShapeUnreadable || len(res.Claims) != 0 {
		t.Fatalf("unpublished: %+v", res)
	}
	// And before any snapshot at all.
	if res = New(r, nil).Answer(ctx, "u1", "p1", "c1", pre, admin, claims.Identity{}); res.Outcome != claimtel.DegradedShapeUnreadable || res.Claims == nil {
		t.Fatalf("no snapshot: %+v", res)
	}
}

// The bound is the project's: a project that says 1h serves no copy older.
func TestAnswer_TheBoundIsPerProject(t *testing.T) {
	ctx := context.Background()
	p, r, c := plane(t, Mode{Mode: FailClosed, LKGMaxAgeS: 3600})
	r.SetField(FactsKey("u1"), "p1", facts("admin"))
	p.Answer(ctx, "u1", "p1", "", pre, admin, claims.Identity{})
	r.Fail(errors.New("down"))
	c.add(59 * time.Minute)
	if res := p.Answer(ctx, "u1", "p1", "", pre, admin, claims.Identity{}); res.Outcome != claimtel.ServedLKG {
		t.Fatalf("59m: %+v", res)
	}
	c.add(2 * time.Minute)
	if res := p.Answer(ctx, "u1", "p1", "", pre, admin, claims.Identity{}); res.Outcome != claimtel.Served || fmt.Sprint(rolesOf(t, res)) != "[admin]" {
		t.Fatalf("61m: %+v", res)
	}
}

// A hung Redis costs one FactsTimeout, then the copy answers.
func TestAnswer_HungRedisIsBounded(t *testing.T) {
	ctx := context.Background()
	p, r, _ := plane(t, Mode{})
	r.SetField(FactsKey("u1"), "p1", facts("admin"))
	p.Answer(ctx, "u1", "p1", "", pre, admin, claims.Identity{})
	r.Hang()
	start := time.Now()
	res := p.Answer(ctx, "u1", "p1", "", pre, admin, claims.Identity{})
	took := time.Since(start)
	t.Logf("measured: hung Redis answered %s in %s", res.Outcome, took.Round(time.Millisecond))
	if res.Outcome != claimtel.ServedLKG || took > 2*FactsTimeout {
		t.Fatalf("%s in %s", res.Outcome, took)
	}
}

// The token's roles are the ones Zitadel listed (ADR-004, prod 4 Nov 2026):
// a role given in Zitadel directly is sent though Syndra's facts lack it, a
// role of Syndra's that Zitadel did not list (queued) is not, and one past its
// end date that Zitadel still lists is kept. The app's renames and
// leave-outs apply as ever; facts enrich, and only for the listed roles. With
// no facts, the shape is answered over the roles alone, the rows that need
// facts left out, and the miss still asks for a repair.
func TestAnswer_RolesAreZitadelsFactsEnrich(t *testing.T) {
	ctx := context.Background()
	r := claimservetest.New()
	shape := claims.Profile{Lands: claims.LandsBoth, IncludeNew: true,
		Roles: []claims.RoleMap{{Key: "admin", As: "boss", Included: true}, {Key: "hidden", Included: false}},
		Claims: []claims.Claim{
			{Path: []string{"roles"}, Kind: claims.KindRoles, Format: claims.FormatArray},
			{Path: []string{"ends"}, Kind: claims.KindRoleEnds},
			{Path: []string{"bundles"}, Kind: claims.KindBundles},
			{Path: []string{"name"}, Kind: claims.KindAttribute, Source: claims.AttrGivenName},
			{Path: []string{"svc"}, Kind: claims.KindAttribute, Source: claims.AttrIsServiceAccount},
			{Path: []string{"uid"}, Kind: claims.KindAttribute, Source: claims.AttrUserID},
			{Path: []string{"is_boss"}, Kind: claims.KindIfRole, Roles: []string{"admin"}},
		}}
	if _, err := Publish(ctx, r, map[string]Project{"p1": {Shapes: ProjectShapes{Default: shape}}}); err != nil {
		t.Fatal(err)
	}
	p := New(r, func() time.Time { return time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC) })
	_ = p.Poll(ctx)
	b, _ := json.Marshal(claims.Facts{UserID: "u1", ProjectID: "p1", Roles: []string{"admin", "queued"}, GivenName: "Charlotte", Bundles: []string{"Makers"},
		RoleEnds: map[string]string{"admin": "2026-12-01", "queued": "2026-12-31", "ended": "2026-11-01"}})
	r.SetField(FactsKey("u1"), "p1", string(b))
	listed := []string{"admin", "ended", "hidden", "zitadel-only"} // Request.Roles order

	res := p.Answer(ctx, "u1", "p1", "", pre, listed, claims.Identity{})
	got, _ := json.Marshal(res.Claims)
	want := `[{"key":"roles","value":["boss","ended","zitadel-only"]},{"key":"bundles","value":["Makers"]},{"key":"ends","value":{"boss":"2026-12-01","ended":"2026-11-01"}},` +
		`{"key":"is_boss","value":true},{"key":"name","value":"Charlotte"},{"key":"svc","value":false},{"key":"uid","value":"u1"}]`
	if res.Outcome != claimtel.Served || string(got) != want {
		t.Fatalf("with facts, %s:\n got %s\nwant %s", res.Outcome, got, want)
	}

	r.HDel(FactsKey("u1"), "p1")
	res = p.Answer(ctx, "u1", "p1", "", pre, listed, claims.Identity{})
	got, _ = json.Marshal(res.Claims)
	want = `[{"key":"roles","value":["boss","ended","zitadel-only"]},{"key":"is_boss","value":true},{"key":"uid","value":"u1"}]`
	if res.Outcome != claimtel.ServedRolesOnly || !res.Miss || string(got) != want {
		t.Fatalf("no facts, %s miss=%v:\n got %s\nwant %s", res.Outcome, res.Miss, got, want)
	}

	// Zitadel listing none is none, whatever Syndra's facts hold.
	r.SetField(FactsKey("u1"), "p1", string(b))
	if res = p.Answer(ctx, "u1", "p1", "", pre, nil, claims.Identity{}); fmt.Sprint(rolesOf(t, res)) != "[]" || res.Claims[3].Value != false { // is_boss, A to Z after the roles
		t.Fatalf("no roles listed: %+v", res)
	}
}

// The old per-project key answers while a person is still on it.
func TestAnswer_ReadsTheOldLayoutUntilMoved(t *testing.T) {
	p, r, _ := plane(t, Mode{})
	r.Set(LegacyFactsKey("u1", "p1"), facts("admin"))
	if res := p.Answer(context.Background(), "u1", "p1", "", pre, admin, claims.Identity{}); res.Outcome != claimtel.Served {
		t.Fatalf("old layout: %+v", res)
	}
}

// Shapes by client, the lands gate, and an unknown function still answered.
func TestAnswer_ShapeLandsAndFunction(t *testing.T) {
	ctx := context.Background()
	r := claimservetest.New()
	own := claims.FromFlat("own.roles", claims.FormatCSV, nil, nil)
	own.Lands = claims.LandsUserinfo
	_, _ = Publish(ctx, r, map[string]Project{"p1": {Shapes: ProjectShapes{
		Default: claims.Builtin(), ByClient: map[string]AppShape{"cOwn": {ApplicationID: "a1", Profile: own}},
	}}})
	p := New(r, nil)
	_ = p.Poll(ctx)
	r.SetField(FactsKey("u1"), "p1", facts("a", "b"))

	if res := p.Answer(ctx, "u1", "p1", "cOwn", pre, admin, claims.Identity{}); res.Outcome != claimtel.NotLanding || !res.ClientKnown || len(res.Claims) != 0 {
		t.Fatalf("not landing: %+v", res)
	}
	if res := p.Answer(ctx, "u1", "p1", "cOwn", claims.FunctionPreUserinfo, ab, claims.Identity{}); res.Outcome != claimtel.Served || res.Claims[0].Value != "a,b" {
		t.Fatalf("own shape: %+v", res)
	}
	if res := p.Answer(ctx, "u1", "p1", "cOther", "function/new", ab, claims.Identity{}); res.Outcome != claimtel.UnknownFunction || res.ClientKnown || len(res.Claims) != 1 {
		t.Fatalf("unknown function, default shape: %+v", res)
	}
	if cfg := p.Config(); cfg.ClientProject["cOwn"] != "p1" {
		t.Fatalf("client→project: %v", cfg.ClientProject)
	}
}

// Publish writes only what changed and moves the generation once per write;
// the plane reloads on the move, keeps its snapshot through a Redis outage,
// and reloads at least every minute even when the move was lost.
func TestPublishAndPoll(t *testing.T) {
	ctx := context.Background()
	r := claimservetest.New()
	c := &clock{t: time.Now()}
	p := New(r, c.now)
	one := map[string]Project{"p1": {Shapes: ProjectShapes{Default: claims.Builtin()}, Mode: minimalSafe()}}

	if n, err := Publish(ctx, r, one); n != 1 || err != nil {
		t.Fatalf("first publish: %d %v", n, err)
	}
	if n, _ := Publish(ctx, r, one); n != 0 || r.Count("INCR") != 1 {
		t.Fatalf("an unchanged publish wrote %d and moved gen %d times", n, r.Count("INCR"))
	}
	if err := p.Poll(ctx); err != nil || p.Config().Gen != 1 || len(p.Config().Projects) != 1 {
		t.Fatalf("load: %v %+v", err, p.Config())
	}
	if p.Poll(ctx); r.Count("SMEMBERS") != 1 {
		t.Fatal("a poll with gen unmoved reloaded")
	}

	// A Redis outage keeps the snapshot and says so.
	r.Fail(errors.New("down"))
	if err := p.Poll(ctx); err == nil || p.Config().Gen != 1 {
		t.Fatal("a failed poll replaced or dropped the snapshot")
	}
	if st := p.Status(); !st.Loaded || st.Failures != 1 || st.LastError == "" {
		t.Fatalf("status: %+v", st)
	}
	r.Fail(nil)

	// A write whose INCR never happened still lands within ReloadAtLeast.
	r.Set(ShapesKey("p1"), `{"default":{"lands":"both","claims":[]},"default_source":"x","by_client":{}}`)
	_ = p.Poll(ctx)
	if p.Config().Projects["p1"].Shapes.DefaultSource == "x" {
		t.Fatal("reloaded without a gen move before ReloadAtLeast")
	}
	c.add(ReloadAtLeast)
	_ = p.Poll(ctx)
	if p.Config().Projects["p1"].Shapes.DefaultSource != "x" {
		t.Fatal("a lost gen move was never picked up")
	}

	// One unreadable project is left out; the rest still load.
	_, _ = Publish(ctx, r, map[string]Project{"p2": {Shapes: ProjectShapes{Default: claims.Builtin()}}})
	r.Set(ShapesKey("p1"), "{{")
	_ = p.Poll(ctx)
	if _, ok := p.Config().Projects["p1"]; ok || len(p.Config().Projects) != 1 {
		t.Fatalf("projects after a corrupt entry: %v", p.Config().Projects)
	}
	// A failure mode that can't be read is fail closed, never a guess.
	r.Set(ModeKey("p2"), "{{")
	c.add(ReloadAtLeast)
	_ = p.Poll(ctx)
	if m := p.Config().Projects["p2"].Mode; m.Mode != "" || len(m.Claims()) != 0 {
		t.Fatalf("unreadable mode: %+v", m)
	}
}

// Reloads race calls safely (run under -race): a call sees one whole
// snapshot or the next, never a mix.
func TestReloadRace(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p, r, _ := plane(t, Mode{})
	r.SetField(FactsKey("u1"), "p1", facts("admin"))
	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				if res := p.Answer(ctx, "u1", "p1", "", pre, admin, claims.Identity{}); res.Outcome != claimtel.Served {
					t.Errorf("mid-reload answer: %+v", res)
					return
				}
				_ = p.Status()
			}
		}()
	}
	for i := 0; i < 200; i++ {
		name := fmt.Sprintf("roles%d", i)
		_, _ = Publish(ctx, r, map[string]Project{"p1": {Shapes: ProjectShapes{Default: claims.FromFlat(name, claims.FormatArray, nil, nil)}}})
		_ = p.Poll(ctx)
	}
	cancel()
	wg.Wait()
	if p.Config().Gen < 200 {
		t.Fatalf("gen %d after 200 publishes", p.Config().Gen)
	}
}

func TestLRU_Bounded(t *testing.T) {
	c := newLRU(3)
	now := time.Now()
	for _, k := range []string{"a", "b", "c"} {
		c.put(k, claims.Facts{UserID: k}, now)
	}
	c.get("a") // a is now the most recent
	c.put("d", claims.Facts{}, now)
	if _, _, ok := c.get("b"); ok || c.len() != 3 {
		t.Fatalf("least recent not evicted: len %d", c.len())
	}
	for _, k := range []string{"a", "c", "d"} {
		if _, _, ok := c.get(k); !ok {
			t.Fatalf("%s evicted", k)
		}
	}
}

func BenchmarkAnswer(b *testing.B) {
	r := claimservetest.New()
	_, _ = Publish(context.Background(), r, map[string]Project{"p1": {Shapes: ProjectShapes{Default: claims.ActionsV1()}}})
	p := New(r, nil)
	_ = p.Poll(context.Background())
	fb, _ := json.Marshal(claims.Facts{UserID: "u1", ProjectID: "p1", Roles: []string{"a", "b", "c", "d"}, Email: "u1@example.edu", GivenName: "U"})
	r.SetField(FactsKey("u1"), "p1", string(fb))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Answer(context.Background(), "u1", "p1", "c1", pre, admin, claims.Identity{})
	}
}

// The names v1's groupsClaim built from Zitadel's user reach the token from
// the payload, facts or not (4 Nov 2026): given_name, family_name, and name as
// first + " " + last; a machine user's as empty strings. The payload wins over
// the facts' copy; a field it lacks keeps the copy. With no facts, the token
// is served whole unless the shape asks for something only facts hold.
func TestAnswer_IdentityFromThePayload(t *testing.T) {
	ctx := context.Background()
	withTeam := claims.ActionsV1()
	withTeam.Claims = append(withTeam.Claims, claims.Claim{Path: []string{"team"}, Kind: claims.KindAttribute, Source: claims.AttrTeam})
	r := claimservetest.New()
	if _, err := Publish(ctx, r, map[string]Project{"p1": {Shapes: ProjectShapes{
		Default: claims.ActionsV1(), ByClient: map[string]AppShape{"cTeam": {ApplicationID: "aTeam", Profile: withTeam}},
	}}}); err != nil {
		t.Fatal(err)
	}
	p := New(r, nil)
	if err := p.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	charlotte := claims.Identity{Human: true, GivenName: "Charlotte", FamilyName: "Flair"}
	answer := func(client string, id claims.Identity) (string, Result) {
		res := p.Answer(ctx, "u1", "p1", client, pre, admin, id)
		b, _ := json.Marshal(res.Claims)
		return string(b), res
	}
	v1Token := func(given, family, name string) string {
		return fmt.Sprintf(`[{"key":"groups","value":["admin"]},{"key":"custom_claims","value":{"family_name":%q,"given_name":%q,"groups":["admin"],"name":%q}}]`, family, given, name)
	}

	// No facts: the names are the payload's, and the token is whole.
	if got, res := answer("", charlotte); got != v1Token("Charlotte", "Flair", "Charlotte Flair") || res.Outcome != claimtel.Served || !res.Miss {
		t.Fatalf("no facts, %s miss=%v: %s", res.Outcome, res.Miss, got)
	}
	// A machine user: v1 sent its three names as empty strings.
	if got, _ := answer("", claims.Identity{Machine: true, Username: "bot"}); got != v1Token("", "", "") {
		t.Fatalf("machine: %s", got)
	}
	// A shape asking for a fact: the names still, the team left out, counted.
	if got, res := answer("cTeam", charlotte); got != v1Token("Charlotte", "Flair", "Charlotte Flair") || res.Outcome != claimtel.ServedRolesOnly {
		t.Fatalf("no facts, a fact asked for, %s: %s", res.Outcome, got)
	}

	// Facts with a stale copy: the payload wins, a field it lacks keeps the
	// copy, and the name is never the display name.
	b, _ := json.Marshal(claims.Facts{UserID: "u1", ProjectID: "p1", GivenName: "Rhea", FamilyName: "Ripley", Name: "Mami", Team: "Fab"})
	r.SetField(FactsKey("u1"), "p1", string(b))
	if got, res := answer("", charlotte); got != v1Token("Charlotte", "Flair", "Charlotte Flair") || res.Outcome != claimtel.Served {
		t.Fatalf("payload over facts, %s: %s", res.Outcome, got)
	}
	if got, _ := answer("", claims.Identity{Human: true, GivenName: "Charlotte"}); got != v1Token("Charlotte", "Ripley", "Charlotte Ripley") {
		t.Fatalf("a field the payload lacks: %s", got)
	}
	if got, _ := answer("", claims.Identity{}); got != v1Token("Rhea", "Ripley", "Mami") {
		t.Fatalf("no user in the payload, the facts stand: %s", got)
	}
	if got, res := answer("cTeam", charlotte); got != v1Token("Charlotte", "Flair", "Charlotte Flair")[:len(v1Token("Charlotte", "Flair", "Charlotte Flair"))-1]+`,{"key":"team","value":"Fab"}]` || res.Outcome != claimtel.Served {
		t.Fatalf("with facts, the team, %s: %s", res.Outcome, got)
	}
}
