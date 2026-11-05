package db

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

// 000079 names its zone and never leans on the session's: the cluster's
// TimeZone is UTC, and a migration that read it would re-key nothing.
func TestMigration000079_NamesTheZone(t *testing.T) {
	up, err := os.ReadFile("../../db/migrations/000079_one_time_zone.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile("../../db/migrations/000079_one_time_zone.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(up)
	if strings.Count(sql, "AT TIME ZONE 'Asia/Kolkata'") < 3 || strings.Contains(sql, "'UTC'") {
		t.Error("000079 must re-key on 'Asia/Kolkata' by name, everywhere")
	}
	if !strings.Contains(sql, "sign_in_daily_recount") || !strings.Contains(string(down), "DROP TABLE IF EXISTS sign_in_daily_recount") {
		t.Error("000079 halves do not match")
	}
}

func runMigration000079(t *testing.T) {
	t.Helper()
	up, err := os.ReadFile("../../db/migrations/000079_one_time_zone.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PG.Exec(t.Context(), string(up)); err != nil {
		t.Fatalf("000079: %v", err)
	}
}

// A snapshot taken at 23:00 UTC was taken at 04:30 the next day in India; the
// first picture of a day is kept when two land on it; running it again changes
// nothing.
func TestMigration000079_RekeysSnapshotsOnTheMakerspaceDay(t *testing.T) {
	ctx := liveDB(t)
	clean := func() {
		_, _ = PG.Exec(ctx, `DELETE FROM insights_snapshots WHERE day BETWEEN '2001-03-01' AND '2001-03-31'`)
		_, _ = PG.Exec(ctx, `DELETE FROM sign_in_daily WHERE day BETWEEN '2001-03-01' AND '2001-03-31'`)
		_, _ = PG.Exec(ctx, `DELETE FROM sign_in_daily_recount`)
	}
	clean()
	t.Cleanup(clean)
	snap := func(day, takenAt string) {
		t.Helper()
		if _, err := PG.Exec(ctx, `INSERT INTO insights_snapshots (day, taken_at, topology, role_people)
			VALUES ($1::date, $2::timestamptz, '{}', $3)`, day, takenAt, `{"p:`+day+`":1}`); err != nil {
			t.Fatal(err)
		}
	}
	snap("2001-03-04", "2001-03-04T23:00:00Z") // 04:30 IST on the 5th
	snap("2001-03-05", "2001-03-05T02:00:00Z") // 07:30 IST on the 5th: a later picture of the same day
	snap("2001-03-06", "2001-03-06T01:00:00Z") // 06:30 IST on the 6th: stays
	if _, err := PG.Exec(ctx, `INSERT INTO sign_in_daily (app_id, day, count) VALUES ('a', '2001-03-04', 3)`); err != nil {
		t.Fatal(err)
	}

	read := func() map[string]string {
		t.Helper()
		rows, err := PG.Query(ctx, `SELECT day::text, to_char(taken_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI') FROM insights_snapshots
			WHERE day BETWEEN '2001-03-01' AND '2001-03-31'`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		out := map[string]string{}
		for rows.Next() {
			var d, at string
			if err := rows.Scan(&d, &at); err != nil {
				t.Fatal(err)
			}
			out[d] = at
		}
		return out
	}
	want := map[string]string{"2001-03-05": "2001-03-04T23:00", "2001-03-06": "2001-03-06T01:00"}

	runMigration000079(t)
	if got := read(); !reflect.DeepEqual(got, want) {
		t.Fatalf("after 000079: %v, want %v", got, want)
	}
	runMigration000079(t)
	if got := read(); !reflect.DeepEqual(got, want) {
		t.Fatalf("000079 twice: %v, want %v", got, want)
	}

	// A total on the UTC day waits for its recount, asked for once.
	var asks int
	var through string
	if err := PG.QueryRow(ctx, `SELECT COUNT(*), MAX(through)::text FROM sign_in_daily_recount`).Scan(&asks, &through); err != nil {
		t.Fatal(err)
	}
	if asks != 1 || through != "2026-11-05" {
		t.Fatalf("recount asked %d times through %s, want once through 2026-11-05", asks, through)
	}
}

// A hold "until 30 Nov" sent as UTC midnight (05:30 IST) now lasts the whole of
// 30 Nov in India. Only a value at exactly midnight UTC (a picked day), still
// ahead, on a hold still in force, moves; and only once.
func TestMigration000079_HoldDatesHoldTheWholeDay(t *testing.T) {
	ctx := liveDB(t)
	subject := "live-ist-hold-" + liveSuffix()
	t.Cleanup(func() { _, _ = PG.Exec(ctx, `DELETE FROM allowances WHERE subject_id = $1`, subject) })
	hold := func(value, expires, review, lifted string) string {
		t.Helper()
		var id string
		if err := PG.QueryRow(ctx, `INSERT INTO allowances (subject_id, target, field, value, direction, actor_id, reason, expires_at, review_date, lifted_at, lifted_by)
			VALUES ($1, 'zitadel', 'group', $2, 'deny', 'op', 'test', NULLIF($3, '')::timestamptz, NULLIF($4, '')::timestamptz, NULLIF($5, '')::timestamptz, CASE WHEN $5 = '' THEN NULL ELSE 'op' END)
			RETURNING id::text`, subject, value, expires, review, lifted).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	picked := hold("a", "2099-11-30T00:00:00Z", "2099-12-01T00:00:00Z", "") // both picked days: both move
	counted := hold("b", "2099-11-30T07:13:22.5Z", "", "")                  // now + N days: an instant, stays
	lifted := hold("c", "2099-11-30T00:00:00Z", "", "2026-11-01T00:00:00Z") // history: stays
	past := hold("d", "2001-11-30T00:00:00Z", "2099-11-30T00:00:00Z", "")   // ended day stays; the review moves
	read := func(id string) (expires, review string) {
		t.Helper()
		var e, r *time.Time
		if err := PG.QueryRow(ctx, `SELECT expires_at, review_date FROM allowances WHERE id::text = $1`, id).Scan(&e, &r); err != nil {
			t.Fatal(err)
		}
		f := func(p *time.Time) string {
			if p == nil {
				return ""
			}
			return p.UTC().Format(time.RFC3339Nano)
		}
		return f(e), f(r)
	}
	for run := 1; run <= 2; run++ {
		runMigration000079(t)
		for _, c := range []struct{ id, expires, review string }{
			{picked, "2099-11-30T18:29:59Z", "2099-12-01T18:29:59Z"},
			{counted, "2099-11-30T07:13:22.5Z", ""},
			{lifted, "2099-11-30T00:00:00Z", ""},
			{past, "2001-11-30T00:00:00Z", "2099-11-30T18:29:59Z"},
		} {
			if e, r := read(c.id); e != c.expires || r != c.review {
				t.Fatalf("run %d: hold %s = %s / %s, want %s / %s", run, c.id, e, r, c.expires, c.review)
			}
		}
	}
	_, _ = PG.Exec(ctx, `DELETE FROM sign_in_daily_recount`)
}

// The recount swaps the days it covers in one go and closes the request; a day
// outside them is untouched.
func TestReplaceSignInDays_SwapsTheDaysAndClosesTheRecount(t *testing.T) {
	ctx := liveDB(t)
	clean := func() {
		_, _ = PG.Exec(ctx, `DELETE FROM sign_in_daily WHERE day BETWEEN '2001-04-01' AND '2001-04-30'`)
		_, _ = PG.Exec(ctx, `DELETE FROM sign_in_daily_recount`)
	}
	clean()
	t.Cleanup(clean)
	if _, err := PG.Exec(ctx, `INSERT INTO sign_in_daily (app_id, day, count) VALUES
		('a', '2001-04-03', 5), ('b', '2001-04-04', 2), ('a', '2001-04-06', 9)`); err != nil {
		t.Fatal(err)
	}
	if _, err := PG.Exec(ctx, `INSERT INTO sign_in_daily_recount (through) VALUES ('2001-04-04')`); err != nil {
		t.Fatal(err)
	}
	first, through, err := SignInRecountPending(ctx)
	if err != nil || through == nil || *through != "2001-04-04" || first == nil {
		t.Fatalf("pending = %v %v %v", first, through, err)
	}
	if err := ReplaceSignInDays(ctx, "2001-04-03", "2001-04-04", map[SignInDayKey]int{
		{AppID: "a", Day: "2001-04-03"}: 4,
		{AppID: "a", Day: "2001-04-04"}: 2,
		{AppID: "a", Day: "2001-04-06"}: 100, // outside the days asked: ignored
	}); err != nil {
		t.Fatal(err)
	}
	rows, err := PG.Query(ctx, `SELECT app_id || ' ' || day::text || ' ' || count FROM sign_in_daily
		WHERE day BETWEEN '2001-04-01' AND '2001-04-30' ORDER BY day, app_id`)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for rows.Next() {
		var s string
		_ = rows.Scan(&s)
		got = append(got, s)
	}
	rows.Close()
	if want := []string{"a 2001-04-03 4", "a 2001-04-04 2", "a 2001-04-06 9"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("totals = %v, want %v", got, want)
	}
	if _, through, _ := SignInRecountPending(ctx); through != nil {
		t.Fatalf("the recount request survived: %s", *through)
	}
}

// "Receiving since" starts at the makerspace's midnight of the first day, not
// the session's (UTC) one.
func TestSignInsSince_StartsAtTheMakerspaceMidnight(t *testing.T) {
	ctx := liveDB(t)
	var at time.Time
	if err := PG.QueryRow(ctx, `SELECT DATE '2001-03-05'::timestamp AT TIME ZONE $1`, "Asia/Kolkata").Scan(&at); err != nil {
		t.Fatal(err)
	}
	if !at.Equal(time.Date(2001, 3, 4, 18, 30, 0, 0, time.UTC)) {
		t.Fatalf("midnight 5 Mar IST = %v, want 18:30 UTC 4 Mar", at.UTC())
	}
}
