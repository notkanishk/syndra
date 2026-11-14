package addons

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"time"
)

// The one read path (design §4): every `effect: read`
// operation is sent by Read and by nothing else. Never dispatched, never
// written to the operations log, never confirmed: it runs on an ordinary page
// load, and a row per page view would fill the log with events that changed
// nothing (decision 14). Who may name the subject is the route's decision, made
// before Read is called: the member's own page passes the member as subject and
// actor, the operator-only person page (GET /users/{id}/targets) the person it
// shows as subject and the operator as actor.
//
// Every add-on answers one `result`, read into the types below, and nothing
// else: a reply without it is not a reading. No JSON Schema is compiled at runtime
// (design §1): these hand-written types are held to the schema files by the
// fixture tests, and a reply missing a key its schema requires is not a
// reading (absence is never a value).

// ErrNotARead is Read asked for an operation whose effect is not read: a write
// goes through addonop.Dispatch, with its record, and never through here.
var ErrNotARead = errors.New("addon: not an effect: read operation")

// ErrReadNotCalled is Call asked for an effect: read operation: a read is
// sent by Read alone, with no call record (design §4).
var ErrReadNotCalled = errors.New("addon: a read is never called with a record; it goes through Read")

// ReadReply is one answer to an effect: read operation, as it travelled.
type ReadReply struct {
	Operation string
	// Target is who answered, for what is said about its reply (takenAt).
	Target string
	// Schema is the registry's result schema for the id, "" when it declares
	// its own.
	Schema  string
	Body    []byte
	Outcome Outcome
	// Status is the add-on's HTTP status, so a caller can tell a specific
	// refusal (422: no bound account) from any other.
	Status int
	Err    error
	// RetryAfter is the add-on refusing a fresh read inside its cooldown with
	// nothing to serve (cooldownOf): the seconds until it may read the app.
	RetryAfter int
	// Code and Refusal are a refusal's code and what it names (a preview
	// refused for an unknown item, a limit reached).
	Code    string
	Refusal Refusal
	// NoChange is a preview answered no_change: the confirm would change
	// nothing (the subject already holds it).
	NoChange bool
	// Clock dates the reply (ahead): the add-on's Date and when it was asked.
	Clock Clock
}

// Read sends one effect: read operation about subject, on behalf of actor.
// fresh asks for a read of the app now (feature freshness): a person pressing
// "Read again", never a page load or a poll.
func Read(ctx context.Context, target, op, subject, actor string, params map[string]any, fresh bool) ReadReply {
	out := ReadReply{Operation: op, Target: target, Outcome: OutcomeUnreached}
	eff, err := ResolveOperation(target, op)
	if err != nil {
		out.Err = err
		return out
	}
	if eff.Effect != EffectRead {
		out.Err = fmt.Errorf("%w: %s/%s", ErrNotARead, target, op)
		return out
	}
	if params == nil {
		params = map[string]any{}
	}
	if err := ValidateParams(eff, params); err != nil {
		out.Err = err
		return out
	}
	a, err := Get(target)
	if err != nil {
		out.Err = err
		return out
	}
	out.Schema = eff.Result
	body := operationBody(op, subject, actor, params)
	// Ignored by the add-on (a read is never deduplicated), and still sent:
	// the request schema requires one.
	body["call_id"] = "read-" + target + "-" + subject
	raw, err := json.Marshal(body)
	if err != nil {
		out.Err = err
		return out
	}
	return out.sent(a.send(ctx, sendSpec{Method: http.MethodPost, Path: "/operations/" + url.PathEscape(op), Body: raw, Fresh: fresh}))
}

// operationBody is an operation request's body as every unrecorded send
// carries it: a read, and a physical operation's preview.
func operationBody(op, subject, actor string, params map[string]any) map[string]any {
	return map[string]any{
		"contract_version": ContractVersion,
		"operation":        op,
		// Empty for a read about the target itself (health.get): it travels
		// empty rather than omitted, the same shape every operation sends.
		"subject": subject,
		"actor":   actor,
		"params":  params,
	}
}

