package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// The differences a reconciliation may not resolve (change
// `reconciliation-as-merge`).
//
// A pass that can name the CAUSE of a difference can also say which causes are
// not its to act on: the target moved and Syndra did not, both moved
// differently, or the account is gone. Each needs a person, and a state that
// needs a person has to outlive the pass that found it — the same reason
// `target_binding_conflicts` exists, learned again on the state that occurs most.

// ErrNoSuchMergeFinding is a finding cited that does not exist or is already
// resolved. Refused rather than treated as done: resolving one twice would let
// a second operator believe they made a decision somebody else had already made
// differently.
var ErrNoSuchMergeFinding = errors.New("db: no standing merge finding")

// ErrMergeFindingDecided refuses a second answer to a question somebody has
// already answered.
//
// The decisions here are opposites — recreate the account, or stop managing it
// — and the first one's work is queued the moment it is made. Taking the second
// would make the outcome depend on which request arrived last, and for
// `unbound` it would release the account on the target while a re-provision sat
// in the outbox.
var ErrMergeFindingDecided = errors.New("db: that finding has already been decided")

// Merge finding resolutions, as the surface offers them.
const (
	// ResolutionKeepOurs applies Syndra's state over the target's. The old
	// default, now a decision with a name on it.
	ResolutionKeepOurs = "keep_ours"
	// ResolutionTakeTheirs adopts the target's value into the desired state,
	// through a per-subject decision that can actually hold it.
	ResolutionTakeTheirs = "take_theirs"
	// ResolutionReprovisioned answers a deleted-upstream account by making it
	// again.
	ResolutionReprovisioned = "reprovisioned"
	// ResolutionUnbound answers it the other way: Syndra stops managing the
	// account rather than recreating it.
	ResolutionUnbound = "unbound"
	// ResolutionAgreed is the one nobody chooses. The two sides now match — a
	// policy changed, or the target did — so the disagreement this row records
	// has stopped existing. Closing it is not automatic resolution: nothing was
	// decided and nothing was written. Leaving it open would be the other way to
	// make a queue unreadable, filling it with problems that are already over.
	ResolutionAgreed = "agreed"
)

// MergeFinding is one standing difference.
type MergeFinding struct {
	ID        string `json:"id"`
	Target    string `json:"target"`
	SubjectID string `json:"subject_id"`
	// Field is empty for an account-level finding.
	Field   string `json:"field,omitempty"`
	Outcome string `json:"outcome"`
	// The three values as the classifier saw them. Absent for a
	// `deleted_upstream`, which is about the account rather than a value.
	Base       json.RawMessage `json:"base,omitempty"`
	Ours       json.RawMessage `json:"ours,omitempty"`
	Theirs     json.RawMessage `json:"theirs,omitempty"`
	DetectedAt time.Time       `json:"detected_at"`
	LastSeenAt time.Time       `json:"last_seen_at"`
	// Decision is what somebody chose before the target caught up. A decided
	// finding is still STANDING: keeping Syndra's state queues a convergence,
	// and closing the row at the moment of the decision would claim a difference
	// was over while it was still there — which the next pass would then raise
	// again as a second finding about the same field.
	Decision  string     `json:"decision,omitempty"`
	DecidedBy string     `json:"decided_by,omitempty"`
	DecidedAt *time.Time `json:"decided_at,omitempty"`
}

