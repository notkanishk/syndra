package services

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/zitadel"
)

// rolePipe is a fake Zitadel plus a log of everything the pipe did, in order,
// so a test can assert the trace committed before Zitadel was touched.
type rolePipe struct {
	failingRoleClient
	log   []string
	roles map[string]zitadel.ProjectRoleResult // what Zitadel holds, by key
	// sendErr is what the next write answers. landAnyway applies the write
	// first, the way a 503 after a retried POST or a timeout can.
	sendErr    error
	landAnyway bool
	readErr    error
	audits     []string
	audit      map[string]any
}

func (p *rolePipe) write(op string, apply func()) error {
	p.log = append(p.log, "zitadel:"+op)
	if p.sendErr == nil || p.landAnyway {
		apply()
	}
	return p.sendErr
}

func (p *rolePipe) AddProjectRole(_ context.Context, _, key, name, group string) error {
	return p.write("add", func() { p.roles[key] = zitadel.ProjectRoleResult{Key: key, DisplayName: name, Group: group} })
}

func (p *rolePipe) UpdateProjectRole(_ context.Context, _, key, name, group string) error {
	return p.write("update", func() { p.roles[key] = zitadel.ProjectRoleResult{Key: key, DisplayName: name, Group: group} })
}

func (p *rolePipe) DeleteProjectRole(_ context.Context, _, key string) error {
	return p.write("delete", func() { delete(p.roles, key) })
}

func (p *rolePipe) ListProjectRoles(context.Context, string, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ProjectRoleResult], error) {
	p.log = append(p.log, "zitadel:read")
	if p.readErr != nil {
		return nil, p.readErr
	}
	out := &zitadel.SearchResult[zitadel.ProjectRoleResult]{}
	for _, r := range p.roles {
		out.Items = append(out.Items, r)
	}
	return out, nil
}

// newRolePipe wires every seam the role pipe touches to the fake and the log.
func newRolePipe(t *testing.T) *rolePipe {
	t.Helper()
	resetRoleDeps(t)
	noopRoleDeps()
	p := &rolePipe{roles: map[string]zitadel.ProjectRoleResult{}}
	zitadel.MgmtClient = p
	svcInTxLockingAccess = func(ctx context.Context, fn func(context.Context) error) error {
		p.log = append(p.log, "tx")
		if err := fn(ctx); err != nil {
			p.log = append(p.log, "rollback")
			return err
		}
		p.log = append(p.log, "commit")
		return nil
	}
	svcInsertAuditLogDetails = func(_ context.Context, _, _, action, _ string, details any) error {
		p.log = append(p.log, "audit:"+action)
		p.audits = append(p.audits, action)
		if m, ok := details.(map[string]any); ok {
			p.audit = m
		}
		return nil
	}
	svcDbCreateRole = func(_ context.Context, _, key, _, _, _, _ string, _, _ *string) (string, error) {
		p.log = append(p.log, "local:create")
		return "row-" + key, nil
	}
	svcDbDeleteRole = func(_ context.Context, id string) error {
		p.log = append(p.log, "local:delete:"+id)
		return nil
	}
	svcDbUpdateRole = func(_ context.Context, _, _, name, _ string) error {
		p.log = append(p.log, "local:update:"+name)
		return nil
	}
	svcDbGetRole = func(_ context.Context, projectID, key string) (models.Role, error) {
		return models.Role{ID: "row-" + key, ProjectID: projectID, RoleKey: key, DisplayName: "Old name"}, nil
	}
	// Nobody holds anything and the read is whole, unless a test says otherwise.
	now := time.Now()
	roleMembersFor = func(context.Context, string, string) (RoleMembersView, error) {
		return RoleMembersView{ObservedOnly: []models.UserProfile{}, Observation: models.ObservationBasis{ReadAt: &now, Current: true}}, nil
	}
	return p
}

// beforeFirst asserts the trace (local row + audit, committed) precedes the
// first Zitadel write.
func (p *rolePipe) assertTracedBefore(t *testing.T, op string) {
	t.Helper()
	call := slices.Index(p.log, "zitadel:"+op)
	commit := slices.Index(p.log, "commit")
	audit := slices.IndexFunc(p.log, func(s string) bool { return strings.HasPrefix(s, "audit:role.") })
	if call < 0 || commit < 0 || audit < 0 || !(audit < commit && commit < call) {
		t.Fatalf("the trace must commit before Zitadel is called; log: %v", p.log)
	}
}

func TestRolePipe_EveryWriteIsTracedBeforeZitadel(t *testing.T) {
	cases := map[string]struct {
		op  string
		run func() error
	}{
		"create": {"add", func() error {
			_, err := CreateRole(context.Background(), CreateRoleRequest{ProjectID: "p1", RoleKey: "trained", DisplayName: "Trained"}, "op")
			return err
		}},
		"update": {"update", func() error {
			return UpdateRole(context.Background(), "p1", "trained", "Trained well", "", "op")
		}},
		"delete": {"delete", func() error {
			return DeleteRole(context.Background(), "p1", "trained", "op", 0)
		}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := newRolePipe(t)
			p.roles["trained"] = zitadel.ProjectRoleResult{Key: "trained", DisplayName: "Old name"}
			if err := tc.run(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			p.assertTracedBefore(t, tc.op)
			if len(p.audits) != 1 || !strings.HasPrefix(p.audits[0], "role.") {
				t.Fatalf("want exactly one role audit row, got %v", p.audits)
			}
		})
	}
}

