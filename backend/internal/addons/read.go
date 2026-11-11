package addons

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"syndra/internal/db"
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
	// AccountID and Standing name the account the outcome is about, and
	// whether it can be used yet; both present only when an account exists.
	AccountID string    `json:"account_id,omitempty"`
	Standing  *Standing `json:"standing,omitempty"`
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
	Username  string `json:"username"`
	AccountID string `json:"account_id"`
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

// planEnvelope is the wire body. There is no acknowledgement on it: contract 2
// gives the cohort guard to the backend, which plans in chunks below the
// add-on's limits.subjects_per_plan (design §11).
type planEnvelope struct {
	ContractVersion int               `json:"contract_version"`
	Subjects        []planSubjectWire `json:"subjects"`
}

type planSubjectWire struct {
	Subject string                     `json:"subject"`
	Email   string                     `json:"email"`
	Desired map[string]json.RawMessage `json:"desired"`
}

// planWire is the reply as it travels (plan_response.json). Pointers where
// absence is never a value (design §2(b)): a plan that does not say it is
// current is not current, one that does not say it is complete is truncated,
// and an outcome that names no effect says nothing.
type planWire struct {
	Outcomes []struct {
		SubjectOutcome
		Effect *string `json:"effect"`
	} `json:"outcomes"`
	Current   *bool   `json:"current"`
	TakenAt   *string `json:"taken_at"`
	Truncated *bool   `json:"truncated"`
}

// takenAt reads a reply's taken_at: present, RFC 3339, and not the zero time.
func takenAt(raw *string) (time.Time, bool) {
	if raw == nil {
		return time.Time{}, false
	}
	ts, err := time.Parse(time.RFC3339, *raw)
	if err != nil || ts.IsZero() {
		return time.Time{}, false
	}
	return ts.UTC(), true
}

// ErrNoSubjects refuses a plan for nobody.
//
// A plan with no subjects would be answered with an empty outcome list, stored
// as an approval, and applied cleanly while changing nothing — the most
// convincing possible way to do nothing.
var ErrNoSubjects = errors.New("addon: refusing to plan a change affecting no subjects")

// Plan asks what a proposed change would do, and mutates nothing.
//
// A cohort larger than the add-on's limits.subjects_per_plan is asked in
// chunks below it and answered as one plan: current only if every chunk was,
// truncated if any was, dated by the oldest read. The blast-radius guard is the
// backend's (the plan surfaces' acknowledgement), at the point the cohort
// exists; the add-on only refuses a plan above its limit (PLAN_TOO_LARGE).
func Plan(ctx context.Context, target string, subjects []PlanSubject) PlanResult {
	if len(subjects) == 0 {
		return PlanResult{Outcome: OutcomeUnreached, Err: ErrNoSubjects}
	}
	a, err := Get(target)
	if err != nil {
		return PlanResult{Outcome: OutcomeUnreached, Err: err}
	}
	size := len(subjects)
	if m, err := a.Manifest(); err == nil && m.Limits.SubjectsPerPlan > 0 {
		size = m.Limits.SubjectsPerPlan
	}
	var out PlanResult
	for start := 0; start < len(subjects); start += size {
		part := planChunk(ctx, a, target, subjects[start:min(start+size, len(subjects))])
		if part.Outcome != OutcomeSucceeded {
			return part
		}
		if start == 0 {
			out = part
			continue
		}
		out.Outcomes = append(out.Outcomes, part.Outcomes...)
		out.Current = out.Current && part.Current
		out.Truncated = out.Truncated || part.Truncated
		if part.TakenAt.Before(out.TakenAt) {
			out.TakenAt = part.TakenAt
		}
	}
	return out
}

