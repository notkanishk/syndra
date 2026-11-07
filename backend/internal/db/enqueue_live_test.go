package db

import (
	"context"
	"testing"

	"syndra/internal/models"
)

// The one outbox insert (architecture deepening, decision 8). Every access
// change reaches the outbox through one function, and these pin what each of
// the two paths it replaced wrote, through their public callers: a write a
// person named (EnqueueDirectGrantPropagation) and a cascade (a bundle given
// and taken back, a direct grant removed). Written against the two old
// functions first, so the merge is shown to change none of it.

type outboxStamps struct {
	op, source, ref, key, grantID string
	cascade                       *string
}

func stampsOf(t *testing.T, ctx context.Context, id string) outboxStamps {
	t.Helper()
	var s outboxStamps
	if err := PG.QueryRow(ctx, `
		SELECT op_type, source, COALESCE(source_ref, ''), idempotency_key::text, COALESCE(zitadel_grant_id, ''), cascade_id::text
		  FROM propagation_outbox WHERE id = $1`, id).Scan(&s.op, &s.source, &s.ref, &s.key, &s.grantID, &s.cascade); err != nil {
		t.Fatalf("read outbox row %s: %v", id, err)
	}
	return s
}

// auditCascade is the cascade id on the one audit row with this action for user.
func auditCascade(t *testing.T, ctx context.Context, user, action string) *string {
	t.Helper()
	var c *string
	if err := PG.QueryRow(ctx, `SELECT cascade_id::text FROM audit_logs WHERE target_zitadel_user_id = $1 AND action = $2`,
		user, action).Scan(&c); err != nil {
		t.Fatalf("read the %s audit row: %v", action, err)
	}
	return c
}

func enqueueSubject(t *testing.T, ctx context.Context) string {
	t.Helper()
	user := freshSubject(t, ctx)
	dropOutboxAfter(t, ctx, user)
	if _, err := PG.Exec(ctx, `INSERT INTO zitadel_grants_index (grant_id, user_id, project_id, role_keys) VALUES ($1,$2,'p1',ARRAY['member'])`,
		"g-"+user, user); err != nil {
		t.Fatal(err)
	}
	return user
}

// A named write: its own row carries the key handed back, no cascade id and
// no grant id it wasn't given; an identical add already waiting answers it;
// the rows the mapping rules owe on top of it are one cascade of their own.
func TestEnqueue_ANamedWrite(t *testing.T) {
	ctx := liveDB(t)
	user := enqueueSubject(t, ctx)

	orig := FollowRules
	t.Cleanup(func() { FollowRules = orig })
	FollowRules = func(_ context.Context, p EnqueueParams, write func() error) ([]EnqueueParams, error) {
		if err := write(); err != nil || p.OpType != "add" {
			return nil, err
		}
		return []EnqueueParams{{UserID: p.UserID, ProjectID: "p1", RoleKeys: []string{"follows"}, OpType: "add",
			Source: "rule", SourceRef: "rule-1", GrantedBy: p.GrantedBy, Reason: "Mapping rule cascade", PayloadJSON: "{}"}}, nil
	}

	give := EnqueueParams{UserID: user, ProjectID: "p1", RoleKeys: []string{"community"}, GrantedBy: "op",
		Reason: "term start", OpType: "add", PayloadJSON: `{}`}
	res, err := EnqueueDirectGrantPropagation(ctx, give)
	if err != nil {
		t.Fatal(err)
	}
	own := stampsOf(t, ctx, res.OutboxID)
	if own.key != res.IdempotencyKey || own.cascade != nil || own.source != "direct" || own.op != "add" {
		t.Fatalf("own row = %+v, want the key handed back (%s), no cascade id, source direct", own, res.IdempotencyKey)
	}
	if len(res.RuleOutboxIDs) != 1 {
		t.Fatalf("rule rows = %v, want one", res.RuleOutboxIDs)
	}
	rule := stampsOf(t, ctx, res.RuleOutboxIDs[0])
	if rule.cascade == nil || rule.key == own.key || rule.source != "rule" || rule.ref != "rule-1" {
		t.Fatalf("rule row = %+v, want its own key and a cascade id", rule)
	}
	var grantID string
	if err := PG.QueryRow(ctx, `SELECT id::text FROM direct_role_grants WHERE user_id = $1 AND zitadel_role_key = 'community'`, user).Scan(&grantID); err != nil {
		t.Fatalf("the ledger row: %v", err)
	}
	var resource string
	var cascade *string
	if err := PG.QueryRow(ctx, `SELECT resource_id, cascade_id::text FROM audit_logs WHERE target_zitadel_user_id = $1 AND action = 'direct_grant.upserted'`,
		user).Scan(&resource, &cascade); err != nil || resource != grantID || cascade != nil {
		t.Fatalf("audit names the grant (%s) with no cascade id, got %q %v (%v)", grantID, resource, cascade, err)
	}

	// The same add again is answered by the one already waiting.
	again, err := EnqueueDirectGrantPropagation(ctx, give)
	if err != nil {
		t.Fatal(err)
	}
	if again.OutboxID != res.OutboxID {
		t.Fatalf("an identical waiting add answers the write, got %s then %s", res.OutboxID, again.OutboxID)
	}

	// A named revoke goes with the grant id it was given, none here: the
	// grant index is read only for a cascade's revokes.
	gone, err := EnqueueDirectGrantPropagation(ctx, EnqueueParams{UserID: user, ProjectID: "p1", RoleKeys: []string{"member"},
		GrantedBy: "op", OpType: "revoke", PayloadJSON: `{}`})
	if err != nil {
		t.Fatal(err)
	}
	if s := stampsOf(t, ctx, gone.OutboxID); s.grantID != "" || s.cascade != nil {
		t.Fatalf("named revoke = %+v, want no grant id and no cascade id", s)
	}
}

