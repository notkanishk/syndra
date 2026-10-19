package db

import (
	"context"
	"reflect"
	"sort"
	"testing"
	"time"

	"syndra/internal/models"
)

// Insights trends (I4) against the real schema. Every window here is in 2001, so
// only the rows a test inserts can fall inside it, whatever else the database
// holds; each test removes what it made.

var (
	trendFrom = time.Date(2001, 3, 1, 0, 0, 0, 0, time.UTC)
	trendTo   = time.Date(2001, 4, 1, 0, 0, 0, 0, time.UTC) // exclusive
)

func trendAt(day, hour int) time.Time { return time.Date(2001, 3, day, hour, 0, 0, 0, time.UTC) }

func liveSuffix() string { return time.Now().Format("150405.000000000") }

// insertRequest writes a request row as it would be after the given outcome,
// bypassing the handlers so created_at/resolved_at are exactly what the test says.
func insertRequest(t *testing.T, ctx context.Context, user, project, role, status, note string, created time.Time, resolved *time.Time) string {
	t.Helper()
	var reviewer *string
	if resolved != nil && status != "withdrawn" {
		r := "op"
		reviewer = &r
	}
	var id string
	err := PG.QueryRow(ctx, `
		INSERT INTO access_requests
			(requester_user_id, zitadel_project_id, zitadel_role_key, justification, status,
			 reviewer_user_id, review_note, created_at, resolved_at)
		VALUES ($1, $2, $3, 'because', $4, $5, NULLIF($6, ''), $7, $8) RETURNING id::text`,
		user, project, role, status, reviewer, note, created, resolved).Scan(&id)
	if err != nil {
		t.Fatalf("insert request: %v", err)
	}
	t.Cleanup(func() { _, _ = PG.Exec(ctx, `DELETE FROM access_requests WHERE id::text = $1`, id) })
	return id
}

func TestAuditDayCounts_AgainstTheSchema(t *testing.T) {
	ctx := liveDB(t)
	user := "live-trend-audit-" + liveSuffix()
	t.Cleanup(func() { _, _ = PG.Exec(ctx, `DELETE FROM audit_logs WHERE target_zitadel_user_id = $1`, user) })

	audit := func(action, resource string, at time.Time) {
		t.Helper()
		if _, err := PG.Exec(ctx, `
			INSERT INTO audit_logs (actor_zitadel_user_id, target_zitadel_user_id, action, resource_id, created_at)
			VALUES ('op', $1, $2, $3, $4)`, user, action, resource, at); err != nil {
			t.Fatal(err)
		}
	}
	audit("bundle.assigned", "b", trendAt(4, 20)) // 01:30 on the 5th in IST
	audit("bundle.assigned", "b", trendAt(4, 20))
	audit("bundle.assigned", "b", trendAt(4, 17)) // 22:30 on the 4th in IST
	audit("bundle.assigned", "b", trendFrom)      // the window's first instant is in
	audit("bundle.assigned", "b", trendTo)        // its end is not
	audit("bundle.assigned", "b", trendFrom.Add(-time.Second))
	audit("bundle.created", "b", trendAt(4, 20)) // not asked for

	// An approval writes its grant too: the grant that points back at the request
	// is the request's, the one nobody asked for is by hand.
	reqUser := user + "-r"
	reqID := insertRequest(t, ctx, reqUser, "pTrendAudit", "viaRequest", "approved", "", trendAt(2, 9), ptr(trendAt(2, 10)))
	var viaRequest, byHand string
	if err := PG.QueryRow(ctx, `
		INSERT INTO direct_role_grants (user_id, zitadel_project_id, zitadel_role_key, granted_by, source, source_ref)
		VALUES ($1, 'pTrendAudit', 'viaRequest', 'op', 'direct', $2) RETURNING id::text`, reqUser, reqID).Scan(&viaRequest); err != nil {
		t.Fatal(err)
	}
	if err := PG.QueryRow(ctx, `
		INSERT INTO direct_role_grants (user_id, zitadel_project_id, zitadel_role_key, granted_by, source)
		VALUES ($1, 'pTrendAudit', 'byHand', 'op', 'direct') RETURNING id::text`, reqUser).Scan(&byHand); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = PG.Exec(ctx, `DELETE FROM direct_role_grants WHERE user_id = $1`, reqUser) })
	audit("direct_grant.upserted", viaRequest, trendAt(10, 6))
	audit("direct_grant.upserted", byHand, trendAt(10, 6))
	audit("direct_grant.upserted", "grant-since-deleted", trendAt(10, 6)) // nothing to point back through: by hand

	actions := []string{"bundle.assigned", "direct_grant.upserted"}
	count := func(offset int) map[string]int {
		t.Helper()
		rows, err := AuditDayCounts(ctx, trendFrom, trendTo, actions, offset)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]int{}
		for _, r := range rows {
			out[r.Day.Format("2006-01-02")+" "+r.Action] += r.N
		}
		return out
	}

	if got, want := count(0), map[string]int{
		"2001-03-01 bundle.assigned": 1, "2001-03-04 bundle.assigned": 3, "2001-03-10 direct_grant.upserted": 2,
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("UTC days = %v, want %v", got, want)
	}
	// The same rows on the operator's calendar: the offset moves them across midnight.
	if got, want := count(5*3600+1800), map[string]int{
		"2001-03-01 bundle.assigned": 1, "2001-03-04 bundle.assigned": 1, "2001-03-05 bundle.assigned": 2,
		"2001-03-10 direct_grant.upserted": 2,
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("IST days = %v, want %v", got, want)
	}
}