func planChunk(ctx context.Context, a *Addon, target string, subjects []PlanSubject) PlanResult {
	wire := make([]planSubjectWire, 0, len(subjects))
	for _, s := range subjects {
		wire = append(wire, planSubjectWire{Subject: s.Subject, Email: s.Email, Desired: s.Desired})
	}
	body, err := json.Marshal(planEnvelope{ContractVersion: ContractVersion, Subjects: wire})
	if err != nil {
		return PlanResult{Outcome: OutcomeUnreached, Err: fmt.Errorf("addon %s: encode plan: %w", target, err)}
	}

	resp, err := a.send(ctx, sendSpec{Method: http.MethodPost, Path: "/plan", Body: body})
	if err != nil {
		return PlanResult{Outcome: OutcomeUnreached, Err: err}
	}

	out := PlanResult{
		Outcome: resp.Outcome, Status: resp.Status, Err: resp.Err,
		LifecycleRefusal: resp.LifecycleRefusal,
	}
	if resp.Outcome == OutcomeSucceeded {
		var decoded planWire
		if err := json.Unmarshal(resp.Body, &decoded); err != nil {
			// A 2xx whose body will not decode is not a plan. Reported as
			// indeterminate rather than as an empty plan, which would be an
			// approval of nothing that reads like an approval of something.
			out.Outcome, out.Err = OutcomeIndeterminate, fmt.Errorf("addon %s: decode plan: %w", target, err)
			return out
		}
		out.Current = decoded.Current != nil && *decoded.Current
		out.Truncated = decoded.Truncated == nil || *decoded.Truncated
		for _, o := range decoded.Outcomes {
			if o.Effect == nil {
				out.Outcome, out.Err = OutcomeIndeterminate, fmt.Errorf("addon %s: a plan outcome for %s names no effect", target, o.Subject)
				return out
			}
			o.SubjectOutcome.Effect = *o.Effect
			out.Outcomes = append(out.Outcomes, o.SubjectOutcome)
		}
		// No readable taken_at is not current and not complete, and then
		// (below) not a plan at all: a chunk merge can never meet a zero date.
		if at, ok := takenAt(decoded.TakenAt); ok {
			out.TakenAt = at
		} else {
			out.Current, out.Truncated = false, true
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
// Its name, its id and its standing, and nothing else of the target's. The
// backend does not know what any other field on a target account means — that
// is the whole point of the entitlement schema being declared rather than
// compiled in — and an inventory listing is answering one question: what else
// lives here that Syndra did not put there.
type TargetAccount struct {
	Username string `json:"username"`
	// AccountID is the target's stable identity for the account (contract 2's
	// account_id), which survives a rename. It is what a binding is matched
	// on, so that an account renamed out of band is recognised rather than
	// reported as somebody else's.
	AccountID string `json:"account_id"`
	// Standing is whether the account can be used yet and who is waited on
	// (design §6): an account Syndra created has no password until its member
	// sets one, which is a legitimate pending state, not nothing having
	// happened. In the add-on's words, never a product word in the core.
	Standing *Standing `json:"standing,omitempty"`
	// Self marks the add-on's own service account on the target.
	//
	// It is a real, genuinely unmanaged account, and it is the one whose
	// deletion removes Syndra's access to the target altogether. The add-on
	// refuses to adopt or purge it whatever any caller asks; this is carried so
	// an operator surface can say so BEFORE somebody meets the refusal.
	Self bool `json:"self,omitempty"`
	// State is what the account currently holds, keyed by the field names the
	// target's own manifest declares — THEIRS, in the three-way comparison
	// reconciliation performs (change `reconciliation-as-merge`).
	//
	// Sent by the add-on in entitlement vocabulary rather than derived here from
	// the target's own words. Mapping `groups` to `group` or `locked` to
	// `enabled` in this package would put what TrueNAS calls things inside the
	// component that must not know, and the next add-on would need a second
	// mapping beside it.
	State map[string]json.RawMessage `json:"state,omitempty"`
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

// subjectsWire is the add-on's answer as it travels (subjects.json). Its
// `subjects` carry target-specific fields this backend deliberately does not
// decode. Pointers where absence is never a value (design §2(b)): a read that
// does not say it is current is not, and one that does not say it is complete
// is truncated, so neither can be concluded from.
type subjectsWire struct {
	Subjects  []TargetAccount `json:"subjects"`
	Current   *bool           `json:"current"`
	TakenAt   *string         `json:"taken_at"`
	Truncated *bool           `json:"truncated"`
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
	resp, err := a.send(ctx, sendSpec{Method: http.MethodGet, Path: "/subjects"})
	if err != nil {
		return SubjectsResult{Outcome: OutcomeUnreached, Err: err}
	}

	out := SubjectsResult{Outcome: resp.Outcome, Status: resp.Status, Err: resp.Err}
	if resp.Outcome != OutcomeSucceeded {
		log.Printf("[ADDON] %s/subjects outcome=%s status=%d err=%v", target, out.Outcome, out.Status, out.Err)
		return out
	}

	var decoded subjectsWire
	if err := json.Unmarshal(resp.Body, &decoded); err != nil {
		out.Outcome, out.Err = OutcomeIndeterminate, fmt.Errorf("addon %s: decode subjects: %w", target, err)
		return out
	}
	out.Accounts = decoded.Subjects
	out.Current = decoded.Current != nil && *decoded.Current
	out.Truncated = decoded.Truncated == nil || *decoded.Truncated
	// Every subject is an account, so every one carries standing.usable; one
	// that does not makes the read one nothing may be concluded from.
	var standing struct {
		Subjects []struct {
			Standing *struct {
				Usable *bool `json:"usable"`
			} `json:"standing"`
		} `json:"subjects"`
	}
	_ = json.Unmarshal(resp.Body, &standing)
	for _, s := range standing.Subjects {
		if s.Standing == nil || s.Standing.Usable == nil {
			log.Printf("[ADDON] %s/subjects lists an account with no standing.usable; the read is not usable", target)
			out.Current, out.Truncated = false, true
			break
		}
	}
	// A read that does not say, readably, when it happened is not one to
	// conclude from (design §2(b)): the accounts are still served, labelled
	// unusable, never dated by Go's zero time.
	if at, ok := takenAt(decoded.TakenAt); ok {
		out.TakenAt = at
	} else {
		out.Current, out.Truncated = false, true
	}
	return out
}

// TargetHealth is what an add-on says about itself and its target: the view
// the target page reads, built from healthWire, never decoded into.
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
	// KeyExpiry says which of three states the credential is in, so an absent
	// date is never read as a probe that failed: `set`, `none` (issued without
	// one, deliberately) or `unrecorded` (nobody told Syndra, and the key can
	// still expire — at which point the target simply stops answering and reads
	// as an outage).
	KeyExpiry string `json:"key_expiry,omitempty"`
	// UnauditedShares names SMB shares with auditing off, and SharesReadable
	// says whether the list could be read at all. Without the second, "nothing
	// is unaudited" and "could not look" are the same empty list — and an
	// activity report on an unaudited share comes back empty whether or not the
	// member used it.
	UnauditedShares []string `json:"unaudited_shares,omitempty"`
	SharesReadable  bool     `json:"shares_readable"`

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
//
// Every read is also counted by the health monitor (HealthOf), so whoever made
// it — a member's page, an operator's, the periodic probe — moves one count.
func Health(ctx context.Context, target string) TargetHealth {
	if _, err := Get(target); err != nil {
		return TargetHealth{Outcome: OutcomeUnreached, Err: err}
	}
	h := readHealth(ctx, target)
	observeHealth(target, h)
	return h
}

func readHealth(ctx context.Context, target string) TargetHealth {
	a, err := Get(target)
	if err != nil {
		return TargetHealth{Outcome: OutcomeUnreached, Err: err}
	}
	// The breaker is consulted, and a health read does NOT clear it on success
	// any differently from another call — it goes through the same record. An
	// operator watching a target come back reads this; letting it bypass the
	// breaker would make the surface disagree with what every other call gets.
	resp, err := a.send(ctx, sendSpec{Method: http.MethodGet, Path: "/health"})
	if err != nil {
		return TargetHealth{Outcome: OutcomeUnreached, Err: err}
	}

	out := TargetHealth{Outcome: resp.Outcome, Status: resp.Status, Err: resp.Err}
	if resp.Outcome != OutcomeSucceeded {
		return out
	}
	var wire healthWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		out.Outcome, out.Err = OutcomeIndeterminate, fmt.Errorf("addon %s: decode health: %w", target, err)
		return out
	}
	return wire.view(resp.Status)
}

// healthWire is health.json as it travels. Contract 2 names the credential's
// expiry credential_expiry and credential_expires_at; the view keeps the
// names the UI reads (key_expiry, key_expires_at).
type healthWire struct {
	Reachable           bool     `json:"reachable"`
	Product             string   `json:"product"`
	ProductVersion      string   `json:"product_version"`
	VersionTested       bool     `json:"version_tested"`
	VersionNote         string   `json:"version_note"`
	CircuitOpen         bool     `json:"circuit_open"`
	Lifecycle           string   `json:"lifecycle"`
	LifecycleNote       string   `json:"lifecycle_note"`
	InFlight            int64    `json:"in_flight"`
	Drained             bool     `json:"drained"`
	LogHead             string   `json:"log_head"`
	LogRecords          int64    `json:"log_records"`
	SnapshotTakenAt     string   `json:"snapshot_taken_at"`
	LastReadAt          string   `json:"last_read_at"`
	CredentialExpiresAt string   `json:"credential_expires_at"`
	CredentialExpiry    string   `json:"credential_expiry"`
	UnauditedShares     []string `json:"unaudited_shares"`
	SharesReadable      bool     `json:"shares_readable"`
}

func (w healthWire) view(status int) TargetHealth {
	return TargetHealth{
		Reachable: w.Reachable, Product: w.Product, ProductVersion: w.ProductVersion,
		VersionTested: w.VersionTested, VersionNote: w.VersionNote, CircuitOpen: w.CircuitOpen,
		Lifecycle: w.Lifecycle, LifecycleNote: w.LifecycleNote, InFlight: w.InFlight, Drained: w.Drained,
		LogHead: w.LogHead, LogRecords: w.LogRecords, SnapshotTakenAt: w.SnapshotTakenAt,
		LastReadAt: w.LastReadAt, KeyExpiresAt: w.CredentialExpiresAt, KeyExpiry: w.CredentialExpiry,
		UnauditedShares: w.UnauditedShares, SharesReadable: w.SharesReadable,
		Outcome: OutcomeSucceeded, Status: status,
	}
}

// SetLifecycle moves an add-on between active, draining and read-only at
// runtime (design §18).
//
// A mutation, so it leaves what every other mutation leaves: an
// `addon_operations` row and its audit line (who flipped it, to which state),
// committed before the call. The subject is the add-on itself, the id is the
// idempotency key, and `reason` stays in the request — the row has no free-text
// column by design.
func SetLifecycle(ctx context.Context, target, state, reason, actor string) (map[string]any, error) {
	a, err := Get(target)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(map[string]any{
		"contract_version": ContractVersion, "state": state, "reason": reason,
	})
	if err != nil {
		return nil, fmt.Errorf("addon %s: encode lifecycle: %w", target, err)
	}

	// SkipBreaker: neither consulted nor fed. The breaker is a health signal
	// about the add-on's ability to serve work, and this is the call an operator
	// makes to stop it serving work — letting a refusal here count towards
	// opening the circuit would conflate "we told it to stop" with "it stopped
	// answering", and an open circuit must not stop the operator's stop.
	resp, err := a.operate(ctx, db.AddonOperationParams{
		Target: target, Operation: "lifecycle.set", ActorID: actor, SubjectID: target, About: state,
	}, sendSpec{Method: http.MethodPost, Path: "/lifecycle", Body: body, SkipBreaker: true})
	if err != nil {
		return nil, err
	}
	if resp.Outcome != OutcomeSucceeded {
		return nil, fmt.Errorf("addon %s: lifecycle change refused (%d): %v", target, resp.Status, resp.Err)
	}
	var out map[string]any
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("addon %s: decode lifecycle: %w", target, err)
	}
	return out, nil
}

// ErrValueNotResolvable is a mapping value the target does not recognise.
//
// Its own error because its operator action is a correction, not a retry: the
// value is a typo or a group somebody has not created yet, and nothing about
// waiting changes it.
var ErrValueNotResolvable = errors.New("addon: the target does not recognise that value")

// ResolvesValue checks that a mapping value names something on the target.
//
// It FAILS OPEN on everything except a definite "no". A target that could not be
// read, a field the add-on cannot enumerate, an add-on that is not registered —
// all of those are the absence of an answer, and refusing a mapping edit because
// a NAS was rebooting would make an outage look like a validation failure. The
// only refusal is the one case where the add-on answered and the value was not
// in the set it returned.
//
// That asymmetry is the whole design of this check: it is a reference check, not
// an authorisation one. Structure — is this a field the schema declares, is it a
// lifecycle field — is Syndra's own and is enforced whatever the target says.
// Resolution is what the check was able to establish.
//
// `Checked` is the half that was missing and the half a surface most needs. The
// function fails open on everything except a definite no, which is right — but
// "the value is fine" and "nobody could be asked" both arrived as a nil error,
// so a screen had no way to tell an operator which of the two it was showing
// them. A check that did not run must not read as a check that passed.
type Resolution struct {
	// The add-on answered and could enumerate this field.
	Checked bool
	// What it enumerated. Present only when Checked, and used to name near
	// misses on a refusal — a typo is answered by seeing the two names it might
	// have been, not by being told to try again.
	Known []string
}

func ResolvesValue(ctx context.Context, target, field, value string) (Resolution, error) {
	a, err := Get(target)
	if err != nil {
		return Resolution{}, nil
	}
	resp, err := a.send(ctx, sendSpec{Method: http.MethodGet, Path: "/values/" + url.PathEscape(field)})
	if err != nil {
		return Resolution{}, nil
	}
	if resp.Outcome != OutcomeSucceeded {
		// Includes a 404 for a field this add-on does not enumerate. Not an
		// answer about the value.
		return Resolution{}, nil
	}

	var decoded struct {
		Values     []string `json:"values"`
		Enumerable bool     `json:"enumerable"`
	}
	if err := json.Unmarshal(resp.Body, &decoded); err != nil {
		return Resolution{}, nil
	}
	if !decoded.Enumerable {
		// The add-on can serve the field and cannot bound it — a path, a quota.
		// Structure only, and never "no value is valid".
		return Resolution{}, nil
	}

	checked := Resolution{Checked: true, Known: decoded.Values}
	for _, candidate := range decoded.Values {
		if candidate == value {
			return checked, nil
		}
	}
	// The value is echoed, and this is the one place in the add-on layer that
	// does it deliberately: a mapping value is a group name an operator typed,
	// not a secret, and showing them what they typed is most of what makes the
	// refusal actionable (16.8).
	return checked, fmt.Errorf("%w: %s has no %s named %q", ErrValueNotResolvable, target, field, value)
}

// StorageStatus is one member's own account on a target, as the target sees it
// right now: storage.status read into Syndra's own view, from the storage
// profile's result (results) or the legacy pocket. How much room is used
// travels once, as the result's meters (ReadView); this view is what the
// storage card decides on.
type StorageStatus struct {
	// Usable is whether they can connect. NOT the same as the account
	// existing: Syndra creates it before any password exists — it has none to
	// set — and the target disables password authentication until the member
	// sets one, so the account is present, correct, and refuses them.
	Usable bool `json:"usable"`
	// NeedsPassword is that state, named as the action that fixes it.
	NeedsPassword bool `json:"needs_password"`
	// NeedsNewPassword is access given back after it was taken away: the
	// account has had a password, and SMB waits for a new one. Distinct from a
	// first password (NeedsPassword) and from re-enrolment.
	NeedsNewPassword bool `json:"needs_new_password"`
	// Places is everywhere the account can connect, read by the target from
	// its own folder permissions for the groups the account is actually in,
	// home folder first. PlacesReadable false is "not read", never "none".
	Places         []Place `json:"places"`
	PlacesReadable bool    `json:"places_readable"`
	// Standing is the account's standing as the add-on reports it in this
	// reply (contract 2): nil when it did not say.
	Standing *Standing `json:"standing,omitempty"`

	Outcome Outcome `json:"-"`
	Err     error   `json:"-"`
}

// Values is what a target says one entitlement field's values may be, for a
// picker. Outcome is not Succeeded when it couldn't say.
type Values struct {
	Values     []string          `json:"values"`
	Labels     map[string]string `json:"labels,omitempty"`
	Enumerable bool              `json:"enumerable"`
	// BuiltinCount and BuiltinExamples are what the target left out of a group
	// list: "It lists 34 groups, all of them built in, like …".
	BuiltinCount    int      `json:"builtin_count,omitempty"`
	BuiltinExamples []string `json:"builtin_examples,omitempty"`

	Outcome Outcome `json:"-"`
}

// ReadValues asks a target for one field's values.
func ReadValues(ctx context.Context, target, field string) Values {
	a, err := Get(target)
	if err != nil {
		return Values{Outcome: OutcomeUnreached}
	}
	resp, err := a.send(ctx, sendSpec{Method: http.MethodGet, Path: "/values/" + url.PathEscape(field)})
	if err != nil {
		return Values{Outcome: OutcomeUnreached}
	}
	if resp.Outcome != OutcomeSucceeded {
		return Values{Outcome: resp.Outcome}
	}
	var out Values
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return Values{Outcome: OutcomeIndeterminate}
	}
	if out.Values == nil {
		out.Values = []string{}
	}
	out.Outcome = OutcomeSucceeded
	return out
}

// TargetSetting is one system-wide choice a target's add-on keeps, such as
// where home folders go, with what it may be.
type TargetSetting struct {
	Name       string            `json:"name"`
	Label      string            `json:"label"`
	Value      string            `json:"value"`
	SetBy      string            `json:"set_by,omitempty"`
	SetAt      *time.Time        `json:"set_at,omitempty"`
	Values     []string          `json:"values"`
	Labels     map[string]string `json:"labels,omitempty"`
	ValuesRead bool              `json:"values_read"`
}

// ReadSettings asks a target for its settings. ok is false when it couldn't
// say, which is not "it has none".
func ReadSettings(ctx context.Context, target string) (settings []TargetSetting, ok bool) {
	resp, ok := settingsCall(ctx, target, http.MethodGet, "/settings", nil)
	if !ok || resp.Outcome != OutcomeSucceeded {
		return nil, false
	}
	var out struct {
		Settings []TargetSetting `json:"settings"`
	}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, false
	}
	return out.Settings, true
}

