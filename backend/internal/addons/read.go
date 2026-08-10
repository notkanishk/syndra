package addons

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"
)

// The read legs of the transport: `POST /plan` and `GET /subjects` (design §5).
//
// Neither mutates, so neither carries a call id, a durable record, or a
// one-shot token — those exist to stop a mutation happening twice, and a read
// that happens twice is a read. What they DO share with the mutating legs is
// `doAuthenticated`: the manifest a policy is intersected against, the diff an
// operator approves, and the inventory an operator adopts from are all decisions
// the backend makes from what the add-on says, so the channel carrying them has
// to be as trustworthy as the one carrying the write.
//
// They differ from each other in one way worth stating. A plan is a question
// about a proposed change and is only ever asked of a reachable target. A
// subjects read deliberately survives an outage: the add-on answers from its
// mirror and labels the answer stale, because the alternative — an empty list —
// is a statement that the target holds no accounts, and every consumer here
// would act on it.

// PlanSubject is one subject inside a proposed change.
//
// It carries no fingerprint: this call is where a fingerprint comes from. It
// carries no call id and no plan id for the same reason — the plan being asked
// for is what will get one.
type PlanSubject struct {
	Subject string
	// Email is what a username is derived from, for a subject with no account on
	// the target yet. Absent for a subject that already has one, where the
	// recorded binding is authoritative.
	Email string
	// Desired is the resolved set by field name, exactly as an apply would carry
	// it — a plan computed from a different set than the apply sends is a plan
	// of nothing.
	Desired map[string]json.RawMessage
}

// SubjectOutcome is what would happen to one subject, in the same shape the
// apply returns afterwards, so one renderer serves the diff and the result.
type SubjectOutcome struct {
	Subject     string `json:"subject"`
	Effect      string `json:"effect"`
	Detail      string `json:"detail"`
	Consequence string `json:"consequence,omitempty"`
	Username    string `json:"username,omitempty"`
	// Fingerprint is the state this outcome was computed against. The apply
	// carries it back and the add-on refuses if the subject moved since.
	Fingerprint string `json:"fingerprint,omitempty"`
	// Conflict is set when an unbound account already holds the name this
	// subject's account would be created under. Reported rather than resolved:
	// that account may belong to somebody else entirely, and adopting it
	// silently would hand them this subject's entitlements.
	Conflict *BindingConflict `json:"conflict,omitempty"`
}

// BindingConflict is an unbound target account holding a derived name.
type BindingConflict struct {
	Username string `json:"username"`
	UID      int64  `json:"uid"`
	// Adoptable says no other subject already claims this account. A conflict
	// that is not adoptable is a bug somewhere else and must not be offered as
	// a one-click resolution.
	Adoptable bool `json:"adoptable"`
	// BoundTo names the subject already holding it, when one does.
	BoundTo string `json:"bound_to,omitempty"`
}

// PlanResult is the whole answer, plus what the backend learned about the call.
type PlanResult struct {
	Outcomes []SubjectOutcome `json:"outcomes"`
	// Current says the add-on computed this against a live read. False means it
	// answered from its mirror because the target was unreachable, and the plan
	// the backend issues from it is provisional: recorded, labelled with the
	// age below, and gated by a re-fingerprint on the target's return rather
	// than by a clock (design §8).
	Current bool `json:"current"`
	// TakenAt is when the read behind these outcomes happened, which is the age
	// a provisional plan is labelled with.
	TakenAt time.Time `json:"taken_at"`
	// Truncated says the read hit the add-on's cap. The add-on already blocks
	// the outcomes that would have concluded an absence from it; this is carried
	// so the surface can say why a cohort came back mostly blocked.
	Truncated bool    `json:"truncated"`
	Outcome   Outcome `json:"-"`
	Status    int     `json:"-"`
	Err       error   `json:"-"`
	// LifecycleRefusal is the add-on declining while draining or read-only. A
	// plan is a read and is not gated by lifecycle state on this add-on, so it
	// is carried for completeness rather than expected.
	LifecycleRefusal bool `json:"-"`
}

