// Package reference is a complete contract-2 add-on over an in-memory target:
// the conformance kit's self-test and the author's guide's worked example
// (openspec/changes/addon-contract, design §7 and §8). It implements the core
// and the features operations and settings, and, when Config.Features names
// it, results: holdings.list, and health.get answering under result.
//
// The target is a print queue: member accounts, the printers each may use, an
// after-hours flag, a longest-job limit. Every behaviour below is derived from
// a field's role, never from its name, which is the point of contract 2: swap
// the schema and the target, keep the rules.
package reference

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ContractVersion is the wire major this add-on speaks (design §2).
const ContractVersion = 2

const subjectsPerPlan = 50

// field is one entry of the manifest's entitlement_schema (design §3).
type field struct {
	Name      string `json:"name"`
	Role      string `json:"role"`
	Label     string `json:"label,omitempty"`
	Unit      string `json:"unit,omitempty"`
	Combine   string `json:"combine,omitempty"`
	Unlimited string `json:"unlimited,omitempty"`
	PerPerson bool   `json:"per_person,omitempty"`
}

var fields = []field{
	{Name: "enabled", Role: "account"},
	{Name: "remote", Role: "channel", Label: "Sends jobs from home"},
	{Name: "printers", Role: "membership", Label: "Printers"},
	{Name: "after_hours", Role: "flag", Label: "Prints after hours"},
	{Name: "job_seconds", Role: "limit", Label: "Longest print job", Unit: "seconds", Combine: "max", Unlimited: "0", PerPerson: true},
}

// printers is the managed set of the membership field. lobby is the target's
// own: every account is in it, /values does not list it, Syndra never removes
// it (unlisted values are preserved, converge), and state never shows it
// (shown).
var printers = []string{"mk4-a", "mk4-b", "xl-1"}

const lobby = "lobby"

func fieldNamed(name string) (field, bool) {
	i := slices.IndexFunc(fields, func(f field) bool { return f.Name == name })
	if i < 0 {
		return field{}, false
	}
	return fields[i], true
}

type account struct {
	id, username string
	state        map[string]any // field name → bool (account, channel) or []string
	password     bool
}

type standing struct {
	Usable    bool   `json:"usable"`
	WaitingOn string `json:"waiting_on"`
	Note      string `json:"note,omitempty"`
	Next      string `json:"next,omitempty"`
}

// shown is the account's state as Syndra sees it: a membership holds only the
// managed set. Syndra compares state with desired over the whole list, so a
// value it can never remove (lobby) would be a difference for ever.
func (acct *account) shown() map[string]any {
	out := maps.Clone(acct.state)
	out["printers"] = slices.DeleteFunc(slices.Clone(acct.state["printers"].([]string)),
		func(p string) bool { return !slices.Contains(printers, p) })
	return out
}

// standing is observed, never desired: why a converged account cannot be used
// yet, and who is waited on (design §6).
func (acct *account) standing() *standing {
	switch {
	case acct.state["enabled"] != true:
		return &standing{WaitingOn: "none", Note: "The account is off."}
	case !acct.password:
		return &standing{WaitingOn: "member", Note: "No password set yet.", Next: "Set a password on your page."}
	}
	return &standing{Usable: true, WaitingOn: "none"}
}

// Config is everything the add-on needs. Defects is empty in any real run.
// Features are the optional features it serves beyond operations and settings
// (OptionalFeatures), so the kit's feature checks can be watched both ways.
type Config struct {
	Target   string
	Secret   []byte
	Features []string
	Defects  map[Defect]bool
}

// OptionalFeatures is every feature Config.Features may name.
var OptionalFeatures = []string{"results"}

func (a *Addon) serves(feature string) bool { return slices.Contains(a.cfg.Features, feature) }

// Addon is the add-on: an in-memory target, the call store, the mutation log
// and the lifecycle, behind one lock. Writes take the lock only around the
// target, so a call's in-flight lease is real (design §5 rule 4).
type Addon struct {
	cfg  Config
	cert tls.Certificate
	auth authenticator

	mu        sync.Mutex
	accounts  map[string]*account // by account_id
	bindings  map[string]string   // Syndra subject → account_id
	calls     map[string]*call    // by call_id
	lifecycle string
	reason    string
	setting   map[string]string // the one setting: value, set_by, set_at
	logHead   string
	logCount  int
	dev       deviations // what the defects keep; empty in any real run
}