// PutSetting asks a target to change one setting. The add-on is the judge of
// what the value may be; its status and body come back for the handler to
// relay. ok is false when no answer came back, and then Outcome says whether
// the request never left (unreached) or may have been applied (indeterminate).
//
// A write, so it leaves the same trace as every other add-on mutation: an
// addon_operations row and its audit line committed before the call, claimed as
// the request goes out, and settled with how it ended. The row's id is the
// call_id in the body: a replay is answered from the first call and writes
// nothing.
func PutSetting(ctx context.Context, target, name, value, actor string) (CallResponse, bool) {
	a, err := Get(target)
	if err != nil {
		// Nothing to record against: the row's target is a foreign key.
		return CallResponse{Outcome: OutcomeUnreached, Err: err}, false
	}
	// The row's id joins the body as call_id in operate (contract 2: in the
	// body, inside the signature; contract 1 sent it as Idempotency-Key).
	body, _ := json.Marshal(map[string]any{"contract_version": ContractVersion, "value": value, "actor": actor})
	resp, _ := a.operate(ctx, db.AddonOperationParams{
		Target: target, Operation: "setting.put", ActorID: actor, SubjectID: name,
		// The value is not a secret: it is a dataset path, shown back on the
		// system page, and it is how a later read settles this record.
		About: value,
	}, sendSpec{Method: http.MethodPut, Path: "/settings/" + url.PathEscape(name), Body: body})
	return resp, resp.Status != 0
}

