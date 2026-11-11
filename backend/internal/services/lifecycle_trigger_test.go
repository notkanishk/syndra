package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/models"
)

// 7.9/7.10 — a role change reaches a target when somebody mapped it there.
//
// The symmetry is the property: gaining a first mapped role creates the account
// through the apply itself, and losing the last one disables it, both through
// the same convergence with nothing special-cased in either direction.

type triggerHarness struct {
	mapped    map[string][]string
	queued    []db.SystemConvergence
	recordErr error
	targetErr error
}

func stubTrigger(t *testing.T, f *resolverFixture, mapped map[string][]string) *triggerHarness {
	t.Helper()
	f.install(t)
	h := &triggerHarness{mapped: mapped}

	origTargets, origRecord := dbTargetsMappedToRole, dbRecordSystemConvergence
	t.Cleanup(func() { dbTargetsMappedToRole, dbRecordSystemConvergence = origTargets, origRecord })

	dbTargetsMappedToRole = func(_ context.Context, project, role string) ([]string, error) {
		return h.mapped[project+"/"+role], h.targetErr
	}
	dbRecordSystemConvergence = func(_ context.Context, c db.SystemConvergence) (string, string, error) {
		h.queued = append(h.queued, c)
		return "plan_1", "outbox_1", h.recordErr
	}
	return h
}

// Gaining a first mapped role queues a convergence whose intent enables the
// account. Creation is part of that convergence rather than a call sequenced
// before it — which is what dissolves the ordering problem instead of answering
// it.
func TestGainingAMappedRoleQueuesAConvergenceThatEnablesTheAccount(t *testing.T) {
	f := &resolverFixture{
		roles:    []db.RoleRef{{ProjectID: "pLab", RoleKey: "trained"}},
		mappings: []db.RoleMapping{mapping("trained", "group", "lab_makers")},
	}
	h := stubTrigger(t, f, map[string][]string{"pLab/trained": {"truenas"}})

	err := triggerTargetConvergence(context.Background(), "op_1", "u1",
		[]roleKey{{projectID: "pLab", roleKey: "trained"}})
	if err != nil {
		t.Fatalf("triggerTargetConvergence: %v", err)
	}
	if len(h.queued) != 1 {
		t.Fatalf("want one convergence, got %d", len(h.queued))
	}
	got := h.queued[0]
	if got.Target != "truenas" || got.SubjectID != "u1" || got.Actor != "op_1" {
		t.Errorf("convergence queued for the wrong thing: %+v", got)
	}
	if string(got.Desired["enabled"]) != "true" {
		t.Errorf("a subject holding a mapped role must be enabled: %s", got.Desired["enabled"])
	}
	if string(got.Desired["group"]) != `["lab_makers"]` {
		t.Errorf("the mapped value did not reach the intent: %s", got.Desired["group"])
	}
}

// Losing the last mapped role disables and never deletes. Deprovisioning is
// reversible by construction (design §12): the account and its home data stay,
// and regaining a role restores it through the same apply.
func TestLosingTheLastMappedRoleDisablesRatherThanDeleting(t *testing.T) {
	// The resolver sees no roles left — this is the state AFTER the removal,
	// which is what the trigger resolves against.
	f := &resolverFixture{}
	h := stubTrigger(t, f, map[string][]string{"pLab/trained": {"truenas"}})

	err := triggerTargetConvergence(context.Background(), "op_1", "u1",
		[]roleKey{{projectID: "pLab", roleKey: "trained"}})
	if err != nil {
		t.Fatalf("triggerTargetConvergence: %v", err)
	}
	if len(h.queued) != 1 {
		t.Fatalf("want one convergence, got %d", len(h.queued))
	}
	got := h.queued[0]
	if string(got.Desired["enabled"]) != "false" || string(got.Desired["smb_enabled"]) != "false" {
		t.Errorf("losing the last mapped role must disable: %v", got.Desired)
	}
	// Nothing in the intent says delete, and there is nowhere for it to: the
	// entitlement schema has no such field. Asserted anyway, because the reason
	// it is absent is a design decision and not an oversight.
	for field := range got.Desired {
		if strings.Contains(field, "delete") || strings.Contains(field, "purge") {
			t.Errorf("an automatic convergence must never carry %q", field)
		}
	}
}

// A role nobody mapped reaches nothing. This is the common case and it has to
// stay free — one indexed read, no convergence, no plan row.
func TestAnUnmappedRoleTriggersNothing(t *testing.T) {
	f := &resolverFixture{}
	h := stubTrigger(t, f, nil)

	err := triggerTargetConvergence(context.Background(), "op_1", "u1",
		[]roleKey{{projectID: "pLab", roleKey: "unmapped"}})
	if err != nil {
		t.Fatalf("triggerTargetConvergence: %v", err)
	}
	if len(h.queued) != 0 {
		t.Fatalf("an unmapped role must queue nothing, got %+v", h.queued)
	}
}