func TestRolePipe_TraceFailureMeansNoZitadelCall(t *testing.T) {
	p := newRolePipe(t)
	svcInsertAuditLogDetails = func(context.Context, string, string, string, string, any) error {
		return errors.New("audit insert failed")
	}
	_, err := CreateRole(context.Background(), CreateRoleRequest{ProjectID: "p1", RoleKey: "trained"}, "op")
	if err == nil {
		t.Fatal("a failed trace must fail the write")
	}
	if slices.Contains(p.log, "zitadel:add") {
		t.Fatalf("no trace, no write: Zitadel was called anyway; log: %v", p.log)
	}
}

func TestCreateRole_Ambiguous503ReadsBackAndKeepsTheRole(t *testing.T) {
	p := newRolePipe(t)
	p.sendErr = &zitadel.StatusError{Code: 503}
	p.landAnyway = true

	role, err := CreateRole(context.Background(), CreateRoleRequest{ProjectID: "p1", RoleKey: "trained", DisplayName: "Trained"}, "op")
	if err != nil {
		t.Fatalf("the role landed; the 503 must not fail the create: %v", err)
	}
	if role.RoleKey != "trained" {
		t.Fatalf("want the created role back, got %+v", role)
	}
	if !slices.Contains(p.log, "zitadel:read") {
		t.Fatalf("an error must be read back before anything is undone; log: %v", p.log)
	}
	if slices.ContainsFunc(p.log, func(s string) bool { return strings.HasPrefix(s, "local:delete") }) {
		t.Fatalf("undoing the local row would orphan the role in Zitadel; log: %v", p.log)
	}
}

func TestCreateRole_RefusalReadBackShowsNothing_UndoesAndSaysSo(t *testing.T) {
	p := newRolePipe(t)
	p.sendErr = &zitadel.StatusError{Code: 400, Message: "invalid"}

	_, err := CreateRole(context.Background(), CreateRoleRequest{ProjectID: "p1", RoleKey: "trained"}, "op")
	if !errors.Is(err, ErrRoleNotChangedInZitadel) {
		t.Fatalf("want ErrRoleNotChangedInZitadel, got %v", err)
	}
	if !slices.Contains(p.log, "local:delete:row-trained") {
		t.Fatalf("the local row must be undone; log: %v", p.log)
	}
	if !slices.Equal(p.audits, []string{auditRoleCreated, auditRoleNotChanged}) {
		t.Fatalf("the undo must be audited too, got %v", p.audits)
	}
}

func TestCreateRole_UnreadableAfterError_UndoesRatherThanGuess(t *testing.T) {
	p := newRolePipe(t)
	p.sendErr = context.DeadlineExceeded
	p.readErr = errors.New("still down")

	_, err := CreateRole(context.Background(), CreateRoleRequest{ProjectID: "p1", RoleKey: "trained"}, "op")
	if !errors.Is(err, ErrRoleNotChangedInZitadel) {
		t.Fatalf("want ErrRoleNotChangedInZitadel, got %v", err)
	}
	if p.audit["read_back_error"] == nil {
		t.Fatalf("the undo's audit row must say the read-back failed: %v", p.audit)
	}
}

func TestUpdateRole_RefusalRestoresTheOldName(t *testing.T) {
	p := newRolePipe(t)
	p.roles["trained"] = zitadel.ProjectRoleResult{Key: "trained", DisplayName: "Old name"}
	p.sendErr = &zitadel.StatusError{Code: 503}

	err := UpdateRole(context.Background(), "p1", "trained", "New name", "", "op")
	if !errors.Is(err, ErrRoleNotChangedInZitadel) {
		t.Fatalf("want ErrRoleNotChangedInZitadel, got %v", err)
	}
	if !slices.Contains(p.log, "local:update:New name") || !slices.Contains(p.log, "local:update:Old name") {
		t.Fatalf("want the record renamed then put back; log: %v", p.log)
	}
}