// settingsCall is one authenticated read of a target's settings or homes
// endpoints. Not being able to ask is `ok == false`, never "it has none".
func settingsCall(ctx context.Context, target, method, path string, body []byte) (CallResponse, bool) {
	a, err := Get(target)
	if err != nil {
		return CallResponse{Outcome: OutcomeUnreached}, false
	}
	resp, err := a.send(ctx, sendSpec{Method: method, Path: path, Body: body})
	if err != nil {
		return CallResponse{Outcome: OutcomeUnreached}, false
	}
	return resp, resp.Status != 0
}

// HomeUsage is one account's home folder on a target, as the target read it.
type HomeUsage struct {
	Username   string `json:"username"`
	AccountID  string `json:"account_id"`
	Home       string `json:"home"`
	UsedBytes  int64  `json:"used_bytes"`
	QuotaBytes int64  `json:"quota_bytes,omitempty"`
}

// Homes is every home folder on a target: a view built from homesWire. Read
// is the backend's own "the add-on answered", never the add-on's to set (task
// 1.0: contract 1 decoded the reply into this type, so an add-on could send
// "read": true).
type Homes struct {
	Read bool `json:"read"`
	// UsageRead is the add-on's quota_read: every quota table was read.
	UsageRead bool        `json:"usage_read"`
	Homes     []HomeUsage `json:"homes"`
}