// RecordMergeFinding raises a finding, or refreshes the one already standing.
//
// Idempotent on the (target, subject, field) a finding is about. A sweep every
// six hours against one unresolved hand edit must produce one row rather than
// four a day: a single problem reported as a growing list reads as it getting
// worse, and a queue that grows on its own is one people stop opening.
//
// The values are refreshed rather than left alone, because a standing finding
// can MOVE — somebody edits the target again while it is open — and an operator
// deciding from the values first recorded would be deciding about a state that
// no longer exists. `detected_at` does not move: when this first became true is
// the age the surface sorts on.
func RecordMergeFinding(ctx context.Context, f MergeFinding) error {
	switch {
	case strings.TrimSpace(f.Target) == "":
		return fmt.Errorf("record merge finding: no target")
	case strings.TrimSpace(f.SubjectID) == "":
		return fmt.Errorf("record merge finding: no subject")
	case strings.TrimSpace(f.Outcome) == "":
		return fmt.Errorf("record merge finding for %s: no outcome", f.SubjectID)
	}

	const q = `
		INSERT INTO target_merge_findings
			(target, subject_id, field, outcome, base_value, ours_value, theirs_value)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (target, subject_id, field) WHERE resolved_at IS NULL
		DO UPDATE SET
			outcome      = EXCLUDED.outcome,
			base_value   = EXCLUDED.base_value,
			ours_value   = EXCLUDED.ours_value,
			theirs_value = EXCLUDED.theirs_value,
			last_seen_at = NOW()`
	if _, err := querier(ctx).Exec(ctx, q, f.Target, f.SubjectID, f.Field, f.Outcome,
		nullableJSON(f.Base), nullableJSON(f.Ours), nullableJSON(f.Theirs)); err != nil {
		return fmt.Errorf("record merge finding for %s on %s: %w", f.SubjectID, f.Target, err)
	}
	return nil
}

// nullableJSON keeps an absent value absent.
//
// A `deleted_upstream` has no values at all, and writing `null` as a JSON
// document rather than as SQL NULL would make "the target reported null" and
// "there was nothing to report" the same row.
func nullableJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return string(raw)
}

