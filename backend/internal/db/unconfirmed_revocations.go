package db

import (
	"context"
	"fmt"
	"time"

	"syndra/internal/models"
)

// The unconfirmed-revocation surface (change `addon-platform` 2.51, 9.9, and
// §15.4's gap).
//
// A queued revocation is retained access. That is the whole reason the
// revocation drain runs in the background while grants wait for an operator —
// but a background drain has a failure mode a foreground one does not: nobody is
// watching it. A row that exhausts its retry budget is terminated with a reason
// and then sits there, correct and invisible.
//
// So the finding is raised rather than the queue being rescued. Two populations,
// deliberately not merged:
//
//   * still queued, and ageing. Nothing is wrong yet; how long it has been
//     ageing is the whole content of the signal.
//   * spent — terminal with a reason, and never going to drain. Somebody has to
//     act, and no amount of waiting produces the revocation.
//
// Counting them together would let a healthy queue of five-minute-old rows hide
// a revocation that failed permanently three days ago.

// UnconfirmedRevocation is one withdrawal that has not reached its target.
type UnconfirmedRevocation struct {
	models.PendingPropagation
	// Age is how long this row has been unconfirmed, computed by the database
	// clock so it does not depend on the difference between two machines'.
	Age time.Duration `json:"age_seconds"`
	// Spent says the row is terminal: its retry budget is gone and nothing will
	// dispatch it again. The distinction the surface renders differently.
	Spent bool `json:"spent"`
}

// UnconfirmedRevocationSummary is the counted form, for an indicator.
type UnconfirmedRevocationSummary struct {
	// Queued is still-draining rows. Not a problem by itself.
	Queued int `json:"queued"`
	// Spent is terminal rows whose access was never withdrawn. Each one is a
	// person who still has access somebody decided to take away.
	Spent int `json:"spent"`
	// OldestAge is the age of the oldest unconfirmed row of either kind. The
	// number an escalation threshold is compared against.
	OldestAge time.Duration `json:"oldest_age_seconds"`
}

// Escalated reports whether this summary has crossed into a security finding.
//
// Any spent row is one immediately: it is retained access that nothing will
// withdraw, and its age is beside the point. A merely queued row escalates on
// time, because a revocation that has not landed in a day is not draining, it is
// stuck behind something.
//
// The rule lives here rather than in a renderer so the indicator, the surface
// and any future alert cannot disagree about what counts.
func (s UnconfirmedRevocationSummary) Escalated(threshold time.Duration) bool {
	return s.Spent > 0 || (s.Queued > 0 && s.OldestAge >= threshold)
}

// unconfirmedRevocationPredicate is the shared WHERE clause.
//
// `op_type` is the restriction that makes this a revocation surface and not an
// outbox listing. `replace` counts: it confers and withdraws in one call, and
// its withdrawal half is exactly as unconfirmed as a revoke's.
const unconfirmedRevocationPredicate = `
	    p.op_type IN ('revoke', 'replace')
	AND p.status IN ('pending', 'in_flight', 'failed')`

// ListUnconfirmedRevocations returns every withdrawal that has not reached its
// target, oldest first.
//
// Oldest first because the ordering IS the triage: the oldest unconfirmed
// revocation is the one that has been retained access for longest, whatever the
// reason.
func ListUnconfirmedRevocations(ctx context.Context, limit int) ([]UnconfirmedRevocation, error) {
	if limit <= 0 {
		limit = 100
	}
	q := `
		SELECT p.id, p.op_type, p.user_id,
		       -- Both are NULL on an add-on row, whose intent lives in its
		       -- snapshot rather than in these columns. Coalesced rather than
		       -- scanned into pointers: an empty project on a TrueNAS row is the
		       -- honest rendering, and a nil one would make every consumer
		       -- handle a case that means the same thing.
		       COALESCE(p.project_id, ''), COALESCE(p.role_keys, ARRAY[]::text[]), p.status,
		       p.attempts, p.last_error, p.created_at, p.target,
		       EXTRACT(EPOCH FROM (NOW() - p.created_at))::bigint,
		       p.status = 'failed'
		  FROM propagation_outbox p
		 WHERE ` + unconfirmedRevocationPredicate + `
		 ORDER BY p.created_at
		 LIMIT $1`

	rows, err := PG.Query(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("list unconfirmed revocations: %w", err)
	}
	defer rows.Close()

	var out []UnconfirmedRevocation
	for rows.Next() {
		var r UnconfirmedRevocation
		var ageSeconds int64
		var lastError *string
		if err := rows.Scan(&r.ID, &r.OpType, &r.UserID, &r.ProjectID, &r.RoleKeys, &r.Status,
			&r.Attempts, &lastError, &r.CreatedAt, &r.Target, &ageSeconds, &r.Spent); err != nil {
			return nil, fmt.Errorf("scan unconfirmed revocation: %w", err)
		}
		if lastError != nil {
			r.LastError = *lastError
		}
		r.Age = time.Duration(ageSeconds) * time.Second
		out = append(out, r)
	}
	return out, rows.Err()
}

// CountUnconfirmedRevocations is the indicator's read: the two populations and
// the oldest age, in one query.
//
// One query rather than three, because three would be three moments — and a
// count of spent rows taken after the oldest age was read can disagree with it.
func CountUnconfirmedRevocations(ctx context.Context) (UnconfirmedRevocationSummary, error) {
	q := `
		SELECT
			COUNT(*) FILTER (WHERE p.status <> 'failed'),
			COUNT(*) FILTER (WHERE p.status = 'failed'),
			COALESCE(MAX(EXTRACT(EPOCH FROM (NOW() - p.created_at))::bigint), 0)
		  FROM propagation_outbox p
		 WHERE ` + unconfirmedRevocationPredicate

	var s UnconfirmedRevocationSummary
	var oldest int64
	if err := PG.QueryRow(ctx, q).Scan(&s.Queued, &s.Spent, &oldest); err != nil {
		return UnconfirmedRevocationSummary{}, fmt.Errorf("count unconfirmed revocations: %w", err)
	}
	s.OldestAge = time.Duration(oldest) * time.Second
	return s, nil
}