// homesWire is profiles/storage/homes.json as it travels.
type homesWire struct {
	Homes     []HomeUsage `json:"homes"`
	QuotaRead *bool       `json:"quota_read"`
	TakenAt   *string     `json:"taken_at"`
	Current   *bool       `json:"current"`
}

// ReadHomes asks a target for every home folder and its use.
func ReadHomes(ctx context.Context, target string) Homes {
	resp, ok := settingsCall(ctx, target, http.MethodGet, "/homes", nil)
	if !ok || resp.Outcome != OutcomeSucceeded {
		return Homes{}
	}
	var wire homesWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil || wire.TakenAt == nil || wire.Current == nil {
		// A reply that does not say when it read, or whether that read is
		// current, is not one to show as a reading (design §2(b)).
		return Homes{}
	}
	// A reply that does not say the usage was read whole did not read it.
	return Homes{Read: true, UsageRead: wire.QuotaRead != nil && *wire.QuotaRead, Homes: wire.Homes}
}

// Reach is what each of a target's own groups opens, as the target last read
// its folder permissions. Read false is "not read from the target yet": the
// target hasn't managed a read since its add-on started, or the add-on can't be
// reached, and there is no last read to show.
//
// A view built from reachWire: Read is the backend's own, never the add-on's.
type Reach struct {
	Read    bool      `json:"read"`
	ReadAt  time.Time `json:"read_at,omitempty"`
	Current bool      `json:"current"`
	// Groups is every group of the makerspace's own; an empty list is "opens
	// nothing yet". Each list is what holding the group ADDS to what every
	// account opens.
	Groups   map[string][]Place `json:"groups,omitempty"`
	Everyone []Place            `json:"everyone,omitempty"`
}

