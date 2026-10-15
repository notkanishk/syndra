package db

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"syndra/internal/models"
)

// Found on production, 15 Oct 2026: one person, one role, five waiting copies
// of the same request; and approving any of them would have cut access they
// held for good down to 64 days. Each guard here is the database's own.

func freshSubject(t *testing.T, ctx context.Context) string {
	t.Helper()
	user := "live-req-" + time.Now().Format("150405.000000000")
	t.Cleanup(func() {
		_, _ = PG.Exec(ctx, `DELETE FROM access_requests WHERE requester_user_id = $1`, user)
		_, _ = PG.Exec(ctx, `DELETE FROM direct_role_grants WHERE user_id = $1`, user)
		_, _ = PG.Exec(ctx, `DELETE FROM zitadel_grants_index WHERE user_id = $1`, user)
		_, _ = PG.Exec(ctx, `DELETE FROM audit_logs WHERE target_zitadel_user_id = $1`, user)
	})
	return user
}

func TestOnlyOneRequestWaitsPerPersonAndRole(t *testing.T) {
	ctx := liveDB(t)
	user := freshSubject(t, ctx)

	first, err := CreateAccessRequest(ctx, user, "pLaser", "trained", "need it", nil)
	if err != nil {
		t.Fatal(err)
	}
	again, err := CreateAccessRequest(ctx, user, "pLaser", "trained", "need it, again", nil)
	if !errors.Is(err, ErrAlreadyAsked) || again != first {
		t.Fatalf("a second identical ask must meet the first: id %q err %v (first %q)", again, err, first)
	}
	var n int
	if err := PG.QueryRow(ctx, `SELECT count(*) FROM access_requests WHERE requester_user_id = $1`, user).Scan(&n); err != nil || n != 1 {
		t.Fatalf("one row, got %d (%v)", n, err)
	}
	// Another role, or the same one once the first is decided, is a new ask.
	if _, err := CreateAccessRequest(ctx, user, "pLaser", "door", "need it", nil); err != nil {
		t.Fatalf("another role is another request: %v", err)
	}
	if _, err := PG.Exec(ctx, `UPDATE access_requests SET status='rejected', reviewer_user_id='op', resolved_at=now() WHERE id=$1`, first); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateAccessRequest(ctx, user, "pLaser", "trained", "asking again", nil); err != nil {
		t.Fatalf("after a decision they may ask again: %v", err)
	}
}