// New derives the transport keys from the secret and starts with an empty
// target, lifecycle active.
func New(cfg Config) (*Addon, error) {
	if strings.TrimSpace(cfg.Target) == "" || len(cfg.Secret) == 0 {
		return nil, errors.New("a target name and a per-target secret are required")
	}
	for _, f := range cfg.Features {
		if !slices.Contains(OptionalFeatures, f) {
			return nil, fmt.Errorf("unknown feature %q; the reference serves %v", f, OptionalFeatures)
		}
	}
	priv, key, err := deriveKeys(cfg.Secret, cfg.Target)
	if err != nil {
		return nil, err
	}
	cert, err := selfSigned(priv, cfg.Target)
	if err != nil {
		return nil, err
	}
	return &Addon{
		cfg: cfg, cert: cert,
		auth:     authenticator{key: key, now: time.Now},
		accounts: map[string]*account{}, bindings: map[string]string{}, calls: map[string]*call{},
		lifecycle: "active", reason: "Started.",
		logHead: emptyLogDigest,
		setting: map[string]string{"value": printers[0]},
	}, nil
}

// Certificate is the TLS identity to serve: the key derived from the secret.
func (a *Addon) Certificate() tls.Certificate { return a.cert }

// Handler serves every route, each one behind the signature check. (deviate
// and withDefects are defects.go's: with no defect set they change nothing.)
func (a *Addon) Handler() http.Handler {
	mux := http.NewServeMux()
	route := func(pattern string, h func(*http.Request, []byte) reply) {
		mux.HandleFunc(pattern, a.auth.authenticated(func(w http.ResponseWriter, r *http.Request, body []byte) {
			a.deviate(pattern, h, r, body).write(w)
		}))
	}
	route("GET /capabilities", a.capabilities)
	route("GET /health", a.health)
	route("GET /subjects", a.subjects)
	route("GET /values/{field}", a.values)
	route("POST /plan", a.plan)
	route("POST /apply", a.apply)
	route("POST /operations/{name}", a.operation)
	route("POST /lifecycle", a.setLifecycle)
	route("GET /settings", a.settings)
	route("PUT /settings/{name}", a.putSetting)
	route("/", func(*http.Request, []byte) reply {
		return problem(404, "UNKNOWN_ROUTE", "This add-on has no such route.", nil)
	})
	return a.withDefects(mux)
}

// ---- reads ----------------------------------------------------------------

func (a *Addon) capabilities(*http.Request, []byte) reply {
	ops := []map[string]any{
		{"id": "health.get", "scope": "admin", "confirm": false, "available": true},
		{"id": "password.set", "scope": "member", "confirm": false, "secret_params": []string{"password"}, "available": true}}
	if a.serves("results") {
		// holdings.list has no pocket to answer under, so it is declared only
		// with results.
		ops = append(ops, map[string]any{"id": "holdings.list", "scope": "member", "confirm": false, "available": true})
	}
	return ok(map[string]any{
		"contract_version": ContractVersion, "product": "reference", "product_version": "1.0.0",
		"kind": "machine", "features": append([]string{"operations", "settings"}, a.cfg.Features...),
		"entitlement_schema": fields,
		"operations":         ops,
		"limits":             map[string]any{"subjects_per_plan": subjectsPerPlan},
	})
}

func (a *Addon) health(*http.Request, []byte) reply {
	a.mu.Lock()
	defer a.mu.Unlock()
	busy := a.busy()
	return ok(map[string]any{
		"reachable": true, "product": "reference", "product_version": "1.0.0", "version_tested": true,
		"credential_expiry": "none", "circuit_open": false,
		"lifecycle": a.lifecycle, "lifecycle_note": a.reason, "in_flight": busy, "drained": busy == 0,
		"log_head": a.logHead, "log_records": a.logCount, "checked_at": now(),
	})
}

type subject struct {
	ID        string         `json:"id"`
	Username  string         `json:"username"`
	AccountID string         `json:"account_id"`
	State     map[string]any `json:"state"`
	Standing  *standing      `json:"standing"`
}

