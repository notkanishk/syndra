package services

import (
	"context"
	"fmt"
	"sort"
	"time"

	"syndra/internal/db"
)

// Accounts Syndra created whose reason for existing has gone (change
// `addon-platform` 9.11/9.12; design §29).
//
// Dormancy is a BACKEND judgement, not a frontend filter, and the reason is the
// one rule this whole surface rests on: **anything an active role still grants
// never appears here.** A client computing that from a list of accounts and a
// list of roles would be re-deriving the resolver, and the first time the two
// disagreed the disagreement would be a person losing an account a role still
// confers.
//
// So it is resolved the same way every other question about entitlement is —
// through `ResolveEntitlements`, per bound subject — and an account whose
// subject resolves to any entitlement at all is not dormant, whatever else is
// true about them.

// Why an account has stopped meaning anything. Each cause has a different
// remedy, which is why the surface groups by it rather than listing forty rows
// under one heading.
const (
	// DormantMembershipEnded — nobody by that subject id is a member any more.
	// Housekeeping.
	DormantMembershipEnded = "membership_ended"
	// DormantRoleDeleted — they are still a member and nothing they hold reaches
	// this target. Same dormancy, opposite action: this is somebody who may be
	// quietly locked out, and removing the account makes that permanent.
	DormantRoleDeleted = "role_deleted"
	// DormantMappingRemoved — the role survives and no longer maps here.
	DormantMappingRemoved = "mapping_removed"
	// DormantNeverAssigned — an account bound to a subject who never had an
	// entitlement to begin with. Usually an adoption somebody thought better of.
	DormantNeverAssigned = "never_assigned"
)

// DormantAccount is one account with no live reason to exist.
type DormantAccount struct {
	Account   string `json:"account"`
	SubjectID string `json:"subject_id,omitempty"`
	// DisplayName is who it belonged to, when that is still knowable. An
	// account whose subject has left resolves to nothing, and the row says the
	// id rather than pretending to a name.
	DisplayName string `json:"display_name,omitempty"`
	Reason      string `json:"reason"`
	// SubjectStillMember is what separates housekeeping from a lockout, and it
	// is the field the surface makes unselectable on.
	SubjectStillMember bool      `json:"subject_still_member"`
	LastSeenAt         time.Time `json:"last_seen_at"`
	// BytesHeld is what the acknowledgement is actually about: the irreversible
	// part of removing these accounts is the data, not the row.
	//
	// A POINTER, because "we do not know" and "nothing" are different answers
	// and only one of them is safe to put in a sentence an operator ticks.
	// Nothing fills it yet: it needs a per-account usage read the add-on does
	// not perform (`pool.dataset.query`, the remainder of 6.19/6.20), and until
	// it does the surface says the size is unknown rather than saying zero.
	BytesHeld *int64 `json:"bytes_held,omitempty"`
}

// DormantReport is the listing, with the provenance every read from a target
// carries.
type DormantReport struct {
	Target string `json:"target"`
	// StateReadAt and Truncated for the same reason as everywhere else: an
	// absence read out of a capped list is not an absence, and this surface's
	// whole action is removing things on the strength of one.
	StateReadAt time.Time        `json:"state_read_at"`
	Truncated   bool             `json:"truncated"`
	Accounts    []DormantAccount `json:"accounts"`
}

// DormantAccounts lists the accounts on a target that nothing explains.
//
// It reads the target rather than only the bindings, because an account Syndra
// created and somebody deleted by hand is not dormant — it is gone, and
// offering to remove it would be offering to remove nothing.
func DormantAccounts(ctx context.Context, target string) (DormantReport, error) {
	if target == db.TargetZitadel {
		return DormantReport{}, fmt.Errorf("dormant accounts: %s holds no accounts of its own", target)
	}
	report := DormantReport{Target: target, Accounts: []DormantAccount{}}

	read := dormantSubjects(ctx, target)
	if !read.Usable() {
		// Refused rather than answered with an empty list. Every row here is a
		// candidate for removal, and concluding "this account has no reason to
		// exist" from a read that did not happen — or that stopped at a cap — is
		// the one conclusion this data cannot support.
		return DormantReport{}, fmt.Errorf("dormant accounts for %s: %w", target, ErrTargetUnplannable)
	}
	report.StateReadAt, report.Truncated = read.TakenAt, read.Truncated

	onTarget := make(map[string]int64, len(read.Accounts))
	for _, account := range read.Accounts {
		onTarget[account.Username] = 0
	}

	bindings, err := dormantBindings(ctx, target)
	if err != nil {
		return DormantReport{}, fmt.Errorf("read bindings for %s: %w", target, err)
	}

	for _, binding := range bindings {
		if _, present := onTarget[binding.Username]; !present {
			// Bound to an account that is not there. Not dormant — nothing to
			// remove — and the reconcile pass is what repairs the binding.
			continue
		}

		set, err := dormantResolve(ctx, binding.SubjectID, target)
		if err != nil {
			return DormantReport{}, fmt.Errorf("resolve %s on %s: %w", binding.SubjectID, target, err)
		}
		// The rule, in one line: anything an active role still grants never
		// appears here. Lifecycle first, because a subject whose account is
		// deliberately disabled by a hold is still entitled — holding access is
		// not the same as having no reason to hold it.
		if set.Lifecycle.Enabled || len(set.Fields) > 0 && anyValues(set.Fields) {
			continue
		}

		account := DormantAccount{
			Account:    binding.Username,
			SubjectID:  binding.SubjectID,
			LastSeenAt: binding.LastSeenAt,
		}
		account.DisplayName, account.SubjectStillMember = dormantSubjectStatus(ctx, binding.SubjectID)
		account.Reason = dormantReason(ctx, binding.SubjectID, target, account.SubjectStillMember)
		report.Accounts = append(report.Accounts, account)
	}

	// Stable, and by cause first: the surface groups on it, and a list that
	// reordered between reads would move the checkbox under the cursor.
	sort.Slice(report.Accounts, func(i, j int) bool {
		if report.Accounts[i].Reason != report.Accounts[j].Reason {
			return report.Accounts[i].Reason < report.Accounts[j].Reason
		}
		return report.Accounts[i].Account < report.Accounts[j].Account
	})
	return report, nil
}

// anyValues reports whether any field resolved to at least one value.
//
// A field present with an empty list is "manage this and make it empty", which
// is a live instruction — but it is not an entitlement, and an account holding
// only those has nothing a role still grants.
func anyValues(fields map[string][]string) bool {
	for _, values := range fields {
		if len(values) > 0 {
			return true
		}
	}
	return false
}

// dormantReason names WHY, because each cause has a different remedy and the
// difference between two of them is somebody being locked out.
func dormantReason(ctx context.Context, subjectID, target string, stillMember bool) string {
	if !stillMember {
		return DormantMembershipEnded
	}
	roles, err := dormantSubjectRoles(ctx, subjectID)
	if err != nil || len(roles) == 0 {
		// Unknown is reported as the conservative one. `role_deleted` is the
		// cause whose row is UNSELECTABLE, so a failure to classify makes an
		// account harder to remove rather than easier.
		return DormantRoleDeleted
	}
	// They hold roles and none of them reaches here. Either the mapping was
	// removed or the role that carried it was, and the two are the same remedy
	// from this surface: give them a role that grants it, or take the account
	// away deliberately.
	if mapped, err := dormantTargetMapped(ctx, target); err == nil && !mapped {
		return DormantMappingRemoved
	}
	return DormantRoleDeleted
}
