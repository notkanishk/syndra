package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Writing a desired-state snapshot (design §15, change `addon-platform` 1.4).
//
// This is the durable record of what a rehearsal proposed for one subject on
// one target. The outbox row does not carry the intent — it carries a citation
// reaching a plan subject, which reaches one of these — so an apply dispatches
// the set that was approved rather than whatever the resolver would say at drain
// time. Re-resolving there would be a different decision wearing the approval's
// plan id.
//
// Two properties come from the schema rather than from this file, and both are
// deliberate. The rows are immutable, enforced by a trigger, because they are
// audit records that outlive the plan citing them. And the version is ALLOCATED
// by a trigger under a pair-scoped lock rather than supplied here, because a
// writer proposing `MAX+1` has to retry when it loses the race, and every writer
// would have to implement the same loop identically for stale-version rejection
// to mean anything.

// ErrInvalidSnapshot refuses a snapshot that would record nothing usable.
var ErrInvalidSnapshot = errors.New("db: invalid desired-state snapshot")

// DesiredStateSnapshot is a written row, as its writer needs to cite it.
type DesiredStateSnapshot struct {
	ID        string `json:"id"`
	SubjectID string `json:"subject_id"`
	Target    string `json:"target"`
	// Version is monotonic per (subject, target) and is what makes a queued
	// grant overtaken by a newer revoke terminate as superseded rather than
	// landing after it.
	Version int64 `json:"version"`
}

// WriteDesiredStateSnapshotTx records one subject's proposed desired state on
// the caller's transaction.
//
// On the caller's transaction because the snapshot and the plan subject citing
// it must commit together: a snapshot with no plan is an unreferenced audit row
// that has already spent a version, and a plan subject citing a snapshot that
// was rolled back is a citation the drain resolves to nothing and fails.
//
// `state` is the whole resolved set by field name, never a delta. `/apply` is
// level-triggered, so the set IS the instruction — and an absent field and an
// empty one say different things: one is "do not manage this", the other is
// "make it empty".
func WriteDesiredStateSnapshotTx(ctx context.Context, tx pgx.Tx, subjectID, target, createdBy string, state map[string]json.RawMessage) (DesiredStateSnapshot, error) {
	switch {
	case strings.TrimSpace(subjectID) == "":
		return DesiredStateSnapshot{}, fmt.Errorf("%w: no subject", ErrInvalidSnapshot)
	case strings.TrimSpace(target) == "":
		return DesiredStateSnapshot{}, fmt.Errorf("%w: no target", ErrInvalidSnapshot)
	case target == TargetZitadel:
		// Zitadel rows carry their intent in the outbox's own project and role
		// columns, and their plan subjects cite no snapshot. A snapshot here
		// would be a second account of one decision, free to disagree with the
		// first — and the drain reads the columns, so the snapshot would be the
		// copy nobody consults and everybody trusts.
		return DesiredStateSnapshot{}, fmt.Errorf("%w: %s carries its intent in the outbox row, not in a snapshot", ErrInvalidSnapshot, target)
	case strings.TrimSpace(createdBy) == "":
		// An audit record with no actor. The whole reason a snapshot outlives
		// its plan is to answer "who decided this and what did they decide", and
		// half of that answer is this column.
		return DesiredStateSnapshot{}, fmt.Errorf("%w: no actor", ErrInvalidSnapshot)
	case state == nil:
		// nil and empty are different, and only one of them is allowed. An empty
		// map is a legitimate instruction — manage nothing — while nil is a
		// caller that resolved nothing and did not notice, and it would encode
		// as `null`, which the drain reads back as "no approved desired state"
		// and fails the row on. Refused here, where the caller can still be told
		// why, rather than at dispatch.
		return DesiredStateSnapshot{}, fmt.Errorf("%w: the resolved state is nil, which records an intent nobody can read", ErrInvalidSnapshot)
	}

	encoded, err := json.Marshal(state)
	if err != nil {
		return DesiredStateSnapshot{}, fmt.Errorf("encode desired state for %s: %w", subjectID, err)
	}

	// `version` is omitted from the column list on purpose: the trigger
	// allocates it, and a supplied value is replaced. Naming it here would let a
	// future edit pass one and believe it took.
	const q = `
		INSERT INTO desired_state_snapshots (subject_id, target, state_json, created_by)
		VALUES ($1, $2, $3::jsonb, $4)
		RETURNING id::text, version`

	out := DesiredStateSnapshot{SubjectID: subjectID, Target: target}
	if err := tx.QueryRow(ctx, q, subjectID, target, string(encoded), createdBy).
		Scan(&out.ID, &out.Version); err != nil {
		return DesiredStateSnapshot{}, fmt.Errorf("write desired state snapshot for %s on %s: %w", subjectID, target, err)
	}
	return out, nil
}