// subjects reads the target now, so the answer is current (design §4: a read
// that began after the request arrived) and complete, so never truncated.
func (a *Addon) subjects(*http.Request, []byte) reply {
	a.mu.Lock()
	defer a.mu.Unlock()
	list := []subject{}
	for _, id := range slices.Sorted(maps.Keys(a.accounts)) {
		acct := a.accounts[id]
		list = append(list, subject{acct.id, acct.username, acct.id, acct.shown(), acct.standing()})
	}
	return ok(map[string]any{"subjects": list, "current": true, "taken_at": now(), "truncated": false})
}

func (a *Addon) values(r *http.Request, _ []byte) reply {
	f, found := fieldNamed(r.PathValue("field"))
	if !found {
		return problem(404, "UNKNOWN_FIELD", "No such entitlement field.", nil)
	}
	v := map[string]any{"field": f.Name, "enumerable": true}
	switch f.Role {
	case "account", "channel":
		v["values"] = []string{"false", "true"}
	case "membership":
		v["values"], v["builtin_count"], v["builtin_examples"] = printers, 1, []string{lobby}
	case "flag":
		v["values"] = []string{"true"}
	case "limit":
		v["values"], v["enumerable"] = []string{}, false
	}
	return ok(v)
}

func (a *Addon) settings(*http.Request, []byte) reply {
	a.mu.Lock()
	defer a.mu.Unlock()
	return ok(map[string]any{"settings": []any{a.settingDoc()}})
}

func (a *Addon) settingDoc() map[string]any {
	s := map[string]any{"name": "home_printer", "label": "Printer new members start on",
		"value": a.setting["value"], "values": printers, "values_read": true}
	if a.setting["set_by"] != "" {
		s["set_by"], s["set_at"] = a.setting["set_by"], a.setting["set_at"]
	}
	return s
}

// ---- plan and apply -------------------------------------------------------

// outcome is what an apply did, or a plan says it would do, to one subject.
type outcome struct {
	Subject     string         `json:"subject"`
	Effect      string         `json:"effect"`
	Detail      string         `json:"detail"`
	Consequence string         `json:"consequence,omitempty"`
	Username    string         `json:"username,omitempty"`
	AccountID   string         `json:"account_id,omitempty"`
	Standing    *standing      `json:"standing,omitempty"`
	Fingerprint string         `json:"fingerprint"`
	Observed    map[string]any `json:"observed,omitempty"`
	Conflict    map[string]any `json:"conflict,omitempty"`
}

type planRequest struct {
	versioned
	Subjects []struct {
		Subject string                     `json:"subject"`
		Email   string                     `json:"email"`
		Desired map[string]json.RawMessage `json:"desired"`
	} `json:"subjects"`
}

