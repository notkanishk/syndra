package db

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestMigration000075_DownUndoesUp(t *testing.T) {
	up, _ := os.ReadFile("../../db/migrations/000075_first_sign_in.up.sql")
	down, _ := os.ReadFile("../../db/migrations/000075_first_sign_in.down.sql")
	if !strings.Contains(string(up), "person_first_sign_in") || !strings.Contains(string(down), "DROP TABLE IF EXISTS person_first_sign_in") {
		t.Error("000075 halves do not match")
	}
}

// Written once, never overwritten, and first only for the recording call.
func TestRecordFirstSignIn_Live(t *testing.T) {
	ctx := liveDB(t)
	user := "fs-" + time.Now().Format("150405.000000")
	t.Cleanup(func() { _, _ = PG.Exec(ctx, `DELETE FROM person_first_sign_in WHERE user_id = $1`, user) })

	at1, first, err := RecordFirstSignIn(ctx, user)
	if err != nil || !first {
		t.Fatalf("first call: first=%v err=%v", first, err)
	}
	time.Sleep(20 * time.Millisecond)
	at2, first, err := RecordFirstSignIn(ctx, user)
	if err != nil || first {
		t.Fatalf("second call: first=%v err=%v (the welcome belongs to one session)", first, err)
	}
	if !at2.Equal(at1) {
		t.Fatalf("overwritten: %v then %v", at1, at2)
	}
	var n int
	_ = PG.QueryRow(ctx, `SELECT count(*) FROM person_first_sign_in WHERE user_id = $1`, user).Scan(&n)
	if n != 1 {
		t.Fatalf("%d rows", n)
	}
}

// The backfill statement of 000075, run against fresh records: each person
// Syndra holds a record of is marked from the earliest one, and a person with
// none is left to be welcomed.
func TestFirstSignInBackfill_Live(t *testing.T) {
	ctx := liveDB(t)
	up, err := os.ReadFile("../../db/migrations/000075_first_sign_in.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(up)
	backfill := sql[strings.Index(sql, "INSERT INTO person_first_sign_in"):]
	stamp := time.Now().Format("150405.000000")
	actor, requester, usage, grant, nobody := "bf-a"+stamp, "bf-r"+stamp, "bf-u"+stamp, "bf-g"+stamp, "bf-n"+stamp
	old := time.Now().UTC().Add(-48 * time.Hour).Truncate(time.Second)
	newer := old.Add(24 * time.Hour)
	clean := func() {
		_, _ = PG.Exec(ctx, `DELETE FROM person_first_sign_in WHERE user_id LIKE 'bf-%'`)
		_, _ = PG.Exec(ctx, `DELETE FROM audit_logs WHERE actor_zitadel_user_id LIKE 'bf-%'`)
		_, _ = PG.Exec(ctx, `DELETE FROM access_requests WHERE requester_user_id LIKE 'bf-%'`)
		_, _ = PG.Exec(ctx, `DELETE FROM usage_last_seen WHERE user_id LIKE 'bf-%'`)
		_, _ = PG.Exec(ctx, `DELETE FROM zitadel_grants_index WHERE user_id LIKE 'bf-%'`)
	}
	clean()
	t.Cleanup(clean)
	must := func(q string, args ...any) {
		t.Helper()
		if _, err := PG.Exec(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	must(`INSERT INTO audit_logs (actor_zitadel_user_id, target_zitadel_user_id, action, created_at) VALUES ($1,'x','a',$2),($1,'x','a',$3)`, actor, newer, old)
	must(`INSERT INTO access_requests (requester_user_id, zitadel_project_id, zitadel_role_key, justification, created_at) VALUES ($1,'p','r','j',$2)`, requester, old)
	must(`INSERT INTO usage_last_seen (user_id, project_id, app_id, kind, last_at) VALUES ($1,'p','a','sign_in',$2)`, usage, old)
	must(`INSERT INTO zitadel_grants_index (grant_id, user_id, project_id, created_at) VALUES ($1,$2,'p',$3)`, "g"+stamp, grant, old)
	must(backfill)

	// Holding a role or signing in to another app is not using Syndra.
	for _, u := range []string{usage, grant} {
		var n int
		if err := PG.QueryRow(ctx, `SELECT count(*) FROM person_first_sign_in WHERE user_id = $1`, u).Scan(&n); err != nil || n != 0 {
			t.Errorf("%s was backfilled (n=%d, err=%v); only people who acted in Syndra are", u, n, err)
		}
	}
	for _, u := range []string{actor, requester} {
		var at time.Time
		if err := PG.QueryRow(ctx, `SELECT first_sign_in_at FROM person_first_sign_in WHERE user_id = $1`, u).Scan(&at); err != nil {
			t.Fatalf("%s not backfilled: %v", u, err)
		}
		if !at.Equal(old) {
			t.Errorf("%s: %v, want the earliest record %v", u, at, old)
		}
	}
	if _, first, err := RecordFirstSignIn(ctx, actor); err != nil || first {
		t.Errorf("a person who has used Syndra must not be welcomed: first=%v err=%v", first, err)
	}
	if _, first, err := RecordFirstSignIn(ctx, nobody); err != nil || !first {
		t.Errorf("a person Syndra has no record of is new: first=%v err=%v", first, err)
	}
}