func TestWhatARequesterHoldsIsReadFromGrantsAndZitadel(t *testing.T) {
	ctx := liveDB(t)
	user := freshSubject(t, ctx)
	kind := func(role string) models.RequestHolding {
		h, err := RequestHolding(ctx, user, "pLaser", role)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	if k := kind("trained").Kind; k != models.HoldingNone {
		t.Fatalf("nothing held, got %s", k)
	}
	until := time.Now().Add(48 * time.Hour).UTC()
	if _, err := PG.Exec(ctx, `INSERT INTO direct_role_grants (user_id, zitadel_project_id, zitadel_role_key, granted_by, expires_at) VALUES ($1,'pLaser','trained','op',$2)`, user, until); err != nil {
		t.Fatal(err)
	}
	if h := kind("trained"); h.Kind != models.HoldingUntil || h.Until == nil {
		t.Fatalf("an ending grant is held until it ends, got %+v", h)
	}
	if _, err := PG.Exec(ctx, `INSERT INTO direct_role_grants (user_id, zitadel_project_id, zitadel_role_key, granted_by) VALUES ($1,'pLaser','door','op')`, user); err != nil {
		t.Fatal(err)
	}
	if k := kind("door").Kind; k != models.HoldingForGood {
		t.Fatalf("a grant with no end is held for good, got %s", k)
	}
	// Held in Zitadel by a bundle or a rule, with no grant from Syndra.
	if _, err := PG.Exec(ctx, `INSERT INTO zitadel_grants_index (grant_id, user_id, project_id, role_keys) VALUES ($1,$2,'pLaser',ARRAY['staff'])`, "g-"+user, user); err != nil {
		t.Fatal(err)
	}
	if k := kind("staff").Kind; k != models.HoldingForGood {
		t.Fatalf("observed in Zitadel with no ending grant is held for good, got %s", k)
	}
}

// Only an extension changes an existing grant's end; every other write keeps
// it (and the handlers refuse giving a role someone already has).
func TestOnlyAnExtensionChangesAnEnd(t *testing.T) {
	ctx := liveDB(t)
	user := freshSubject(t, ctx)
	soon := time.Now().Add(10 * 24 * time.Hour).UTC().Truncate(time.Second)
	later := time.Now().Add(64 * 24 * time.Hour).UTC().Truncate(time.Second)
	if _, err := PG.Exec(ctx, `INSERT INTO direct_role_grants (user_id, zitadel_project_id, zitadel_role_key, granted_by, expires_at) VALUES ($1,'pLaser','trained','op',$2)`, user, soon); err != nil {
		t.Fatal(err)
	}
	write := func(rule EndRule) *time.Time {
		t.Helper()
		if _, err := EnqueueDirectGrantPropagation(ctx, EnqueueParams{
			UserID: user, ProjectID: "pLaser", RoleKeys: []string{"trained"}, GrantedBy: "op",
			Reason: "r", ExpiresAt: &later, OpType: "add", PayloadJSON: `{}`, EndRule: rule,
		}); err != nil {
			t.Fatal(err)
		}
		var got *time.Time
		if err := PG.QueryRow(ctx, `SELECT expires_at FROM direct_role_grants WHERE user_id=$1`, user).Scan(&got); err != nil {
			t.Fatal(err)
		}
		return got
	}
	if got := write(EndKept); got == nil || !got.Equal(soon) {
		t.Errorf("a write that isn't an extension keeps the end %v, got %v", soon, got)
	}
	if got := write(EndSet); got == nil || !got.Equal(later) {
		t.Errorf("an extension sets the end %v, got %v", later, got)
	}
}

// The migration closes copies already waiting, keeping the earliest, before
// the index that stops new ones. Run against copies made with the index off.
func TestTheMigrationClosesWaitingCopiesAndKeepsTheFirst(t *testing.T) {
	ctx := liveDB(t)
	user := freshSubject(t, ctx)
	up, err := os.ReadFile("../../db/migrations/000054_one_waiting_request.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PG.Exec(ctx, `DROP INDEX IF EXISTS idx_access_requests_one_waiting`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = PG.Exec(ctx, string(up)) })
	for i := 0; i < 3; i++ {
		if _, err := PG.Exec(ctx, `INSERT INTO access_requests (requester_user_id, zitadel_project_id, zitadel_role_key, justification, created_at)
			VALUES ($1,'pLaser','trained','copy', now() + make_interval(secs => $2))`, user, i); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := PG.Exec(ctx, string(up)); err != nil {
		t.Fatalf("migration: %v", err)
	}
	var waiting, closed, audited int
	_ = PG.QueryRow(ctx, `SELECT count(*) FILTER (WHERE status='pending'), count(*) FILTER (WHERE status='rejected' AND reviewer_user_id='system')
		FROM access_requests WHERE requester_user_id=$1`, user).Scan(&waiting, &closed)
	_ = PG.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE target_zitadel_user_id=$1 AND action='access_request.closed_as_copy'`, user).Scan(&audited)
	if waiting != 1 || closed != 2 || audited != 2 {
		t.Fatalf("want 1 waiting, 2 closed by system, 2 audited; got %d %d %d", waiting, closed, audited)
	}
	var kept string
	_ = PG.QueryRow(ctx, `SELECT justification FROM access_requests WHERE requester_user_id=$1 AND status='pending'`, user).Scan(&kept)
	var earliest time.Time
	_ = PG.QueryRow(ctx, `SELECT min(created_at) FROM access_requests WHERE requester_user_id=$1`, user).Scan(&earliest)
	var keptAt time.Time
	_ = PG.QueryRow(ctx, `SELECT created_at FROM access_requests WHERE requester_user_id=$1 AND status='pending'`, user).Scan(&keptAt)
	if !keptAt.Equal(earliest) {
		t.Fatalf("the earliest copy is the one kept: kept %v, earliest %v", keptAt, earliest)
	}
}

// A write that says nothing about an end (a bundle cascade, adopting drift,
// assigning on the Zitadel page) keeps the end a grant already has. It used to
// write "no end", turning access that ended into access that never does.
func TestAWriteThatDecidesNoEndKeepsTheOneThere(t *testing.T) {
	ctx := liveDB(t)
	user := freshSubject(t, ctx)
	soon := time.Now().Add(10 * 24 * time.Hour).UTC().Truncate(time.Second)
	if _, err := PG.Exec(ctx, `INSERT INTO direct_role_grants (user_id, zitadel_project_id, zitadel_role_key, granted_by, expires_at) VALUES ($1,'pLaser','trained','op',$2)`, user, soon); err != nil {
		t.Fatal(err)
	}
	if _, err := EnqueueDirectGrantPropagation(ctx, EnqueueParams{
		UserID: user, ProjectID: "pLaser", RoleKeys: []string{"trained"}, GrantedBy: "system",
		Reason: "bundle", Source: "bundle", OpType: "add", PayloadJSON: `{}`,
	}); err != nil {
		t.Fatal(err)
	}
	var got *time.Time
	if err := PG.QueryRow(ctx, `SELECT expires_at FROM direct_role_grants WHERE user_id=$1`, user).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got == nil || !got.Equal(soon) {
		t.Fatalf("the end must stay %v, got %v", soon, got)
	}
}

// One hold of a kind in force: a second identical one made lifting the first
// look done while the person stayed held, and revoking twice rotated twice.
// A person's own value replaces the last, so two never hide behind each other.
func TestOneAllowanceOfAKindIsInForce(t *testing.T) {
	ctx := liveDB(t)
	user := freshSubject(t, ctx)
	t.Cleanup(func() { _, _ = PG.Exec(ctx, `DELETE FROM allowances WHERE subject_id = $1`, user) })
	if _, err := PG.Exec(ctx, `INSERT INTO targets (target) VALUES ('truenas') ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	review := time.Now().Add(30 * 24 * time.Hour)
	hold := Allowance{SubjectID: user, Target: "truenas", Field: "enabled", Value: "true",
		Direction: AllowanceDeny, ActorID: "op", Reason: "paused", ReviewDate: &review}

	first, err := CreateAllowance(ctx, hold)
	if err != nil {
		t.Fatal(err)
	}
	again, err := CreateAllowance(ctx, hold)
	if !errors.Is(err, ErrAllowanceAlreadyInForce) || again.ID != first.ID {
		t.Fatalf("a second identical hold must meet the first: %v %q", err, again.ID)
	}

	limit := Allowance{SubjectID: user, Target: "truenas", Field: "home_quota", Value: "53687091200",
		Direction: AllowanceSet, ActorID: "op", Reason: "thesis", ReviewDate: &review}
	if _, err := CreateAllowance(ctx, limit); err != nil {
		t.Fatal(err)
	}
	limit.Value = "107374182400"
	newer, err := CreateAllowance(ctx, limit)
	if err != nil {
		t.Fatal(err)
	}
	var inForce int
	var kept string
	if err := PG.QueryRow(ctx, `SELECT count(*), max(id::text) FROM allowances
		WHERE subject_id = $1 AND field = 'home_quota' AND lifted_at IS NULL`, user).Scan(&inForce, &kept); err != nil {
		t.Fatal(err)
	}
	if inForce != 1 || kept != newer.ID {
		t.Fatalf("one own limit in force, the newest: %d in force, kept %s, newest %s", inForce, kept, newer.ID)
	}
}


// A waiting request for a role the person already has is closed by the
// migration, attributed to system and audited.
func TestTheMigrationClosesRequestsForRolesAlreadyHeld(t *testing.T) {
	ctx := liveDB(t)
	user := freshSubject(t, ctx)
	up, err := os.ReadFile("../../db/migrations/000054_one_waiting_request.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PG.Exec(ctx, `INSERT INTO direct_role_grants (user_id, zitadel_project_id, zitadel_role_key, granted_by) VALUES ($1,'pLaser','trained','op')`, user); err != nil {
		t.Fatal(err)
	}
	var heldID, otherID string
	if err := PG.QueryRow(ctx, `INSERT INTO access_requests (requester_user_id, zitadel_project_id, zitadel_role_key, justification) VALUES ($1,'pLaser','trained','again') RETURNING id`, user).Scan(&heldID); err != nil {
		t.Fatal(err)
	}
	if err := PG.QueryRow(ctx, `INSERT INTO access_requests (requester_user_id, zitadel_project_id, zitadel_role_key, justification) VALUES ($1,'pLaser','door','new') RETURNING id`, user).Scan(&otherID); err != nil {
		t.Fatal(err)
	}
	if _, err := PG.Exec(ctx, string(up)); err != nil {
		t.Fatalf("migration: %v", err)
	}
	var heldStatus, otherStatus string
	_ = PG.QueryRow(ctx, `SELECT status FROM access_requests WHERE id=$1`, heldID).Scan(&heldStatus)
	_ = PG.QueryRow(ctx, `SELECT status FROM access_requests WHERE id=$1`, otherID).Scan(&otherStatus)
	if heldStatus != "rejected" || otherStatus != "pending" {
		t.Fatalf("held one closed, the other left: %s / %s", heldStatus, otherStatus)
	}
	var audited int
	_ = PG.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE resource_id=$1 AND action='access_request.closed_as_held'`, heldID).Scan(&audited)
	if audited != 1 {
		t.Fatalf("the closure is audited once, got %d", audited)
	}
}

func requestStatus(t *testing.T, ctx context.Context, id string) (status, by, note string) {
	t.Helper()
	if err := PG.QueryRow(ctx, `SELECT status, COALESCE(reviewer_user_id,''), COALESCE(review_note,'') FROM access_requests WHERE id=$1`, id).Scan(&status, &by, &note); err != nil {
		t.Fatal(err)
	}
	return
}

func dropOutboxAfter(t *testing.T, ctx context.Context, user string) {
	t.Cleanup(func() { _, _ = PG.Exec(ctx, `DELETE FROM propagation_outbox WHERE user_id = $1`, user) })
}

// Given directly while a request for it waited (15 Oct): the request closes
// as fulfilled, by the person who gave it, in the grant's own transaction.
func TestAGrantClosesTheRequestItAnswers(t *testing.T) {
	ctx := liveDB(t)
	user := freshSubject(t, ctx)
	dropOutboxAfter(t, ctx, user)
	asked, err := CreateAccessRequest(ctx, user, "pLaser", "trained", "need it", nil)
	if err != nil {
		t.Fatal(err)
	}
	other, err := CreateAccessRequest(ctx, user, "pLaser", "door", "need it", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := EnqueueDirectGrantPropagation(ctx, EnqueueParams{
		UserID: user, ProjectID: "pLaser", RoleKeys: []string{"trained"}, GrantedBy: "op",
		Reason: "r", OpType: "add", PayloadJSON: `{}`,
	}); err != nil {
		t.Fatal(err)
	}
	if status, by, note := requestStatus(t, ctx, asked); status != "fulfilled" || by != "op" || note == "" {
		t.Fatalf("the answered request closes as fulfilled by the giver, got %s by %q (%q)", status, by, note)
	}
	if status, _, _ := requestStatus(t, ctx, other); status != "pending" {
		t.Fatalf("a request for another role is left waiting, got %s", status)
	}
	var audited int
	_ = PG.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE resource_id=$1 AND action='access_request.closed_as_given'`, asked).Scan(&audited)
	if audited != 1 {
		t.Fatalf("the closure is audited once, got %d", audited)
	}
	// Closed, so asking again once it ends is a new request, not "already asked".
	if _, err := PG.Exec(ctx, `DELETE FROM direct_role_grants WHERE user_id=$1`, user); err != nil {
		t.Fatal(err)
	}
	if _, err := PG.Exec(ctx, `DELETE FROM propagation_outbox WHERE user_id=$1`, user); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateAccessRequest(ctx, user, "pLaser", "trained", "again", nil); err != nil {
		t.Fatalf("after it was given and taken away they may ask again: %v", err)
	}
}

// The same through a bundle, which writes no ledger row: the cascade path closes it.
func TestABundleClosesTheRequestItAnswers(t *testing.T) {
	ctx := liveDB(t)
	user := freshSubject(t, ctx)
	dropOutboxAfter(t, ctx, user)
	asked, err := CreateAccessRequest(ctx, user, "pLaser", "trained", "need it", nil)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := PG.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := enqueueCascadeRows(ctx, tx, nil, []EnqueueParams{{
		UserID: user, ProjectID: "pLaser", RoleKeys: []string{"trained"}, GrantedBy: "op",
		OpType: "add", PayloadJSON: `{}`, Source: "bundle", SourceRef: "b1",
	}}); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if status, _, note := requestStatus(t, ctx, asked); status != "fulfilled" || note != givenNote("bundle") {
		t.Fatalf("closed as given through a bundle, got %s (%q)", status, note)
	}
}

// Given but not sent is held and says so; a role whose removal is queued is
// not held, even while Zitadel still lists it.
func TestHoldingReadsWhatIsQueued(t *testing.T) {
	ctx := liveDB(t)
	user := freshSubject(t, ctx)
	dropOutboxAfter(t, ctx, user)
	read := func(role string) models.RequestHolding {
		t.Helper()
		h, err := RequestHolding(ctx, user, "pLaser", role)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	seedOutbox(t, ctx, "add", "pending", user, "pLaser", "bundle", "b1", []string{"trained"})
	if h := read("trained"); h.Kind != models.HoldingForGood || !h.Waiting {
		t.Fatalf("a queued bundle add is held and waiting, got %+v", h)
	}
	if _, err := PG.Exec(ctx, `INSERT INTO zitadel_grants_index (grant_id, user_id, project_id, role_keys) VALUES ($1,$2,'pLaser',ARRAY['door'])`, "g-"+user, user); err != nil {
		t.Fatal(err)
	}
	if h := read("door"); h.Kind != models.HoldingForGood || h.Waiting {
		t.Fatalf("observed and nothing queued is held, not waiting, got %+v", h)
	}
	seedOutbox(t, ctx, "revoke", "pending", user, "pLaser", "direct", "", []string{"door"})
	if h := read("door"); h.Kind != models.HoldingNone {
		t.Fatalf("a role whose removal is queued is not held, got %+v", h)
	}
}

// 000055 turns 000054's "declined, already held" closures into fulfilled ones.
func TestTheMigrationCallsAlreadyHeldClosuresFulfilled(t *testing.T) {
	ctx := liveDB(t)
	user := freshSubject(t, ctx)
	up, err := os.ReadFile("../../db/migrations/000055_request_fulfilled.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	var held, declined string
	if err := PG.QueryRow(ctx, `INSERT INTO access_requests (requester_user_id, zitadel_project_id, zitadel_role_key, justification, status, reviewer_user_id, review_note, resolved_at) VALUES ($1,'pLaser','trained','x','rejected','system','They already have this, so there was nothing to approve.',now()) RETURNING id`, user).Scan(&held); err != nil {
		t.Fatal(err)
	}
	if err := PG.QueryRow(ctx, `INSERT INTO access_requests (requester_user_id, zitadel_project_id, zitadel_role_key, justification, status, reviewer_user_id, resolved_at) VALUES ($1,'pLaser','door','x','rejected','op',now()) RETURNING id`, user).Scan(&declined); err != nil {
		t.Fatal(err)
	}
	if _, err := PG.Exec(ctx, string(up)); err != nil {
		t.Fatalf("migration: %v", err)
	}
	if status, _, _ := requestStatus(t, ctx, held); status != "fulfilled" {
		t.Fatalf("an already-held closure is fulfilled, got %s", status)
	}
	if status, _, _ := requestStatus(t, ctx, declined); status != "rejected" {
		t.Fatalf("a real decline stays declined, got %s", status)
	}
}
