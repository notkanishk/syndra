package db

import (
	"context"
	"testing"
	"time"
)

// Access over time's read against the real schema, in one transaction that is
// rolled back: a live bundle from its assignment, an ended bundle through the
// version its assignment pinned, an ended grant dated by the request that gave
// it, an expiry with no start on record, nothing before the window and nothing
// of anyone else's, live rows first under the cap.
func TestAccessHistory_AgainstTheSchema(t *testing.T) {
	ctx := liveDB(t)
	tx, err := PG.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	ctx = context.WithValue(ctx, txKey, tx)

	suffix := time.Now().Format("150405.000000000")
	me, other := "live-history-"+suffix, "live-history-other-"+suffix
	now := time.Now().UTC().Truncate(time.Second)
	ago := func(days int) time.Time { return now.AddDate(0, 0, -days) }
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	bundle := func(name string) string {
		var id string
		if err := tx.QueryRow(ctx, `INSERT INTO bundles (name) VALUES ($1) RETURNING id`, name+suffix).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	version := func(bundleID string, n int, published time.Time, project, role string) string {
		var id string
		if err := tx.QueryRow(ctx, `INSERT INTO bundle_versions (bundle_id, version, published_at) VALUES ($1, $2, $3) RETURNING id`,
			bundleID, n, published).Scan(&id); err != nil {
			t.Fatal(err)
		}
		exec(`INSERT INTO bundle_version_roles (version_id, zitadel_project_id, zitadel_role_key) VALUES ($1, $2, $3)`, id, project, role)
		return id
	}
	audit := func(user, action, resource string, at time.Time) {
		exec(`INSERT INTO audit_logs (actor_zitadel_user_id, target_zitadel_user_id, action, resource_id, created_at) VALUES ('op', $1, $2, $3, $4)`,
			user, action, resource, at)
	}

	// Held today: a bundle assigned 100 days ago.
	welcome := bundle("live welcome ")
	v := version(welcome, 1, ago(200), "pA", "member")
	exec(`INSERT INTO user_bundle_assignments (user_id, bundle_id, version_id, assigned_at, source) VALUES ($1, $2, $3, $4, 'direct')`, me, welcome, v, ago(100))

	// Ended: assigned when v2 was the latest, so v2's role, never v1's.
	tools := bundle("live tools ")
	version(tools, 1, ago(300), "pB", "old")
	version(tools, 2, ago(150), "pB", "new")
	version(tools, 3, ago(60), "pB", "newer") // published after, never given to them
	audit(me, "bundle.assigned", tools, ago(120))
	audit(me, "bundle.unassigned", tools, ago(50))

	// Ended: given by an approved request, removed later.
	exec(`INSERT INTO access_requests (requester_user_id, zitadel_project_id, zitadel_role_key, justification, status, reviewer_user_id, resolved_at)
		VALUES ($1, 'p1', 'laser', 'j', 'approved', 'op', $2)`, me, ago(80))
	audit(me, "direct_grant.removed", "p1/laser", ago(30))
	// Ended by expiry, given by hand: no start on record.
	audit(me, "direct_grant.revoked_by_expiry", "p1/lathe", ago(20))
	// Before the window, and someone else's.
	audit(me, "direct_grant.removed", "p1/old", ago(400))
	audit(other, "direct_grant.removed", "p1/laser", ago(10))

	spans, err := AccessHistory(ctx, me, ago(365), 10)
	if err != nil {
		t.Fatal(err)
	}
	type key struct{ project, role string }
	got := map[key]AccessSpan{}
	for _, s := range spans {
		got[key{s.ProjectID, s.RoleKey}] = s
	}
	is := func(k key, from *time.Time, to *time.Time) {
		t.Helper()
		s, ok := got[k]
		if !ok {
			t.Fatalf("no span for %v in %+v", k, spans)
		}
		same := func(a, b *time.Time) bool { return a == nil && b == nil || a != nil && b != nil && a.Equal(*b) }
		if !same(s.From, from) || !same(s.To, to) {
			t.Fatalf("%v: %v to %v, want %v to %v", k, s.From, s.To, from, to)
		}
	}
	at := func(days int) *time.Time { d := ago(days); return &d }
	if len(spans) != 4 {
		t.Fatalf("spans %+v, want four", spans)
	}
	is(key{"pA", "member"}, at(100), nil)
	is(key{"pB", "new"}, at(120), at(50))
	is(key{"p1", "laser"}, at(80), at(30))
	is(key{"p1", "lathe"}, nil, at(20))
	if spans[0].To != nil || spans[1].To == nil || !spans[1].To.Equal(*at(20)) {
		t.Fatalf("the live bundle first, then the newest ending: %+v", spans)
	}

	capped, err := AccessHistory(ctx, me, ago(365), 2)
	if err != nil || len(capped) != 2 || capped[0].RoleKey != "member" || capped[1].RoleKey != "lathe" {
		t.Fatalf("under a cap of two: the live bundle and the newest ending, got %+v %v", capped, err)
	}
}