func ptr[T any](v T) *T { return &v }

func TestRequestsInWindow_AgainstTheSchema(t *testing.T) {
	ctx := liveDB(t)
	u := "live-trend-req-" + liveSuffix()
	p := "pTrendReq" + liveSuffix()
	insertRequest(t, ctx, u, p, "a", "approved", "ok", trendAt(3, 9), ptr(trendAt(3, 11)))      // 2h
	insertRequest(t, ctx, u, p, "b", "rejected", "Not now", trendAt(3, 9), ptr(trendAt(3, 13))) // 4h
	insertRequest(t, ctx, u, p, "c", "rejected", "", trendAt(4, 9), ptr(trendAt(5, 9)))         // 24h, no note
	insertRequest(t, ctx, u, p, "d", "withdrawn", "", trendAt(6, 9), ptr(trendAt(6, 10)))
	insertRequest(t, ctx, u, p, "e", "pending", "", trendAt(7, 9), nil)
	insertRequest(t, ctx, u, p, "f", "fulfilled", "given another way", trendAt(8, 9), ptr(trendAt(8, 10)))
	insertRequest(t, ctx, u, p, "g", "approved", "", trendTo, ptr(trendTo.Add(time.Hour))) // created at the exclusive end

	got, err := RequestsInWindow(ctx, trendFrom, trendTo)
	if err != nil {
		t.Fatal(err)
	}
	sort.Float64s(got.DecideSeconds)
	sort.Strings(got.DeclineNotes)
	want := RequestTrendRows{
		CameIn: 6, Approved: 1, Declined: 2, Withdrawn: 1,
		DecideSeconds: []float64{7200, 14400, 86400}, // approved + declined only; a withdrawal is not a decision
		DeclineNotes:  []string{"", "Not now"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}

	// An empty window is zeros and nil lists, not an error.
	empty, err := RequestsInWindow(ctx, trendFrom.AddDate(-10, 0, 0), trendFrom.AddDate(-10, 0, 1))
	if err != nil || empty.CameIn != 0 || empty.DecideSeconds != nil || empty.DeclineNotes != nil {
		t.Fatalf("empty window = %+v %v", empty, err)
	}
}

func TestOutsideBundleRoles_AgainstTheSchema(t *testing.T) {
	ctx := liveDB(t)
	sfx := liveSuffix()
	proj := "pTrendOut" + sfx
	bundleID, err := CreateBundle(ctx, "trend outside "+sfx, "", "auto", []models.BundleRole{
		{ProjectID: proj, RoleKey: "given"}, {ProjectID: proj, RoleKey: "dropped"},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = PG.Exec(ctx, `DELETE FROM bundles WHERE id = $1`, bundleID) })
	// Version 2 stops giving "dropped": only the LATEST version counts.
	if _, _, err := PublishVersionAndEnqueue(ctx, "live", bundleID, "drop one",
		[]models.BundleRole{{ProjectID: proj, RoleKey: "given"}}, nil, nil); err != nil {
		t.Fatal(err)
	}

	for i, role := range []string{"given", "given", "given", "dropped", "dropped", "wanted", "wanted", "wanted", "rare"} {
		insertRequest(t, ctx, "live-trend-out-"+sfx+string(rune('a'+i)), proj, role, "pending", "", trendAt(3, 9), nil)
	}
	// Created outside the window: not counted.
	insertRequest(t, ctx, "live-trend-out-"+sfx+"z", proj, "wanted", "pending", "", trendTo, nil)

	got, err := OutsideBundleRoles(ctx, trendFrom, trendTo, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := []models.TrendOutsideRole{{ProjectID: proj, RoleKey: "wanted", N: 3}, {ProjectID: proj, RoleKey: "dropped", N: 2}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v (given is covered; dropped is not; limit 2 cuts rare)", got, want)
	}
}

func TestSendSeconds_AgainstTheSchema(t *testing.T) {
	ctx := liveDB(t)
	sfx := liveSuffix()
	u := "live-trend-send-" + sfx
	resolved := trendAt(10, 12)
	approved := insertRequest(t, ctx, u, "pTrendSend", "a", "approved", "", trendAt(10, 9), &resolved)
	rejected := insertRequest(t, ctx, u, "pTrendSend", "b", "rejected", "", trendAt(10, 9), &resolved)
	late := trendTo.Add(time.Hour)
	outside := insertRequest(t, ctx, u, "pTrendSend", "c", "approved", "", trendTo, &late)

	send := func(ref, status string, completed *time.Time, target string) {
		t.Helper()
		if _, err := PG.Exec(ctx, `
			INSERT INTO propagation_outbox
				(op_type, user_id, project_id, role_keys, payload_json, idempotency_key, initiated_by,
				 source, source_ref, target, status, completed_at)
			VALUES ('add', $1, 'pTrendSend', ARRAY['a'], '{}', uuid_generate_v4(), 'op',
			        'direct', $2, $3, $4, $5)`, u, ref, target, status, completed); err != nil {
			t.Fatal(err)
		}
	}
	sent := resolved.Add(90 * time.Second)
	send(approved, "applied", &sent, "zitadel")                     // counted: 90s
	send(approved, "pending", nil, "zitadel")                       // not sent yet
	send(approved, "failed", &sent, "zitadel")                      // never arrived
	send(rejected, "applied", &sent, "zitadel")                     // not an approval
	send(outside, "applied", ptr(late.Add(time.Minute)), "zitadel") // approved outside the window
	send("not-a-request", "applied", &sent, "zitadel")              // another kind of source
	early := resolved.Add(-time.Minute)
	send(approved, "applied", &early, "zitadel") // clock skew never reads as a negative wait

	got, err := SendSeconds(ctx, trendFrom, trendTo)
	if err != nil {
		t.Fatal(err)
	}
	sort.Float64s(got)
	if want := []float64{0, 90}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestDriftInWindow_AgainstTheSchema(t *testing.T) {
	ctx := liveDB(t)
	sfx := liveSuffix()
	user := func(n string) string { return "live-trend-drift-" + n + "-" + sfx }
	t.Cleanup(func() {
		_, _ = PG.Exec(ctx, `DELETE FROM drift_items WHERE user_id LIKE $1`, "live-trend-drift-%-"+sfx)
	})
	add := func(n string, detected time.Time, resolved *time.Time) {
		t.Helper()
		status := "pending_triage"
		if resolved != nil {
			status = "resolved"
		}
		if _, err := PG.Exec(ctx, `
			INSERT INTO drift_items (target, user_id, project_id, role_keys, detection_source, drift_type, status, detected_at, resolved_at, resolved_by)
			VALUES ('zitadel', $1, 'pTrendDrift', ARRAY['r'], 'reconciliation_sweep', $2, $3, $4, $5, CASE WHEN $5::timestamptz IS NULL THEN NULL ELSE 'op' END)`,
			user(n), DriftTargetOnly, status, detected, resolved); err != nil {
			t.Fatal(err)
		}
	}
	add("found-and-cleared", trendAt(5, 0), ptr(trendAt(5, 3))) // 3h
	add("found-open", trendAt(6, 0), nil)
	add("older-cleared-now", trendFrom.AddDate(0, -1, 0), ptr(trendAt(9, 0))) // found before, cleared inside
	add("later", trendTo, ptr(trendTo.Add(time.Hour)))                        // both after the window

	got, err := DriftInWindow(ctx, trendFrom, trendTo)
	if err != nil {
		t.Fatal(err)
	}
	if got.Found != 2 || got.Cleared != 2 || len(got.ClearSeconds) != 2 {
		t.Fatalf("got %+v: found = detected in the window (2), cleared = resolved in it (2)", got)
	}
	sort.Float64s(got.ClearSeconds)
	older := trendAt(9, 0).Sub(trendFrom.AddDate(0, -1, 0)).Seconds()
	if got.ClearSeconds[0] != 3*3600 || got.ClearSeconds[1] != older {
		t.Fatalf("clear seconds = %v, want [10800 %v]", got.ClearSeconds, older)
	}
}

func TestGrantsEndingSoon_AgainstTheSchema(t *testing.T) {
	ctx := liveDB(t)
	sfx := liveSuffix()
	now := time.Date(2001, 6, 1, 12, 0, 0, 0, time.UTC)
	pA, pB := "pTrendEndA"+sfx, "pTrendEndB"+sfx
	t.Cleanup(func() {
		_, _ = PG.Exec(ctx, `DELETE FROM direct_role_grants WHERE zitadel_project_id IN ($1, $2)`, pA, pB)
	})
	days := func(n int) *time.Time { return ptr(now.AddDate(0, 0, n)) }
	grant := func(user, project, role string, expires *time.Time) {
		t.Helper()
		// Made a year before now: the schema refuses an end before the grant was made.
		if _, err := PG.Exec(ctx, `
			INSERT INTO direct_role_grants (user_id, zitadel_project_id, zitadel_role_key, granted_by, expires_at, created_at)
			VALUES ($1, $2, $3, 'op', $4, $5)`, user+sfx, project, role, expires, now.AddDate(-1, 0, 0)); err != nil {
			t.Fatal(err)
		}
	}
	grant("u1", pA, "r1", days(10)) // u1 holds two ending grants in A: one person, not two
	grant("u1", pA, "r2", days(50))
	grant("u2", pA, "r1", days(50))
	grant("u3", pA, "r1", days(90)) // the 90th day is in
	grant("u4", pA, "r1", days(91))
	grant("u5", pA, "r1", days(-1)) // already ended
	grant("u6", pA, "r1", nil)      // never ends
	grant("u7", pB, "r1", days(30)) // the 30th day is in every bucket

	rows, err := GrantsEndingSoon(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]EndingSoonRow{}
	for _, r := range rows {
		got[r.ProjectID] = r
	}
	want := map[string]EndingSoonRow{
		pA: {ProjectID: pA, D30: 1, D60: 2, D90: 3},
		pB: {ProjectID: pB, D30: 1, D60: 1, D90: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}
