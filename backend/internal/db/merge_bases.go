package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// The merge base: what the target REPORTED after the last successful apply
// (change `reconciliation-as-merge`).
//
// Reconciliation without it compares two states and can only pick a winner. The
// same difference is produced by Syndra having moved, by somebody editing the
// target by hand, by both moving to the same value, and by both moving to
// different ones — and only the first is safe to resolve by writing. The base is
// what tells them apart.
//
// Reported, never intended. A base recorded from the desired state equals it by
// construction, so no comparison against it can ever produce a conflict; that is
// the current behaviour with more machinery, and it is why the read-back landed
// as its own change before this one.

// MergeBase is one subject's last observed state on one target.
type MergeBase struct {
	Target    string `json:"target"`
	SubjectID string `json:"subject_id"`
	// Base is field -> the value the target reported. The shape belongs to the
	// target rather than to this schema: a list of groups, a boolean, and
	// whatever a later add-on declares.
	Base map[string]json.RawMessage `json:"base"`
	// ObservedAt dates the READ, not the row. An operator asking what a value
	// used to be is asking about a moment on the target.
	ObservedAt time.Time `json:"observed_at"`
}

// RecordMergeBase writes what the target reported after an apply.
//
// An empty observation is refused rather than stored. The add-on omits
// `observed` when it wrote and could not read back — deliberately, so that an
// unverified write cannot become a base — and storing an empty map for one
// would claim the target held nothing, which classifies every managed field as
// changed-by-them on the next pass. A refusal here is the caller's signal that
// there is nothing to record, and the subject stays baseless, which is a state
// the classifier already handles.
func RecordMergeBase(ctx context.Context, base MergeBase) error {
	switch {
	case strings.TrimSpace(base.Target) == "":
		return fmt.Errorf("record merge base: no target")
	case strings.TrimSpace(base.SubjectID) == "":
		return fmt.Errorf("record merge base: no subject")
	case len(base.Base) == 0:
		return fmt.Errorf("record merge base for %s on %s: nothing was observed",
			base.SubjectID, base.Target)
	}
	encoded, err := json.Marshal(base.Base)
	if err != nil {
		return fmt.Errorf("encode merge base for %s: %w", base.SubjectID, err)
	}

	const q = `
		INSERT INTO target_merge_bases (target, subject_id, base, observed_at)
		VALUES ($1, $2, $3::jsonb, NOW())
		ON CONFLICT (target, subject_id)
		DO UPDATE SET base = EXCLUDED.base, observed_at = EXCLUDED.observed_at`
	if _, err := querier(ctx).Exec(ctx, q, base.Target, base.SubjectID, string(encoded)); err != nil {
		return fmt.Errorf("record merge base for %s on %s: %w", base.SubjectID, base.Target, err)
	}
	return nil
}

// MergeBasesFor reads every base on a target, keyed by subject.
//
// One query for the whole cohort, because the sweep classifies every managed
// subject in a pass and a per-subject read would make reconciliation cost one
// round trip per person.
func MergeBasesFor(ctx context.Context, target string) (map[string]MergeBase, error) {
	const q = `SELECT subject_id, base, observed_at FROM target_merge_bases WHERE target = $1`
	rows, err := querier(ctx).Query(ctx, q, target)
	if err != nil {
		return nil, fmt.Errorf("read merge bases for %s: %w", target, err)
	}
	defer rows.Close()

	out := map[string]MergeBase{}
	for rows.Next() {
		var subject string
		var raw []byte
		var observed time.Time
		if err := rows.Scan(&subject, &raw, &observed); err != nil {
			return nil, fmt.Errorf("scan merge base on %s: %w", target, err)
		}
		fields := map[string]json.RawMessage{}
		if err := json.Unmarshal(raw, &fields); err != nil {
			// A base that will not decode is not a base. Skipped rather than
			// fatal, and loud: the subject falls back to having none, which is
			// the conservative reading — it converges as it did before this
			// mechanism existed instead of being classified against a value
			// nobody can read.
			return nil, fmt.Errorf("decode merge base for %s on %s: %w", subject, target, err)
		}
		out[subject] = MergeBase{Target: target, SubjectID: subject, Base: fields, ObservedAt: observed}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read merge bases for %s: %w", target, err)
	}
	return out, nil
}

// ForgetMergeBase drops a subject's base when Syndra stops managing the account.
//
// It goes with the binding, always. A base outliving its binding is a claim
// about an account nobody here manages any more — and if that subject is later
// bound to a DIFFERENT account, the stale base would be compared against a
// person it was never about, producing conflicts whose "what it used to be"
// names somebody else's state.
func ForgetMergeBase(ctx context.Context, target, subjectID string) error {
	const q = `DELETE FROM target_merge_bases WHERE target = $1 AND subject_id = $2`
	if _, err := querier(ctx).Exec(ctx, q, target, subjectID); err != nil {
		return fmt.Errorf("forget merge base for %s on %s: %w", subjectID, target, err)
	}
	return nil
}
