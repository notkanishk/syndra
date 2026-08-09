package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Plan storage: the rehearsal made durable (design §8).
//
// The weakness this replaces is not tampering — no plan ever crossed the wire.
// It is the gap between the two REQUESTS: an operator read one computed diff
// and a second request recomputed another, and nothing bound them together. So
// the rehearsal writes what it showed, and the apply cites it rather than
// asking for the same computation to be run again against a world that moved.

// Sentinel refusals. Each is a different operator action, which is why they are
// not one error: "re-plan" (expired), "you already did this" (applied), "wrong
// screen" (not citable here), "ask the person who approved it" (not yours).
var (
	ErrPlanNotFound = errors.New("db: no such plan")
	// ErrPlanExpired: the lifetime bounds how long an unexecuted plan may be
	// cited, because a fingerprint taken long enough ago describes a world
	// nobody is looking at any more.
	ErrPlanExpired = errors.New("db: the plan has expired and must be re-planned")
	// ErrPlanAlreadyApplied: one approval, one apply.
	ErrPlanAlreadyApplied = errors.New("db: the plan has already been applied")
	// ErrPlanNotCitableHere: the plan exists but belongs to another surface or
	// another target. A drift-triage plan cited on the bulk-grant endpoint is
	// this error, not a mysterious empty apply.
	ErrPlanNotCitableHere = errors.New("db: the plan was not issued for this surface and target")
	// ErrPlanNotYours: approval is a person's, not the system's. An admin who
	// reads a plan id out of a log or a screenshot has not reviewed the diff.
	ErrPlanNotYours  = errors.New("db: the plan was approved by a different operator")
	ErrPlanNoSubject = errors.New("db: the plan has no subject rows")
	ErrInvalidPlan   = errors.New("db: invalid plan")
)

// PlanOutcome is what the operator was shown for one subject, in the shape the
// existing rehearsal surfaces already speak.
//
// The type is the secret exclusion. `outcome_json` is JSONB and would accept
// anything, so the guarantee cannot be "we agree not to write parameters
// there": it is that no caller holds a route to. Every field here is a string
// or a list of them, decided by the backend — there is no map, no `any`, and
// nothing a submitted parameter set could be assigned to. A declared secret
// rides the apply request and is discarded with it (design §5).
//
// Deliberately absent: name and email. They are read from the directory when a
// plan is rendered. Persisting them would make the plan a second, stale copy of
// a profile, and a plan is a record of intent, not of people.
type PlanOutcome struct {
	Effect      string   `json:"effect"`
	Detail      string   `json:"detail"`
	Consequence string   `json:"consequence,omitempty"`
	GrantIDs    []string `json:"grant_ids,omitempty"`
}

