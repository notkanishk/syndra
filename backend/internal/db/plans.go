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
	ErrPlanNotYours = errors.New("db: the plan was approved by a different operator")
	// ErrPlanRequestMismatch: the plan is this operator's, on this surface, and
	// unspent — but the request beside it is not the one it was computed for.
	// Its own error because its own action: re-plan with what you actually want,
	// rather than "re-plan, something moved" for a world that did not move.
	ErrPlanRequestMismatch = errors.New("db: the submitted request is not the one this plan was computed for")
	ErrPlanNoSubject       = errors.New("db: the plan has no subject rows")
	ErrInvalidPlan         = errors.New("db: invalid plan")
)

// Effects a rehearsal may record for a subject. Three, because a plan states
// what WILL happen; `applied`, `failed`, and `queued` are what became of it and
// belong to the outbox row, not to the approval.
//
// Duplicated from internal/services rather than imported — that package imports
// this one — and held to the same vocabulary by a coherence guard.
const (
	PlanEffectApply    = "apply"
	PlanEffectNoChange = "no_change"
	PlanEffectBlocked  = "blocked"
)

// validPlanEffect reports whether e is one of the three effects a plan may
// record.
//
// A switch over the constants, not a lookup in a slice. An exported slice would
// have been a mutable package variable: any package could append to it — or
// replace it — before CreatePlan ran, and the "closed" vocabulary would then be
// open to precisely the callers this check exists to bound. A constant cannot
// be reassigned, and a switch over constants cannot be widened at runtime.
func validPlanEffect(e string) bool {
	switch e {
	case PlanEffectApply, PlanEffectNoChange, PlanEffectBlocked:
		return true
	default:
		return false
	}
}

// PlanOutcome is the decision recorded for one subject: what will be done, and
// to which identified rows.
//
// It is not what the operator was SHOWN. The sentences a rehearsal displays —
// "Gains this role", "Keeps the role via the Safety bundle" — are rendered from
// the snapshot at read time and are not stored, for the same reason name and
// email are not stored: a rendering persisted beside the thing it renders is a
// second copy free to go stale, and the plan's authority is its fingerprint and
// its snapshot, never its prose.
//
// That deletion is also the secret exclusion, and the earlier version of this
// type got it wrong. `outcome_json` is JSONB and will take anything, so a
// closed struct is not enough when its fields are free strings: a submitted
// password IS a string, and `Detail: fmt.Sprintf(...)` is a route to the column
// however carefully the first writer avoids it. No character class separates a
// password from a role name either. What separates them is membership in a set
// the backend owns: `Effect` is one of three constants, and every entry in
// `GrantIDs` must be a grant this database allocated to the subject the row is
// about — looked up, not pattern-matched, because a uuid shape is a syntax and
// not a provenance, and a fabricated uuid is not a grant. There is no field
// here a caller can put an arbitrary value in (design §5).
type PlanOutcome struct {
	Effect   string   `json:"effect"`
	GrantIDs []string `json:"grant_ids,omitempty"`
}

func (o PlanOutcome) validate() error {
	if !validPlanEffect(o.Effect) {
		// The vocabulary, never the value: a rejected effect is by definition
		// not one of three known constants, which makes it the likeliest thing
		// here to be something that should never be written down. Spelled from
		// the constants, so the message cannot be widened either.
		return fmt.Errorf("%w: effect must be one of %s, %s, %s",
			ErrInvalidPlan, PlanEffectApply, PlanEffectNoChange, PlanEffectBlocked)
	}
	for i, id := range o.GrantIDs {
		if !looksLikeUUID(id) {
			// Shape only, and shape is not provenance — CreatePlan verifies
			// that separately against the grants table. This check earns its
			// place by refusing before the database: an id that is not uuid
			// text would fail the cast in that lookup instead of being answered.
			//
			// The position, never the value. A value failing this check is by
			// definition not an identifier at all, which makes it the likeliest
			// thing in the struct to be a misplaced secret — and an error
			// string is logged, returned, and traced.
			return fmt.Errorf("%w: grant_ids[%d] is not shaped like an identifier", ErrInvalidPlan, i)
		}
	}
	return nil
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
	// RequestFingerprint binds the plan to the request it was computed for.
	// Never rendered: it is an integrity value, and a client that can read it
	// can tell an operator's mistyped duration from a world that moved without
	// asking the backend, which is the backend's answer to give.
	RequestFingerprint string `json:"-"`
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
	// RequestFingerprint is the digest of the request this rehearsal was
	// computed for, or "" for a plan that binds no request. Only what changes
	// the effect belongs in it — a cohort, an operation's parameters — never an
	// annotation the operator writes at apply time, which would make fixing a
	// typo in a reason cost a re-plan.
	RequestFingerprint string
	Subjects           []NewPlanSubject
}