// ReadReach asks a target what its groups open.
func ReadReach(ctx context.Context, target string) Reach {
	a, err := Get(target)
	if err != nil {
		return Reach{}
	}
	resp, err := a.send(ctx, sendSpec{Method: http.MethodGet, Path: "/reach"})
	if err != nil {
		return Reach{}
	}
	if resp.Outcome != OutcomeSucceeded {
		return Reach{}
	}
	var wire reachWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil || wire.TakenAt == nil || wire.TakenAt.IsZero() {
		return Reach{}
	}
	out := Reach{Read: true, ReadAt: *wire.TakenAt, Current: wire.Current != nil && *wire.Current,
		Groups: map[string][]Place{}, Everyone: placesOf(wire.Everyone)}
	for g, ps := range wire.Groups {
		out.Groups[g] = placesOf(ps)
	}
	return out
}

// reachWire is profiles/storage/reach.json as it travels (contract 1's read_at
// is taken_at). A reply that does not say it is current is not.
type reachWire struct {
	TakenAt  *time.Time             `json:"taken_at"`
	Current  *bool                  `json:"current"`
	Groups   map[string][]placeWire `json:"groups"`
	Everyone []placeWire            `json:"everyone"`
}

// placeWire is a place as an add-on sends it (reach.json $defs/Place): no
// addresses and no roles, which Syndra fills (Place.Address, the role
// attribution) and an add-on must never be able to set (task 1.0).
type placeWire struct {
	Share   string `json:"share"`
	Folder  string `json:"folder"`
	Path    string `json:"path"`
	Access  string `json:"access"`
	Dataset string `json:"dataset"`
	Home    bool   `json:"home"`
	Usage   *struct {
		UsedBytes  int64 `json:"used_bytes"`
		QuotaBytes int64 `json:"quota_bytes,omitempty"`
	} `json:"usage"`
}

func placesOf(ws []placeWire) []Place {
	out := make([]Place, 0, len(ws))
	for _, w := range ws {
		out = append(out, Place{Share: w.Share, Folder: w.Folder, Path: w.Path, Access: w.Access,
			Dataset: w.Dataset, Home: w.Home, Usage: w.Usage})
	}
	return out
}