// Two changed roles mapped to one target produce ONE convergence, not two. The
// apply is level-triggered and carries the whole resolved set, so a second row
// would dispatch the same state twice.
func TestTwoChangedRolesOnOneTargetConvergeOnce(t *testing.T) {
	f := &resolverFixture{
		roles: []db.RoleRef{{ProjectID: "pLab", RoleKey: "trained"}},
		mappings: []db.RoleMapping{
			mapping("trained", "group", "lab_makers"),
		},
	}
	h := stubTrigger(t, f, map[string][]string{
		"pLab/trained": {"truenas"},
		"pLab/safety":  {"truenas"},
	})

	err := triggerTargetConvergence(context.Background(), "op_1", "u1", []roleKey{
		{projectID: "pLab", roleKey: "trained"},
		{projectID: "pLab", roleKey: "safety"},
	})
	if err != nil {
		t.Fatalf("triggerTargetConvergence: %v", err)
	}
	if len(h.queued) != 1 {
		t.Fatalf("want one convergence for one target, got %d", len(h.queued))
	}
}

// A convergence that cannot be queued fails the transaction rather than being
// logged past. The role change and the convergence that follows from it commit
// together, or the person's access changes in Zitadel and nowhere else with
// nothing on any surface disagreeing.
func TestAConvergenceThatCannotBeQueuedFailsTheChange(t *testing.T) {
	f := &resolverFixture{roles: []db.RoleRef{{ProjectID: "pLab", RoleKey: "trained"}}}
	h := stubTrigger(t, f, map[string][]string{"pLab/trained": {"truenas"}})
	h.recordErr = errors.New("the outbox is unavailable")

	err := triggerTargetConvergence(context.Background(), "op_1", "u1",
		[]roleKey{{projectID: "pLab", roleKey: "trained"}})
	if err == nil {
		t.Fatal("a convergence that failed to queue must fail the role change with it")
	}
}

// The delta is what decides WHETHER to converge; the resolved set is what is
// converged TO. Sending the delta would be a level-triggered apply carrying only
// what moved, which removes everything that did not.
func TestTheIntentIsTheWholeResolvedSetAndNotTheDelta(t *testing.T) {
	f := &resolverFixture{
		roles: []db.RoleRef{
			{ProjectID: "pLab", RoleKey: "trained"},
			{ProjectID: "pLab", RoleKey: "safety"},
		},
		mappings: []db.RoleMapping{
			mapping("trained", "group", "lab_makers"),
			mapping("safety", "group", "fabrication"),
		},
	}
	h := stubTrigger(t, f, map[string][]string{"pLab/safety": {"truenas"}})

	// Only `safety` changed, and the intent must still carry both groups.
	err := triggerTargetConvergence(context.Background(), "op_1", "u1",
		[]roleKey{{projectID: "pLab", roleKey: "safety"}})
	if err != nil {
		t.Fatalf("triggerTargetConvergence: %v", err)
	}
	if got := string(h.queued[0].Desired["group"]); got != `["fabrication","lab_makers"]` {
		t.Errorf("the intent must be the whole set the subject should hold, got %s", got)
	}
}

// The trigger hangs off the planner's writes: every planned Change reaches the
// outbox through one of these, and each registers the convergence its changed
// roles reach before it builds the params (systemParams for system work,
// task 1.9; writeAct, writeChange and writeCohort for the operator's acts). A
// source guard, since an omission is invisible to every behavioural test that
// does not know to look for it.
func TestTheTriggerHangsOffThePlannersWrites(t *testing.T) {
	for file, fn := range map[string]string{
		"planner_system.go": "systemParams", "planner_act.go": "writeAct",
		"planner_bulk.go": "writeChange",
	} {
		body := sliceBetween(t, readServiceSource(t, file), "func "+fn+"(", "\nfunc ")
		if !strings.Contains(body, "deferTargetConvergence(") {
			t.Errorf("%s no longer registers the lifecycle trigger, so a change reaches Zitadel and no mapped target", fn)
		}
		// It must REGISTER, never resolve: this runs before the source write,
		// so resolving here reads the world the change has not happened in.
		if strings.Contains(body, "triggerTargetConvergence(") {
			t.Errorf("%s resolves the entitlement set before the source write, which inverts it", fn)
		}
		if strings.Index(body, "deferTargetConvergence(") > strings.Index(body, ".params(") {
			t.Errorf("%s: the trigger must register before the params are built, so a failure to register stops the change", fn)
		}
	}

	// And nothing outside the planner turns a closure delta into enqueue
	// params by hand: that would bypass the trigger entirely.
	for _, file := range []string{"bundle_publish.go", "role_members.go", "cascade.go"} {
		if strings.Contains(readServiceSource(t, file), "db.EnqueueParams{") {
			t.Errorf("%s builds enqueue params directly instead of planning them", file)
		}
	}
}