// NewPlanSubject is one row of the rehearsal.
type NewPlanSubject struct {
	SubjectID string
	// SnapshotID cites a desired-state snapshot that already exists. Empty for a
	// Zitadel plan, whose intent is the outbox row's own columns.
	SnapshotID string
	// DesiredState is the resolved set this rehearsal proposes, for a caller
	// that has computed one and has no snapshot yet. CreatePlan writes it INSIDE
	// its own transaction and cites what it wrote.
	//
	// Here rather than in the caller because the two have to commit together. A
	// snapshot written first and a plan that then fails is an audit row citing
	// nothing that has already spent a version; a plan written first and a
	// snapshot that then fails is a citation the drain resolves to nothing and
	// terminally fails an approved change on. Mutually exclusive with SnapshotID:
	// two ways to say which intent was approved is one way for them to disagree.
	DesiredState map[string]json.RawMessage
	Fingerprint  string
	Outcome      PlanOutcome
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
	// RequestFingerprint is recomputed from the body submitted alongside the
	// citation. It is a citation dimension rather than a check the gate performs
	// first, so a request that does not match loses in the database — and loses
	// without spending the approval, because a claim that matched nothing
	// changed nothing.
	RequestFingerprint string
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
		case s.SnapshotID != "" && s.DesiredState != nil:
			// Both would leave the writer choosing, and whichever it chose the
			// other would sit on the row looking authoritative.
			return fmt.Errorf("%w: subject %s both cites a snapshot and carries one", ErrInvalidPlan, s.SubjectID)
		case s.DesiredState != nil && p.Target == TargetZitadel:
			// Zitadel's intent is the outbox row's project and role columns, and
			// its plan subjects cite no snapshot. Writing one here would be a
			// second account of one decision that nothing reads.
			return fmt.Errorf("%w: subject %s carries a desired-state snapshot for %s, which holds its intent in the outbox row", ErrInvalidPlan, s.SubjectID, TargetZitadel)
		}
		if err := s.Outcome.validate(); err != nil {
			return fmt.Errorf("%w (subject %s)", err, s.SubjectID)
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

	// Joins the caller's access-mutation transaction when there is one, so a
	// plan minted as part of a cascade commits with the role change that caused
	// it. A separate transaction would let the grant land and the convergence
	// approving it roll back — or the reverse, which is an approval for a change
	// that never happened.
	tx, owned, err := beginOrJoin(ctx)
	if err != nil {
		return Plan{}, err
	}
	if owned {
		defer tx.Rollback(ctx) // no-op after a successful Commit
	}
	plan, err := createPlanTx(ctx, tx, p)
	if err != nil {
		return Plan{}, err
	}
	if owned {
		if err := tx.Commit(ctx); err != nil {
			return Plan{}, fmt.Errorf("commit plan tx: %w", err)
		}
	}
	return plan, nil
}

