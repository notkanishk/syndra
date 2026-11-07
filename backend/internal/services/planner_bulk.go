package services

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"

	"syndra/internal/clock"
	"syndra/internal/db"
	"syndra/internal/models"
)

// A bulk plan through the planner: one snapshot for every subject, read once
// under the lock (settled 5 Nov), one Change per subject, and an apply that
// reads it again, plans every row again, and writes only if every row is the
// one reviewed (decisions 3, 4). The subjects are people, or for a request
// decision the requests and for an adoption the drift rows (BulkRequest.UserIDs).

// ErrPlanRequired: a publish whose plan reached somebody was applied without
// citing it.
var ErrPlanRequired = errors.New("applying requires the plan_id returned by the rehearsal")

// action is one subject's part of the request.
func (req BulkRequest) action(id string) Action {
	a := Action{Op: req.Op, BundleID: req.BundleID, ProjectID: req.ProjectID, RoleKey: req.RoleKey, Days: req.DurationDays}
	switch req.Op {
	case OpApprove:
		a.RequestID = id
	case OpAdopt:
		a.DriftID = id
	default:
		a.UserID = id
	}
	return a
}

// cohortOp: one write repins every row's person together (publish, move).
func cohortOp(op string) bool { return op == OpPublish || op == OpMove }

// takesAway: an operation that takes access away leaves out the person asking
// (#32, B4): their own page is where they change it.
func takesAway(op string) bool { return op == BulkOpRemoveBundle || op == BulkOpRemoveRole }

// bulkPeople is who the plan is about, read before the lock so the directory
// can be asked about them: the subjects, or the people behind them.
func bulkPeople(ctx context.Context, req BulkRequest, subjects []string) ([]string, error) {
	var people []string
	switch req.Op {
	case OpApprove:
		for _, id := range subjects {
			if r, err := svcGetAccessRequestByID(ctx, id); err == nil {
				people = append(people, r.RequesterID)
			}
		}
	case OpAdopt:
		for _, id := range subjects {
			if item, err := svcGetDriftItem(ctx, id); err == nil {
				people = append(people, item.UserID)
			}
		}
	case OpPublish:
		holders, err := svcGetBundleHoldersByVersion(ctx, req.BundleID)
		if err != nil {
			return nil, err
		}
		for _, h := range holders {
			people = append(people, h.UserID)
		}
	default:
		people = subjects
	}
	return dedupeIDs(people), nil
}

// planSnapshot adds what Syndra's own tables say, read under the lock the
// caller holds: one pass for everyone in the plan. It returns the subjects,
// which for a publish are whoever holds the bundle now.
func planSnapshot(ctx context.Context, req BulkRequest, s PlanSnapshot, subjects []string) (PlanSnapshot, []string, error) {
	var err error
	s.Now = clock.Now()
	people := subjects
	if req.Op == OpExtend && len(req.GrantIDs) > 0 {
		s.Ticked = map[string]bool{}
		for _, id := range req.GrantIDs {
			s.Ticked[id] = true
		}
	}
	switch req.Op {
	case BulkOpAssignBundle, BulkOpRemoveBundle:
		if s.Bundle, err = svcGetBundleByID(ctx, req.BundleID); err != nil {
			return s, nil, err
		}
		if req.Op == BulkOpAssignBundle && s.Bundle.RetiredAt == nil {
			// The LATEST PUBLISHED version: what an assignment pins and gives.
			if s.Version, s.VersionRoles, err = svcLatestVersionRoles(ctx, req.BundleID); err != nil {
				return s, nil, err
			}
		}
	case OpPublish:
		if s.Draft, err = BundleDraft(ctx, req.BundleID); err != nil {
			return s, nil, err
		}
		// What the next version gives, includes and all.
		s.VersionRoles = s.Draft.WorkingEffective
		if s.VersionRoles == nil {
			s.VersionRoles = s.Draft.Working
		}
		s.Version = models.BundleVersion{BundleID: req.BundleID, Version: s.Draft.NextVersion}
		s.LeaveHolders = !req.Migrate
		if subjects, err = s.readHolders(ctx, req.BundleID); err != nil {
			return s, nil, err
		}
		people = subjects
	case OpMove:
		// Ownership is checked here: a version of another bundle made a plan
		// that read "v2 → v0".
		owns, err := svcVersionBelongsTo(ctx, req.BundleID, req.VersionID)
		if err != nil {
			return s, nil, err
		}
		if !owns {
			return s, nil, fmt.Errorf("version %s does not belong to bundle %s", req.VersionID, req.BundleID)
		}
		if s.VersionRoles, err = svcGetRolesForVersion(ctx, req.VersionID); err != nil {
			return s, nil, err
		}
		versions, err := svcListBundleVersions(ctx, req.BundleID)
		if err != nil {
			return s, nil, err
		}
		for _, v := range versions {
			if v.ID == req.VersionID {
				s.Version = v
			}
		}
		if _, err = s.readHolders(ctx, req.BundleID); err != nil {
			return s, nil, err
		}
	case OpApprove:
		s.Requests, s.Holdings, people = map[string]models.AccessRequest{}, map[string]models.RequestHolding{}, nil
		for _, id := range subjects {
			r, err := svcGetAccessRequestByID(ctx, id)
			if err != nil {
				continue // the row says it may have been withdrawn
			}
			if s.Holdings[id], err = svcRequestHolding(ctx, r.RequesterID, r.ProjectID, r.RoleKey); err != nil {
				return s, nil, err
			}
			s.Requests[id] = r
			people = append(people, r.RequesterID)
		}
	case OpAdopt, OpRevoke:
		s.Drift, people = map[string]models.DriftItem{}, nil
		for _, id := range subjects {
			if item, err := svcGetDriftItem(ctx, id); err == nil {
				s.Drift[id] = item
				people = append(people, item.UserID)
			}
		}
	}
	s.Access, err = svcReadAccessSnapshot(ctx, dedupeIDs(people))
	return s, subjects, err
}

