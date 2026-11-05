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

	in1, err := RecordSignIn(ctx, user)
	if err != nil || !in1.First || in1.Previous != nil {
		t.Fatalf("first call: %+v err=%v (the first sign-in has no previous)", in1, err)
	}
	time.Sleep(20 * time.Millisecond)
	in2, err := RecordSignIn(ctx, user)
	if err != nil || in2.First {
		t.Fatalf("second call: %+v err=%v (the welcome belongs to one session)", in2, err)
	}
	if !in2.FirstAt.Equal(in1.FirstAt) {
		t.Fatalf("overwritten: %v then %v", in1.FirstAt, in2.FirstAt)
	}
	var n int
	_ = PG.QueryRow(ctx, `SELECT count(*) FROM person_first_sign_in WHERE user_id = $1`, user).Scan(&n)
	if n != 1 {
		t.Fatalf("%d rows", n)
	}
}

func TestMigration000076_DownUndoesUp(t *testing.T) {
	up, _ := os.ReadFile("../../db/migrations/000076_last_sign_in.up.sql")
	down, _ := os.ReadFile("../../db/migrations/000076_last_sign_in.down.sql")
	for _, name := range []string{"last_sign_in_at", "previous_sign_in_at", "person_sign_in_order"} {
		if !strings.Contains(string(up), name) || !strings.Contains(string(down), name) {
			t.Errorf("000076 halves do not both name %s", name)
		}
	}
}

// Each sign-in moves current into previous; a retried callback within the
// settle window moves nothing; the order guard refuses an incoherent row.
func TestRecordSignIn_ShiftsPreviousAndCurrent_Live(t *testing.T) {
	ctx := liveDB(t)
	user := "si-" + time.Now().Format("150405.000000")
	t.Cleanup(func() { _, _ = PG.Exec(ctx, `DELETE FROM person_first_sign_in WHERE user_id = $1`, user) })
	read := func() (last time.Time, prev *time.Time) {
		t.Helper()
		if err := PG.QueryRow(ctx, `SELECT last_sign_in_at, previous_sign_in_at FROM person_first_sign_in WHERE user_id = $1`, user).Scan(&last, &prev); err != nil {
			t.Fatal(err)
		}
		return
	}

	if in, err := RecordSignIn(ctx, user); err != nil || !in.First || in.Previous != nil {
		t.Fatalf("first: %+v %v", in, err)
	}
	// A retried callback is the same sign-in.
	if in, err := RecordSignIn(ctx, user); err != nil || in.First || in.Previous != nil {
		t.Fatalf("retry: %+v %v (nothing should move inside the settle window)", in, err)
	}

	// Age the current sign-in, as a day passing would.
	day1 := time.Now().UTC().Add(-72 * time.Hour).Truncate(time.Second)
	if _, err := PG.Exec(ctx, `UPDATE person_first_sign_in SET first_sign_in_at = $2, last_sign_in_at = $2 WHERE user_id = $1`, user, day1); err != nil {
		t.Fatal(err)
	}
	in, err := RecordSignIn(ctx, user)
	if err != nil || in.Previous == nil || !in.Previous.Equal(day1) {
		t.Fatalf("second sign-in: %+v %v, want previous = %v", in, err, day1)
	}
	last2, _ := read()
	if !last2.After(day1) {
		t.Fatalf("current did not advance: %v", last2)
	}

	// Third sign-in: previous is the second, not the first.
	day2 := time.Now().UTC().Add(-24 * time.Hour).Truncate(time.Second)
	if _, err := PG.Exec(ctx, `UPDATE person_first_sign_in SET last_sign_in_at = $2 WHERE user_id = $1`, user, day2); err != nil {
		t.Fatal(err)
	}
	if in, err := RecordSignIn(ctx, user); err != nil || in.Previous == nil || !in.Previous.Equal(day2) {
		t.Fatalf("third sign-in: %+v %v, want previous = %v", in, err, day2)
	}

	// The coherence guard: a previous later than current is refused.
	if _, err := PG.Exec(ctx, `UPDATE person_first_sign_in SET previous_sign_in_at = last_sign_in_at + interval '1 hour' WHERE user_id = $1`, user); err == nil {
		t.Error("previous after current was accepted")
	}
	if _, err := PG.Exec(ctx, `UPDATE person_first_sign_in SET last_sign_in_at = first_sign_in_at - interval '1 hour' WHERE user_id = $1`, user); err == nil {
		t.Error("current before first was accepted")
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
	// 000076 made the current sign-in required; the 000075 statement predates it.
	backfill = strings.Replace(backfill, "(user_id, first_sign_in_at)", "(user_id, first_sign_in_at, last_sign_in_at)", 1)
	backfill = strings.Replace(backfill, "SELECT user_id, MIN(at) FROM", "SELECT user_id, MIN(at), MIN(at) FROM", 1)
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
	if in, err := RecordSignIn(ctx, actor); err != nil || in.First {
		t.Errorf("a person who has used Syndra must not be welcomed: first=%v err=%v", in.First, err)
	}
	if in, err := RecordSignIn(ctx, nobody); err != nil || !in.First {
		t.Errorf("a person Syndra has no record of is new: first=%v err=%v", in.First, err)
	}
}