// createPlanTx is the write itself, on a transaction somebody else owns.
func createPlanTx(ctx context.Context, tx pgx.Tx, p NewPlan) (Plan, error) {
	if err := p.validate(); err != nil {
		return Plan{}, err
	}

	// Canonicalise before anything compares or stores an identifier. Postgres
	// writes uuids in lowercase, so an uppercase citation matches the row in SQL
	// — where the comparison happens after a parse — and then fails to match the
	// id that comes back, which would refuse a legitimate plan as fabricated.
	// Normalising the value rather than the check also keeps the stored row
	// comparable: every later reader compares against what the database returns.
	subjects := canonicalSubjects(p.Subjects)

	// Provenance before persistence. Shape was checked above and shape is not
	// provenance: a uuid is a syntax, and a value in that syntax that names no
	// row is not a reference to anything the apply can act on.
	if err := verifyGrantProvenance(ctx, tx, subjects); err != nil {
		return Plan{}, err
	}
	if err := verifySnapshotProvenance(ctx, tx, p.Target, subjects); err != nil {
		return Plan{}, err
	}

	// The lifetime is measured by the database clock, because the expiry
	// predicate is evaluated by the database clock. Computing the deadline here
	// would make a plan's validity depend on the difference between two clocks.
	const insertPlan = `
		INSERT INTO plans (target, surface, created_by, expires_at, provisional, state_read_at, request_fingerprint)
		VALUES ($1, $2, $3,
		        CASE WHEN $5::boolean THEN NULL ELSE NOW() + $4::interval END,
		        $5, $6, $7)
		RETURNING id, created_at, expires_at, applied_at`

	var (
		lifetime    = fmt.Sprintf("%d seconds", int64(p.Lifetime/time.Second))
		stateReadAt *time.Time
	)
	if !p.StateReadAt.IsZero() {
		t := p.StateReadAt
		stateReadAt = &t
	}

	plan := Plan{Target: p.Target, Surface: p.Surface, CreatedBy: p.CreatedBy, Provisional: p.Provisional,
		StateReadAt: stateReadAt, RequestFingerprint: p.RequestFingerprint}
	if err := tx.QueryRow(ctx, insertPlan,
		p.Target, p.Surface, p.CreatedBy, lifetime, p.Provisional, stateReadAt, p.RequestFingerprint,
	).Scan(&plan.ID, &plan.CreatedAt, &plan.ExpiresAt, &plan.AppliedAt); err != nil {
		return Plan{}, fmt.Errorf("insert plan: %w", err)
	}

	const insertSubject = `
		INSERT INTO plan_subjects (plan_id, subject_id, snapshot_id, fingerprint, outcome_json)
		VALUES ($1, $2, $3, $4, $5)`
	for _, s := range subjects {
		outcome, err := json.Marshal(s.Outcome)
		if err != nil {
			return Plan{}, fmt.Errorf("marshal plan outcome for %s: %w", s.SubjectID, err)
		}
		var snapshot *string
		if s.DesiredState != nil {
			// Written on this transaction, so the intent and the approval citing
			// it commit together or neither does. Provenance needs no check: the
			// row was created here, for this subject, on this target.
			written, err := WriteDesiredStateSnapshotTx(ctx, tx, s.SubjectID, p.Target, p.CreatedBy, s.DesiredState)
			if err != nil {
				return Plan{}, err
			}
			s.SnapshotID = written.ID
		}
		if s.SnapshotID != "" {
			id := s.SnapshotID
			snapshot = &id
		}
		if _, err := tx.Exec(ctx, insertSubject, plan.ID, s.SubjectID, snapshot, s.Fingerprint, outcome); err != nil {
			return Plan{}, fmt.Errorf("insert plan subject %s: %w", s.SubjectID, err)
		}
	}

	return plan, nil
}

// verifyGrantProvenance reads the grants a plan names and hands the judgement
// to matchGrantOwners.
//
// Split that way on purpose: the lookup is the only part that needs a database,
// so the rule it enforces stays testable. It runs on the plan's own
// transaction, so the rows it read are the rows the plan is written against.
func verifyGrantProvenance(ctx context.Context, tx pgx.Tx, subjects []NewPlanSubject) error {
	var cited []string
	for _, s := range subjects {
		cited = append(cited, s.Outcome.GrantIDs...)
	}
	if len(cited) == 0 {
		return nil
	}

	// text[] cast to uuid[] rather than a uuid[] parameter: pgx sends a
	// []string in binary and has no binary encoding for uuid[], so the typed
	// form fails at dispatch time. Every element is already known to be uuid
	// text, which is what makes the cast safe.
	const q = `SELECT id, user_id FROM direct_role_grants WHERE id = ANY($1::text[]::uuid[])`

	rows, err := tx.Query(ctx, q, cited)
	if err != nil {
		return fmt.Errorf("read cited grants: %w", err)
	}
	defer rows.Close()

	owner := make(map[string]string, len(cited))
	for rows.Next() {
		var id, userID string
		if err := rows.Scan(&id, &userID); err != nil {
			return fmt.Errorf("scan cited grant: %w", err)
		}
		// Lowercased on the way in as well as on the way out. Postgres renders
		// uuids canonically, so under Postgres this is a no-op and no test can
		// tell it from its absence — it is here because the whole bug class is
		// two halves of one comparison disagreeing about text, and paying a
		// ToLower to stop that recurring on a driver or database that renders
		// differently is cheaper than the refusal it would otherwise produce.
		owner[strings.ToLower(id)] = userID
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read cited grants: %w", err)
	}
	return matchGrantOwners(subjects, owner)
}