// sent fills r from what send answered.
func (r ReadReply) sent(resp CallResponse, err error) ReadReply {
	if err != nil {
		r.Err = err
		return r
	}
	r.Body, r.Outcome, r.Status, r.Err, r.RetryAfter = resp.Body, resp.Outcome, resp.Status, resp.Err, cooldownOf(resp)
	r.Code, r.Refusal, r.NoChange, r.Clock = resp.Code, resp.Refusal, resp.NoChange, resp.Clock
	return r
}

// ErrNotPhysical is Preview asked for an operation whose effect is not
// physical: only a physical operation is confirmed after a preview.
var ErrNotPhysical = errors.New("addon: only a physical operation is previewed")

// Preview sends a physical operation with `preview: true` (design §4 physical
// rule 1): the add-on answers the record the confirm would make (or no_change)
// and does nothing. Never recorded, never claimed, and never with a call_id,
// so nothing the add-on keeps can be replayed as the confirm's answer; the
// confirm is a separate send, recorded, with its own fresh id (Dispatch).
// Only to an add-on declaring calls, the feature the request field is under.
// The caller binds subject and actor (addonop.Preview).
func Preview(ctx context.Context, target, op, subject, actor string, params map[string]any) ReadReply {
	out := ReadReply{Operation: op, Target: target, Outcome: OutcomeUnreached}
	eff, err := ResolveOperation(target, op)
	if err != nil {
		out.Err = err
		return out
	}
	if eff.Effect != EffectPhysical {
		out.Err = fmt.Errorf("%w: %s/%s", ErrNotPhysical, target, op)
		return out
	}
	if err := ValidateParams(eff, params); err != nil {
		out.Err = err
		return out
	}
	a, err := Get(target)
	if err != nil {
		out.Err = err
		return out
	}
	// Resolved available means calls is declared (resolveOperations); asked
	// again here because the field must never reach an add-on without it.
	if err := a.declares("calls"); err != nil {
		out.Err = err
		return out
	}
	out.Schema = eff.Result
	body := operationBody(op, subject, actor, params)
	body["preview"] = true
	raw, err := json.Marshal(body)
	if err != nil {
		out.Err = err
		return out
	}
	out = out.sent(a.send(ctx, sendSpec{Method: http.MethodPost, Path: "/operations/" + url.PathEscape(op), Body: raw}))
	if out.Outcome == OutcomeSucceeded {
		resp := readOutcome(CallResponse{Outcome: out.Outcome, Body: out.Body}, true)
		out.Outcome, out.Err, out.NoChange = resp.Outcome, resp.Err, resp.NoChange
	}
	return out
}

// ViewOf is a physical operation's reply (the confirm's, or the one GET /calls
// recorded) for the generic renderer, through the same decoder as a read's:
// one shape, one reader. Empty for a secret reply, which is never shown.
func ViewOf(target, op string, body []byte, c Clock) ReadView {
	p := operationPolicy[op]
	if secretResult(op) {
		return ReadView{Operation: op, Shape: shapes[p.Result].shape}
	}
	return ReadReply{Operation: op, Target: target, Schema: p.Result, Body: body, Outcome: OutcomeSucceeded, Clock: c}.View()
}

// answer is the reply's payload: `result`, the one key a read is answered
// under. A reply without it (an old per-read key such as `storage` in its
// place) carries no reading, and is refused, never guessed at.
func (r ReadReply) answer() (json.RawMessage, error) {
	if r.Outcome != OutcomeSucceeded {
		return nil, fmt.Errorf("read %s: %s: %w", r.Operation, r.Outcome, r.Err)
	}
	// A map, not a struct: encoding/json matches a struct's keys without
	// regard to case, and the key is `result`, exactly.
	var env map[string]json.RawMessage
	if err := json.Unmarshal(r.Body, &env); err != nil {
		return nil, fmt.Errorf("read %s: %w", r.Operation, err)
	}
	if raw := env["result"]; len(raw) > 0 && string(raw) != "null" {
		return raw, nil
	}
	return nil, fmt.Errorf("read %s: the reply carries no \"result\"", r.Operation)
}