// planEnvelope is the wire body.
type planEnvelope struct {
	ContractVersion  int               `json:"contract_version"`
	Subjects         []planSubjectWire `json:"subjects"`
	AcknowledgeScope bool              `json:"acknowledge_scope,omitempty"`
}

type planSubjectWire struct {
	Subject string                     `json:"subject"`
	Email   string                     `json:"email"`
	Desired map[string]json.RawMessage `json:"desired"`
}

// ErrNoSubjects refuses a plan for nobody.
//
// A plan with no subjects would be answered with an empty outcome list, stored
// as an approval, and applied cleanly while changing nothing — the most
// convincing possible way to do nothing.
var ErrNoSubjects = errors.New("addon: refusing to plan a change affecting no subjects")

// Plan asks what a proposed change would do, and mutates nothing.
//
// `acknowledgeScope` is the caller confirming an oversized cohort. It is passed
// through rather than decided here: the add-on sees one request and cannot
// observe a cohort spanning several, so the authoritative blast-radius guard is
// the backend's, at the point the cohort exists.
func Plan(ctx context.Context, target string, subjects []PlanSubject, acknowledgeScope bool) PlanResult {
	if len(subjects) == 0 {
		return PlanResult{Outcome: OutcomeUnreached, Err: ErrNoSubjects}
	}
	a, err := Get(target)
	if err != nil {
		return PlanResult{Outcome: OutcomeUnreached, Err: err}
	}
	if !a.br.allow(timeNow()) {
		return PlanResult{Outcome: OutcomeUnreached, Err: fmt.Errorf("%w: %s", ErrCircuitOpen, target)}
	}
	cred, err := credentialFor(a.Registration)
	if err != nil {
		return PlanResult{Outcome: OutcomeUnreached, Err: fmt.Errorf("addon %s: %w", target, err)}
	}

	wire := make([]planSubjectWire, 0, len(subjects))
	for _, s := range subjects {
		wire = append(wire, planSubjectWire{Subject: s.Subject, Email: s.Email, Desired: s.Desired})
	}
	body, err := json.Marshal(planEnvelope{
		ContractVersion: ContractVersion, Subjects: wire, AcknowledgeScope: acknowledgeScope,
	})
	if err != nil {
		return PlanResult{Outcome: OutcomeUnreached, Err: fmt.Errorf("addon %s: encode plan: %w", target, err)}
	}

	resp := doAuthenticated(ctx, cred, http.MethodPost, a.Registration.BaseURL+"/plan", body, callTimeout)
	a.br.record(timeNow(), resp)

	out := PlanResult{
		Outcome: resp.Outcome, Status: resp.Status, Err: resp.Err,
		LifecycleRefusal: resp.LifecycleRefusal,
	}
	if resp.Outcome == OutcomeSucceeded {
		var decoded struct {
			Outcomes  []SubjectOutcome `json:"outcomes"`
			Current   bool             `json:"current"`
			TakenAt   string           `json:"taken_at"`
			Truncated bool             `json:"truncated"`
		}
		if err := json.Unmarshal(resp.Body, &decoded); err != nil {
			// A 2xx whose body will not decode is not a plan. Reported as
			// indeterminate rather than as an empty plan, which would be an
			// approval of nothing that reads like an approval of something.
			out.Outcome, out.Err = OutcomeIndeterminate, fmt.Errorf("addon %s: decode plan: %w", target, err)
			return out
		}
		out.Outcomes, out.Current, out.Truncated = decoded.Outcomes, decoded.Current, decoded.Truncated
		if ts, err := time.Parse(time.RFC3339, decoded.TakenAt); err == nil {
			out.TakenAt = ts.UTC()
		}
		if !out.Current && out.TakenAt.IsZero() {
			// A provisional plan whose age cannot be read is refused rather than
			// issued with no age: "computed against last-known state" with no
			// number beside it is exactly the label an operator cannot act on,
			// and the plan store requires the read time for the same reason.
			out.Outcome = OutcomeIndeterminate
			out.Err = fmt.Errorf("addon %s: the plan is not current and carries no read time to date it by", target)
		}
	}
	if out.Outcome != OutcomeSucceeded {
		// The bodies are not logged. The request carries a resolved entitlement
		// set — a person's access — and the response carries whatever the least
		// trusted component chose to send back.
		log.Printf("[ADDON] %s/plan subjects=%d outcome=%s status=%d err=%v",
			target, len(subjects), out.Outcome, out.Status, out.Err)
	}
	return out
}

