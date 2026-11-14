package db

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	pgx5 "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

// The live half of 000088 (task 1.7). Gated on SYNDRA_TEST_DATABASE_URL.

func seedAddonTarget(t *testing.T, ctx context.Context) {
	t.Helper()
	if _, err := PG.Exec(ctx, `INSERT INTO targets (target) VALUES ('truenas') ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	clean := func() {
		_, _ = PG.Exec(context.Background(), `DELETE FROM target_binding_conflicts WHERE target = 'truenas'`)
		_, _ = PG.Exec(context.Background(), `DELETE FROM target_account_bindings WHERE target = 'truenas'`)
		_, _ = PG.Exec(context.Background(), `UPDATE targets SET field_roles = NULL, retired_fields = '{}', accepted_shape = NULL WHERE target = 'truenas'`)
	}
	clean()
	t.Cleanup(clean)
}

func strp(s string) *string { return &s }

// Both columns are written for one release, a row a contract-1 backend wrote
// (account_uid alone) is read and backfilled, and the backfill is re-runnable.
func TestAccountIDIsDualWrittenAndBackfilled(t *testing.T) {
	ctx := liveDB(t)
	seedAddonTarget(t, ctx)
	if err := RecordTargetBinding(ctx, TargetBinding{Target: "truenas", SubjectID: "u-rey", Username: "rey_mysterio",
		AccountID: strp("3051"), BoundBy: "apply"}); err != nil {
		t.Fatal(err)
	}
	var id *string
	var uid *int64
	if err := PG.QueryRow(ctx, `SELECT account_id, account_uid FROM target_account_bindings WHERE subject_id = 'u-rey'`).Scan(&id, &uid); err != nil {
		t.Fatal(err)
	}
	if id == nil || *id != "3051" || uid == nil || *uid != 3051 {
		t.Fatalf("both columns must be written: account_id %v account_uid %v", id, uid)
	}

	// A contract-1 backend, rolled back, writes account_uid alone.
	if _, err := PG.Exec(ctx, `INSERT INTO target_account_bindings (target, subject_id, username, account_uid, bound_by)
		VALUES ('truenas', 'u-seth', 'seth_rollins', 3077, 'apply')`); err != nil {
		t.Fatal(err)
	}
	b, found, err := GetTargetBinding(ctx, "truenas", "u-seth")
	if err != nil || !found || b.AccountID == nil || *b.AccountID != "3077" {
		t.Fatalf("a contract-1 row must read by its account_uid: %+v %v", b, err)
	}
	for range 2 {
		if err := BackfillAccountIDs(ctx); err != nil {
			t.Fatalf("the backfill must re-run cleanly: %v", err)
		}
	}
	if err := PG.QueryRow(ctx, `SELECT account_id FROM target_account_bindings WHERE subject_id = 'u-seth'`).Scan(&id); err != nil || id == nil || *id != "3077" {
		t.Fatalf("the backfill did not fill account_id: %v %v", id, err)
	}

	// A contract-1 backend, rolled back, UPDATES a binding's uid (an account
	// recreated): the next start's backfill corrects account_id to it.
	if _, err := PG.Exec(ctx, `UPDATE target_account_bindings SET account_uid = 3099 WHERE subject_id = 'u-seth'`); err != nil {
		t.Fatal(err)
	}
	if err := BackfillAccountIDs(ctx); err != nil {
		t.Fatal(err)
	}
	if err := PG.QueryRow(ctx, `SELECT account_id FROM target_account_bindings WHERE subject_id = 'u-seth'`).Scan(&id); err != nil || id == nil || *id != "3099" {
		t.Fatalf("a uid a contract-1 backend updated must correct account_id: %v %v", id, err)
	}
	// And a contract-2 upsert with a non-numeric id clears the uid, so the
	// backfill can never put the old number back.
	if err := RecordTargetBinding(ctx, TargetBinding{Target: "truenas", SubjectID: "u-seth", Username: "seth_rollins",
		AccountID: strp("7a2c3d4e-uuid"), BoundBy: "apply"}); err != nil {
		t.Fatal(err)
	}
	_ = BackfillAccountIDs(ctx)
	if err := PG.QueryRow(ctx, `SELECT account_id, account_uid FROM target_account_bindings WHERE subject_id = 'u-seth'`).Scan(&id, &uid); err != nil || id == nil || *id != "7a2c3d4e-uuid" || uid != nil {
		t.Fatalf("a contract-2 id must survive the backfill: %v %v %v", id, uid, err)
	}

	// The unique index on the id: one account, one subject.
	err = RecordTargetBinding(ctx, TargetBinding{Target: "truenas", SubjectID: "u-other", Username: "someone_else",
		AccountID: strp("3051"), BoundBy: "apply"})
	if !errors.Is(err, ErrBindingConflict) {
		t.Fatalf("a second subject on one account id must conflict, got %v", err)
	}
	if holder, found, err := BindingHolder(ctx, "truenas", "renamed", "3051"); err != nil || !found || holder != "u-rey" {
		t.Fatalf("the holder is found by account id: %q %v %v", holder, found, err)
	}

	// A UUID-shaped id has no account_uid.
	if err := RecordTargetBinding(ctx, TargetBinding{Target: "truenas", SubjectID: "u-uuid", Username: "bianca",
		AccountID: strp("7f3a2c1b-0d4e-4f5a-8b6c-1d2e3f4a5b6c"), BoundBy: "apply"}); err != nil {
		t.Fatal(err)
	}
	if err := PG.QueryRow(ctx, `SELECT account_uid FROM target_account_bindings WHERE subject_id = 'u-uuid'`).Scan(&uid); err != nil || uid != nil {
		t.Fatalf("a non-numeric id must leave account_uid NULL: %v %v", uid, err)
	}
}

// target_binding_conflicts keeps resolved history: a second conflict on the
// same account inserts once the first is resolved (the account_id index is on
// the bindings table only).
func TestASecondConflictOnAnAccountInsertsAfterTheFirstIsResolved(t *testing.T) {
	ctx := liveDB(t)
	seedAddonTarget(t, ctx)
	if err := RecordTargetBinding(ctx, TargetBinding{Target: "truenas", SubjectID: "u-a", Username: "rey_mysterio",
		AccountID: strp("3051"), BoundBy: "apply"}); err != nil {
		t.Fatal(err)
	}
	conflict := BindingConflict{Target: "truenas", Username: "rey_mysterio", AccountID: strp("3051"),
		ConvergedSubjectID: "u-b", BoundSubjectID: "u-a", OutboxID: "6f1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d"}
	if err := RecordBindingConflict(ctx, conflict); err != nil {
		t.Fatal(err)
	}
	standing, err := StandingBindingConflicts(ctx, "truenas")
	if err != nil || len(standing) != 1 || standing[0].AccountID == nil || *standing[0].AccountID != "3051" {
		t.Fatalf("the conflict must carry its account id: %+v %v", standing, err)
	}
	if _, err := ResolveBindingConflict(ctx, standing[0].ID, "u-a", "operator", "Rey keeps it."); err != nil {
		t.Fatal(err)
	}
	conflict.OutboxID = "7a2c3d4e-5f6a-4b7c-9d8e-0f1a2b3c4d5e"
	if err := RecordBindingConflict(ctx, conflict); err != nil {
		t.Fatalf("a second conflict on the account must insert after the first is resolved: %v", err)
	}
	if standing, _ := StandingBindingConflicts(ctx, "truenas"); len(standing) != 1 {
		t.Fatalf("want the new conflict standing, got %d", len(standing))
	}
}

// targets.field_roles holds the role map as accepted, and is absent before any
// contract-2 manifest was.
func TestTargetFieldRolesArePersisted(t *testing.T) {
	ctx := liveDB(t)
	seedAddonTarget(t, ctx)
	if _, found, err := TargetFieldRoles(ctx, "truenas"); err != nil || found {
		t.Fatalf("no roles before a manifest is accepted: %v %v", found, err)
	}
	want := FieldRoleMap{Roles: map[string]string{"enabled": "account", "smb_enabled": "channel", "group": "membership"}}
	if err := SaveTargetFieldRoles(ctx, "truenas", want); err != nil {
		t.Fatal(err)
	}
	got, found, err := TargetFieldRoles(ctx, "truenas")
	if err != nil || !found || len(got.Roles) != 3 || got.Roles["enabled"] != "account" || len(got.Retired) != 0 {
		t.Fatalf("roles did not round-trip: %v %v %v", got, found, err)
	}
	// A field the manifest dropped keeps its role and is marked retired.
	want.Retired = []string{"smb_enabled"}
	if err := SaveTargetFieldRoles(ctx, "truenas", want); err != nil {
		t.Fatal(err)
	}
	if got, _, err := TargetFieldRoles(ctx, "truenas"); err != nil || len(got.Retired) != 1 || got.Retired[0] != "smb_enabled" || got.Roles["smb_enabled"] != "channel" {
		t.Fatalf("a retired field must keep its role and be marked: %v %v", got, err)
	}
	if err := SaveTargetFieldRoles(ctx, "no-such-target", want); err == nil {
		t.Error("saving roles for a target that does not exist must fail")
	}
	// The accepted shape (000095) rides the same write, kept opaque: none
	// saved reads back nil, one saved reads back the same JSON value.
	if got, _, _ := TargetFieldRoles(ctx, "truenas"); got.Shape != nil {
		t.Fatalf("no shape saved must read nil, got %s", got.Shape)
	}
	want.Shape = []byte(`{"kind":"storage","product":"TrueNAS","entitlement_schema":[{"name":"home_quota","role":"limit","unit":"bytes"}]}`)
	if err := SaveTargetFieldRoles(ctx, "truenas", want); err != nil {
		t.Fatal(err)
	}
	got, _, err = TargetFieldRoles(ctx, "truenas")
	var shape struct {
		Kind   string `json:"kind"`
		Schema []struct {
			Unit string `json:"unit"`
		} `json:"entitlement_schema"`
	}
	if err != nil || json.Unmarshal(got.Shape, &shape) != nil || shape.Kind != "storage" || len(shape.Schema) != 1 || shape.Schema[0].Unit != "bytes" {
		t.Fatalf("the shape did not round-trip: %s %v", got.Shape, err)
	}
}

// 000095 runs down and up on the real engine: down drops accepted_shape and
// nothing it does not own, up brings it back empty. Left where it began.
func TestLiveMigration95RunsDownAndUp(t *testing.T) {
	ctx := liveDB(t)
	var version int64
	var dirty bool
	if err := PG.QueryRow(ctx, `SELECT version, dirty FROM schema_migrations`).Scan(&version, &dirty); err != nil {
		t.Fatal(err)
	}
	if version < 95 || dirty {
		t.Skipf("the test database is at %d (dirty %t), before this branch's 95", version, dirty)
	}
	config, err := pgxpool.ParseConfig(os.Getenv("SYNDRA_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	driver, err := pgx5.WithInstance(stdlib.OpenDB(*config.ConnConfig), &pgx5.Config{})
	if err != nil {
		t.Fatal(err)
	}
	m, err := migrate.NewWithDatabaseInstance("file://../../db/migrations", "postgres", driver)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if v, _, _ := m.Version(); v != uint(version) {
			_ = m.Migrate(uint(version))
		}
	})
	seedAddonTarget(t, ctx)
	if err := SaveTargetFieldRoles(ctx, "truenas", FieldRoleMap{Roles: map[string]string{"enabled": "account"}, Shape: []byte(`{"kind":"storage"}`)}); err != nil {
		t.Fatal(err)
	}
	column := func() (has bool) {
		if err := PG.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'targets' AND column_name = 'accepted_shape')`).Scan(&has); err != nil {
			t.Fatal(err)
		}
		return has
	}
	if err := m.Migrate(94); err != nil {
		t.Fatalf("000095 down: %v", err)
	}
	var roles string
	if err := PG.QueryRow(ctx, `SELECT field_roles::text FROM targets WHERE target = 'truenas'`).Scan(&roles); column() || err != nil || roles == "" {
		t.Fatalf("after down: column %t, the role map %q (%v), want the column gone and the roles kept", column(), roles, err)
	}
	if err := m.Migrate(95); err != nil {
		t.Fatalf("000095 up: %v", err)
	}
	if got, found, err := TargetFieldRoles(ctx, "truenas"); !column() || err != nil || !found || got.Shape != nil {
		t.Fatalf("after up: column %t, found %t, shape %s (%v), want it back and empty", column(), found, got.Shape, err)
	}
}