// Every cascade takes the lock through the wrapper that flushes the trigger.
// One that took `svcInTxLockingAccess` directly would build deltas into a
// context with no pending list — a role change with no path to its mapped
// targets — and a source guard is the only thing that catches the omission,
// because the cascade's own tests stub the resolver and never notice.
func TestEveryCascadeTakesTheLockThroughTheFlushingWrapper(t *testing.T) {
	for _, file := range []string{"cascade.go", "bundle_publish.go", "role_members.go"} {
		src := readServiceSource(t, file)
		if strings.Contains(src, "svcInTxLockingAccess(ctx,") {
			t.Errorf("%s takes the access lock directly, so its deltas never reach the lifecycle trigger", file)
		}
	}
	wrapper := sliceBetween(t, readServiceSource(t, "lifecycle_trigger.go"), "func withLockedAccess(", "\nfunc ")
	if strings.Index(wrapper, "fn(ctx)") > strings.Index(wrapper, "triggerTargetConvergence(") {
		t.Error("the flush must run after fn, or it resolves the state before the write again")
	}
}

// A delta built outside the wrapper fails loudly. Silently skipping the trigger
// would be the same divergence in a quieter form: the roles commit, the target
// never hears, and no surface disagrees.
func TestADeltaBuiltOutsideTheLockIsRefused(t *testing.T) {
	err := deferTargetConvergence(context.Background(), "op_1", "u1",
		[]roleKey{{projectID: "pLab", roleKey: "trained"}})
	if !errors.Is(err, errConvergenceOutsideLock) {
		t.Fatalf("want a refusal outside the lock, got %v", err)
	}
}

// The one test that would have caught the inversion: it drives a real cascade
// rather than calling the trigger with an after-state fixture no caller
// produces.
//
// The fixture's answer changes when the assignment is written, exactly as the
// database's would. Resolving at `deltaParams` — before that write — reads a
// subject who holds no mapped role and queues "disable the account", which is
// the inverse of what assigning the bundle means.
func TestAssigningABundleQueuesTheStateAFTERTheAssignment(t *testing.T) {
	resetCascadeDeps(t)

	// Nothing is held until svcAssignBundleAndEnqueue runs, and the mapping read
	// answers for the roles it is actually given — a stub that returns the
	// mapping regardless of the role set would resolve the group either way and
	// hide the very thing this test is for.
	var assigned bool
	origRefs, origMappings, origAllowances := svcEffectiveRoleRefs, dbMappingsForRoles, dbAllowancesInForce
	t.Cleanup(func() {
		svcEffectiveRoleRefs, dbMappingsForRoles, dbAllowancesInForce = origRefs, origMappings, origAllowances
	})
	svcEffectiveRoleRefs = func(context.Context, string) ([]db.RoleRef, error) {
		if !assigned {
			return nil, nil
		}
		return []db.RoleRef{{ProjectID: "pLab", RoleKey: "trained"}}, nil
	}
	dbMappingsForRoles = func(_ context.Context, _ string, roles []db.RoleRef) ([]db.RoleMapping, error) {
		var out []db.RoleMapping
		for _, r := range roles {
			if r.RoleKey == "trained" {
				out = append(out, mapping("trained", "group", "lab_makers"))
			}
		}
		return out, nil
	}
	dbAllowancesInForce = func(context.Context, string, string) ([]db.Allowance, error) { return nil, nil }

	var queued []db.SystemConvergence
	origTargets, origRecord := dbTargetsMappedToRole, dbRecordSystemConvergence
	t.Cleanup(func() { dbTargetsMappedToRole, dbRecordSystemConvergence = origTargets, origRecord })
	dbTargetsMappedToRole = func(_ context.Context, project, role string) ([]string, error) {
		if project == "pLab" && role == "trained" {
			return []string{"truenas"}, nil
		}
		return nil, nil
	}
	dbRecordSystemConvergence = func(_ context.Context, c db.SystemConvergence) (string, string, error) {
		queued = append(queued, c)
		return "plan_1", "outbox_1", nil
	}

	svcGetBundleByID = func(ctx context.Context, id string) (models.Bundle, error) {
		return models.Bundle{ID: id}, nil
	}
	svcLatestVersionRoles = func(context.Context, string) (models.BundleVersion, []models.BundleRole, error) {
		return models.BundleVersion{ID: "v1", Version: 1},
			[]models.BundleRole{{ProjectID: "pLab", RoleKey: "trained"}}, nil
	}
	svcGetDirectGrantsForUser = noDirects
	svcGetUserBundleRolesGrouped = noBundleRoles
	svcGetActiveMappingRules = noRules
	svcAssignBundleAndEnqueue = func(context.Context, string, string, string, string, string, []db.EnqueueParams) ([]string, bool, error) {
		assigned = true
		return []string{"o1"}, true, nil
	}

	if _, err := CascadeBundleAssignedToUser(context.Background(), "op_1", "u1", "b1"); err != nil {
		t.Fatalf("CascadeBundleAssignedToUser: %v", err)
	}
	if len(queued) != 1 {
		t.Fatalf("want one convergence, got %d", len(queued))
	}
	if string(queued[0].Desired["enabled"]) != "true" {
		t.Errorf("gaining a mapped role must queue an enabled account, got %s", queued[0].Desired["enabled"])
	}
	if string(queued[0].Desired["group"]) != `["lab_makers"]` {
		t.Errorf("the group the bundle confers must reach the intent, got %s", queued[0].Desired["group"])
	}
}