// matchGrantOwners refuses a plan citing a grant this database did not
// allocate, or one that belongs to somebody else.
//
// The second half is not pedantry. A plan row is what the apply acts on, so a
// subject's row naming another person's grant is an instruction to mutate that
// person — reviewed under a heading with somebody else's name on it, and
// fingerprinted against a subject whose state it does not describe.
func matchGrantOwners(subjects []NewPlanSubject, owner map[string]string) error {
	for _, s := range subjects {
		for i, id := range s.Outcome.GrantIDs {
			switch holder, known := owner[id]; {
			case !known:
				// Position, not value: an id naming no grant is exactly the
				// case where the value might be something else entirely.
				return fmt.Errorf("%w: subject %s cites grant_ids[%d], which this database did not allocate", ErrInvalidPlan, s.SubjectID, i)
			case holder != s.SubjectID:
				// And not the holder's id either — naming the other person
				// discloses one subject's grants on another's refusal.
				return fmt.Errorf("%w: subject %s cites grant_ids[%d], which belongs to a different person", ErrInvalidPlan, s.SubjectID, i)
			}
		}
	}
	return nil
}

// verifySnapshotProvenance refuses a plan citing a snapshot that is not this
// subject's desired state for this target.
//
// The foreign key proves the snapshot exists and nothing more. Existence was
// never the property: one approval, one durable object means the snapshot and
// the fingerprint that verifies it describe the same person on the same target.
// A snapshot taken for somebody else, stored under this subject's heading, is a
// desired state the operator did not approve for the person it will be applied
// to — and the fingerprint beside it would verify a subject the snapshot does
// not describe.
//
// Checking here also keeps a fabricated id away from the foreign key, whose
// violation message quotes the value that broke it.
func verifySnapshotProvenance(ctx context.Context, tx pgx.Tx, target string, subjects []NewPlanSubject) error {
	var cited []string
	for _, s := range subjects {
		if s.SnapshotID != "" {
			cited = append(cited, s.SnapshotID)
		}
	}
	if len(cited) == 0 {
		return nil
	}

	const q = `SELECT id, subject_id, target FROM desired_state_snapshots WHERE id = ANY($1::text[]::uuid[])`

	rows, err := tx.Query(ctx, q, cited)
	if err != nil {
		return fmt.Errorf("read cited snapshots: %w", err)
	}
	defer rows.Close()

	taken := make(map[string]snapshotRef, len(cited))
	for rows.Next() {
		var id string
		var ref snapshotRef
		if err := rows.Scan(&id, &ref.subject, &ref.target); err != nil {
			return fmt.Errorf("scan cited snapshot: %w", err)
		}
		taken[strings.ToLower(id)] = ref // no-op under Postgres; see verifyGrantProvenance
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read cited snapshots: %w", err)
	}
	return matchSnapshotSubjects(subjects, target, taken)
}

// snapshotRef is who and what a desired-state snapshot was taken for.
type snapshotRef struct{ subject, target string }

// matchSnapshotSubjects is the judgement half, kept out of the database so the
// rule it enforces can be tested.
func matchSnapshotSubjects(subjects []NewPlanSubject, target string, taken map[string]snapshotRef) error {
	for _, s := range subjects {
		if s.SnapshotID == "" {
			continue
		}
		switch ref, known := taken[s.SnapshotID]; {
		case !known:
			return fmt.Errorf("%w: subject %s cites a snapshot this database did not record", ErrInvalidPlan, s.SubjectID)
		case ref.subject != s.SubjectID:
			// Not naming whose it is: a refusal is not a lookup service for
			// other people's desired state.
			return fmt.Errorf("%w: subject %s cites a snapshot taken for a different person", ErrInvalidPlan, s.SubjectID)
		case ref.target != target:
			return fmt.Errorf("%w: subject %s cites a snapshot taken for a different target", ErrInvalidPlan, s.SubjectID)
		}
	}
	return nil
}

