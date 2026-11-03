package db

import (
	"os"
	"strings"
	"testing"
	"time"

	"syndra/internal/claims"
)

// Every column the shape queries read must be one 000071 creates.
func TestClaimShapeQueriesMatchMigration000071(t *testing.T) {
	up, err := os.ReadFile("../../db/migrations/000071_app_claim_shapes.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, col := range []string{"application_id", "version", "project_id", "client_id", "profile", "template_id",
		"template_version", "copied_from_application_id", "copied_from_version", "restored_from", "note", "saved_by", "saved_at"} {
		if !strings.Contains(claimShapeColumns, col) || !strings.Contains(string(up), "    "+col+" ") {
			t.Errorf("column %q: queried or created on one side only", col)
		}
	}
	down, err := os.ReadFile("../../db/migrations/000071_app_claim_shapes.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"DROP TABLE IF EXISTS app_claim_shape_versions", "DROP TABLE IF EXISTS app_claim_shape_seed"} {
		if !strings.Contains(string(down), want) {
			t.Errorf("000071 down migration missing %q", want)
		}
	}
}

// Versions are immutable and numbered per app; the seed writes only into an
// app with no shape; the project read returns each app's latest.
func TestClaimShapeVersions_Live(t *testing.T) {
	ctx := liveDB(t)
	stamp := time.Now().Format("150405.000000")
	app, other, pid := "app"+stamp, "other"+stamp, "p"+stamp
	t.Cleanup(func() {
		_, _ = PG.Exec(ctx, `DELETE FROM app_claim_shape_versions WHERE project_id = $1`, pid)
	})

	seed := ClaimShapeVersion{ApplicationID: app, ProjectID: pid, ClientID: "c-" + stamp, Profile: claims.ActionsV1(),
		TemplateID: "custom", TemplateVersion: 1, Note: "As Actions v1 sent it", SavedBy: "system"}
	v1, wrote, err := InsertClaimShapeVersion(ctx, seed, true)
	if err != nil || !wrote || v1.Version != 1 {
		t.Fatalf("seed: v=%+v wrote=%v err=%v", v1, wrote, err)
	}
	if _, wrote, err := InsertClaimShapeVersion(ctx, seed, true); err != nil || wrote {
		t.Fatalf("a second seed must write nothing: wrote=%v err=%v", wrote, err)
	}

	next := seed
	next.Profile, next.Note, next.CopiedFromApp, next.CopiedFromVersion = claims.Builtin(), "", other, 3
	v2, _, err := InsertClaimShapeVersion(ctx, next, false)
	if err != nil || v2.Version != 2 || v2.CopiedFromApp != other || v2.CopiedFromVersion != 3 {
		t.Fatalf("save: %+v err=%v", v2, err)
	}
	if _, _, err := InsertClaimShapeVersion(ctx, ClaimShapeVersion{ApplicationID: other, ProjectID: pid,
		Profile: claims.Builtin(), SavedBy: "op"}, false); err != nil {
		t.Fatal(err)
	}

	latest, err := LatestClaimShapes(ctx, pid)
	if err != nil || len(latest) != 2 {
		t.Fatalf("latest per app: %+v err=%v", latest, err)
	}
	for _, l := range latest {
		if l.ApplicationID == app && (l.Version != 2 || l.Profile.Claims[0].Path[0] != "roles") {
			t.Fatalf("latest of %s: %+v", app, l)
		}
	}
	old, ok, err := ClaimShapeAt(ctx, app, 1)
	if err != nil || !ok || len(old.Profile.Claims) != 5 || old.Note != "As Actions v1 sent it" {
		t.Fatalf("version 1 is kept as it was: %+v ok=%v err=%v", old, ok, err)
	}
	hist, err := ClaimShapeHistory(ctx, app)
	if err != nil || len(hist) != 2 || hist[0].Version != 2 {
		t.Fatalf("history newest first: %+v err=%v", hist, err)
	}
	if _, err := ClaimShapeSeedCutoff(ctx); err != nil {
		t.Fatal(err)
	}
}