// TargetAccount is one account the target holds, as the backend understands it.
//
// Username and uid and nothing else. The backend does not know what any other
// field on a target account means — that is the whole point of the entitlement
// schema being declared rather than compiled in — and an inventory listing is
// answering one question: what else lives here that Syndra did not put there.
type TargetAccount struct {
	Username string `json:"username"`
	// UID is the target's stable identity for the account, which survives a
	// rename. It is what a binding is matched on, so that an account renamed out
	// of band is recognised rather than reported as somebody else's.
	UID int64 `json:"uid"`
}

// SubjectsResult is a full state read.
type SubjectsResult struct {
	Accounts []TargetAccount
	// Current says the read is live rather than served from the add-on's mirror.
	// A stale read may be shown to an operator, labelled with its age; it must
	// never be diffed, because every change made during the outage would be
	// reported as drift.
	Current bool
	// Truncated says the read hit the add-on's cap. A truncated read is current
	// and still cannot support a conclusion about ABSENCE, which is what an
	// inventory listing and half a drift diff both are.
	Truncated bool
	TakenAt   time.Time

	Outcome Outcome
	Status  int
	Err     error
}

// Usable reports whether this read may be concluded from.
//
// Both conditions, and stated once here rather than re-derived at each consumer:
// a read that is not current describes a world that has moved, and a read that
// was truncated describes part of the world it did see. Either one turns "this
// account is not on the target" — the sentence every consumer of this read
// writes — into a guess.
func (r SubjectsResult) Usable() bool {
	return r.Outcome == OutcomeSucceeded && r.Current && !r.Truncated
}

// subjectsEnvelope is the add-on's answer. Its `subjects` carry target-specific
// state fields this backend deliberately does not decode.
type subjectsEnvelope struct {
	Subjects  []TargetAccount `json:"subjects"`
	Current   bool            `json:"current"`
	TakenAt   string          `json:"taken_at"`
	Truncated bool            `json:"truncated"`
}

// Subjects reads every account the target holds.
//
// Unlike Plan this tolerates an unreachable target, because the add-on does: it
// answers from its mirror and says so. What it must not do is turn a failure
// into an empty list — every consumer reads absence from this list, and an
// empty one says the target holds nothing at all.
func Subjects(ctx context.Context, target string) SubjectsResult {
	a, err := Get(target)
	if err != nil {
		return SubjectsResult{Outcome: OutcomeUnreached, Err: err}
	}
	if !a.br.allow(timeNow()) {
		return SubjectsResult{Outcome: OutcomeUnreached, Err: fmt.Errorf("%w: %s", ErrCircuitOpen, target)}
	}
	cred, err := credentialFor(a.Registration)
	if err != nil {
		return SubjectsResult{Outcome: OutcomeUnreached, Err: fmt.Errorf("addon %s: %w", target, err)}
	}

	resp := doAuthenticated(ctx, cred, http.MethodGet, a.Registration.BaseURL+"/subjects", nil, callTimeout)
	a.br.record(timeNow(), resp)

	out := SubjectsResult{Outcome: resp.Outcome, Status: resp.Status, Err: resp.Err}
	if resp.Outcome != OutcomeSucceeded {
		log.Printf("[ADDON] %s/subjects outcome=%s status=%d err=%v", target, out.Outcome, out.Status, out.Err)
		return out
	}

	var decoded subjectsEnvelope
	if err := json.Unmarshal(resp.Body, &decoded); err != nil {
		out.Outcome, out.Err = OutcomeIndeterminate, fmt.Errorf("addon %s: decode subjects: %w", target, err)
		return out
	}
	out.Accounts, out.Current, out.Truncated = decoded.Subjects, decoded.Current, decoded.Truncated
	if decoded.TakenAt != "" {
		// A read whose timestamp will not parse is served without one rather
		// than refused: the accounts are still what the target holds, and the
		// staleness label is the thing that degrades. Consumers that need the
		// age check `Current` first, which does not depend on this.
		if ts, err := time.Parse(time.RFC3339, decoded.TakenAt); err == nil {
			out.TakenAt = ts.UTC()
		}
	}
	return out
}