// failed is a reply that did not decode: sent, answered, unreadable.
func (r ReadReply) failed(err error) (Outcome, error) {
	if r.Outcome != OutcomeSucceeded {
		return r.Outcome, r.Err
	}
	return OutcomeIndeterminate, err
}

// ---- the shapes ------------------------------------------------------------

// Observation is when a read reply was taken from the app (design §4): every
// result carries it.
type Observation struct {
	TakenAt time.Time `json:"taken_at"`
	Current bool      `json:"current"`
	// Stale says why it is not current; present exactly when Current is false.
	Stale *Stale `json:"stale,omitempty"`
}

// Stale is common/stale.json.
type Stale struct {
	Reason     string `json:"reason"`
	RetryAfter *int   `json:"retry_after,omitempty"`
	Note       string `json:"note,omitempty"`
}

// Records is common/records.json: what the subject holds or did.
type Records struct {
	Records []Record `json:"records"`
	Observation
}

// Record is one thing the subject holds or did.
type Record struct {
	ID       string     `json:"id"`
	Label    string     `json:"label"`
	Category string     `json:"category,omitempty"`
	Since    *time.Time `json:"since,omitempty"`
	Due      *time.Time `json:"due,omitempty"`
	Standing *Standing  `json:"standing,omitempty"`
}

// Status is common/status.json: whether the subject can use the target now,
// and how much room is left.
type Status struct {
	Usable   bool      `json:"usable"`
	Standing *Standing `json:"standing"`
	Meters   []Meter   `json:"meters"`
	Observation
}

// Meter is one amount used, against its ceiling when there is one.
type Meter struct {
	Label string   `json:"label"`
	Used  float64  `json:"used"`
	Limit *float64 `json:"limit,omitempty"`
	Unit  string   `json:"unit"`
}

// StorageResult is the storage profile's result for a generic renderer: the
// status, and whether its meters are complete. QuotaRead false is "couldn't
// read" (some or all of the usage), never "nothing used": the meters that were
// read stay beside it, each a true observation, and the flag says the list is
// not the whole of it.
type StorageResult struct {
	Status
	QuotaRead bool `json:"quota_read"`
}

// resultShape is one registry result schema, for a generic renderer: its
// shape, what it requires (absence is never a value), the keys this backend
// knows (the rest is what the add-on also says), and the type it decodes into.
type resultShape struct {
	shape  string
	need   needSpec
	keys   keySpec
	result func() any
}

// shapes is every registry result schema this backend reads. A profile of
// status is a status: every valid reply is a valid common one.
var shapes = map[string]resultShape{
	"common/records.json": {"records", recordsNeed, keySpec{
		"records":  {"id": nil, "label": nil, "category": nil, "since": nil, "due": nil, "standing": nil},
		"taken_at": nil, "current": nil, "stale": nil}, func() any { return new(Records) }},
	"common/activity.json": {"activity", activityNeed, activityKeys, func() any { return new(ActivityReport) }},
	"common/status.json":   {"status", statusNeed, statusKeys, func() any { return new(Status) }},
	storageProfile: {"status", storageNeed, func() keySpec {
		k := maps.Clone(statusKeys)
		for _, key := range storageExtra {
			k[key] = nil
		}
		return k
	}(), func() any { return new(StorageResult) }},
}

// storageProfile is storage.status's result: the common status plus the keys
// the storage card decides on (storageExtra), decoded by Storage.
const storageProfile = "profiles/storage/status.json"

var storageExtra = []string{"places", "places_readable", "needs_password", "needs_new_password", "quota_read"}