// Place is somewhere a member can connect, and what they can do there.
type Place struct {
	Share string `json:"share"`
	// Folder is the path inside the share, empty for the share's root.
	Folder  string `json:"folder,omitempty"`
	Path    string `json:"path"`
	Access  string `json:"access"` // "read" or "change"
	Dataset string `json:"dataset,omitempty"`
	Home    bool   `json:"home,omitempty"`
	// Usage is only on places the member can change, when it could be read.
	// UsedBytes 0 is "nothing saved here yet".
	Usage *struct {
		UsedBytes  int64 `json:"used_bytes"`
		QuotaBytes int64 `json:"quota_bytes,omitempty"`
	} `json:"usage,omitempty"`
	// SMBAddress and UNCAddress are filled by Syndra, never by an add-on:
	// the storage kind's view builds them from the connection
	// (handlers/kinds.Address), the one place an address is built.
	SMBAddress string `json:"smb_address,omitempty"`
	UNCAddress string `json:"unc_address,omitempty"`
	// Roles are the member's own roles whose groups open this place, filled
	// by Syndra: "from your Fabrication lead role". Empty for a home folder.
	Roles []PlaceRole `json:"roles,omitempty"`
}

// PlaceRole is one role, as a project and key.
type PlaceRole struct {
	ProjectID string `json:"project_id"`
	RoleKey   string `json:"role_key"`
}

// Storage is a storage.status reply read into Syndra's view.
func (r ReadReply) Storage() StorageStatus {
	raw, err := r.answer()
	var w storageWire
	switch {
	case err != nil:
	case r.Results:
		// The storage profile (profiles/storage/status.json): the common
		// status, plus the five keys the storage card decides on.
		err = decodeShape("status", raw, &w, storageExtra...)
	default:
		err = json.Unmarshal(raw, &w)
	}
	if err != nil {
		out := StorageStatus{}
		out.Outcome, out.Err = r.failed(err)
		return out
	}
	return StorageStatus{Usable: w.Usable, NeedsPassword: w.NeedsPassword, NeedsNewPassword: w.NeedsNewPassword,
		Places: placesOf(w.Places), PlacesReadable: w.PlacesReadable, Standing: w.Standing, Outcome: OutcomeSucceeded}
}

// storageWire is what Storage reads of either shape, the profile's result or
// the legacy pocket: the keys they share, with places as the add-on sends them,
// never the view's.
type storageWire struct {
	Usable           bool        `json:"usable"`
	NeedsPassword    bool        `json:"needs_password"`
	NeedsNewPassword bool        `json:"needs_new_password"`
	Places           []placeWire `json:"places"`
	PlacesReadable   bool        `json:"places_readable"`
	Standing         *Standing   `json:"standing"`
}

// ActivityReport is what a target's own audit log says about one subject, and
// what that log could not see (common/activity.json).
//
// Deliberately NOT merged into Syndra's audit feed by the surface that renders
// it. Syndra's ledger records what Syndra did; this records what the account
// did on the target, including everything that happened without Syndra's
// involvement. Rendering the two as one feed would make the second look like
// the first, which is the whole reason the target is reconciled rather than
// trusted.
type ActivityReport struct {
	Events []ActivityEvent `json:"events"`
	// Uncovered names the resources that were not recording this member —
	// switched off, or scoped past them by a watch or ignore list. Without it
	// an empty result reads as "no activity", which is a different and far more
	// reassuring statement than "nobody was watching".
	Uncovered []string `json:"uncovered"`
	// Truncated says older events exist than the newest ones returned.
	Truncated bool `json:"truncated"`
	// Observation is when the log was read; nil from a legacy pocket, which
	// never said.
	*Observation

	Outcome Outcome `json:"-"`
	// Status is the add-on's raw HTTP status, carried so a caller can tell a
	// specific rejection (422: no bound account) from every other one without
	// string-matching Err's text.
	Status int   `json:"-"`
	Err    error `json:"-"`
}