// plan mutates nothing. It is computed against the target as it is now.
func (a *Addon) plan(_ *http.Request, body []byte) reply {
	var req planRequest
	if refused := parse(body, &req); refused != nil {
		return *refused
	}
	if n := len(req.Subjects); n > subjectsPerPlan {
		return problem(422, "PLAN_TOO_LARGE", "Too many subjects for one plan.", map[string]any{"subjects": n, "limit": subjectsPerPlan})
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	outcomes := []outcome{}
	for _, s := range req.Subjects {
		if strings.TrimSpace(s.Subject) == "" {
			return problem(400, "BAD_REQUEST", "Every planned subject needs a subject.", nil)
		}
		out, acct, _ := a.decide(s.Subject, s.Email, s.Desired)
		describe(&out, acct)
		outcomes = append(outcomes, out)
	}
	return ok(map[string]any{"outcomes": outcomes, "current": true, "taken_at": now(), "truncated": false})
}

type applyRequest struct {
	versioned
	CallID      string                     `json:"call_id"`
	Subject     string                     `json:"subject"`
	Email       string                     `json:"email"`
	Fingerprint string                     `json:"fingerprint"`
	PlanID      string                     `json:"plan_id"`
	Actor       string                     `json:"actor"`
	Desired     map[string]json.RawMessage `json:"desired"`
}

// apply converges one subject. The order is the contract (design §5): a replay
// is answered from the record BEFORE the fingerprint is checked, because the
// backend re-plans before re-sending and the fingerprint will have moved.
func (a *Addon) apply(_ *http.Request, body []byte) reply {
	var req applyRequest
	if refused := parse(body, &req); refused != nil {
		return *refused
	}
	if blank(req.CallID) || blank(req.Subject) || req.Desired == nil {
		return problem(400, "BAD_REQUEST", "call_id, subject and desired are required.", nil)
	}
	identity := map[string]any{"subject": req.Subject, "desired": plain(req.Desired)}
	return a.once(req.CallID, "apply", identity, func() reply {
		if refused := a.refuseUnlessActive(); refused != nil {
			return *refused
		}
		if blank(req.Fingerprint) {
			return problem(400, "FINGERPRINT_REQUIRED", "An apply must carry the plan's fingerprint.", nil)
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		out, acct, next := a.decide(req.Subject, req.Email, req.Desired)
		if out.Fingerprint != req.Fingerprint {
			return problem(409, "PLAN_STALE", "The subject changed since the plan.", nil)
		}
		return a.commit(out, acct, next)
	})
}

// decide works out what desired means for one subject: the plan's answer, and
// the apply's before it writes. next is nil when nothing is to be written.
// Called with a.mu held.
func (a *Addon) decide(subj, email string, raw map[string]json.RawMessage) (outcome, *account, map[string]any) {
	acct := a.accounts[a.bindings[subj]]
	out := outcome{Subject: subj, Fingerprint: fingerprint(acct)}
	desired, err := parseDesired(raw)
	if err != nil {
		out.Effect, out.Detail, out.Consequence = "blocked", "The desired state is not valid here.", err.Error()
		return out, acct, nil
	}
	if acct == nil {
		// Withdraw-only, or nothing said about the account: never create.
		if desired["enabled"] != true {
			out.Effect, out.Detail = "no_change", "No account, and none is wanted."
			return out, nil, nil
		}
		name := username(subj, email)
		for _, other := range a.accounts {
			if other.username == name {
				out.Effect, out.Detail = "blocked", "The name this subject would get is taken."
				out.Consequence = "An operator adopts " + name + " or renames it."
				out.Conflict = map[string]any{"username": name, "account_id": other.id, "adoptable": true}
				return out, nil, nil
			}
		}
		acct = &account{username: name, state: map[string]any{
			"enabled": false, "remote": false, "printers": []string{lobby}, "after_hours": []string{}, "job_seconds": []string{"0"}}}
		out.Effect, out.Detail, out.Consequence = "apply", "Creates the account.", "Creates "+name+"."
		out.Username = name
		return out, acct, converge(acct.state, desired)
	}
	next := converge(acct.state, desired)
	changed := []string{}
	for _, f := range fields {
		if !equal(next[f.Name], acct.state[f.Name]) {
			changed = append(changed, f.Name)
		}
	}
	if len(changed) == 0 {
		out.Effect, out.Detail = "no_change", "Already as desired."
		return out, acct, nil
	}
	out.Effect, out.Detail, out.Consequence = "apply", "Changes the account.", "Changes "+strings.Join(changed, ", ")+"."
	return out, acct, next
}

// converge applies desired to a state by role (design §3). A field absent from
// desired is not touched; a channel is off whenever the account is off; a
// membership keeps the values Syndra does not manage; a limit is combined, and
// a limit given no value (no role sets it) is not touched either: clearing it
// would mean "no limit", which nobody decided.
func converge(cur, desired map[string]any) map[string]any {
	next := maps.Clone(cur)
	for _, f := range fields {
		v, present := desired[f.Name]
		switch {
		case !present:
		case f.Role == "limit" && len(v.([]string)) == 0:
		case f.Role == "membership":
			kept := slices.DeleteFunc(slices.Clone(cur[f.Name].([]string)), func(s string) bool { return slices.Contains(printers, s) })
			next[f.Name] = sorted(append(kept, v.([]string)...))
		case f.Role == "limit":
			next[f.Name] = combine(f, v.([]string))
		default:
			next[f.Name] = v
		}
	}
	if next["enabled"] == false {
		for _, f := range fields {
			if f.Role == "channel" {
				next[f.Name] = false
			}
		}
	}
	return next
}

// combine is max, with unlimited dominant: max("0", 5 GiB) is no limit at all.
// It happens here, once; observed state carries the effective limit, always
// one value. vals is never empty (converge).
func combine(f field, vals []string) []string {
	best := uint64(0)
	for _, v := range vals {
		if v == f.Unlimited {
			return []string{f.Unlimited}
		}
		n, _ := strconv.ParseUint(v, 10, 64) // parseDesired checked it
		best = max(best, n)
	}
	return []string{strconv.FormatUint(best, 10)}
}

// commit writes a decision and answers it. Called with a.mu held.
func (a *Addon) commit(out outcome, acct *account, next map[string]any) reply {
	switch {
	case out.Conflict != nil:
		return problem(409, "BINDING_CONFLICT", out.Detail, extensions(out))
	case out.Effect == "blocked":
		return problem(422, "APPLY_BLOCKED", out.Detail, extensions(out))
	}
	if next != nil {
		if acct.id == "" {
			acct.id = strconv.Itoa(1001 + len(a.accounts))
			a.accounts[acct.id], a.bindings[out.Subject] = acct, acct.id
		}
		acct.state = next
		a.record("apply " + acct.id)
		out.Effect, out.Detail = "applied", "Done."
	}
	describe(&out, acct)
	if acct != nil {
		out.Observed, out.Fingerprint = acct.shown(), fingerprint(acct)
	}
	return ok(out)
}

// describe adds what only an existing account has: standing and its id travel
// together, and only when there is an account (design §6).
func describe(out *outcome, acct *account) {
	if acct != nil && acct.id != "" {
		out.Username, out.AccountID, out.Standing = acct.username, acct.id, acct.standing()
	}
}

func extensions(out outcome) map[string]any {
	raw, _ := json.Marshal(out)
	var ext map[string]any
	_ = json.Unmarshal(raw, &ext)
	return ext
}

// parseDesired checks each value against its field's role.
func parseDesired(raw map[string]json.RawMessage) (map[string]any, error) {
	out := map[string]any{}
	for name, v := range raw {
		f, found := fieldNamed(name)
		if !found {
			return nil, fmt.Errorf("%s is not a field of this add-on", name)
		}
		if f.Role == "account" || f.Role == "channel" {
			var b bool
			if json.Unmarshal(v, &b) != nil {
				return nil, fmt.Errorf("%s is true or false", name)
			}
			out[name] = b
			continue
		}
		var list []string
		if json.Unmarshal(v, &list) != nil || list == nil {
			return nil, fmt.Errorf("%s is a list of strings", name)
		}
		for _, s := range list {
			n, err := strconv.ParseUint(s, 10, 64)
			switch {
			case f.Role == "membership" && !slices.Contains(printers, s):
				return nil, fmt.Errorf("%s: there is no printer %q", name, s)
			case f.Role == "flag" && s != "true":
				return nil, fmt.Errorf(`%s is ["true"] or []`, name)
			case f.Role == "limit" && (err != nil || strconv.FormatUint(n, 10) != s):
				return nil, fmt.Errorf("%s: %q is not a whole number of %s", name, s, f.Unit)
			}
		}
		out[name] = list
	}
	return out, nil
}

// fingerprint digests the state a plan was computed against; "no account" has
// a value of its own, distinct from "an account with nothing".
func fingerprint(acct *account) string {
	in := []byte("absent")
	if acct != nil {
		in, _ = json.Marshal([]any{acct.id, acct.state})
	}
	sum := sha256.Sum256(in)
	return hex.EncodeToString(sum[:])
}

// username is derived once, at create; the binding is authoritative after.
func username(subj, email string) string {
	local, _, _ := strings.Cut(strings.ToLower(email), "@")
	name := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
			return r
		}
		return -1
	}, local)
	if name == "" {
		return "member-" + subj
	}
	return name
}