// readHolders reads who holds the bundle, by person, and returns their ids.
func (s *PlanSnapshot) readHolders(ctx context.Context, bundleID string) ([]string, error) {
	holders, err := svcGetBundleHoldersByVersion(ctx, bundleID)
	if err != nil {
		return nil, err
	}
	s.Holders = make(map[string]models.BundleHolder, len(holders))
	ids := make([]string, 0, len(holders))
	for _, h := range holders {
		s.Holders[h.UserID] = h
		ids = append(ids, h.UserID)
	}
	sort.Strings(ids)
	return ids, nil
}

// requestFingerprint binds what a cohort's plan was reviewed against beyond
// its rows: a publish's version contents and holders, a move's version and
// people. Computed from the snapshot, so the apply checks it under the lock.
func (s PlanSnapshot) requestFingerprint(req BulkRequest, subjects []string) string {
	switch req.Op {
	case OpPublish:
		holders := make([]models.BundleHolder, 0, len(s.Holders))
		for _, h := range s.Holders {
			holders = append(holders, h)
		}
		return FingerprintBundlePublish(req.BundleID, req.Migrate, publishedContents(s.Draft), holders)
	case OpMove:
		return FingerprintMoveHolders(req.BundleID, req.VersionID, s.VersionRoles, subjects)
	}
	return ""
}

// rehearsePlanned is the preview: the snapshot read once under the lock, and
// the planner's row for every subject.
func rehearsePlanned(ctx context.Context, req BulkRequest) (BulkPlan, error) {
	plan, _, err := planBulk(ctx, req)
	return plan, err
}

// planBulk is the preview and the snapshot it was planned on.
func planBulk(ctx context.Context, req BulkRequest) (BulkPlan, PlanSnapshot, error) {
	changes, excluded, snap, subjects, err := planEvery(ctx, req)
	if err != nil {
		return BulkPlan{}, snap, err
	}
	plan := BulkPlan{Op: req.Op, Excluded: excluded, Outcomes: []BulkOutcome{}}
	for _, ch := range changes {
		plan.Outcomes = append(plan.Outcomes, ch.Row)
	}
	sortOutcomes(plan.Outcomes)
	plan.Summary = summarize(plan.Outcomes)
	plan.RequestFingerprint = snap.requestFingerprint(req, subjects)
	return plan, snap, nil
}

// PlanBulk reads the snapshot once under the lock and plans every subject, in
// the order selected.
func PlanBulk(ctx context.Context, req BulkRequest) ([]Change, []BulkExclusion, error) {
	changes, excluded, _, _, err := planEvery(ctx, req)
	return changes, excluded, err
}

func planEvery(ctx context.Context, req BulkRequest) ([]Change, []BulkExclusion, PlanSnapshot, []string, error) {
	subjects := dedupeIDs(req.UserIDs)
	people, err := bulkPeople(ctx, req, subjects)
	if err != nil {
		return nil, nil, PlanSnapshot{}, nil, err
	}
	snap, err := planDirectory(ctx, people)
	if err != nil {
		return nil, nil, snap, nil, err
	}
	if err := svcInTxLockingAccess(ctx, func(ctx context.Context) error {
		snap, subjects, err = planSnapshot(ctx, req, snap, subjects)
		return err
	}); err != nil {
		return nil, nil, snap, nil, err
	}
	var changes []Change
	var excluded []BulkExclusion
	for _, id := range subjects {
		if takesAway(req.Op) && req.Actor != "" && id == req.Actor {
			excluded = append(excluded, BulkExclusion{UserID: id, Reason: "self"})
			continue
		}
		changes = append(changes, Plan(req.action(id), snap))
	}
	return changes, excluded, snap, subjects, nil
}