// A cascade: one cascade id on every row and on its audit row (when Change
// history shows it), a key per row, no reuse of an identical waiting add, and
// a revoke's grant id from the grant index.
func TestEnqueue_ACascade(t *testing.T) {
	ctx := liveDB(t)
	user := enqueueSubject(t, ctx)
	bundle, err := CreateBundle(ctx, "Enqueue "+user, "", []models.BundleRole{{ProjectID: "p1", RoleKey: "member"}, {ProjectID: "p1", RoleKey: "trained"}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = PG.Exec(ctx, `DELETE FROM user_bundle_assignments WHERE bundle_id = $1`, bundle)
		_, _ = PG.Exec(ctx, `DELETE FROM bundles WHERE id = $1`, bundle)
	})
	v, err := LatestVersion(ctx, bundle)
	if err != nil {
		t.Fatal(err)
	}
	waiting := seedOutbox(t, ctx, "add", "pending", user, "p1", "direct", "", []string{"member"})

	write := func(role, op string) EnqueueParams {
		return EnqueueParams{UserID: user, ProjectID: "p1", RoleKeys: []string{role}, OpType: op, Source: "bundle", SourceRef: bundle,
			GrantedBy: "op", Reason: "Bundle membership cascade", PayloadJSON: "{}"}
	}
	ids, assigned, err := AssignBundleAndEnqueue(ctx, "op", "", user, bundle, v.ID, []EnqueueParams{write("member", "add"), write("trained", "add")})
	if err != nil || !assigned || len(ids) != 2 {
		t.Fatalf("assign: ids %v assigned %v err %v", ids, assigned, err)
	}
	a, b := stampsOf(t, ctx, ids[0]), stampsOf(t, ctx, ids[1])
	if a.cascade == nil || b.cascade == nil || *a.cascade != *b.cascade || a.key == b.key {
		t.Fatalf("rows = %+v / %+v, want one cascade id and a key each", a, b)
	}
	if c := auditCascade(t, ctx, user, "bundle.assigned"); c == nil || *c != *a.cascade {
		t.Fatalf("the audit row carries the cascade id %s, got %v", *a.cascade, c)
	}
	if ids[0] == waiting {
		t.Fatal("a cascade's add is its own row, not the waiting one")
	}

	// Sent, then taken back: the revoke is owed, and takes the grant id.
	if _, err := PG.Exec(ctx, `UPDATE propagation_outbox SET status = 'applied' WHERE id = $1`, ids[1]); err != nil {
		t.Fatal(err)
	}
	gone, err := RemoveBundleFromUserAndEnqueue(ctx, "op", "", user, bundle, []EnqueueParams{write("trained", "revoke")})
	if err != nil || len(gone) != 1 {
		t.Fatalf("remove: %v %v", gone, err)
	}
	if s := stampsOf(t, ctx, gone[0]); s.grantID != "g-"+user || s.cascade == nil || *s.cascade == *a.cascade {
		t.Fatalf("revoke = %+v, want grant id g-%s and a cascade id of its own", s, user)
	}

	// A direct grant's removal: its rows are one batch, but its audit row has
	// no cascade id, because Change history doesn't show direct rows.
	if _, err := EnqueueDirectGrantPropagation(ctx, EnqueueParams{UserID: user, ProjectID: "p1", RoleKeys: []string{"door"},
		GrantedBy: "op", OpType: "add", NoPropagation: true, PayloadJSON: "{}"}); err != nil {
		t.Fatal(err)
	}
	var grantID string
	if err := PG.QueryRow(ctx, `SELECT id::text FROM direct_role_grants WHERE user_id = $1 AND zitadel_role_key = 'door'`, user).Scan(&grantID); err != nil {
		t.Fatal(err)
	}
	if _, err := PG.Exec(ctx, `INSERT INTO propagation_outbox (op_type, user_id, project_id, role_keys, payload_json, idempotency_key, initiated_by, source, target, status)
		VALUES ('add', $1, 'p1', ARRAY['door'], '{}', gen_random_uuid(), 'op', 'direct', 'zitadel', 'applied')`, user); err != nil {
		t.Fatal(err)
	}
	removed, err := DeleteDirectGrantAndEnqueue(ctx, "op", user, grantID, []EnqueueParams{{UserID: user, ProjectID: "p1",
		RoleKeys: []string{"door"}, OpType: "revoke", Source: "direct", SourceRef: grantID, GrantedBy: "op", PayloadJSON: "{}"}})
	if err != nil || len(removed) != 1 {
		t.Fatalf("remove the grant: %v %v", removed, err)
	}
	if s := stampsOf(t, ctx, removed[0]); s.cascade == nil || s.grantID != "g-"+user {
		t.Fatalf("direct removal row = %+v, want a cascade id and the grant id", s)
	}
	if c := auditCascade(t, ctx, user, "direct_grant.removed"); c != nil {
		t.Fatalf("a direct removal's audit row has no cascade id, got %s", *c)
	}
}