// ActivityEvent is one entry from the target's audit log.
type ActivityEvent struct {
	At      string `json:"at"`
	Event   string `json:"event"`
	Success bool   `json:"success"`
	// Resource is where on the target it happened (a share, a door), named as
	// Uncovered names them.
	Resource string `json:"resource,omitempty"`
	// Address and Detail are what make a refusal actionable. Measured against
	// the live NAS: a week of SMB events was 553 rows, every one an
	// AUTHENTICATION failure, and without these two the whole week renders as
	// 553 identical lines saying nothing.
	Address string `json:"address,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

// Activity reads the target's audit log for one subject.
//
// The add-on has implemented `activity.get` since the platform landed and the
// backend's policy table has always declared it, but nothing ever called it —
// so the two TrueNAS roles it needs, `audit.query` and `sharing.smb.query`,
// were configured on the deployment's API key and exercised by nothing. The
// same shape as every other defect this branch found: two halves that agree
// with each other and never meet.
func Activity(ctx context.Context, target, subject, since string) ActivityReport {
	params := map[string]any{}
	if since != "" {
		params["since"] = since
	}
	return Read(ctx, target, "activity.get", subject, subject, params).Activity()
}

// Activity is an activity.get reply read into Syndra's view.
func (r ReadReply) Activity() ActivityReport {
	raw, err := r.answer()
	var out ActivityReport
	switch {
	case err != nil:
	case r.Results:
		err = decodeActivity(raw, &out)
	default:
		// ponytail: the legacy pocket's names, read until major 3.
		var w struct {
			Events []struct {
				ActivityEvent
				Share string `json:"share"`
			} `json:"events"`
			UncoveredShares []string `json:"uncovered_shares"`
			Truncated       bool     `json:"truncated"`
		}
		if err = json.Unmarshal(raw, &w); err == nil {
			out = ActivityReport{Uncovered: w.UncoveredShares, Truncated: w.Truncated}
			for _, e := range w.Events {
				e.Resource = e.Share
				out.Events = append(out.Events, e.ActivityEvent)
			}
		}
	}
	if err != nil {
		out = ActivityReport{Status: r.Status}
		out.Outcome, out.Err = r.failed(err)
		return out
	}
	// Never nil to a caller: a JSON `null` and an empty list render as
	// different things, and "no events" is the answer this read most often
	// gives.
	if out.Events == nil {
		out.Events = []ActivityEvent{}
	}
	if out.Uncovered == nil {
		out.Uncovered = []string{}
	}
	out.Outcome, out.Status = OutcomeSucceeded, r.Status
	return out
}

func decodeActivity(raw json.RawMessage, into *ActivityReport) error {
	return decodeShape("activity", raw, into)
}

// SystemReport is the NAS's own account of itself: what it is, what is wrong
// with it, how full it is, and which of its services are running.
//
// Separate from `TargetHealth`, which is the ADD-ON's account of the target — is it
// answering, is the version tested, has the mutation log been rewritten. That
// one is cheap and polled; this one costs four calls to the target and is read
// when somebody opens the page. Merging them would make every poll four times
// heavier to carry something nobody is watching change.
type SystemReport struct {
	System   *SystemInfo    `json:"system,omitempty"`
	Alerts   []TargetAlert  `json:"alerts,omitempty"`
	Pools    []PoolStatus   `json:"pools,omitempty"`
	Services []ServiceState `json:"services,omitempty"`
	// Degraded names the sources that could not be read. A partial answer that
	// does not say which part is missing is a whole answer that is wrong.
	Degraded []string `json:"degraded,omitempty"`

	Outcome Outcome `json:"-"`
	Err     error   `json:"-"`
}

// SystemInfo is the target identifying itself. The add-on drops the chassis
// serial, the license and the manufacturer before they reach here.
type SystemInfo struct {
	Hostname      string  `json:"hostname,omitempty"`
	Version       string  `json:"version,omitempty"`
	UptimeSeconds float64 `json:"uptime_seconds,omitempty"`
}

// TargetAlert is one alert the NAS raised about itself. Text arrives with the
// target's markup already stripped by the add-on.
type TargetAlert struct {
	Level     string `json:"level"`
	Class     string `json:"klass,omitempty"`
	Text      string `json:"text"`
	At        string `json:"at,omitempty"`
	Dismissed bool   `json:"dismissed"`
}

// PoolStatus is one pool's health and how full it is.
type PoolStatus struct {
	Name           string `json:"name"`
	Status         string `json:"status"`
	Healthy        bool   `json:"healthy"`
	Warning        bool   `json:"warning"`
	FreeBytes      int64  `json:"free_bytes"`
	AllocatedBytes int64  `json:"allocated_bytes"`
	SizeBytes      int64  `json:"size_bytes"`
}

// ServiceState is one service and whether it is running.
type ServiceState struct {
	Service string `json:"service"`
	State   string `json:"state"`
	Enabled bool   `json:"enable"`
}

// SystemHealth reads what the target says about itself.
//
// `health.get` was declared in the manifest, implemented by the add-on,
// dispatched by it and given a policy entry from the day the platform landed,
// and no line of backend code ever called it — so the surface that lists what a
// target can do advertised a capability with nothing behind it. The four reads
// it makes are the ones an operator wants first when a NAS misbehaves: a failing
// disk, a degraded pool, a stopped `cifs`.
func SystemHealth(ctx context.Context, target string) SystemReport {
	return Read(ctx, target, "health.get", "", "", nil).Health()
}

// Health is a health.get reply read into Syndra's view: `result` from a
// results add-on, the `health` pocket otherwise, the same shape either way.
//
// ponytail: SystemReport is TrueNAS-shaped; at major 3 it goes and the raw
// renderer serves health.get (it declares no registry result).
func (r ReadReply) Health() SystemReport {
	raw, err := r.answer()
	var out SystemReport
	if err == nil {
		err = json.Unmarshal(raw, &out)
	}
	if err != nil {
		out = SystemReport{}
		out.Outcome, out.Err = r.failed(err)
		return out
	}
	out.Outcome = OutcomeSucceeded
	return out
}