func TestUpdateRole_ZitadelOnlyRoleHasNoLocalRowToChange(t *testing.T) {
	p := newRolePipe(t)
	svcDbGetRole = func(context.Context, string, string) (models.Role, error) {
		return models.Role{}, fmt.Errorf("get role: %w", pgx.ErrNoRows)
	}
	if err := UpdateRole(context.Background(), "p1", "trained", "New", "", "op"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if slices.ContainsFunc(p.log, func(s string) bool { return strings.HasPrefix(s, "local:update") }) {
		t.Fatalf("no local row, nothing local to update; log: %v", p.log)
	}
	p.assertTracedBefore(t, "update")
}

func TestDeleteRole_HeldInZitadel_RefusedUntilTheCountIsConfirmed(t *testing.T) {
	p := newRolePipe(t)
	p.roles["trained"] = zitadel.ProjectRoleResult{Key: "trained"}
	now := time.Now()
	roleMembersFor = func(context.Context, string, string) (RoleMembersView, error) {
		return RoleMembersView{
			ObservedOnly: []models.UserProfile{{ID: "u2", Name: "grace Hopper"}, {ID: "u1", Name: "Ada Lovelace"}},
			Observation:  models.ObservationBasis{ReadAt: &now, Current: true},
		}, nil
	}

	for _, confirm := range []int{0, 1, 3} {
		err := DeleteRole(context.Background(), "p1", "trained", "op", confirm)
		var held *RoleHeldError
		if !errors.As(err, &held) || held.Unknown {
			t.Fatalf("confirm=%d: want RoleHeldError, got %v", confirm, err)
		}
		if len(held.Holders) != 2 || held.Holders[0].ID != "u1" {
			t.Fatalf("want both holders, alphabetically, got %+v", held.Holders)
		}
	}
	if slices.Contains(p.log, "zitadel:delete") || len(p.audits) != 0 {
		t.Fatalf("a refused delete must not reach Zitadel or the log; log: %v audits: %v", p.log, p.audits)
	}

	if err := DeleteRole(context.Background(), "p1", "trained", "op", 2); err != nil {
		t.Fatalf("confirmed delete: %v", err)
	}
	if got := p.audit["holders_lost"]; !slices.Equal(got.([]string), []string{"u1", "u2"}) {
		t.Fatalf("the audit row must name who lost it, got %v", got)
	}
	p.assertTracedBefore(t, "delete")
}

func TestDeleteRole_RefusedWhileSyndraGivesIt(t *testing.T) {
	p := newRolePipe(t)
	svcDbGetRoleUsageCounts = func(context.Context) (map[string]db.RoleUsage, error) {
		return map[string]db.RoleUsage{"p1:trained": {BundleCount: 1}}, nil
	}
	err := DeleteRole(context.Background(), "p1", "trained", "op", 0)
	var inUse *RoleInUseError
	if !errors.As(err, &inUse) || inUse.Bundles != 1 {
		t.Fatalf("want RoleInUseError naming the bundle, got %v", err)
	}

	svcDbGetRoleUsageCounts = func(context.Context) (map[string]db.RoleUsage, error) { return nil, nil }
	roleMembersFor = func(context.Context, string, string) (RoleMembersView, error) {
		return RoleMembersView{Members: []RoleMember{{User: models.UserProfile{ID: "u1"}}}}, nil
	}
	if err := DeleteRole(context.Background(), "p1", "trained", "op", 1); !errors.As(err, &inUse) || inUse.Given != 1 {
		t.Fatalf("a confirmation must not override Syndra's own giving, got %v", err)
	}
	if slices.Contains(p.log, "zitadel:delete") {
		t.Fatalf("refused, yet Zitadel was called; log: %v", p.log)
	}
}

func TestDeleteRole_HoldersUnknown_Refused(t *testing.T) {
	cases := map[string]RoleMembersView{
		"never observed":   {},
		"last read failed": {ObservedOnly: []models.UserProfile{}, Observation: models.ObservationBasis{Current: false}},
		"read was capped":  {ObservedOnly: []models.UserProfile{}, Observation: models.ObservationBasis{Current: true, Truncated: true}},
	}
	for name, view := range cases {
		t.Run(name, func(t *testing.T) {
			p := newRolePipe(t)
			roleMembersFor = func(context.Context, string, string) (RoleMembersView, error) { return view, nil }
			err := DeleteRole(context.Background(), "p1", "trained", "op", 0)
			var held *RoleHeldError
			if !errors.As(err, &held) || !held.Unknown {
				t.Fatalf("want an unknown-holders refusal, got %v", err)
			}
			if slices.Contains(p.log, "zitadel:delete") {
				t.Fatalf("refused, yet Zitadel was called; log: %v", p.log)
			}
		})
	}
}

func TestDeleteRole_TimeoutButGone_IsDone(t *testing.T) {
	p := newRolePipe(t)
	p.roles["trained"] = zitadel.ProjectRoleResult{Key: "trained"}
	p.sendErr = context.DeadlineExceeded
	p.landAnyway = true

	if err := DeleteRole(context.Background(), "p1", "trained", "op", 0); err != nil {
		t.Fatalf("the role is gone; the timeout must not fail the delete: %v", err)
	}
	if slices.Contains(p.log, "local:create") {
		t.Fatalf("the local row must not come back for a role Zitadel no longer has; log: %v", p.log)
	}
}

func TestCreateRole_RejectsABadKeyBeforeAnything(t *testing.T) {
	p := newRolePipe(t)
	_, err := CreateRole(context.Background(), CreateRoleRequest{ProjectID: "p1", RoleKey: "has space"}, "op")
	if !errors.Is(err, ErrInvalidRoleKey) {
		t.Fatalf("want ErrInvalidRoleKey, got %v", err)
	}
	if len(p.log) != 0 {
		t.Fatalf("nothing may happen for an invalid key; log: %v", p.log)
	}
}