// ---- operations, lifecycle, settings ----------------------------------------

type operationRequest struct {
	versioned
	CallID      string         `json:"call_id"`
	Operation   string         `json:"operation"`
	Subject     string         `json:"subject"`
	Actor       string         `json:"actor"`
	PlanID      string         `json:"plan_id"`
	Fingerprint string         `json:"fingerprint"`
	Params      map[string]any `json:"params"`
}

func (a *Addon) operation(r *http.Request, body []byte) reply {
	name := r.PathValue("name")
	if name != "password.set" && name != "health.get" && (name != "holdings.list" || !a.serves("results")) {
		return problem(404, "OPERATION_FAILED", "This add-on has no such operation.", nil)
	}
	var req operationRequest
	if refused := parse(body, &req); refused != nil {
		return *refused
	}
	if req.Operation != name {
		return problem(400, "OPERATION_MISMATCH", "The body names another operation.", nil)
	}
	switch name {
	case "health.get":
		return a.healthGet(req)
	case "holdings.list":
		return a.holdingsList(req)
	}
	switch password, _ := req.Params["password"].(string); {
	case blank(req.CallID) || blank(req.Subject):
		return problem(400, "BAD_REQUEST", "call_id and subject are required.", nil)
	case password == "" || len(req.Params) != 1:
		return problem(400, "OPERATION_FAILED", "password.set takes one parameter, password.", nil)
	}
	// Secret parameters are outside the identity, so the store never holds even
	// a hash of a password, and a re-send with another password is a replay.
	params := map[string]any{}
	for name, v := range req.Params {
		if name != "password" {
			params[name] = v
		}
	}
	identity := map[string]any{"name": req.Operation, "subject": req.Subject, "params": params}
	return a.once(req.CallID, "operation", identity, func() reply {
		if refused := a.refuseUnlessActive(); refused != nil {
			return *refused
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		acct := a.accounts[a.bindings[req.Subject]]
		if acct == nil {
			return problem(409, "OPERATION_FAILED", "This subject has no account here.", nil)
		}
		// ponytail: an in-memory target keeps only the fact; a real one hands
		// the password to the target and forgets it.
		acct.password = true
		a.record("password.set " + acct.id)
		return ok(map[string]any{"operation": req.Operation, "subject": req.Subject, "outcome": "succeeded", "detail": "Password set."})
	})
}

// healthGet is an effect: read operation. Its call_id is ignored: never
// recorded, held or checked for reuse, because Syndra sends one call_id for
// every read. Not lifecycle-gated either: reads answer in every state.
// With results it answers under `result`, without it under the `health`
// pocket a contract-2 add-on wrote before results.
func (a *Addon) healthGet(req operationRequest) reply {
	if len(req.Params) != 0 {
		return problem(400, "OPERATION_FAILED", "health.get takes no parameters.", nil)
	}
	key := "health"
	if a.serves("results") {
		key = "result"
	}
	return ok(map[string]any{"operation": req.Operation, "subject": req.Subject, "outcome": "succeeded",
		"detail": "The print queue is up.", key: map[string]any{"system": map[string]any{"hostname": "reference", "version": "1.0.0"}}})
}

// record is common/records.json's Record.
type record struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Category string `json:"category,omitempty"`
}

