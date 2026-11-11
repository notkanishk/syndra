package addons

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"time"
)

// The one read path (feature results, design §4): every `effect: read`
// operation is sent by Read and by nothing else. Never dispatched, never
// written to the operations log, never confirmed: it runs on an ordinary page
// load, and a row per page view would fill the log with events that changed
// nothing (decision 14). Who may name the subject is the route's decision, made
// before Read is called: the member's own page passes the member as subject and
// actor, the operator-only person page (GET /users/{id}/targets) the person it
// shows as subject and the operator as actor.
//
// An add-on declaring `results` answers one `result`, read into the types
// below; one without is read from its typed pocket (OperationPolicy.Pocket), and only
// for the three reads that had one. No JSON Schema is compiled at runtime
// (design §1): these hand-written types are held to the schema files by the
// fixture tests, and a reply missing a key its schema requires is not a
// reading (absence is never a value).

// ErrNotARead is Read asked for an operation whose effect is not read: a write
// goes through addonop.Dispatch, with its record, and never through here.
var ErrNotARead = errors.New("addon: not an effect: read operation")

// ReadReply is one answer to an effect: read operation, as it travelled.
type ReadReply struct {
	Operation string
	// Schema is the registry's result schema for the id, "" when it declares
	// its own; Pocket the key it was answered under before results.
	Schema string
	Pocket string
	// Results is the add-on declaring feature results: its answer is
	// `result`, and no pocket is read. Without it only the pocket is.
	Results bool
	Body    []byte
	Outcome Outcome
	// Status is the add-on's HTTP status, so a caller can tell a specific
	// refusal (422: no bound account) from any other.
	Status int
	Err    error
}

// Read sends one effect: read operation about subject, on behalf of actor.
func Read(ctx context.Context, target, op, subject, actor string, params map[string]any) ReadReply {
	out := ReadReply{Operation: op, Outcome: OutcomeUnreached}
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
	m, err := a.Manifest()
	if err != nil {
		out.Err = err
		return out
	}
	out.Schema, out.Pocket, out.Results = eff.Result, eff.Pocket, m.Declares("results")
	body, err := json.Marshal(map[string]any{
		"contract_version": ContractVersion,
		// Ignored by the add-on (a read is never deduplicated), and still sent:
		// the request schema requires one.
		"call_id":   "read-" + target + "-" + subject,
		"operation": op,
		// Empty for a read about the target itself (health.get): it travels
		// empty rather than omitted, the same shape every operation sends.
		"subject": subject,
		"actor":   actor,
		"params":  params,
	})
	if err != nil {
		out.Err = err
		return out
	}
	resp, err := a.send(ctx, sendSpec{Method: http.MethodPost, Path: "/operations/" + url.PathEscape(op), Body: body})
	if err != nil {
		out.Err = err
		return out
	}
	out.Body, out.Outcome, out.Status, out.Err = resp.Body, resp.Outcome, resp.Status, resp.Err
	return out
}

// answer is the reply's payload: `result` from an add-on declaring results,
// the legacy pocket from one that does not. An add-on without results that
// sends `result` anyway is not read: the feature is the promise of its shape.
func (r ReadReply) answer() (json.RawMessage, error) {
	if r.Outcome != OutcomeSucceeded {
		return nil, fmt.Errorf("read %s: %s: %w", r.Operation, r.Outcome, r.Err)
	}
	key := "result"
	if !r.Results {
		key = r.Pocket
	}
	var env map[string]json.RawMessage
	if err := json.Unmarshal(r.Body, &env); err != nil {
		return nil, fmt.Errorf("read %s: %w", r.Operation, err)
	}
	if raw := env[key]; key != "" && len(raw) > 0 && string(raw) != "null" {
		return raw, nil
	}
	return nil, fmt.Errorf("read %s: the reply carries no %q", r.Operation, key)
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

// shapes is what each registry result schema is, for a generic renderer, with
// the keys this backend knows (the rest is what the add-on also says). A
// profile of status is a status: every valid reply is a valid common one.
var shapes = map[string]struct {
	shape string
	keys  keySpec
	// extra is what a profile requires beyond its shape.
	extra []string
}{
	"common/records.json": {shape: "records", keys: keySpec{
		"records":  {"id": nil, "label": nil, "category": nil, "since": nil, "due": nil, "standing": nil},
		"taken_at": nil, "current": nil, "stale": nil}},
	"common/activity.json": {shape: "activity", keys: activityKeys},
	"common/status.json":   {shape: "status", keys: statusKeys},
	storageProfile: {shape: "status", extra: storageExtra, keys: func() keySpec {
		k := keySpec{}
		for _, key := range storageExtra {
			k[key] = nil
		}
		for key, v := range statusKeys {
			k[key] = v
		}
		return k
	}()},
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

// required names, per shape, the keys its schema requires at the top and on
// each item: decoded only when present, because absence is never a value.
var required = map[string]struct {
	top  []string
	list string
	item []string
}{
	"records":  {top: []string{"records", "taken_at", "current"}, list: "records", item: []string{"id", "label"}},
	"activity": {top: []string{"events", "uncovered", "truncated", "taken_at", "current"}, list: "events", item: []string{"at", "event", "success"}},
	"status":   {top: []string{"usable", "standing", "meters", "taken_at", "current"}, list: "meters", item: []string{"label", "used", "unit"}},
}

// decodeShape unmarshals raw into into once every key the shape requires is
// there, on the object and on each item of its list.
func decodeShape(shape string, raw json.RawMessage, into any, extra ...string) error {
	req := required[shape]
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return err
	}
	if err := need(top, slices.Concat(req.top, extra)); err != nil {
		return err
	}
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(top[req.list], &items); err != nil {
		return fmt.Errorf("%s: %w", req.list, err)
	}
	for i, item := range items {
		if err := need(item, req.item); err != nil {
			return fmt.Errorf("%s[%d]: %w", req.list, i, err)
		}
	}
	return json.Unmarshal(raw, into)
}

func need(m map[string]json.RawMessage, keys []string) error {
	for _, k := range keys {
		if v, ok := m[k]; !ok || string(v) == "null" {
			return fmt.Errorf("no %q", k)
		}
	}
	return nil
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
}

// rawResultCap bounds an id's own result shown raw: the least trusted
// component chooses its size.
const rawResultCap = 4 << 10

// View is the reply for a generic renderer. Only a results add-on's reply has
// one; a legacy pocket is read by its own typed reader alone.
func (r ReadReply) View() ReadView {
	v := ReadView{Operation: r.Operation, Shape: shapes[r.Schema].shape}
	raw, err := r.answer()
	if err != nil || !r.Results {
		return v
	}
	var decoded any
	switch v.Shape {
	case "records":
		var x Records
		err, decoded = decodeShape(v.Shape, raw, &x), &x
	case "activity":
		var x ActivityReport
		err, decoded = decodeActivity(raw, &x), &x
	case "status":
		var x Status
		err, decoded = decodeShape(v.Shape, raw, &x, shapes[r.Schema].extra...), &x
	default:
		shown := string(raw)
		if len(shown) > rawResultCap {
			shown = shown[:rawResultCap] + "…"
		}
		decoded = shown
	}
	if err != nil {
		return v
	}
	var doc any
	_ = json.Unmarshal(raw, &doc)
	if v.Shape != "" {
		v.AlsoSays = unknownKeys("", doc, shapes[r.Schema].keys)
	}
	v.Readable, v.Result = true, decoded
	return v
}