// StandingMergeFindings is what is still standing on one target: undecided, or
// decided and not yet caught up with.
func StandingMergeFindings(ctx context.Context, target string) ([]MergeFinding, error) {
	const q = `
		SELECT id, target, subject_id, field, outcome, base_value, ours_value, theirs_value,
		       detected_at, last_seen_at, decision, decided_by, decided_at
		FROM target_merge_findings
		WHERE target = $1 AND resolved_at IS NULL
		ORDER BY detected_at DESC`
	rows, err := querier(ctx).Query(ctx, q, target)
	if err != nil {
		return nil, fmt.Errorf("read merge findings for %s: %w", target, err)
	}
	defer rows.Close()

	out := []MergeFinding{}
	for rows.Next() {
		var f MergeFinding
		var base, ours, theirs []byte
		var decision, decidedBy *string
		if err := rows.Scan(&f.ID, &f.Target, &f.SubjectID, &f.Field, &f.Outcome,
			&base, &ours, &theirs, &f.DetectedAt, &f.LastSeenAt,
			&decision, &decidedBy, &f.DecidedAt); err != nil {
			return nil, fmt.Errorf("scan merge finding on %s: %w", target, err)
		}
		f.Base, f.Ours, f.Theirs = json.RawMessage(base), json.RawMessage(ours), json.RawMessage(theirs)
		if decision != nil {
			f.Decision = *decision
		}
		if decidedBy != nil {
			f.DecidedBy = *decidedBy
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read merge findings for %s: %w", target, err)
	}
	return out, nil
}

// CountStandingMergeFindings is the governance summary's read.
//
// A count rather than the rows, because the landing page's question is whether
// anything needs a person — and a finding that cannot be counted there sits
// behind a page that says nothing does.
func CountStandingMergeFindings(ctx context.Context) (int, error) {
	const q = `SELECT COUNT(*) FROM target_merge_findings WHERE resolved_at IS NULL`
	var n int
	if err := querier(ctx).QueryRow(ctx, q).Scan(&n); err != nil {
		return 0, fmt.Errorf("count merge findings: %w", err)
	}
	return n, nil
}

// GetStandingMergeFinding reads one, so a resolution acts on what it cites.
//
// Read inside the resolving transaction rather than trusted from the request:
// the resolution's meaning depends on the finding's outcome and field, and a
// caller naming an id is not a caller who has seen the row.
func GetStandingMergeFinding(ctx context.Context, id string) (MergeFinding, error) {
	const q = `
		SELECT id, target, subject_id, field, outcome, base_value, ours_value, theirs_value,
		       detected_at, last_seen_at, decision, decided_by, decided_at
		FROM target_merge_findings
		WHERE id = $1::uuid AND resolved_at IS NULL`
	var f MergeFinding
	var base, ours, theirs []byte
	var decision, decidedBy *string
	err := querier(ctx).QueryRow(ctx, q, id).Scan(&f.ID, &f.Target, &f.SubjectID, &f.Field,
		&f.Outcome, &base, &ours, &theirs, &f.DetectedAt, &f.LastSeenAt,
		&decision, &decidedBy, &f.DecidedAt)
	if err != nil {
		return MergeFinding{}, fmt.Errorf("%w: %s", ErrNoSuchMergeFinding, id)
	}
	f.Base, f.Ours, f.Theirs = json.RawMessage(base), json.RawMessage(ours), json.RawMessage(theirs)
	// The decision travels, and dropping it here was not a cosmetic omission: a
	// caller reading this row saw an undecided finding and acted on it, so a
	// second request could answer a question somebody had already answered
	// differently — after the first answer had queued work.
	if decision != nil {
		f.Decision = *decision
	}
	if decidedBy != nil {
		f.DecidedBy = *decidedBy
	}
	return f, nil
}

// RecordMergeDecision writes what somebody chose, and leaves the finding
// standing.
//
// Standing is the point. The decision queues work — a convergence for keeping
// Syndra's state, a policy change for adopting the target's — and the difference
// is still there until that work lands. Closing the row now would claim
// otherwise, and the next sweep would raise a second finding about the same
// field, so one decision would produce a queue that refills itself every six
// hours until the drain caught up.
//
// The row closes when a pass observes that the two sides agree, carrying this
// decision rather than the anonymous `agreed`.
func RecordMergeDecision(ctx context.Context, id, actor, decision string) (MergeFinding, error) {
	switch {
	case strings.TrimSpace(actor) == "":
		return MergeFinding{}, fmt.Errorf("record merge decision: no actor")
	case !isMergeResolution(decision):
		return MergeFinding{}, fmt.Errorf("record merge decision: %q is not a decision", decision)
	}
	// `decision IS NULL` is the whole guarantee. Without it this was an
	// unconditional overwrite: a second request could replace a decision whose
	// work was already queued, and for `unbound` that meant releasing the
	// account on the target while a re-provision sat in the outbox.
	//
	// One writer wins and the loser is told so. Fail-closed rather than
	// last-write-wins, because the two answers here are opposites — recreate the
	// account, or stop managing it — and silently taking the second would make
	// the outcome depend on which HTTP request happened to arrive last.
	const q = `
		UPDATE target_merge_findings
		SET decision = $3, decided_by = $2, decided_at = NOW()
		WHERE id = $1::uuid AND resolved_at IS NULL AND decision IS NULL
		RETURNING id, target, subject_id, field, outcome, detected_at, last_seen_at`
	var f MergeFinding
	err := querier(ctx).QueryRow(ctx, q, id, actor, decision).Scan(&f.ID, &f.Target,
		&f.SubjectID, &f.Field, &f.Outcome, &f.DetectedAt, &f.LastSeenAt)
	if err != nil {
		// Either it is gone, or somebody decided first. Told apart here, because
		// they are different sentences to an operator: one is "that is already
		// settled", the other is "somebody else just answered this".
		if standing, readErr := GetStandingMergeFinding(ctx, id); readErr == nil && standing.Decision != "" {
			return MergeFinding{}, fmt.Errorf("%w: %s already decided as %s by %s",
				ErrMergeFindingDecided, id, standing.Decision, standing.DecidedBy)
		}
		return MergeFinding{}, fmt.Errorf("%w: %s", ErrNoSuchMergeFinding, id)
	}
	f.Decision, f.DecidedBy = decision, actor
	return f, nil
}

// ReleaseMergeDecision undoes a reservation whose work did not happen.
//
// The reservation is taken before the target is called, so that no second
// request can decide the opposite while the first is mid-flight. If that call
// then fails, the reservation must go — otherwise one unreachable add-on wedges
// the finding as decided-but-never-done, and the surface offers no way back.
//
// Conditional on the decision still being the one this caller made. A
// reservation somebody else has since replaced is not this caller's to clear.
func ReleaseMergeDecision(ctx context.Context, id, actor, decision string) error {
	const q = `
		UPDATE target_merge_findings
		SET decision = NULL, decided_by = NULL, decided_at = NULL
		WHERE id = $1::uuid AND resolved_at IS NULL AND decision = $3 AND decided_by = $2`
	if _, err := querier(ctx).Exec(ctx, q, id, actor, decision); err != nil {
		return fmt.Errorf("release merge decision %s: %w", id, err)
	}
	return nil
}

func isMergeResolution(v string) bool {
	switch v {
	case ResolutionKeepOurs, ResolutionTakeTheirs, ResolutionReprovisioned, ResolutionUnbound:
		return true
	}
	return false
}

// ClearMergeFinding closes a finding whose difference no longer exists.
//
// Attributed to the sweep rather than left unattributed, because the schema
// refuses a resolution with no actor and because "who closed this" has a real
// answer here: nobody decided, a pass observed that the two sides now agree.
//
// Keyed on the finding's subject and field rather than on an id, since the
// caller is a sweep that just classified state — it knows what agrees, not which
// row said otherwise.
func ClearMergeFinding(ctx context.Context, target, subjectID, field, actor string) error {
	// A decided finding closes carrying WHAT WAS DECIDED, and attributed to
	// whoever decided it. The sweep only observed that the difference is over;
	// saying it resolved one somebody else answered would erase the only record
	// that a person was involved.
	const q = `
		UPDATE target_merge_findings
		SET resolved_at = NOW(),
		    resolved_by = COALESCE(NULLIF(decided_by, ''), $4),
		    resolution  = COALESCE(decision, $5)
		WHERE target = $1 AND subject_id = $2 AND field = $3 AND resolved_at IS NULL`
	if _, err := querier(ctx).Exec(ctx, q, target, subjectID, field, actor, ResolutionAgreed); err != nil {
		return fmt.Errorf("clear merge finding for %s on %s: %w", subjectID, target, err)
	}
	return nil
}

// ResolveMergeFinding closes one, naming who decided and what they decided.
//
// It does not perform the resolution. Applying Syndra's state queues a
// convergence; adopting the target's value writes a per-subject decision; both
// happen in the service layer, and both must be durable before this row is
// closed — a finding marked resolved by an action that then failed is a
// difference nothing will raise again until it changes a second time.
func ResolveMergeFinding(ctx context.Context, id, actor, resolution string) (MergeFinding, error) {
	switch {
	case strings.TrimSpace(actor) == "":
		return MergeFinding{}, fmt.Errorf("resolve merge finding: no actor")
	case resolution != ResolutionKeepOurs && resolution != ResolutionTakeTheirs &&
		resolution != ResolutionReprovisioned && resolution != ResolutionUnbound:
		// `agreed` is deliberately not accepted here. It is what a sweep writes
		// when a difference stops existing, and a person choosing it would be
		// dismissing a finding by asserting something they have not checked.
		return MergeFinding{}, fmt.Errorf("resolve merge finding: %q is not a resolution", resolution)
	}

	const q = `
		UPDATE target_merge_findings
		SET resolved_at = NOW(), resolved_by = $2, resolution = $3
		WHERE id = $1::uuid AND resolved_at IS NULL
		RETURNING id, target, subject_id, field, outcome, detected_at, last_seen_at`
	var f MergeFinding
	err := querier(ctx).QueryRow(ctx, q, id, actor, resolution).Scan(&f.ID, &f.Target,
		&f.SubjectID, &f.Field, &f.Outcome, &f.DetectedAt, &f.LastSeenAt)
	if err != nil {
		return MergeFinding{}, fmt.Errorf("%w: %s", ErrNoSuchMergeFinding, id)
	}
	return f, nil
}