// Plan is one approval.
type Plan struct {
	ID        string    `json:"id"`
	Target    string    `json:"target"`
	Surface   string    `json:"surface"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	// ExpiresAt is nil exactly for provisional plans, whose gate is the
	// re-fingerprint on the target's return rather than a clock.
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	Provisional bool       `json:"provisional"`
	// StateReadAt is when the read behind the fingerprints was taken. Not
	// derivable from CreatedAt: a provisional plan is computed against a
	// last-known read that may be days older than the plan, and the surface has
	// to say so.
	StateReadAt *time.Time `json:"state_read_at,omitempty"`
	AppliedAt   *time.Time `json:"applied_at,omitempty"`
}

// PlanSubject is one subject's row: the desired state approved for them, and
// the fingerprint of the state it was approved against.
type PlanSubject struct {
	ID        string `json:"id"`
	PlanID    string `json:"plan_id"`
	SubjectID string `json:"subject_id"`
	// SnapshotID is nil for Zitadel plans, whose intent is the outbox row's own
	// columns.
	SnapshotID  *string     `json:"snapshot_id,omitempty"`
	Fingerprint string      `json:"fingerprint"`
	Outcome     PlanOutcome `json:"outcome"`
}

// NewPlan is a rehearsal about to become durable.
type NewPlan struct {
	Target    string
	Surface   string
	CreatedBy string
	// Lifetime bounds how long a confirmed plan may be cited. Required for a
	// confirmed plan, forbidden for a provisional one.
	Lifetime    time.Duration
	Provisional bool
	// StateReadAt is required for a provisional plan and welcome on any plan.
	StateReadAt time.Time
	Subjects    []NewPlanSubject
}

// NewPlanSubject is one row of the rehearsal.
type NewPlanSubject struct {
	SubjectID   string
	SnapshotID  string
	Fingerprint string
	Outcome     PlanOutcome
}

// PlanCitation is an apply naming the plan it is executing.
//
// A struct rather than four string arguments: they are all strings, they are
// all identifiers, and two of them transposed at a call site would produce a
// gate that compares a surface against a target and refuses everything — or, if
// the mistake is symmetric, one that compares nothing meaningful at all.
type PlanCitation struct {
	PlanID  string
	Target  string
	Surface string
	Actor   string
}

func (p NewPlan) validate() error {
	switch {
	case strings.TrimSpace(p.Target) == "":
		return fmt.Errorf("%w: target is required", ErrInvalidPlan)
	case strings.TrimSpace(p.Surface) == "":
		return fmt.Errorf("%w: surface is required", ErrInvalidPlan)
	case strings.TrimSpace(p.CreatedBy) == "":
		return fmt.Errorf("%w: created_by is required", ErrInvalidPlan)
	case len(p.Subjects) == 0:
		// A plan with no subjects would apply cleanly, mutate nothing, and
		// report success — the most convincing possible way to do nothing.
		return fmt.Errorf("%w: a plan with no subjects is not a plan", ErrInvalidPlan)
	}

	if p.Provisional {
		if p.StateReadAt.IsZero() {
			return fmt.Errorf("%w: a provisional plan must record the age of the state it was computed against", ErrInvalidPlan)
		}
		if p.Lifetime != 0 {
			return fmt.Errorf("%w: a provisional plan must not carry a lifetime — its gate is the re-fingerprint, not a clock", ErrInvalidPlan)
		}
	} else if p.Lifetime <= 0 {
		return fmt.Errorf("%w: a confirmed plan must carry a lifetime", ErrInvalidPlan)
	}

	seen := make(map[string]struct{}, len(p.Subjects))
	for _, s := range p.Subjects {
		switch {
		case strings.TrimSpace(s.SubjectID) == "":
			return fmt.Errorf("%w: a subject row with no subject", ErrInvalidPlan)
		case strings.TrimSpace(s.Fingerprint) == "":
			// The one refusal that has to be here rather than in the database.
			// Verification compares the recorded fingerprint against a live
			// read; an empty recorded value matches an empty live one, so a
			// subject stored without a fingerprint is a subject that passes
			// verification precisely when the target could not be read.
			return fmt.Errorf("%w: subject %s has no fingerprint, and a row that cannot be verified verifies vacuously", ErrInvalidPlan, s.SubjectID)
		case s.SnapshotID != "" && !looksLikeUUID(s.SnapshotID):
			return fmt.Errorf("%w: subject %s cites a malformed snapshot id", ErrInvalidPlan, s.SubjectID)
		}
		if _, dup := seen[s.SubjectID]; dup {
			return fmt.Errorf("%w: subject %s appears twice", ErrInvalidPlan, s.SubjectID)
		}
		seen[s.SubjectID] = struct{}{}
	}
	return nil
}

// CreatePlan persists a rehearsal and its per-subject rows in one transaction.
//
// Both or neither: a plan with no subject rows is an approval of nothing that
// would nonetheless satisfy an apply, and subject rows under no plan are
// unreachable by any citation.
func CreatePlan(ctx context.Context, p NewPlan) (Plan, error) {
	if err := p.validate(); err != nil {
		return Plan{}, err
	}

	tx, err := PG.Begin(ctx)
	if err != nil {
		return Plan{}, fmt.Errorf("begin plan tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after a successful Commit

	// The lifetime is measured by the database clock, because the expiry
	// predicate is evaluated by the database clock. Computing the deadline here
	// would make a plan's validity depend on the difference between two clocks.
	const insertPlan = `
		INSERT INTO plans (target, surface, created_by, expires_at, provisional, state_read_at)
		VALUES ($1, $2, $3,
		        CASE WHEN $5::boolean THEN NULL ELSE NOW() + $4::interval END,
		        $5, $6)
		RETURNING id, created_at, expires_at, applied_at`

	var (
		lifetime    = fmt.Sprintf("%d seconds", int64(p.Lifetime/time.Second))
		stateReadAt *time.Time
	)
	if !p.StateReadAt.IsZero() {
		t := p.StateReadAt
		stateReadAt = &t
	}

	plan := Plan{Target: p.Target, Surface: p.Surface, CreatedBy: p.CreatedBy, Provisional: p.Provisional, StateReadAt: stateReadAt}
	if err := tx.QueryRow(ctx, insertPlan,
		p.Target, p.Surface, p.CreatedBy, lifetime, p.Provisional, stateReadAt,
	).Scan(&plan.ID, &plan.CreatedAt, &plan.ExpiresAt, &plan.AppliedAt); err != nil {
		return Plan{}, fmt.Errorf("insert plan: %w", err)
	}

	const insertSubject = `
		INSERT INTO plan_subjects (plan_id, subject_id, snapshot_id, fingerprint, outcome_json)
		VALUES ($1, $2, $3, $4, $5)`
	for _, s := range p.Subjects {
		outcome, err := json.Marshal(s.Outcome)
		if err != nil {
			return Plan{}, fmt.Errorf("marshal plan outcome for %s: %w", s.SubjectID, err)
		}
		var snapshot *string
		if s.SnapshotID != "" {
			id := s.SnapshotID
			snapshot = &id
		}
		if _, err := tx.Exec(ctx, insertSubject, plan.ID, s.SubjectID, snapshot, s.Fingerprint, outcome); err != nil {
			return Plan{}, fmt.Errorf("insert plan subject %s: %w", s.SubjectID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return Plan{}, fmt.Errorf("commit plan tx: %w", err)
	}
	return plan, nil
}

// ClaimPlanTx spends an approval and returns what it approved.
//
// It runs on the caller's transaction deliberately. The apply's own writes —
// outbox rows, audit rows — belong in the same transaction as the claim, so a
// failure halfway through does not leave an operator holding a plan that was
// consumed by an apply that did not happen.
//
// The conditional UPDATE is the authority. Every dimension of the citation
// appears in its predicate, so a plan claimed concurrently, expired, already
// applied, or cited from the wrong surface loses the race in the database
// rather than in a check some future caller might route around. The SELECT
// afterwards only explains a refusal; it grants nothing.
func ClaimPlanTx(ctx context.Context, tx pgx.Tx, c PlanCitation) (Plan, []PlanSubject, error) {
	// A malformed id must not reach Postgres. `invalid input syntax for type
	// uuid` inside the caller's transaction aborts every statement that
	// follows it, so a client citing rubbish would take down the apply's
	// bookkeeping rather than being told no.
	if !looksLikeUUID(c.PlanID) {
		return Plan{}, nil, fmt.Errorf("%w: %s", ErrPlanNotFound, c.PlanID)
	}

	const claim = `
		UPDATE plans
		   SET applied_at = NOW()
		 WHERE id = $1
		   AND target = $2
		   AND surface = $3
		   AND created_by = $4
		   AND applied_at IS NULL
		   AND (expires_at IS NULL OR expires_at > NOW())
		RETURNING id, target, surface, created_by, created_at, expires_at, provisional, state_read_at, applied_at`

	plan, err := scanPlan(tx.QueryRow(ctx, claim, c.PlanID, c.Target, c.Surface, c.Actor))
	if errors.Is(err, pgx.ErrNoRows) {
		return Plan{}, nil, explainPlanRefusal(ctx, tx, c)
	}
	if err != nil {
		return Plan{}, nil, fmt.Errorf("claim plan: %w", err)
	}

	subjects, err := planSubjectsTx(ctx, tx, plan.ID)
	if err != nil {
		return Plan{}, nil, err
	}
	if len(subjects) == 0 {
		// Unreachable through CreatePlan, which refuses a subjectless plan.
		// Left here because the alternative is an apply that mutates nothing
		// and reports success, and that is not a failure mode worth trusting a
		// single writer to prevent forever.
		return Plan{}, nil, fmt.Errorf("%w: %s", ErrPlanNoSubject, plan.ID)
	}
	return plan, subjects, nil
}

