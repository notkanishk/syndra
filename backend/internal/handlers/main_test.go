package handlers

import (
	"cmp"
	"context"
	"os"
	"testing"

	"syndra/internal/addons"
	"syndra/internal/claimserve"
	"syndra/internal/db"
	"syndra/internal/services"
)

// TestMain opens the project gate for the package by default. Almost every
// handler test is about something else and names whatever project it likes;
// the tests ABOUT the gate (target_project_test.go) put the real rules back.
// truenasRoles is the TrueNAS add-on's persisted role map, which the handlers'
// tests resolve against unless one says otherwise: the core names no field.
var truenasRoles = addons.Roles{"group": addons.RoleMembership, "home": addons.RoleFlag,
	"home_quota": addons.RoleLimit, "enabled": addons.RoleAccount, "smb_enabled": addons.RoleChannel}

// truenasSchema is its accepted manifest's entitlement schema.
var truenasSchema = []addons.EntitlementField{
	{Name: "group", Role: addons.RoleMembership, Label: "Group"},
	{Name: "home", Role: addons.RoleFlag, Label: "Gives a home folder"},
	{Name: "home_quota", Role: addons.RoleLimit, Label: "Home folder limit", Unit: "bytes", Combine: "max", Unlimited: "0", PerPerson: true},
	{Name: "enabled", Role: addons.RoleAccount},
	{Name: "smb_enabled", Role: addons.RoleChannel, Label: "SMB"},
}

func TestMain(m *testing.M) {
	addonsFieldRoles = func(context.Context, string) (addons.Roles, error) { return truenasRoles, nil }
	dbRequireTargetProject = func(context.Context, string) error { return nil }
	resolveMappingProject = func(_ context.Context, _ string, requested string) (string, error) { return requested, nil }
	svcConvergeProjectChange = func(context.Context, string, string, string) error { return nil }
	svcConvergeSubjectOn = func(context.Context, string, string, string) error { return nil }
	dbTargetProjects = func(context.Context) (map[string]string, error) { return map[string]string{}, nil }
	// No system is named by an operator unless a test says so (it reads Postgres).
	dbTargetName = func(context.Context, string) (string, error) { return "", nil }
	checkRoleOffered = func(context.Context, string, string) error { return nil }
	// No bundle includes another unless a test says so (B8).
	dbBundleIncludesAnything = func(context.Context, string) (bool, error) { return false, nil }
	dbRetireBlockers = func(context.Context, string) ([]string, error) { return nil, nil }
	dbInTxLockingAccess = func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }
	// The welcome switch is off unless a test says so (it reads Postgres).
	svcWelcomeClaims = func(context.Context) (claimserve.Welcome, error) { return claimserve.Welcome{}, nil }
	// A Change is delivered as its plan said, with nothing sent anywhere: a
	// write that waits waits with its reason, every other row is sent and
	// taken. Delivery tests stub their own.
	svcDeliverChange = func(_ context.Context, ch services.Change) services.Delivery {
		d := services.Delivery{Reason: map[string]string{}}
		for _, w := range ch.Writes {
			switch {
			case w.OutboxID == "":
			case w.Delivery == services.DeliveryWaits:
				d.Waiting = append(d.Waiting, w.OutboxID)
				d.Reason[w.OutboxID] = cmp.Or(w.WaitReason, db.WaitReasonRoleWaits)
			default:
				d.Sent = append(d.Sent, w.OutboxID)
			}
		}
		return d
	}
	os.Exit(m.Run())
}
