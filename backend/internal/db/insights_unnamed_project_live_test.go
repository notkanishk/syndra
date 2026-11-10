package db

import (
	"encoding/json"
	"os"
	"testing"
)

// Migration 000085: a stored day's unlisted project, labelled with its id,
// gets the empty label (the map's one "no name" signal); a named project and
// every other node are left as they were, in their order; running it again
// changes nothing; the down migration brings the id back.
func TestStoredMapsLoseTheIdAsAProjectsName(t *testing.T) {
	ctx := liveDB(t)
	const day = "1999-01-02"
	t.Cleanup(func() { _, _ = PG.Exec(ctx, `DELETE FROM insights_snapshots WHERE day = $1`, day) })
	topology := `{"nodes":[
		{"id":"project:p1","label":"Laser Lab","kind":"project","project_id":"p1"},
		{"id":"project:382075","label":"382075","kind":"project","project_id":"382075"},
		{"id":"role:382075:trained","label":"Trained","kind":"role","project_id":"382075"}],
		"edges":[]}`
	if _, err := PG.Exec(ctx, `INSERT INTO insights_snapshots (day, topology, role_people) VALUES ($1::date, $2::jsonb, '{}')`, day, topology); err != nil {
		t.Fatal(err)
	}
	labels := func() []string {
		t.Helper()
		var raw []byte
		if err := PG.QueryRow(ctx, `SELECT topology FROM insights_snapshots WHERE day = $1::date`, day).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var g struct{ Nodes []struct{ Label string } }
		if err := json.Unmarshal(raw, &g); err != nil {
			t.Fatal(err)
		}
		out := make([]string, len(g.Nodes))
		for i, n := range g.Nodes {
			out[i] = n.Label
		}
		return out
	}
	run := func(file string) {
		t.Helper()
		sql, err := os.ReadFile("../../db/migrations/" + file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := PG.Exec(ctx, string(sql)); err != nil {
			t.Fatal(err)
		}
	}

	run("000085_insights_unnamed_project_labels.up.sql")
	run("000085_insights_unnamed_project_labels.up.sql") // idempotent
	if got := labels(); len(got) != 3 || got[0] != "Laser Lab" || got[1] != "" || got[2] != "Trained" {
		t.Fatalf("labels %q, want [Laser Lab, \"\", Trained] in order", got)
	}
	run("000085_insights_unnamed_project_labels.down.sql")
	if got := labels(); got[1] != "382075" || got[0] != "Laser Lab" {
		t.Fatalf("down: labels %q, want the id back as the unlisted project's label", got)
	}
}