// holdingsList is the subject's holdings (feature results): a read like
// healthGet, so call_id is ignored and every lifecycle answers. Here what an
// account holds is the printers it may use, read now from the target, so the
// answer is current.
func (a *Addon) holdingsList(req operationRequest) reply {
	if len(req.Params) != 0 {
		return problem(400, "OPERATION_FAILED", "holdings.list takes no parameters.", nil)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	acct := a.accounts[a.bindings[req.Subject]]
	if acct == nil {
		return problem(422, "OPERATION_FAILED", "This subject has no account here.", nil)
	}
	records := []record{}
	for _, p := range acct.shown()["printers"].([]string) {
		records = append(records, record{ID: p, Label: "Printer " + p, Category: "printer"})
	}
	return ok(map[string]any{"operation": req.Operation, "subject": req.Subject, "outcome": "succeeded",
		"result": map[string]any{"records": records, "taken_at": now(), "current": true}})
}

type lifecycleRequest struct {
	versioned
	CallID string `json:"call_id"`
	State  string `json:"state"`
	Reason string `json:"reason"`
}

// setLifecycle is the operator's stop switch: read_only and draining refuse
// every write with LIFECYCLE_REFUSAL while reads go on answering.
func (a *Addon) setLifecycle(_ *http.Request, body []byte) reply {
	var req lifecycleRequest
	if refused := parse(body, &req); refused != nil {
		return *refused
	}
	switch {
	case blank(req.CallID):
		return problem(400, "BAD_REQUEST", "call_id is required.", nil)
	case blank(req.Reason):
		return problem(400, "REASON_REQUIRED", "A lifecycle change needs a reason.", nil)
	case !slices.Contains([]string{"active", "draining", "read_only"}, req.State):
		return problem(422, "UNKNOWN_LIFECYCLE_STATE", "The state is active, draining or read_only.", nil)
	}
	return a.once(req.CallID, "lifecycle", map[string]any{"state": req.State, "reason": req.Reason}, func() reply {
		a.mu.Lock()
		defer a.mu.Unlock()
		a.lifecycle, a.reason = req.State, req.Reason
		busy := a.busy()
		return ok(map[string]any{"state": a.lifecycle, "reason": a.reason, "in_flight": busy, "drained": busy == 0})
	})
}

type settingRequest struct {
	versioned
	CallID string `json:"call_id"`
	Value  string `json:"value"`
	Actor  string `json:"actor"`
}

func (a *Addon) putSetting(r *http.Request, body []byte) reply {
	var req settingRequest
	if refused := parse(body, &req); refused != nil {
		return *refused
	}
	switch name := r.PathValue("name"); {
	case name != "home_printer":
		return problem(404, "NO_SUCH_SETTING", "No such setting.", nil)
	case blank(req.CallID):
		return problem(400, "BAD_REQUEST", "call_id is required.", nil)
	case req.Value != "" && !slices.Contains(printers, req.Value):
		return problem(400, "INVALID_BODY", "That printer is not offered.", nil)
	}
	return a.once(req.CallID, "setting", map[string]any{"name": "home_printer", "value": req.Value}, func() reply {
		if refused := a.refuseUnlessActive(); refused != nil {
			return *refused
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		a.setting = map[string]string{"value": req.Value, "set_by": req.Actor, "set_at": now()}
		a.record("setting home_printer")
		return ok(a.settingDoc())
	})
}

func (a *Addon) refuseUnlessActive() *reply {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.lifecycle == "active" {
		return nil
	}
	r := problem(503, "LIFECYCLE_REFUSAL", "The add-on is "+a.lifecycle+": "+a.reason, map[string]any{"lifecycle": a.lifecycle})
	return &r
}

// emptyLogDigest is SHA-256 of no bytes: the head of a log with no record,
// reported from the first /health. Syndra refuses an empty head at first sight.
const emptyLogDigest = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// record appends to the mutation log: a count and a hash chain.
func (a *Addon) record(entry string) {
	sum := sha256.Sum256([]byte(a.logHead + "\n" + entry))
	a.logHead, a.logCount = hex.EncodeToString(sum[:]), a.logCount+1
}

// ---- idempotency (design §5) ------------------------------------------------

type call struct {
	kind, digest string
	busy         bool
	reply        reply
}

// once runs a write at most once per call_id. Same id and identity: the
// recorded reply, byte for byte. Same id, other identity: 422 CALL_ID_REUSED.
// Same id still running: 409 CALL_IN_PROGRESS. Only a 2xx is recorded, so a
// refusal is re-evaluated when it is re-sent.
//
// ponytail: calls are held for the life of the process. A real add-on keeps
// them on disk for at least constants.call_memory (P30D).
func (a *Addon) once(callID, kind string, identity any, run func() reply) reply {
	digest := identityDigest(identity)
	a.mu.Lock()
	c, seen := a.calls[callID]
	switch {
	case seen && (c.kind != kind || c.digest != digest):
		a.mu.Unlock()
		return problem(422, "CALL_ID_REUSED", "This call_id was used for a different request.", nil)
	case seen && c.busy:
		a.mu.Unlock()
		return problem(409, "CALL_IN_PROGRESS", "A request with this call_id is running.", nil)
	case seen:
		a.mu.Unlock()
		return c.reply
	}
	a.calls[callID] = &call{kind: kind, digest: digest, busy: true} // the in-flight lease
	a.mu.Unlock()

	rep := run()
	a.mu.Lock()
	defer a.mu.Unlock()
	if rep.status/100 == 2 {
		a.calls[callID] = &call{kind: kind, digest: digest, reply: rep}
	} else {
		delete(a.calls, callID)
	}
	return rep
}

func (a *Addon) busy() int {
	n := 0
	for _, c := range a.calls {
		if c.busy && c.kind != "lifecycle" {
			n++
		}
	}
	return n
}

// identityDigest is SHA-256 over the RFC 8785 (JCS) canonical JSON of the
// identity object (design §5); identity_test.go holds it to the contract's
// four vectors.
func identityDigest(v any) string {
	sum := sha256.Sum256(canonical(v))
	return hex.EncodeToString(sum[:])
}

// canonical writes v, built of plain decoded values, as RFC 8785 does for
// identity objects: keys sorted, no insignificant space, strings raw UTF-8.
// encoding/json with HTML escaping off is exactly that here, but for corners
// no identity reaches in practice, each still deterministic and injective:
// U+2028 and U+2029 are escaped, keys outside the BMP sort by code point
// rather than UTF-16 unit, and a non-integer number is not ES6-formatted
// (identity objects carry strings, booleans and lists, no numbers).
func canonical(v any) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
}

// plain decodes each raw value, so two spellings of the same JSON ("a" and
// "\u0061", [ "x" ] and ["x"]) are one identity.
func plain(raw map[string]json.RawMessage) map[string]any {
	out := make(map[string]any, len(raw))
	for k, v := range raw {
		var x any
		_ = json.Unmarshal(v, &x)
		out[k] = x
	}
	return out
}

// ---- the wire ---------------------------------------------------------------

type versioned struct {
	ContractVersion *int `json:"contract_version"`
}

func (v versioned) version() *int { return v.ContractVersion }

// parse decodes a request strictly (a field this add-on does not know is
// refused, never ignored, design §2) and checks the contract major.
func parse(body []byte, into interface{ version() *int }) *reply {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	err := dec.Decode(into)
	if err == nil && dec.More() {
		err = errors.New("more than one JSON document")
	}
	var r reply
	switch v := into.version(); {
	case err != nil && strings.HasPrefix(err.Error(), "json: unknown field "):
		r = problem(400, "UNKNOWN_REQUEST_FIELD", "The request carries a field this add-on does not know: "+strings.TrimPrefix(err.Error(), "json: unknown field "), nil)
	case err != nil:
		r = problem(400, "BAD_REQUEST", "The body is not the JSON this route takes.", nil)
	case v == nil || *v != ContractVersion:
		caller := 0
		if v != nil {
			caller = *v
		}
		r = problem(422, "CONTRACT_VERSION_MISMATCH", "This add-on speaks contract 2.",
			map[string]any{"caller_version": caller, "addon_version": ContractVersion})
	default:
		return nil
	}
	return &r
}

type reply struct {
	status     int
	body       []byte
	retryAfter bool // send Retry-After: the code's retry class is retry_after
}

func ok(v any) reply {
	body, _ := json.Marshal(v)
	return reply{status: 200, body: body}
}

// problem is the one error body (RFC 9457, design decision 25). The title is
// the registry's, which is the code in words.
func problem(status int, code, detail string, ext map[string]any) reply {
	p := maps.Clone(ext)
	if p == nil {
		p = map[string]any{}
	}
	retry := "never"
	switch code {
	case "LIFECYCLE_REFUSAL", "CALL_IN_PROGRESS":
		retry = "retry_after"
	case "PLAN_STALE":
		retry = "replan"
	}
	title := strings.ReplaceAll(strings.ToLower(code), "_", " ")
	maps.Copy(p, map[string]any{"type": "urn:syndra:addon:error:" + code, "title": strings.ToUpper(title[:1]) + title[1:],
		"status": status, "detail": detail, "code": code, "retry": retry})
	r := ok(p)
	r.status, r.retryAfter = status, retry == "retry_after"
	return r
}

func (r reply) write(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	if r.status >= 400 {
		w.Header().Set("Content-Type", "application/problem+json")
	}
	if r.retryAfter {
		w.Header().Set("Retry-After", "30")
	}
	w.WriteHeader(r.status)
	_, _ = w.Write(r.body)
}

func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func blank(s string) bool { return strings.TrimSpace(s) == "" }

func sorted(s []string) []string { slices.Sort(s); return slices.Compact(s) }

func equal(x, y any) bool {
	a, _ := json.Marshal(x)
	b, _ := json.Marshal(y)
	return bytes.Equal(a, b)
}