var (
	statusKeys = keySpec{
		"usable": nil, "standing": nil, "meters": {"label": nil, "used": nil, "limit": nil, "unit": nil},
		"taken_at": nil, "current": nil, "stale": nil}
	activityKeys = keySpec{
		"events":    {"at": nil, "event": nil, "success": nil, "address": nil, "resource": nil, "detail": nil},
		"uncovered": nil, "truncated": nil, "taken_at": nil, "current": nil, "stale": nil}
)

// needSpec is what a schema requires: keys on the object, and, for a key whose
// value is an object or a list of objects, what each requires wherever it is
// present (a record's standing is optional; a standing that is there has its
// own required keys).
type needSpec struct {
	keys   []string
	nested map[string]needSpec
}

var (
	standingNeed = needSpec{keys: []string{"usable", "waiting_on"}}
	metersNeed   = needSpec{keys: []string{"label", "used", "unit"}}
	staleNeed    = needSpec{keys: []string{"reason"}}
	recordsNeed  = needSpec{keys: []string{"records", "taken_at", "current"}, nested: map[string]needSpec{
		"stale":   staleNeed,
		"records": {keys: []string{"id", "label"}, nested: map[string]needSpec{"standing": standingNeed}}}}
	activityNeed = needSpec{keys: []string{"events", "uncovered", "truncated", "taken_at", "current"}, nested: map[string]needSpec{
		"stale":  staleNeed,
		"events": {keys: []string{"at", "event", "success"}}}}
	statusNeed = needSpec{keys: []string{"usable", "standing", "meters", "taken_at", "current"}, nested: map[string]needSpec{
		"standing": standingNeed, "meters": metersNeed, "stale": staleNeed}}
	// storageNeed is the status's, the storage card's five keys, and each
	// place's own (profiles/storage/reach.json, Place).
	storageNeed = needSpec{keys: slices.Concat(statusNeed.keys, storageExtra), nested: map[string]needSpec{
		"standing": standingNeed, "meters": metersNeed, "stale": staleNeed,
		"places": {keys: []string{"share", "path", "access"}, nested: map[string]needSpec{"usage": {keys: []string{"used_bytes"}}}}}}
)

