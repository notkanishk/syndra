package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Allowances: the second layer of the access model (design §6).
//
// Layer 1 is the Zitadel role, mapped to a target entitlement. Layer 2 is this
// — an explicit per-user overlay, never inferred — so that "why does this
// person have access to X" answers with exactly one of: the role gives it, a
// rule derived it, or somebody granted it, with actor and time.
//
// Phase 1 ships the subtractive half only. The `allow` direction exists in the
// schema because generality is cheap there and a later migration is not; the
// code refuses it, because an additive resolver arm with no phase-1 consumer is
// an abstraction with nothing behind it.

// Directions an allowance may carry.
const (
	AllowanceDeny  = "deny"
	AllowanceAllow = "allow"
)

var (
	// ErrAllowanceUnbounded refuses a denial with neither an expiry nor a
	// review date. An open-ended carve-out nobody is ever prompted to revisit is
	// how a temporary measure becomes permanent by inattention.
	ErrAllowanceUnbounded = errors.New("db: a denial must carry an expiry or a review date")
	// ErrAllowanceAdditiveUnsupported refuses the additive arm, which has no
	// phase-1 consumer. Refused rather than accepted and ignored: a stored
	// allowance that resolves to nothing is worse than one that was never
	// accepted, because somebody will read it and believe it applies.
	ErrAllowanceAdditiveUnsupported = errors.New("db: additive allowances are not supported yet")
	ErrAllowanceInvalid             = errors.New("db: invalid allowance")
	ErrAllowanceNotFound            = errors.New("db: no such allowance")
)

