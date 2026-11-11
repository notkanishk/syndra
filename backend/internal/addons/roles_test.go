package addons

import (
	"context"
	"errors"
	"slices"
	"testing"

	"syndra/internal/db"
)

// The role map is the column (design §3): with no add-on registered at all,
// so nothing reachable and no manifest in memory, a target's roles still
// answer, which is what lets a revocation plan with the add-on down after a
// restart. No column is ErrNoManifest, never a guess.
func TestFieldRolesAnswerFromTheColumnWithTheRegistryEmpty(t *testing.T) {
	forgetRoles(t)
	if _, err := Get("ruby"); !errors.Is(err, ErrNotRegistered) {
		t.Fatalf("the registry must be empty for this test: %v", err)
	}
	if _, err := FieldRoles(context.Background(), "ruby"); !errors.Is(err, ErrNoManifest) {
		t.Fatalf("no column must be no roles: %v", err)
	}
	memRoles.Lock()
	memRoles.byTarget["ruby"] = db.FieldRoleMap{Roles: map[string]string{"active": RoleAccount, "web": RoleChannel, "groups": RoleMembership, "materials": RoleFlag}}
	memRoles.Unlock()

	roles, err := FieldRoles(context.Background(), "ruby")
	if err != nil {
		t.Fatal(err)
	}
	if roles.Account() != "active" || !slices.Equal(roles.Of(RoleChannel), []string{"web"}) {
		t.Fatalf("account %q, channels %v", roles.Account(), roles.Of(RoleChannel))
	}
	if !roles.IsLifecycle("active") || !roles.IsLifecycle("web") || roles.IsLifecycle("groups") || roles.IsLifecycle("enabled") {
		t.Fatal("only the account and its channels are lifecycle fields, by role")
	}
	if (Roles{"a": RoleAccount, "b": RoleAccount}).Account() != "" {
		t.Fatal("two account fields name no account")
	}
}

// A limit combines as the add-on combines it (design §3): max, and the
// unlimited value dominates; nothing given, or a value that is not a whole
// number, is no limit to compare.
func TestCombineLimit(t *testing.T) {
	quota := EntitlementField{Name: "home_quota", Role: RoleLimit, Unit: "bytes", Combine: "max", Unlimited: "0"}
	for _, tc := range []struct {
		values []string
		want   string
		ok     bool
	}{
		{[]string{"1073741824", "5368709120"}, "5368709120", true},
		{[]string{"5368709120", "0"}, "0", true},
		{[]string{"0"}, "0", true},
		{[]string{" 7 "}, "7", true},
		{nil, "", false},
		{[]string{"5 GiB"}, "", false},
		{[]string{"-1"}, "", false},
	} {
		if got, ok := quota.CombineLimit(tc.values); got != tc.want || ok != tc.ok {
			t.Errorf("CombineLimit(%v) = %q, %v; want %q, %v", tc.values, got, ok, tc.want, tc.ok)
		}
	}
	if _, ok := (EntitlementField{Name: "group", Role: RoleMembership}).CombineLimit([]string{"1"}); ok {
		t.Error("only a limit combines")
	}
}

// A field the manifest drops is retired, not forgotten (§37, addon-contract
// 2.1): it keeps its role in the store, so it can never come back meaning
// something else, and it leaves the role map every reader reads, so it is
// never sent to a target that now refuses it. Declared again, it returns. The
// account can never be retired: a manifest without it is refused and the last
// good map stands.
func TestADroppedFieldIsRetiredAndTheAccountNeverIs(t *testing.T) {
	registerTrueNAS(t)
	forgetRoles(t)
	if err := refreshWith(t, goodManifest()); err != nil {
		t.Fatal(err)
	}
	noChannel := goodManifest()
	noChannel.EntitlementSchema = noChannel.EntitlementSchema[:2] // group, enabled
	if err := refreshWith(t, noChannel); err != nil {
		t.Fatalf("dropping a channel is not a change of meaning: %v", err)
	}
	roles, err := FieldRoles(context.Background(), "truenas")
	if err != nil || roles.Declares("smb_enabled") || len(roles.Of(RoleChannel)) != 0 || roles.Account() != "enabled" {
		t.Fatalf("a dropped channel must leave the role map read: %v %v", roles, err)
	}
	memRoles.Lock()
	stored := memRoles.byTarget["truenas"]
	memRoles.Unlock()
	if stored.Roles["smb_enabled"] != RoleChannel || !slices.Equal(stored.Retired, []string{"smb_enabled"}) {
		t.Fatalf("the dropped channel must be kept with its role and marked retired: %+v", stored)
	}
	var refused *ErrManifestRefused
	if err := refreshWith(t, withField(goodManifest(), "smb_enabled", RoleFlag)); !errors.As(err, &refused) {
		t.Fatalf("a retired field came back meaning something else: %v", err)
	}

	if err := refreshWith(t, goodManifest()); err != nil {
		t.Fatal(err)
	}
	if roles, _ := FieldRoles(context.Background(), "truenas"); roles["smb_enabled"] != RoleChannel {
		t.Fatalf("declared again, the channel returns: %v", roles)
	}

	noAccount := goodManifest()
	noAccount.EntitlementSchema = []EntitlementField{noAccount.EntitlementSchema[0], noAccount.EntitlementSchema[2]}
	if err := refreshWith(t, noAccount); !errors.As(err, &refused) {
		t.Fatalf("a manifest retiring the account must be refused: %v", err)
	}
	if roles, _ := FieldRoles(context.Background(), "truenas"); roles.Account() != "enabled" || !roles.Declares("smb_enabled") {
		t.Fatalf("the last good map must stand: %v", roles)
	}
}
