// Package drift contains the backend-side scheduler that periodically
// reconciles Zitadel grants against Syndra's expected set (direct grants +
// rule-derived expectations + operator exclusions), flagging unexplained
// Zitadel grants as drift and replaying missed direct-grant propagations.
package drift

import (
	"context"
	"encoding/json"

	"syndra/internal/addons"

	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/services"
)

// Injectable dependencies. Mirrors the save-swap-restore pattern used across
// the backend (see services/expiry/deps.go). Tests exercise sweep logic
// without a live DB/Zitadel by swapping these.
var (
	svcAllDirectGrants = func(ctx context.Context) ([]models.DirectGrant, error) {
		return db.GetAllDirectGrants(ctx, false) // active grants only — expired grants are not expected in Zitadel
	}
	// What the bundles account for, deployment-wide, resolved through each
	// holder's PINNED version. Without this the sweep had no way to know a
	// bundle explains a live grant, and reported every projected bundle role as
	// drift for ever. See db.GetAllBundleDerivedGrants.
	svcAllBundleDerivedGrants = db.GetAllBundleDerivedGrants
	svcGetActiveMappingRules  = db.GetActiveMappingRules
	svcGetExclusions          = func(ctx context.Context, target string) ([]models.ExternalGrantExclusion, error) {
		return db.GetExclusions(ctx, target)
	}
	// (ctx,target,user,project,roleKeys,grantID,source,type,evidence) (id,inserted,err).
	// Carries db.DriftEvidence so a finding can cite the observation that
	// produced it — see the two call sites in sweep.go and driftItemSelect's
	// doc comment in db/drift.go.
	upsertDriftItem = db.UpsertDriftItemWithEvidence

	// The retraction half. A finding the sweep raised because it could not see
	// Syndra's intent has to be closable by the sweep once it can — otherwise
	// the only exit is the operator's Adopt button, which writes a redundant
	// direct grant and quietly breaks bundle removal.
	svcPendingDriftItems = func(ctx context.Context, target string) ([]models.DriftItem, error) {
		// Empty Status defaults to pending_triage, and the target narrows the
		// read to the system this sweep actually looked at.
		return db.GetDriftItems(ctx, db.DriftFilter{Target: target})
	}
	retractExplainedDrift = db.RetractExplainedDrift

	// The merge base, written by the Zitadel sweep from its own complete read
	// and forgotten when a user holds nothing. Seams, because the assertions
	// that matter are about what is NOT written: a base must not advance past a
	// finding, and nothing may be recorded from a truncated read.
	// What Syndra landed and the target accepted. Read beside the bases, because
	// the two answer "was this ever really there" from different sides — a read
	// that may never have happened, and a write that certainly did.
	listPropagations = db.PropagationsFor

	saveMergeBase          = db.RecordMergeBase
	forgetMergeBase        = db.ForgetMergeBase
	pendingOutboxAddExists = db.PendingOutboxAddExists   // (ctx,target,user,project,role) (bool,err) — dedupes syndra_only replay
	insertPending          = db.InsertPendingPropagation // re-enqueue path (syndra_only) — Zitadel-shaped by construction

	// What the sweep reads instead of Zitadel (one-truth-many-checks, "The last
	// two readers"). The org observation's own age is what the sweep cites and
	// what bounds what it may conclude — never a per-row timestamp, because an
	// out-of-band grant is not in the store at all until a sweep covers it, and
	// no row can speak for an absence.
	latestOrgObservation = db.LatestOrgObservation // ErrNoObservation ⇒ "not checked yet"
	allObservedGrants     = db.AllObservedGrants

	// idempotency-key minting for re-enqueued rows: reuse the outbox's crypto/rand
	// helper (the repo has NO uuid module). Returns (string, error); the sweep
	// handles the error.
	newIdempotencyKey = db.NewOutboxIdempotencyKey // () (string, error)

	// How current Syndra's picture of the target is. Written by the sweep
	// itself because the sweep is the only thing that knows whether the read it
	// consumed was one it can stand behind.
	markUnreconciled = db.MarkTargetUnreconciled // (ctx, target, reason)
	markReconciled   = db.MarkTargetReconciled   // (ctx, target)
)

// classification helpers are pure and shared with the reconciliation endpoint.
var (
	buildHolderSet  = services.BuildHolderSet
	expectedViaRule = services.ExpectedViaRule
	isExcluded      = services.IsExcluded
)

// The add-on reconciler's seams (1.18, 1.22).
//
// Separate from the Zitadel sweep's on purpose: they are a different reader with
// different failure modes, and a test has to be able to fail either — a target
// that answered from its mirror and was diffed anyway, an unmanaged account
// entered into triage, a convergence queued for a blocked row.
var (
	addonSubjects = addons.Subjects
	addonPlan     = addons.Plan
	listBindings  = db.ListTargetBindings

	recordConvergence = db.RecordSystemConvergence

	// The mutation-log anchor (2.28). Two seams rather than one, because the
	// read and the comparison fail differently: a health read that did not
	// happen is no evidence, and a comparison that says the chain was trimmed is
	// the strongest evidence this system produces.
	addonHealth   = addons.Health
	anchorLogHead = db.RecordLogHead

	// The merge bases for a whole target, read once per pass. Its own seam
	// because the assertions that matter are about what the sweep does WITHOUT
	// one: a subject with no base must converge exactly as it did before this
	// mechanism existed, and a test proving that has to be able to answer with
	// nothing.
	listMergeBases = db.MergeBasesFor

	// The durable half. Separate seams from the base's, because the assertions
	// are opposites: a finding must be WRITTEN for a difference the pass may not
	// resolve, and a base must NOT be advanced past one.
	saveMergeFinding  = db.RecordMergeFinding
	clearMergeFinding = db.ClearMergeFinding

	resolveIntent = func(ctx context.Context, subjectID, target string) (map[string]json.RawMessage, error) {
		set, err := services.ResolveEntitlements(ctx, subjectID, target)
		if err != nil {
			return nil, err
		}
		return set.Desired(), nil
	}
)