// TargetHealth is what an add-on says about itself and its target.
//
// Decoded field by field rather than passed through as a blob, because two of
// these are read by machinery and not only by people: `LogHead` and `LogRecords`
// are what the backend anchors, and a shape it does not understand is a shape it
// cannot anchor.
type TargetHealth struct {
	Reachable      bool   `json:"reachable"`
	Product        string `json:"product"`
	ProductVersion string `json:"product_version"`
	VersionTested  bool   `json:"version_tested"`
	VersionNote    string `json:"version_note,omitempty"`
	// CircuitOpen is the ADD-ON's own breaker against its target, which is a
	// different thing from the backend's breaker against the add-on. Both exist
	// and an operator reading only one would look in the wrong place.
	CircuitOpen   bool   `json:"circuit_open"`
	Lifecycle     string `json:"lifecycle"`
	LifecycleNote string `json:"lifecycle_note,omitempty"`
	InFlight      int64  `json:"in_flight"`
	Drained       bool   `json:"drained"`
	// LogHead and LogRecords are the mutation log's chain head and length. The
	// backend anchors them: a chain verifies its own contents and cannot notice
	// its own truncation, so somebody outside has to remember where the head was.
	LogHead    string `json:"log_head"`
	LogRecords int64  `json:"log_records"`
	// SnapshotTakenAt dates the add-on's mirror, which is what it serves a read
	// from when its target is unreachable.
	SnapshotTakenAt string `json:"snapshot_taken_at,omitempty"`
	LastReadAt      string `json:"last_read_at,omitempty"`
	KeyExpiresAt    string `json:"key_expires_at,omitempty"`

	Outcome Outcome `json:"-"`
	Status  int     `json:"-"`
	Err     error   `json:"-"`
}

// Health reads one add-on's own account of itself.
//
// Served by the add-on even when its target is unreachable, deliberately: a
// health endpoint that fails whole because one of five reads failed tells an
// operator nothing about the other four, which are the ones that would have
// explained it.
func Health(ctx context.Context, target string) TargetHealth {
	a, err := Get(target)
	if err != nil {
		return TargetHealth{Outcome: OutcomeUnreached, Err: err}
	}
	// The breaker is consulted, and a health read does NOT clear it on success
	// any differently from another call — it goes through the same record. An
	// operator watching a target come back reads this; letting it bypass the
	// breaker would make the surface disagree with what every other call gets.
	if !a.br.allow(timeNow()) {
		return TargetHealth{Outcome: OutcomeUnreached, Err: fmt.Errorf("%w: %s", ErrCircuitOpen, target)}
	}
	cred, err := credentialFor(a.Registration)
	if err != nil {
		return TargetHealth{Outcome: OutcomeUnreached, Err: fmt.Errorf("addon %s: %w", target, err)}
	}

	resp := doAuthenticated(ctx, cred, http.MethodGet, a.Registration.BaseURL+"/health", nil, callTimeout)
	a.br.record(timeNow(), resp)

	out := TargetHealth{Outcome: resp.Outcome, Status: resp.Status, Err: resp.Err}
	if resp.Outcome != OutcomeSucceeded {
		return out
	}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		out.Outcome, out.Err = OutcomeIndeterminate, fmt.Errorf("addon %s: decode health: %w", target, err)
		return out
	}
	out.Outcome, out.Status = OutcomeSucceeded, resp.Status
	return out
}