// StalePlan is the apply's refusal when the fresh read gives a different
// Change for these subjects than the plan they reviewed: other writes, or a
// queued change the row never named (decisions 3, 4). Nothing was written and
// the plan is still unspent.
type StalePlan struct{ IDs []string }

func (e *StalePlan) Error() string {
	return fmt.Sprintf("the plan is stale for %d subject(s)", len(e.IDs))
}

// ApplyPlannedBulk is the apply. In one transaction under the lock it spends
// the plan, reads the snapshot again, plans every row again, and refuses the
// whole plan, unspent and with nothing written, if any row's Change differs
// from the one stored. Otherwise it writes every row, admitting only the
// cancellations the rows named. It returns the rows' words and, per subject,
// the Change as written, each write with its outbox row: sending it is the
// caller's (DeliverChange), after the commit, because the lock is never held
// across a call to Zitadel. For a publish or a move the request fingerprint
// is the snapshot's (cite's is ignored): the version and the cohort it binds
// are read under the lock with the rows.
func ApplyPlannedBulk(ctx context.Context, req BulkRequest, cite db.PlanCitation) (BulkPlan, map[string]Change, error) {
	plan := BulkPlan{Op: req.Op, Applied: true, Outcomes: []BulkOutcome{}}
	written := map[string]Change{}
	var subjects []db.PlanSubject
	var changes map[string]Change

	// The plan's people are the request's (the request fingerprint binds
	// them), so the directory is read before the lock like the preview's.
	people, err := bulkPeople(ctx, req, dedupeIDs(req.UserIDs))
	if err != nil {
		return BulkPlan{}, nil, err
	}
	fromZitadel, err := planDirectory(ctx, people)
	if err != nil {
		return BulkPlan{}, nil, err
	}
	err = withLockedAccess(ctx, func(ctx context.Context) error {
		var snap PlanSnapshot
		var err error
		if cohortOp(req.Op) {
			var fresh []string
			if snap, fresh, err = planSnapshot(ctx, req, fromZitadel, dedupeIDs(req.UserIDs)); err != nil {
				return err
			}
			cite.RequestFingerprint = snap.requestFingerprint(req, fresh)
			if cite.PlanID == "" {
				// Only a publish nobody holds has no plan: there was nobody to
				// approve. Somebody holding it now makes it one that needs one.
				if req.Op != OpPublish || len(fresh) > 0 {
					return ErrPlanRequired
				}
			} else if _, subjects, err = svcClaimPlanJoined(ctx, cite); err != nil {
				return err
			}
		} else {
			if _, subjects, err = svcClaimPlanJoined(ctx, cite); err != nil {
				return err
			}
			ids := make([]string, len(subjects))
			for i, s := range subjects {
				ids[i] = s.SubjectID
			}
			if snap, _, err = planSnapshot(ctx, req, fromZitadel, ids); err != nil {
				return err
			}
		}
		changes = make(map[string]Change, len(subjects))
		var moved, named []string
		for _, s := range subjects {
			ch := Plan(req.action(s.SubjectID), snap)
			if ch.Row.Fingerprint != s.Fingerprint || !slices.Equal(ch.Row.CancelIDs, s.Outcome.CancelIDs) {
				moved = append(moved, s.SubjectID)
				continue
			}
			changes[s.SubjectID] = ch
			named = append(named, s.Outcome.CancelIDs...)
		}
		if len(moved) > 0 {
			sort.Strings(moved)
			return &StalePlan{IDs: moved}
		}
		db.CancellationsNamed(ctx, named)

		if cohortOp(req.Op) {
			written, plan.Version, err = writeCohort(ctx, req, snap, subjects, changes)
			return err
		}
		for _, s := range subjects {
			ch := changes[s.SubjectID]
			if ch.Row.Effect != EffectApply {
				continue
			}
			if written[s.SubjectID], err = writeBulkRow(ctx, req, req.action(s.SubjectID), ch, snap); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return BulkPlan{}, nil, err
	}
	for _, s := range subjects {
		plan.Outcomes = append(plan.Outcomes, changes[s.SubjectID].Row)
	}
	if takesAway(req.Op) && req.Actor != "" && slices.Contains(dedupeIDs(req.UserIDs), req.Actor) {
		plan.Excluded = []BulkExclusion{{UserID: req.Actor, Reason: "self"}}
	}
	sortOutcomes(plan.Outcomes)
	plan.Summary = summarize(plan.Outcomes)
	return plan, written, nil
}

// writeBulkRow writes one row's Change in the caller's transaction, as the
// single act of the same kind writes it.
func writeBulkRow(ctx context.Context, req BulkRequest, a Action, ch Change, snap PlanSnapshot) (Change, error) {
	a, snap, _ = snap.resolve(a)
	ch, err := writeAct(ctx, a, ActWrite{Actor: req.Actor, Reason: req.Reason, Note: req.Note}, ch, snap)
	if err == nil && a.Op == OpApprove {
		err = decisionAudit(ctx, req.Actor, snap.Request)
	}
	return ch, err
}

// errNotAssigned: the assignment found them already holding the bundle, which
// the plan just checked under the same lock. Unreachable; refused, not trusted.
var errNotAssigned = errors.New("they already hold this bundle, which the plan said they didn't")

// writeChange writes one row's bundle assignment or its removal, its audit
// row and its outbox rows, in the caller's transaction, all with the
// operator's reason. It returns the Change with each write's row.
func writeChange(ctx context.Context, req BulkRequest, ch Change) (Change, error) {
	if err := deferTargetConvergence(ctx, req.Actor, ch.UserID, ch.changed()); err != nil {
		return ch, err
	}
	var ids []string
	var err error
	switch req.Op {
	case BulkOpAssignBundle:
		var assigned bool
		ids, assigned, err = svcAssignBundleAndEnqueue(ctx, req.Actor, req.Reason, ch.UserID, req.BundleID, ch.VersionID,
			ch.params(req.Actor, req.Reason))
		if err == nil && !assigned {
			err = errNotAssigned
		}
	case BulkOpRemoveBundle:
		ids, err = svcRemoveBundleFromUserAndEnqueue(ctx, req.Actor, req.Reason, ch.UserID, req.BundleID,
			ch.params(req.Actor, req.Reason))
	default:
		err = fmt.Errorf("the planner does not apply %q", req.Op)
	}
	if err != nil {
		return ch, err
	}
	return ch.written(ids)
}

// writeCohort writes a publish or a move: every row's person repinned in one
// write, the version cut first for a publish, and every row's writes queued
// with it. A row a guard stopped is not moved. It returns each moved row's
// Change with its outbox rows, and the version a publish cut.
func writeCohort(ctx context.Context, req BulkRequest, snap PlanSnapshot, subjects []db.PlanSubject, changes map[string]Change) (map[string]Change, models.BundleVersion, error) {
	var version models.BundleVersion
	var moved []string
	var params []db.EnqueueParams
	reason := "Bundle holder moved between versions"
	if req.Op == OpPublish {
		reason = fmt.Sprintf("Bundle published at v%d", snap.Draft.NextVersion)
	}
	for _, s := range subjects {
		ch := changes[s.SubjectID]
		if ch.Refusal != "" {
			continue
		}
		moved = append(moved, s.SubjectID)
		if err := deferTargetConvergence(ctx, req.Actor, ch.UserID, ch.changed()); err != nil {
			return nil, version, err
		}
		params = append(params, ch.params(req.Actor, reason)...)
	}
	var ids []string
	var err error
	switch req.Op {
	case OpPublish:
		if snap.Draft.Empty() {
			return nil, version, fmt.Errorf("nothing to publish: the bundle matches v%d", snap.Draft.LatestVersion)
		}
		includers, err := checkIncluderChoices(ctx, PublishRequest{BundleID: req.BundleID, Includers: req.Includers})
		if err != nil {
			return nil, version, err
		}
		// The version contains exactly what the plan was computed from.
		if version, ids, err = svcPublishVersionAndEnqueue(ctx, req.Actor, req.BundleID, req.Note, snap.Draft.Working, moved, params); err != nil {
			return nil, version, err
		}
		if err := svcSnapshotVersionIncludes(ctx, version.ID, snap.Draft.Includes); err != nil {
			return nil, version, err
		}
		// A child's publish never changes a parent's people: "Update" puts
		// the new version into the parent's working copy (B8).
		for _, parent := range includers {
			if _, err := svcSetWorkingInclude(ctx, req.Actor, parent, req.BundleID, version.Version); err != nil {
				return nil, version, err
			}
		}
	case OpMove:
		if len(moved) == 0 {
			return map[string]Change{}, version, nil
		}
		if ids, err = svcMoveHoldersAndEnqueue(ctx, req.Actor, req.BundleID, req.VersionID, moved, params); err != nil {
			return nil, version, err
		}
	}
	inOrder := make([]Change, len(moved))
	for i, id := range moved {
		inOrder[i] = changes[id]
	}
	paired, err := pairWritten(inOrder, ids)
	if err != nil {
		return nil, version, err
	}
	out := make(map[string]Change, len(moved))
	for i, id := range moved {
		out[id] = paired[i]
	}
	return out, version, nil
}