// explainPlanRefusal re-reads the plan to say why the claim lost.
func explainPlanRefusal(ctx context.Context, tx pgx.Tx, c PlanCitation) error {
	const read = `
		SELECT id, target, surface, created_by, created_at, expires_at, provisional, state_read_at, applied_at
		  FROM plans WHERE id = $1`

	plan, err := scanPlan(tx.QueryRow(ctx, read, c.PlanID))
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%w: %s", ErrPlanNotFound, c.PlanID)
	}
	if err != nil {
		return fmt.Errorf("read refused plan: %w", err)
	}
	if reason := planRefusal(plan, c, time.Now()); reason != nil {
		return fmt.Errorf("%w: %s", reason, c.PlanID)
	}
	// The row now looks claimable, so it was claimed and released between the
	// UPDATE and this read. Saying "already applied" would assert something
	// this read just contradicted; the claim was still refused.
	return fmt.Errorf("db: the plan %s could not be claimed and no longer says why", c.PlanID)
}

// planRefusal names the reason a plan is not claimable under this citation, or
// nil if it is. Pure, and never the authority: the UPDATE predicate above is,
// and the guard test holds the two to the same set of conditions.
//
// Identity first, state second. "This is not that plan" is a different sentence
// from "this plan is spent", and reporting expiry for a plan the operator never
// approved would send them to re-plan something that was never theirs.
func planRefusal(p Plan, c PlanCitation, now time.Time) error {
	switch {
	case p.Target != c.Target, p.Surface != c.Surface:
		return ErrPlanNotCitableHere
	case p.CreatedBy != c.Actor:
		return ErrPlanNotYours
	case p.AppliedAt != nil:
		return ErrPlanAlreadyApplied
	case p.ExpiresAt != nil && !p.ExpiresAt.After(now):
		return ErrPlanExpired
	}
	return nil
}