// check holds raw to spec: every key it requires, then each nested object, or
// each item of a nested list, to its own. A null is never a value: a required
// key that is null is missing, and a nested one that is null is not an object.
func (spec needSpec) check(raw json.RawMessage) error {
	if string(raw) == "null" {
		return errors.New("null")
	}
	var list []json.RawMessage
	if json.Unmarshal(raw, &list) == nil {
		for i, item := range list {
			if err := spec.check(item); err != nil {
				return fmt.Errorf("[%d]: %w", i, err)
			}
		}
		return nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	for _, k := range spec.keys {
		if v, ok := m[k]; !ok || string(v) == "null" {
			return fmt.Errorf("no %q", k)
		}
	}
	for k, sub := range spec.nested {
		if v, ok := m[k]; ok {
			if err := sub.check(v); err != nil {
				return fmt.Errorf("%s: %w", k, err)
			}
		}
	}
	return nil
}

// decodeShape unmarshals raw into into once it holds to spec and to the stale
// invariant, and holds its taken_at to this clock (ahead): a reply dated the
// zero time is not a reading, and one dated ahead is read as now.
func (r ReadReply) decodeShape(spec needSpec, raw json.RawMessage, into any) error {
	if err := spec.check(raw); err != nil {
		return err
	}
	if err := staleInvariant(raw); err != nil {
		return err
	}
	if err := json.Unmarshal(raw, into); err != nil {
		return err
	}
	if o, ok := into.(observed); ok {
		if seen := o.observation(); seen != nil {
			if seen.TakenAt.IsZero() {
				return errors.New("taken_at is the zero time")
			}
			seen.TakenAt = ahead(r.Target, seen.TakenAt, r.Clock)
		}
	}
	return nil
}

// observed is a result that carries an Observation: every shape does, by
// embedding it (a nil embedded *Observation answers nil).
type observed interface{ observation() *Observation }

func (o *Observation) observation() *Observation { return o }

// StaleCooldown is stale.reason for a fresh read asked inside the add-on's
// cooldown and answered with the last read (common/stale.json).
const StaleCooldown = "cooldown"

// currentWithinCooldown reports a cooldown answer to a fresh read whose copy
// the add-on read live within its cooldown window (taken_at, on this clock,
// no older than FreshCooldown): the same data a second live read would
// return, since every write through the add-on forgets its copies, so it is
// current for the caller that asked fresh (7l). The add-on's words stay
// honest on the wire; this is the backend's reading of them, the way an HTTP
// cache serves a response still fresh by its age (RFC 9111 §4.2). A copy
// older than the window, or one answering a read that did not ask fresh, is
// the cooldown it says.
func currentWithinCooldown(target string, fresh bool, stale *Stale, takenAt time.Time) bool {
	return fresh && stale != nil && stale.Reason == StaleCooldown && !takenAt.IsZero() &&
		timeNow().Sub(takenAt) <= FreshCooldown(target)
}

// staleInvariant holds a reply to design §4's `current == false` ⇔ `stale`
// present (common/stale.json#/$defs/Invariant). A reply breaking it says two
// things about one read, and is not a reading. One with no `current` says
// nothing here; whether it must is the shape's own need.
func staleInvariant(raw []byte) error {
	var o struct {
		Current *bool           `json:"current"`
		Stale   json.RawMessage `json:"stale"`
	}
	if json.Unmarshal(raw, &o) != nil || o.Current == nil {
		return nil
	}
	if hasStale := len(o.Stale) > 0 && string(o.Stale) != "null"; *o.Current == hasStale {
		return fmt.Errorf("current is %t and stale is %s: current is false exactly when stale is present", *o.Current,
			map[bool]string{true: "present", false: "absent"}[hasStale])
	}
	return nil
}

// cooldownOf is a reply refusing a fresh read inside the add-on's cooldown
// with nothing to serve (FRESH_READ_COOLDOWN, 429): the seconds until another
// may read the app, at least 1, so a missing Retry-After still reads as a
// cooldown. 0 for any other reply.
func cooldownOf(resp CallResponse) int {
	if resp.Code != codeFreshReadCooldown {
		return 0
	}
	return max(1, resp.RetryAfter)
}

// ReadView is one read for a generic renderer: the shape, the decoded result,
// and what the add-on also says (x- keys and anything else this backend does
// not know), quoted and capped, never used for a decision.
type ReadView struct {
	Operation string `json:"operation"`
	// Shape is records, activity or status; "" is an id's own result, shown
	// raw (Result is then the JSON text, capped).
	Shape    string   `json:"shape"`
	Readable bool     `json:"readable"`
	Result   any      `json:"result,omitempty"`
	AlsoSays []string `json:"also_says,omitempty"`
	// RetryAfter is a fresh read refused inside the add-on's cooldown with
	// nothing to serve: unreadable, and readable again in this many seconds.
	RetryAfter int `json:"retry_after,omitempty"`
}

// rawResultCap bounds an id's own result shown raw: the least trusted
// component chooses its size.
const rawResultCap = 4 << 10

// View is the reply for a generic renderer.
func (r ReadReply) View() ReadView {
	sh := shapes[r.Schema]
	v := ReadView{Operation: r.Operation, Shape: sh.shape, RetryAfter: r.RetryAfter}
	raw, err := r.answer()
	if err != nil {
		return v
	}
	if sh.shape == "" {
		if staleInvariant(raw) != nil {
			return v
		}
		shown := string(raw)
		if len(shown) > rawResultCap {
			shown = shown[:rawResultCap] + "…"
		}
		v.Readable, v.Result = true, shown
		return v
	}
	decoded := sh.result()
	if err := r.decodeShape(sh.need, raw, decoded); err != nil {
		return v
	}
	var doc any
	_ = json.Unmarshal(raw, &doc)
	v.Readable, v.Result, v.AlsoSays = true, decoded, unknownKeys("", doc, sh.keys)
	return v
}
