package db

import (
	"reflect"
	"testing"

	"syndra/internal/models"
)

// Insights snapshots (I5) against the real schema. Days are in 2001, so only rows
// a test inserts can sit between its dates; the table's real rows are all later.
// Bounds are compared with whatever the table holds, so they assert only what
// the test's own rows force.

func TestInsightsSnapshots_AgainstTheSchema(t *testing.T) {
	ctx := liveDB(t)
	days := []string{"2001-03-05", "2001-03-09"}
	t.Cleanup(func() { _, _ = PG.Exec(ctx, `DELETE FROM insights_snapshots WHERE day = ANY($1::date[])`, days) })
	_, _ = PG.Exec(ctx, `DELETE FROM insights_snapshots WHERE day = ANY($1::date[])`, days)

	graph := models.TopologyGraph{
		Nodes: []models.TopologyNode{{ID: "role:p:r", Label: "Operator \"quoted\" ünï"}},
		Edges: []models.TopologyEdge{},
	}
	people := map[string]int{"p:r": 4, "p:zero": 0}

	if ok, err := InsightsSnapshotExists(ctx, days[0]); err != nil || ok {
		t.Fatalf("exists before insert: %v, %v", ok, err)
	}
	if inserted, err := InsertInsightsSnapshot(ctx, InsightsSnapshot{Day: days[0], Topology: graph, RolePeople: people}); err != nil || !inserted {
		t.Fatalf("first insert: %v, %v", inserted, err)
	}
	// ON CONFLICT (day) DO NOTHING: the first picture of a day stays.
	other := models.TopologyGraph{Nodes: []models.TopologyNode{}, Edges: []models.TopologyEdge{}}
	if inserted, err := InsertInsightsSnapshot(ctx, InsightsSnapshot{Day: days[0], Topology: other, RolePeople: map[string]int{}}); err != nil || inserted {
		t.Fatalf("second insert same day: %v, %v; want a quiet no-op", inserted, err)
	}
	if ok, err := InsightsSnapshotExists(ctx, days[0]); err != nil || !ok {
		t.Fatalf("exists after insert: %v, %v", ok, err)
	}
	if _, err := InsertInsightsSnapshot(ctx, InsightsSnapshot{Day: days[1], Topology: other, RolePeople: map[string]int{}}); err != nil {
		t.Fatal(err)
	}

	// JSONB round trip, and the kept row is the first one.
	got, err := InsightsSnapshotOnOrBefore(ctx, days[0])
	if err != nil || got == nil {
		t.Fatalf("on or before its own day: %v, %v", got, err)
	}
	if got.Day != days[0] || !reflect.DeepEqual(got.Topology, graph) || !reflect.DeepEqual(got.RolePeople, people) || got.TakenAt.IsZero() {
		t.Fatalf("round trip = %+v", got)
	}

	// Nearest on or before: the 7th has no row, so it is the 5th's.
	if got, err = InsightsSnapshotOnOrBefore(ctx, "2001-03-07"); err != nil || got == nil || got.Day != days[0] {
		t.Fatalf("2001-03-07 = %+v, %v; want %s", got, err, days[0])
	}
	if got, err = InsightsSnapshotOnOrBefore(ctx, "2001-03-31"); err != nil || got == nil || got.Day != days[1] {
		t.Fatalf("2001-03-31 = %+v, %v; want %s", got, err, days[1])
	}
	// Before everything stored there is nothing, not the nearest after.
	if got, err = InsightsSnapshotOnOrBefore(ctx, "2001-03-04"); err != nil || got != nil {
		t.Fatalf("2001-03-04 = %+v, %v; want nil", got, err)
	}

	first, last, err := InsightsSnapshotBounds(ctx)
	if err != nil || first == nil || last == nil || *first > days[0] || *last < days[1] {
		t.Fatalf("bounds = %v, %v, %v", first, last, err)
	}
}