// Allowance is one overlay decision.
type Allowance struct {
	ID         string     `json:"id"`
	SubjectID  string     `json:"subject_id"`
	Target     string     `json:"target"`
	Field      string     `json:"field"`
	Value      string     `json:"value"`
	Direction  string     `json:"direction"`
	ActorID    string     `json:"actor_id"`
	Reason     string     `json:"reason"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	ReviewDate *time.Time `json:"review_date,omitempty"`
	LiftedAt   *time.Time `json:"lifted_at,omitempty"`
	LiftedBy   string     `json:"lifted_by,omitempty"`
}

// InForce reports whether this allowance currently suppresses anything.
//
// Lapsed is not lifted, and neither is deleted. All three are states an
// operator asks about differently, so the row survives all of them.
func (a Allowance) InForce(now time.Time) bool {
	switch {
	case a.LiftedAt != nil:
		return false
	case a.ExpiresAt != nil && !a.ExpiresAt.After(now):
		return false
	}
	return true
}

// ReviewDue reports whether an indefinite suspension has reached the date
// somebody promised to look at it again.
//
// It says nothing about whether the suspension applies. A passed review date
// surfaces the decision; it never lifts it, because lapsing on a date nobody
// acted on would restore access by inattention — the exact failure the review
// date exists to prevent, running backwards.
func (a Allowance) ReviewDue(now time.Time) bool {
	return a.InForce(now) && a.ReviewDate != nil && !a.ReviewDate.After(now)
}

func (a Allowance) validate() error {
	for _, f := range []struct{ name, value string }{
		{"subject_id", a.SubjectID}, {"target", a.Target},
		{"field", a.Field}, {"value", a.Value},
		{"actor_id", a.ActorID}, {"reason", a.Reason},
	} {
		if strings.TrimSpace(f.value) == "" {
			return fmt.Errorf("%w: %s is required", ErrAllowanceInvalid, f.name)
		}
	}
	switch a.Direction {
	case AllowanceDeny:
	case AllowanceAllow:
		// The refusal names the phase rather than the field, because there is
		// nothing the caller can correct about the request — the arm does not
		// exist yet.
		return ErrAllowanceAdditiveUnsupported
	default:
		return fmt.Errorf("%w: direction must be %s or %s", ErrAllowanceInvalid, AllowanceDeny, AllowanceAllow)
	}
	if a.ExpiresAt == nil && a.ReviewDate == nil {
		// The error offers both valid forms and names the per-person permanent
		// path, which is revoking the role grant — never editing the mapping,
		// which changes access for every holder of that role and is a blast
		// radius disguised as a policy fix.
		return fmt.Errorf("%w: give it an expiry, or a review date if the suspension is open-ended. To remove this person's access permanently, revoke their role grant rather than editing the mapping, which would change access for everyone holding that role", ErrAllowanceUnbounded)
	}
	return nil
}

// CreateAllowance records one overlay decision.
func CreateAllowance(ctx context.Context, a Allowance) (Allowance, error) {
	if err := a.validate(); err != nil {
		return Allowance{}, err
	}
	const q = `
		INSERT INTO allowances (subject_id, target, field, value, direction, actor_id, reason, expires_at, review_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at`
	err := querier(ctx).QueryRow(ctx, q, a.SubjectID, a.Target, a.Field, a.Value, a.Direction,
		a.ActorID, a.Reason, a.ExpiresAt, a.ReviewDate).Scan(&a.ID, &a.CreatedAt)
	if err != nil {
		return Allowance{}, fmt.Errorf("create allowance: %w", err)
	}
	return a, nil
}

// LiftAllowance ends an allowance and records who ended it.
//
// Never a delete. An allowance is a decision somebody took, and removing the
// row erases the only record that the suspension ever happened — which is the
// history the whole layer exists to keep attached to the person.
func LiftAllowance(ctx context.Context, id, actor string) error {
	if !looksLikeUUID(id) {
		return fmt.Errorf("%w: %s", ErrAllowanceNotFound, id)
	}
	const q = `UPDATE allowances SET lifted_at = NOW(), lifted_by = $2 WHERE id = $1 AND lifted_at IS NULL`
	tag, err := querier(ctx).Exec(ctx, q, id, actor)
	if err != nil {
		return fmt.Errorf("lift allowance: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: %s (or it was already lifted)", ErrAllowanceNotFound, id)
	}
	return nil
}

// AllowancesInForce returns the allowances currently applying to a subject on a
// target.
//
// Expiry is compared in the predicate rather than in Go, so the answer is the
// database's own clock — the same clock the expiry sweep deletes by, and the
// only way the two cannot disagree about the instant a suspension ends.
func AllowancesInForce(ctx context.Context, subjectID, target string) ([]Allowance, error) {
	const q = `
		SELECT id, subject_id, target, field, value, direction, actor_id, reason,
		       created_at, expires_at, review_date, lifted_at, COALESCE(lifted_by, '')
		  FROM allowances
		 WHERE subject_id = $1 AND target = $2
		   AND lifted_at IS NULL
		   AND (expires_at IS NULL OR expires_at > NOW())
		 ORDER BY created_at`
	rows, err := querier(ctx).Query(ctx, q, subjectID, target)
	if err != nil {
		return nil, fmt.Errorf("read allowances in force: %w", err)
	}
	defer rows.Close()
	return scanAllowances(rows)
}

// AllowancesForSubject returns every allowance ever recorded for a subject,
// including lifted and lapsed ones — the lineage view's read.
func AllowancesForSubject(ctx context.Context, subjectID string) ([]Allowance, error) {
	const q = `
		SELECT id, subject_id, target, field, value, direction, actor_id, reason,
		       created_at, expires_at, review_date, lifted_at, COALESCE(lifted_by, '')
		  FROM allowances WHERE subject_id = $1 ORDER BY created_at DESC`
	rows, err := querier(ctx).Query(ctx, q, subjectID)
	if err != nil {
		return nil, fmt.Errorf("read allowances for %s: %w", subjectID, err)
	}
	defer rows.Close()
	return scanAllowances(rows)
}

// AllowancesDueForReview lists indefinite suspensions whose review date has
// passed and which nobody has decided about.
//
// They stay in force. This is a prompt, not a lapse.
func AllowancesDueForReview(ctx context.Context) ([]Allowance, error) {
	const q = `
		SELECT id, subject_id, target, field, value, direction, actor_id, reason,
		       created_at, expires_at, review_date, lifted_at, COALESCE(lifted_by, '')
		  FROM allowances
		 WHERE lifted_at IS NULL AND direction = 'deny'
		   AND review_date IS NOT NULL AND review_date <= NOW()
		   AND (expires_at IS NULL OR expires_at > NOW())
		 ORDER BY review_date`
	rows, err := querier(ctx).Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("read allowances due for review: %w", err)
	}
	defer rows.Close()
	return scanAllowances(rows)
}

// LapsedAllowances lists allowances whose expiry has passed and which the sweep
// has not yet resolved.
//
// Lapsing is not lifting: the row keeps `lifted_at` NULL until the sweep
// records who — or what — ended it, so an allowance that expired and one an
// operator ended stay distinguishable forever.
func LapsedAllowances(ctx context.Context, limit int) ([]Allowance, error) {
	if limit <= 0 {
		limit = 500
	}
	const q = `
		SELECT id, subject_id, target, field, value, direction, actor_id, reason,
		       created_at, expires_at, review_date, lifted_at, COALESCE(lifted_by, '')
		  FROM allowances
		 WHERE lifted_at IS NULL AND expires_at IS NOT NULL AND expires_at <= NOW()
		 ORDER BY expires_at
		 LIMIT $1`
	rows, err := querier(ctx).Query(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("read lapsed allowances: %w", err)
	}
	defer rows.Close()
	return scanAllowances(rows)
}

// ResolveLapsedAllowance records that an allowance ended because its date
// arrived, and writes the audit row in the same statement.
//
// The actor is a clock, and the row says so rather than naming whoever's sweep
// happened to run. Both writes together, because "the suspension ended" and
// "the timeline records that it ended" coming apart is exactly the gap somebody
// investigates later and cannot close.
func ResolveLapsedAllowance(ctx context.Context, id string) error {
	const q = `
		WITH lapsed AS (
			UPDATE allowances
			   SET lifted_at = NOW(), lifted_by = 'expiry_sweep'
			 WHERE id = $1 AND lifted_at IS NULL
			   AND expires_at IS NOT NULL AND expires_at <= NOW()
			RETURNING id, subject_id, target
		)
		INSERT INTO audit_logs (actor_zitadel_user_id, target_zitadel_user_id, action, resource_id)
		SELECT 'system', l.subject_id, 'allowance.' || l.target || '.lapsed', l.id FROM lapsed l`
	tag, err := querier(ctx).Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("resolve lapsed allowance: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Renewed, lifted by hand, or already swept. Not an error, and
		// deliberately not reported as one: the sweep's job is that no lapsed
		// allowance stays in force, and something else having done it satisfies
		// that.
		return nil
	}
	return nil
}

func scanAllowances(rows pgx.Rows) ([]Allowance, error) {
	var out []Allowance
	for rows.Next() {
		var a Allowance
		if err := rows.Scan(&a.ID, &a.SubjectID, &a.Target, &a.Field, &a.Value, &a.Direction,
			&a.ActorID, &a.Reason, &a.CreatedAt, &a.ExpiresAt, &a.ReviewDate,
			&a.LiftedAt, &a.LiftedBy); err != nil {
			return nil, fmt.Errorf("scan allowance: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
