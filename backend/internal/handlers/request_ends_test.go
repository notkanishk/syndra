package handlers

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/models"
)

// A settled request to keep a role remembers both ends: approval records the end they had and
// the end it set, a decline only the end they had, and an ordinary request neither. What is
// recorded is services.DecideRequest's (and the planner's approval); here, how a settled
// extension is read back.

func TestSettledRequestJSONOmitsEndsWhenNull(t *testing.T) {
	end := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	out := jsonString(t, models.AccessRequest{ID: "a", EndedBefore: &end})
	if !strings.Contains(out, `"ended_before":"2026-11-18T00:00:00Z"`) || strings.Contains(out, "ends_after") {
		t.Fatalf("ended_before present, ends_after omitted: %s", out)
	}
	if out := jsonString(t, models.AccessRequest{ID: "a"}); strings.Contains(out, "ended_before") || strings.Contains(out, "ends_after") {
		t.Fatalf("both omitted when null: %s", out)
	}
}

func TestMyChangesCarryTheEndsOfASettledExtension(t *testing.T) {
	a := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	b := a.AddDate(0, 1, 0)
	out := jsonString(t, shapeMyChanges([]db.MyChangeRow{
		{Kind: "request_approved", RequestKind: "extend", EndedBefore: &a, EndsAfter: &b},
		{Kind: "request_declined", RequestKind: "extend", EndedBefore: &a},
		{Kind: "given"},
	}, nil))
	if strings.Count(out, `"ended_before"`) != 2 || strings.Count(out, `"ends_after"`) != 1 {
		t.Fatalf("ends ride on the settled extensions only: %s", out)
	}
}

func jsonString(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