func readServiceSource(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(raw)
}

func sliceBetween(t *testing.T, src, from, to string) string {
	t.Helper()
	i := strings.Index(src, from)
	if i < 0 {
		t.Fatalf("source does not contain %q", from)
	}
	rest := src[i+len(from):]
	if j := strings.Index(rest, to); j >= 0 {
		return rest[:j]
	}
	return rest
}

// The deactivation webhook's convergence (ConvergeSubjectOn) is built from the
// resolved set, so a departure is queued withdraw-only and drains on its own.
func TestConvergeSubjectOn_ADepartureIsWithdrawOnly(t *testing.T) {
	origResolve, origRecord := svcResolveEntitlements, dbRecordSystemConvergence
	t.Cleanup(func() { svcResolveEntitlements, dbRecordSystemConvergence = origResolve, origRecord })
	svcResolveEntitlements = func(context.Context, string, string) (EntitlementSet, error) {
		return EntitlementSet{Roles: truenasRoles, Lifecycle: LifecycleState{NotAMember: true}}, nil
	}
	var got db.SystemConvergence
	dbRecordSystemConvergence = func(_ context.Context, c db.SystemConvergence) (string, string, error) {
		got = c
		return "p", "o", nil
	}
	if err := convergeOn(context.Background(), "zitadel:user_deactivated", "u1", "truenas"); err != nil {
		t.Fatal(err)
	}
	if !got.WithdrawsOnly || got.Target != "truenas" || got.SubjectID != "u1" || string(got.Desired[FieldEnabled]) != "false" {
		t.Fatalf("queued %+v, want truenas/u1 withdraw-only, the account switched off", got)
	}
}

// A role change reaching two targets, one of which has no role map yet (its
// add-on has never had a manifest accepted), queues the other and does not
// fail: nothing says which field is that target's account, and rolling back
// the role change over it would block every grant until it speaks. The drift
// sweep converges it once its manifest is accepted (addon-contract 2.1).
func TestATargetWithNoRoleMapIsSkippedNotFatal(t *testing.T) {
	f := &resolverFixture{
		roles:    []db.RoleRef{{ProjectID: "pLab", RoleKey: "trained"}},
		mappings: []db.RoleMapping{mapping("trained", "group", "lab_makers")},
	}
	h := stubTrigger(t, f, map[string][]string{"pLab/trained": {"truenas", "unifi"}})
	orig := svcFieldRoles
	t.Cleanup(func() { svcFieldRoles = orig })
	svcFieldRoles = func(_ context.Context, target string) (addons.Roles, error) {
		if target == "unifi" {
			return nil, fmt.Errorf("%w: unifi has no accepted contract-2 manifest yet", addons.ErrNoManifest)
		}
		return truenasRoles, nil
	}

	if err := triggerTargetConvergence(context.Background(), "op_1", "u1", []roleKey{{projectID: "pLab", roleKey: "trained"}}); err != nil {
		t.Fatalf("a target with no role map must not fail the role change: %v", err)
	}
	if len(h.queued) != 1 || h.queued[0].Target != "truenas" {
		t.Fatalf("want truenas queued and unifi skipped, got %+v", h.queued)
	}

	// Any other failure to resolve still fails the change.
	svcFieldRoles = func(context.Context, string) (addons.Roles, error) { return nil, errors.New("db down") }
	if err := triggerTargetConvergence(context.Background(), "op_1", "u1", []roleKey{{projectID: "pLab", roleKey: "trained"}}); err == nil {
		t.Fatal("an unreadable role map is not a target that has not spoken")
	}
}