// canonicalSubjects returns the rows with every cited identifier in the form
// the database stores, leaving the caller's slice untouched.
//
// Uppercase uuid text is legitimate input — it is the same identifier — but it
// is not the text Postgres returns, and every check downstream compares Go
// strings against what Postgres returned.
func canonicalSubjects(subjects []NewPlanSubject) []NewPlanSubject {
	out := make([]NewPlanSubject, len(subjects))
	for i, s := range subjects {
		s.SnapshotID = strings.ToLower(s.SnapshotID)
		if len(s.Outcome.GrantIDs) > 0 {
			ids := make([]string, len(s.Outcome.GrantIDs))
			for j, id := range s.Outcome.GrantIDs {
				ids[j] = strings.ToLower(id)
			}
			s.Outcome.GrantIDs = ids
		}
		out[i] = s
	}
	return out
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
		   AND request_fingerprint = $5
		   AND applied_at IS NULL
		   AND (expires_at IS NULL OR expires_at > NOW())
		RETURNING id, target, surface, created_by, created_at, expires_at, provisional, state_read_at, applied_at, request_fingerprint`

	plan, err := scanPlan(tx.QueryRow(ctx, claim, c.PlanID, c.Target, c.Surface, c.Actor, c.RequestFingerprint))
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

// ClaimPlanVerified spends an approval only if the world it describes is still
// the world.
//
// It opens the transaction ClaimPlanTx needs and runs `verify` inside it, so a
// plan whose subjects have moved leaves the approval **unspent**: the claim is
// rolled back with the rest. That ordering is the whole point. Claiming first
// and verifying after would burn the one apply an approval gets on a stale
// plan, and the operator's next move — re-plan and apply — would then be
// refused as already-applied for something that never happened.
//
// The caller's own mutations are deliberately NOT in this transaction. They are
// per-subject writes with their own transactions, cache rebuilds and inline
// drains, and pulling them in here would hold a transaction open across
// Management API calls. The cost is stated rather than hidden: a crash between
// this commit and the first mutation spends an approval that applied nothing,
// and the operator re-plans. That is the fail-closed direction — the other one
// applies a diff twice.
func ClaimPlanVerified(ctx context.Context, c PlanCitation, verify func([]PlanSubject) error) (Plan, []PlanSubject, error) {
	tx, err := PG.Begin(ctx)
	if err != nil {
		return Plan{}, nil, fmt.Errorf("begin plan claim: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after a successful Commit

	plan, subjects, err := ClaimPlanTx(ctx, tx, c)
	if err != nil {
		return Plan{}, nil, err
	}
	if verify != nil {
		if err := verify(subjects); err != nil {
			return Plan{}, nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Plan{}, nil, fmt.Errorf("commit plan claim: %w", err)
	}
	return plan, subjects, nil
}

// explainPlanRefusal re-reads the plan to say why the claim lost.
func explainPlanRefusal(ctx context.Context, tx pgx.Tx, c PlanCitation) error {
	const read = `
		SELECT id, target, surface, created_by, created_at, expires_at, provisional, state_read_at, applied_at, request_fingerprint
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
	case p.RequestFingerprint != c.RequestFingerprint:
		// Identity, still: this plan is not a plan for that request. Reported
		// ahead of state for the same reason the two above are — telling an
		// operator their approval went stale, when what actually happened is
		// that they edited the form, sends them to re-review a world that never
		// moved.
		return ErrPlanRequestMismatch
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
		&p.ExpiresAt, &p.Provisional, &p.StateReadAt, &p.AppliedAt, &p.RequestFingerprint)
	return p, err
}

// looksLikeUUID reports whether s is shaped like the ids these tables allocate.
// Shape only — existence is the database's answer, not this function's.
//
// Case-insensitive, because uppercase uuid text names the same identifier. That
// is safe only in company: Postgres returns the lowercase form, so anything
// compared or stored must go through canonicalSubjects first, or an uppercase
// citation would match its row in SQL and then fail to match the id that came
// back — a legitimate plan refused as fabricated.
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
