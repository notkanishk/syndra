package db

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

// runMigration51 runs the real 000051 file against a scratch schema holding
// only the two tables it reads, seeded by the caller, so its inference can be
// exercised on data the shared test database never has.
func runMigration51(t *testing.T, ctx context.Context, seed string) error {
	t.Helper()
	up, err := os.ReadFile("../../db/migrations/000051_one_project_per_target.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := PG.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	schema := "m51_" + strings.ReplaceAll(strings.ToLower(t.Name()), "/", "_")
	t.Cleanup(func() { _, _ = PG.Exec(context.Background(), `DROP SCHEMA IF EXISTS `+schema+` CASCADE`) })
	if _, err := conn.Exec(ctx, `DROP SCHEMA IF EXISTS `+schema+` CASCADE; CREATE SCHEMA `+schema+`; SET search_path TO `+schema); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = conn.Exec(context.Background(), `SET search_path TO DEFAULT`) }()
	if _, err := conn.Exec(ctx, `
		CREATE TABLE targets (target TEXT PRIMARY KEY);
		CREATE TABLE target_role_mappings (
			id SERIAL PRIMARY KEY, target TEXT NOT NULL REFERENCES targets(target),
			project_id VARCHAR(255) NOT NULL, role_key VARCHAR(255) NOT NULL,
			field VARCHAR(255) NOT NULL, value TEXT NOT NULL);
		`+seed); err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec(ctx, string(up))
	return err
}

func TestMigration51InfersTheOneProject(t *testing.T) {
	ctx := liveDB(t)
	err := runMigration51(t, ctx, `
		INSERT INTO targets VALUES ('truenas'), ('door');
		INSERT INTO target_role_mappings (target, project_id, role_key, field, value) VALUES
			('truenas', 'p-net', 'community', 'group', 'Community'),
			('truenas', 'p-net', 'staff', 'group', 'Staff');`)
	if err != nil {
		t.Fatalf("one project per target must migrate: %v", err)
	}
	schema := "m51_" + strings.ToLower(t.Name())
	var truenas, door *string
	if err := PG.QueryRow(ctx, `SELECT (SELECT project_id FROM `+schema+`.targets WHERE target='truenas'),
		(SELECT project_id FROM `+schema+`.targets WHERE target='door')`).Scan(&truenas, &door); err != nil {
		t.Fatal(err)
	}
	if truenas == nil || *truenas != "p-net" || door != nil {
		t.Fatalf("want truenas=p-net and door unset, got %v %v", truenas, door)
	}
}

func TestMigration51RefusesToGuessBetweenProjects(t *testing.T) {
	ctx := liveDB(t)
	err := runMigration51(t, ctx, `
		INSERT INTO targets VALUES ('truenas');
		INSERT INTO target_role_mappings (target, project_id, role_key, field, value) VALUES
			('truenas', 'p-net', 'community', 'group', 'Community'),
			('truenas', 'p-laser', 'operator', 'group', 'Laser');`)
	if err == nil || !strings.Contains(err.Error(), "truenas (p-laser, p-net)") {
		t.Fatalf("want a refusal naming the target and both projects, got %v", err)
	}
}

func TestMigration51RefusesOneProjectOnTwoTargets(t *testing.T) {
	ctx := liveDB(t)
	err := runMigration51(t, ctx, `
		INSERT INTO targets VALUES ('truenas'), ('door');
		INSERT INTO target_role_mappings (target, project_id, role_key, field, value) VALUES
			('truenas', 'p-net', 'community', 'group', 'Community'),
			('door', 'p-net', 'community', 'zone', 'Lobby');`)
	if err == nil || !strings.Contains(err.Error(), "p-net (door, truenas)") {
		t.Fatalf("want a refusal naming the project and both targets, got %v", err)
	}
}

func seedProjectTargets(t *testing.T, ctx context.Context, targets ...string) {
	t.Helper()
	for _, tg := range targets {
		if _, err := PG.Exec(ctx, `INSERT INTO targets (target) VALUES ($1) ON CONFLICT DO NOTHING`, tg); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, tg := range targets {
			_, _ = PG.Exec(context.Background(), `DELETE FROM target_role_mappings WHERE target = $1`, tg)
			_, _ = PG.Exec(context.Background(), `DELETE FROM targets WHERE target = $1`, tg)
		}
	})
}

func TestATargetServesOneProjectAndAProjectOneTarget(t *testing.T) {
	ctx := liveDB(t)
	seedProjectTargets(t, ctx, "tp-nas", "tp-door")

	if err := RequireTargetProject(ctx, "tp-nas"); !errors.Is(err, ErrProjectRequired) {
		t.Fatalf("no project yet must refuse a giving write, got %v", err)
	}
	if err := RequireTargetProject(ctx, TargetZitadel); err != nil {
		t.Fatalf("the Zitadel target is never asked for a project, got %v", err)
	}
	if err := SetTargetProject(ctx, "tp-nas", "tp-proj"); err != nil {
		t.Fatal(err)
	}
	var taken ErrProjectTaken
	if err := SetTargetProject(ctx, "tp-door", "tp-proj"); !errors.As(err, &taken) || taken.Target != "tp-nas" {
		t.Fatalf("want ErrProjectTaken naming tp-nas, got %v", err)
	}

	if _, err := PG.Exec(ctx, `INSERT INTO target_role_mappings (target, project_id, role_key, field, value, created_by)
		VALUES ('tp-nas', 'tp-proj', 'community', 'group', 'Community', 'test')`); err != nil {
		t.Fatal(err)
	}
	var exist ErrMappingsExist
	if err := SetTargetProject(ctx, "tp-nas", "tp-other"); !errors.As(err, &exist) || exist.Count != 1 {
		t.Fatalf("want ErrMappingsExist with the count, got %v", err)
	}
	if _, err := PG.Exec(ctx, `INSERT INTO target_role_mappings (target, project_id, role_key, field, value, created_by)
		VALUES ('tp-nas', 'tp-elsewhere', 'operator', 'group', 'Laser', 'test')`); err == nil {
		t.Fatal("a mapping from another project must be refused by the database")
	}
}