func planSubjectsTx(ctx context.Context, tx pgx.Tx, planID string) ([]PlanSubject, error) {
	const q = `
		SELECT id, plan_id, subject_id, snapshot_id, fingerprint, outcome_json
		  FROM plan_subjects WHERE plan_id = $1 ORDER BY subject_id`

	rows, err := tx.Query(ctx, q, planID)
	if err != nil {
		return nil, fmt.Errorf("read plan subjects: %w", err)
	}
	defer rows.Close()

	var subjects []PlanSubject
	for rows.Next() {
		var (
			s   PlanSubject
			raw []byte
		)
		if err := rows.Scan(&s.ID, &s.PlanID, &s.SubjectID, &s.SnapshotID, &s.Fingerprint, &raw); err != nil {
			return nil, fmt.Errorf("scan plan subject: %w", err)
		}
		if err := json.Unmarshal(raw, &s.Outcome); err != nil {
			return nil, fmt.Errorf("decode plan outcome for %s: %w", s.SubjectID, err)
		}
		subjects = append(subjects, s)
	}
	return subjects, rows.Err()
}

func scanPlan(row pgx.Row) (Plan, error) {
	var p Plan
	err := row.Scan(&p.ID, &p.Target, &p.Surface, &p.CreatedBy, &p.CreatedAt,
		&p.ExpiresAt, &p.Provisional, &p.StateReadAt, &p.AppliedAt)
	return p, err
}

// looksLikeUUID reports whether s is shaped like the ids these tables allocate.
// Shape only — existence is the database's answer, not this function's.
func looksLikeUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, r := range s {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			isHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
			if !isHex {
				return false
			}
		}
	}
	return true
}
