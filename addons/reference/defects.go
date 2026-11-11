package reference

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Defect switches on one deliberate violation of the contract, so that the
// conformance kit can be watched failing the check that exists to catch it
// (tasks 4.4). Every defect is off unless it is named in Config.Defects;
// nothing sets one in a normal run, and the binary warns loudly when one is.
//
// This file owns every deviation. The handlers in reference.go are written as
// a correct add-on writes them; a defect acts on the request before it reaches
// them or on the target and the reply after, and with none set, deviate and
// withDefects change nothing.
type Defect string

const (
	IgnoresFingerprint        Defect = "ignores-fingerprint"          // applies over a wrong fingerprint
	CreatesOnWithdraw         Defect = "creates-on-withdraw"          // a withdraw-only apply creates an account
	EchoesSecret              Defect = "echoes-secret"                // password.set repeats the password
	ReplaysAfterFingerprint   Defect = "replays-after-fingerprint"    // checks the fingerprint before the replay
	RemembersRefusal          Defect = "remembers-refusal"            // records every non-2xx reply
	RemembersPlanStale        Defect = "remembers-plan-stale"         // records a 409 PLAN_STALE only
	OmitsTruncated            Defect = "omits-truncated"              // /subjects carries no truncated
	CurrentFromMirror         Defect = "current-from-mirror"          // says current: true from a mirror
	AcceptsAlteredQuery       Defect = "accepts-altered-query"        // also accepts a MAC over the path alone
	LimitIgnoresUnlimited     Defect = "limit-ignores-unlimited"      // max("0", 5 GiB) is 5 GiB
	WithdrawTouchesMembership Defect = "withdraw-touches-membership"  // turning the account off empties memberships
	WithdrawStripsManaged     Defect = "withdraw-strips-managed"      // turning it off drops the values Syndra gave
	NumericAccountID          Defect = "numeric-account-id"           // account_id as a JSON number
	FailsAfterCreate          Defect = "fails-after-create"           // creates the account, then answers 500
	LogNeverMoves             Defect = "log-never-moves"              // health.log_records stays 0
	CommitsLate               Defect = "commits-late"                 // a creating apply commits 300 ms after it arrives
	RebindsSubject            Defect = "rebinds-subject"              // an apply to an existing account rebinds its subject to another
	AdoptsExisting            Defect = "adopts-existing"              // a create answers with an account that was already there
	BindsOnWithdraw           Defect = "binds-on-withdraw"            // a withdraw for an unbound subject binds and disables an existing account
	IgnoresReadOnly           Defect = "ignores-read-only"            // accepts read_only and stays active
	OperationNamesOther       Defect = "operation-names-other"        // password.set answers with another account's id
	RebindsOnOperation        Defect = "rebinds-on-operation"         // a password.set rebinds its subject to another account
	PlanForgetsAccount        Defect = "plan-forgets-account"         // a plan for an existing account names none
	CreateAnswersMember       Defect = "create-answers-member"        // a create answers with a new member's account, not one of its own
	RebindsQuietly            Defect = "rebinds-quietly"              // an apply to an existing account rebinds its subject, and plans then name no account
	CreateOmitsAccount        Defect = "create-omits-account"         // the create makes the account and answers 200 naming none
	LimitEmptyClears          Defect = "limit-empty-clears"           // a limit given [] is cleared
	LogHeadEmpty              Defect = "log-head-empty"               // health.log_head is "" until the first record
	ReadsTakeCallID           Defect = "reads-take-call-id"           // health.get is deduplicated on call_id like a write
	StateShowsUnlisted        Defect = "state-shows-unlisted"         // /subjects lists lobby, which /values leaves out
	ResultOffSchema           Defect = "result-off-schema"            // holdings.list answers a record with no label (with results)
	ResultInPocket            Defect = "result-in-pocket"             // holdings.list answers under a pocket of its own, no result (with results)
	CachedAsCurrent           Defect = "cached-as-current"            // a mirror answer says current: true, no stale (with freshness)
	ReadsPerRequest           Defect = "reads-per-request"            // no collapsing; the cooldown runs from a read's end (with freshness)
	CooldownIgnored           Defect = "cooldown-ignored"             // every fresh read reads the app (with freshness)
	PreviewRecorded           Defect = "preview-recorded"             // a checkout preview writes a log record (with results, calls)
	ReexecutesInDoubt         Defect = "reexecutes-in-doubt"          // a checkout runs while an earlier one is in doubt (with results, calls)
	CheckoutUnserialised      Defect = "checkout-unserialised"        // no per-subject lock, a pause between the checks and the loan (with results, calls)
	NoChangeAsSucceeded       Defect = "no-change-as-succeeded"       // an item already held answers succeeded and logs (with results, calls)
	CallsEchoParams           Defect = "calls-echo-params"            // a call keeps its params, and /calls answers them (with calls)
	StatsUndeclaredDimension  Defect = "stats-undeclared-dimension"   // every point carries a dimension the manifest does not declare (with stats)
	StatsLocalNameSubject     Defect = "stats-local-name-subject"     // a per-person point names the account id, not the subject (with stats)
	StatsZeroForGap           Defect = "stats-zero-for-gap"           // a zero point inside each window it could not observe (with stats)
	StatsWatermarkAhead       Defect = "stats-watermark-ahead"        // the watermark is tomorrow, past a day still running (with stats)
	StatsSubjectsAboveTotal   Defect = "stats-subjects-above-total"   // a total below its people: the sum less one, the largest less one (with stats)
	StatsSinceIgnored         Defect = "stats-since-ignored"          // answers from the start of the buffer whatever since says (with stats)
	StatsFinalMoves           Defect = "stats-final-moves"            // a final point's value changes with every read (with stats)
	StatsUnknownMetricOK      Defect = "stats-unknown-metric-ok"      // an undeclared metric is 200 with nothing in it (with stats)
	StatsPageOverlap          Defect = "stats-page-overlap"           // next_cursor names the second-last point, so a page repeats one (with stats)
	StatsNoBufferGap          Defect = "stats-no-buffer-gap"          // the window the buffer no longer holds has no gap (with stats)
	StatsTotalMissing         Defect = "stats-total-missing"          // a day with per-person points has no total (with stats)
	StatsDimensionNamesPerson Defect = "stats-dimension-names-person" // the mk4-a printer is named for the first account's username (with stats)
)

// Defects lists every defect, for the kit's self-tests and the binary's help.
var Defects = []Defect{IgnoresFingerprint, CreatesOnWithdraw, EchoesSecret, ReplaysAfterFingerprint,
	RemembersRefusal, RemembersPlanStale, OmitsTruncated, CurrentFromMirror, AcceptsAlteredQuery,
	LimitIgnoresUnlimited, WithdrawTouchesMembership, WithdrawStripsManaged, NumericAccountID,
	FailsAfterCreate, LogNeverMoves, CommitsLate, RebindsSubject, AdoptsExisting, BindsOnWithdraw, IgnoresReadOnly, OperationNamesOther,
	RebindsOnOperation, PlanForgetsAccount, CreateAnswersMember, RebindsQuietly, CreateOmitsAccount,
	LimitEmptyClears, LogHeadEmpty, ReadsTakeCallID, StateShowsUnlisted, ResultOffSchema, ResultInPocket,
	CachedAsCurrent, ReadsPerRequest, CooldownIgnored, PreviewRecorded, ReexecutesInDoubt, CheckoutUnserialised,
	NoChangeAsSucceeded, CallsEchoParams, StatsUndeclaredDimension, StatsLocalNameSubject, StatsZeroForGap,
	StatsWatermarkAhead, StatsSubjectsAboveTotal, StatsSinceIgnored, StatsFinalMoves, StatsUnknownMetricOK,
	StatsPageOverlap, StatsNoBufferGap, StatsTotalMissing, StatsDimensionNamesPerson}

// ParseDefects turns names into a set, refusing any name it does not know.
func ParseDefects(names []string) (map[Defect]bool, error) {
	set := map[Defect]bool{}
	for _, n := range names {
		d := Defect(strings.TrimSpace(n))
		if !slices.Contains(Defects, d) {
			return nil, fmt.Errorf("unknown defect %q", n)
		}
		set[d] = true
	}
	return set, nil
}

// deviations is what the defects keep between requests.
type deviations struct {
	mirror   []byte           // CurrentFromMirror: the stale /subjects
	mirrorAt int              // the log count it was taken at
	refused  map[string]reply // RemembersRefusal, RemembersPlanStale: by call_id
	late     int              // CommitsLate: applies waiting to commit, reported as in flight
	rebound  bool             // RebindsOnOperation: done once
	quiet    bool             // RebindsQuietly: rebound, and plans no longer name the account
	params   map[string]any   // CallsEchoParams: each call's params, by call_id
	reads    int              // StatsFinalMoves: /stats answered so far
}

// openIntent is the subject's checkout in doubt, nil when none. Under a.mu.
// ReexecutesInDoubt never sees one, so a retry runs over an unsettled call.
func (a *Addon) openIntent(subject string) *intent {
	if a.cfg.Defects[ReexecutesInDoubt] {
		return nil
	}
	return a.intents[subject]
}

// serialise holds the subject's checkouts to one at a time; it returns the
// unlock. CheckoutUnserialised holds nothing.
func (a *Addon) serialise(subject string) (unlock func()) {
	if a.cfg.Defects[CheckoutUnserialised] {
		return func() {}
	}
	v, _ := a.locks.LoadOrStore(subject, new(sync.Mutex))
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// gap is nothing, unless CheckoutUnserialised: then a.mu (held) is let go
// between the checks and the loan, so two checkouts can both pass the limit.
func (a *Addon) gap() {
	if a.cfg.Defects[CheckoutUnserialised] {
		a.mu.Unlock()
		time.Sleep(2 * time.Millisecond)
		a.mu.Lock()
	}
}

// withDefects wraps the whole handler for the one defect that lives below the
// routes: a request whose MAC covers its path alone verifies, its query dropped.
func (a *Addon) withDefects(next http.Handler) http.Handler {
	if !a.cfg.Defects[AcceptsAlteredQuery] {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(io.LimitReader(r.Body, maxRequestBytes+1))
		bare := r.Clone(r.Context())
		bare.URL.RawQuery, bare.Body = "", io.NopCloser(bytes.NewReader(body))
		if _, err := a.auth.verify(bare); err == nil && r.URL.RawQuery != "" {
			bare.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, bare)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		next.ServeHTTP(w, r)
	})
}

// deviate serves one request through the defects that are on.
func (a *Addon) deviate(route string, h func(*http.Request, []byte) reply, r *http.Request, body []byte) reply {
	d := a.cfg.Defects
	if len(d) == 0 {
		return h(r, body)
	}
	var req struct {
		CallID      string                     `json:"call_id"`
		Subject     string                     `json:"subject"`
		Fingerprint string                     `json:"fingerprint"`
		Desired     map[string]json.RawMessage `json:"desired"`
		Params      map[string]any             `json:"params"`
		Preview     bool                       `json:"preview"`
	}
	_ = json.Unmarshal(body, &req)
	apply := route == "POST /apply"
	passwordSet := route == "POST /operations/{name}" && r.PathValue("name") == "password.set"
	holdings := route == "POST /operations/{name}" && r.PathValue("name") == "holdings.list"
	checkout := route == "POST /operations/{name}" && r.PathValue("name") == "item.checkout"
	if route == "POST /operations/{name}" && r.PathValue("name") == "health.get" && d[ReadsTakeCallID] {
		// A read deduplicated like a write: recorded, and refused when its
		// call_id was used before for anything else.
		identity := map[string]any{"name": "health.get", "subject": req.Subject, "params": map[string]any{}}
		return a.once(req.CallID, "operation", identity, func() reply { return h(r, body) })
	}

	a.mu.Lock()
	acct := a.accounts[a.bindings[req.Subject]]
	remembered, isRemembered := a.dev.refused[req.CallID]
	if apply && d[CreatesOnWithdraw] && acct == nil && string(req.Desired["enabled"]) == "false" {
		acct = &account{id: strconv.Itoa(1001 + len(a.accounts)), username: username(req.Subject, ""), state: map[string]any{
			"enabled": false, "remote": false, "printers": []string{lobby}, "after_hours": []string{}, "job_seconds": []string{"0"}}}
		a.accounts[acct.id], a.bindings[req.Subject] = acct, acct.id
		a.record("apply " + acct.id)
		body = withField(body, "fingerprint", fingerprint(acct))
	}
	if apply && d[BindsOnWithdraw] && acct == nil && string(req.Desired["enabled"]) == "false" && len(a.accounts) > 0 {
		// The plan said "no account"; the apply binds the oldest account and
		// carries the withdraw out on it.
		acct = a.accounts[slices.Sorted(maps.Keys(a.accounts))[0]]
		a.bindings[req.Subject] = acct.id
		body = withField(body, "fingerprint", fingerprint(acct))
	}
	if route == "POST /lifecycle" && d[IgnoresReadOnly] {
		body = withField(body, "state", "active")
	}
	if apply && d[CreateAnswersMember] && acct == nil && string(req.Desired["enabled"]) == "true" && req.Fingerprint == fingerprint(nil) {
		// A member's account, made at that moment, answered as the create's,
		// with no username in the reply.
		member := &account{id: strconv.Itoa(1001 + len(a.accounts)), username: "kofi-kingston", state: map[string]any{
			"enabled": true, "remote": false, "printers": []string{lobby}, "after_hours": []string{}, "job_seconds": []string{"0"}}}
		a.accounts[member.id], a.bindings[req.Subject] = member, member.id
		out := outcome{Subject: req.Subject, Effect: "applied", Detail: "Done.", AccountID: member.id, Standing: member.standing(),
			Observed: member.state, Fingerprint: fingerprint(member)}
		rep := ok(out)
		a.mu.Unlock()
		return rep
	}
	if apply && d[AdoptsExisting] && acct == nil && string(req.Desired["enabled"]) == "true" &&
		req.Fingerprint == fingerprint(nil) && len(a.accounts) > 0 {
		// Binds the subject to the oldest account and says it was created,
		// writing nothing to that account.
		other := a.accounts[slices.Sorted(maps.Keys(a.accounts))[0]]
		a.bindings[req.Subject] = other.id
		out := outcome{Subject: req.Subject, Effect: "applied", Detail: "Done."}
		describe(&out, other)
		out.Observed, out.Fingerprint = other.state, fingerprint(other)
		rep := ok(out)
		a.mu.Unlock()
		return rep
	}
	current, accounts, existed := fingerprint(acct), len(a.accounts), acct != nil
	a.mu.Unlock()

	switch {
	case req.CallID != "" && isRemembered:
		return remembered
	case apply && d[ReplaysAfterFingerprint] && req.Fingerprint != current:
		return problem(409, "PLAN_STALE", "The subject changed since the plan.", nil)
	case apply && d[IgnoresFingerprint] && req.Fingerprint != "":
		body = withField(body, "fingerprint", current)
	}
	if apply && d[CommitsLate] && acct == nil && string(req.Desired["enabled"]) == "true" {
		a.mu.Lock()
		a.dev.late++
		a.mu.Unlock()
		time.Sleep(300 * time.Millisecond) // the caller may have given up; the commit still happens
		defer func() { a.mu.Lock(); a.dev.late--; a.mu.Unlock() }()
	}

	if route == "GET /stats" {
		q := r.URL.Query()
		if d[StatsUnknownMetricOK] && statsMetric(q.Get("metric")) == nil {
			return ok(map[string]any{"contract_version": ContractVersion, "metric": q.Get("metric"), "taken_at": now(),
				"watermark": stamp3339(dayOf(time.Now())), "points": []any{}, "unbound": map[string]int{"accounts": 0},
				"gaps": []any{}, "next_cursor": nil})
		}
		if d[StatsSinceIgnored] {
			q.Set("since", time.Now().AddDate(-1, 0, 0).UTC().Format(time.RFC3339))
			r = r.Clone(r.Context())
			r.URL.RawQuery = q.Encode()
		}
	}

	var rep reply
	if fresh := route == "GET /subjects" && r.URL.Query().Get("fresh") == "true"; fresh && (d[ReadsPerRequest] || d[CooldownIgnored]) {
		rep = a.readsAlone(d[ReadsPerRequest])
	} else {
		rep = h(r, body)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if d[CallsEchoParams] && route == "POST /operations/{name}" && req.CallID != "" {
		if a.dev.params == nil {
			a.dev.params = map[string]any{}
		}
		a.dev.params[req.CallID] = req.Params
	}
	if code := codeOf(rep); req.CallID != "" && (d[RemembersRefusal] && rep.status >= 400 || d[RemembersPlanStale] && code == "PLAN_STALE") {
		if a.dev.refused == nil {
			a.dev.refused = map[string]reply{}
		}
		a.dev.refused[req.CallID] = rep
	}
	if acct = a.accounts[a.bindings[req.Subject]]; apply && rep.status == 200 && acct != nil {
		if d[FailsAfterCreate] && len(a.accounts) > accounts {
			return problem(500, "APPLY_FAILED", "The target failed after the account was made.", nil)
		}
		if limit := req.Desired["job_seconds"]; d[LimitEmptyClears] && string(limit) == "[]" {
			acct.state["job_seconds"] = []string{"0"} // "no limit": what clearing means on most apps
		}
		if limit := req.Desired["job_seconds"]; d[LimitIgnoresUnlimited] && bytes.Contains(limit, []byte(`"0"`)) {
			var vals []string
			_ = json.Unmarshal(limit, &vals)
			acct.state["job_seconds"] = combine(field{}, slices.DeleteFunc(vals, func(v string) bool { return v == "0" }))
		}
		if acct.state["enabled"] == false && d[WithdrawTouchesMembership] {
			acct.state["printers"] = []string{}
		}
		if existed && d[RebindsQuietly] && !a.dev.quiet {
			for _, id := range slices.Sorted(maps.Keys(a.accounts)) {
				if id != acct.id {
					a.bindings[req.Subject], a.dev.quiet = id, true
					break
				}
			}
		}
		if len(a.accounts) > accounts && d[CreateOmitsAccount] {
			var doc map[string]any
			_ = json.Unmarshal(rep.body, &doc)
			delete(doc, "account_id")
			delete(doc, "standing")
			delete(doc, "username")
			rep.body, _ = json.Marshal(doc)
		}
		if existed && d[RebindsSubject] {
			for _, id := range slices.Sorted(maps.Keys(a.accounts)) {
				if id != acct.id {
					a.bindings[req.Subject] = id // someone else's account, now answering for this subject
					break
				}
			}
		}
		if acct.state["enabled"] == false && d[WithdrawStripsManaged] {
			acct.state["printers"] = slices.DeleteFunc(slices.Clone(acct.state["printers"].([]string)),
				func(p string) bool { return slices.Contains(printers, p) })
		}
	}
	rewrite := func(edit func(map[string]any)) {
		var doc map[string]any
		if json.Unmarshal(rep.body, &doc) == nil {
			edit(doc)
			rep.body, _ = json.Marshal(doc)
		}
	}
	numeric := func(list any) {
		for _, e := range list.([]any) {
			if m := e.(map[string]any); m["account_id"] != nil {
				m["account_id"], _ = strconv.Atoi(m["account_id"].(string))
			}
		}
	}
	switch {
	case route == "GET /subjects" && d[CachedAsCurrent]:
		rewrite(func(doc map[string]any) {
			if s, _ := doc["stale"].(map[string]any); s["reason"] == "cached" {
				delete(doc, "stale")
				doc["current"] = true
			}
		})
	case route == "GET /subjects" && d[OmitsTruncated]:
		rewrite(func(doc map[string]any) { delete(doc, "truncated") })
	case route == "GET /subjects" && d[CurrentFromMirror]:
		// A mirror refreshed only by this add-on's own writes, served as current.
		if a.dev.mirror == nil || a.dev.mirrorAt != a.logCount {
			a.dev.mirror, a.dev.mirrorAt = rep.body, a.logCount
		}
		rep.body = a.dev.mirror
	case route == "GET /subjects" && d[StateShowsUnlisted]:
		rewrite(func(doc map[string]any) {
			for _, s := range doc["subjects"].([]any) {
				m := s.(map[string]any)
				m["state"].(map[string]any)["printers"] = a.accounts[m["account_id"].(string)].state["printers"]
			}
		})
	case route == "GET /subjects" && d[NumericAccountID]:
		rewrite(func(doc map[string]any) { numeric(doc["subjects"]) })
	case route == "POST /plan" && d[NumericAccountID] && rep.status == 200:
		rewrite(func(doc map[string]any) { numeric(doc["outcomes"]) })
	case route == "GET /health" && d[LogNeverMoves]:
		rewrite(func(doc map[string]any) { doc["log_records"] = 0 })
	case route == "GET /health" && d[LogHeadEmpty] && a.logCount == 0:
		rewrite(func(doc map[string]any) { doc["log_head"] = "" })
	case route == "GET /health" && a.dev.late > 0:
		rewrite(func(doc map[string]any) { doc["in_flight"], doc["drained"] = a.dev.late, false })
	case passwordSet && d[RebindsOnOperation] && rep.status == 200 && !a.dev.rebound:
		for _, id := range slices.Sorted(maps.Keys(a.accounts)) {
			if id != a.bindings[req.Subject] {
				a.bindings[req.Subject], a.dev.rebound = id, true
				break
			}
		}
	case route == "POST /plan" && (d[PlanForgetsAccount] || d[RebindsQuietly] && a.dev.quiet) && rep.status == 200:
		rewrite(func(doc map[string]any) {
			for _, o := range doc["outcomes"].([]any) {
				delete(o.(map[string]any), "account_id")
				delete(o.(map[string]any), "standing")
			}
		})
	case passwordSet && d[OperationNamesOther] && rep.status == 200:
		for _, id := range slices.Sorted(maps.Keys(a.accounts)) {
			if id != a.bindings[req.Subject] {
				rewrite(func(doc map[string]any) { doc["account_id"] = id })
				break
			}
		}
	case holdings && d[ResultOffSchema] && rep.status == 200:
		rewrite(func(doc map[string]any) {
			doc["result"].(map[string]any)["records"] = []any{map[string]any{"id": printers[0]}}
		})
	case holdings && d[ResultInPocket] && rep.status == 200:
		rewrite(func(doc map[string]any) { doc["holdings"] = doc["result"]; delete(doc, "result") })
	case checkout && req.Preview && d[PreviewRecorded] && rep.status == 200:
		a.record("preview item.checkout")
	case checkout && !req.Preview && d[NoChangeAsSucceeded] && rep.status == 200 && bytes.Contains(rep.body, []byte(`"no_change"`)):
		rewrite(func(doc map[string]any) { doc["outcome"] = "succeeded" })
		a.record("item.checkout again")
	case route == "GET /calls/{call_id}" && d[CallsEchoParams] && a.dev.params[r.PathValue("call_id")] != nil:
		rewrite(func(doc map[string]any) {
			if reply, ok := doc["reply"].(map[string]any); ok {
				reply["params"] = a.dev.params[r.PathValue("call_id")]
			}
		})
	case route == "GET /stats" && rep.status == 200:
		rewrite(func(doc map[string]any) { a.statsDeviate(r.URL.Query().Get("metric"), doc) })
	case passwordSet && d[EchoesSecret] && rep.status == 200:
		rewrite(func(doc map[string]any) { doc["detail"] = fmt.Sprintf("Password set to %v.", req.Params["password"]) })
	}
	return rep
}

// readsAlone is a fresh /subjects as a broken add-on serves it: every request
// reads the app (no collapsing), with no cooldown at all, or with one stamped
// when a read ends instead of when it begins (cooled).
func (a *Addon) readsAlone(cooled bool) reply {
	a.mu.Lock()
	if cooled && a.fresh.kept && time.Since(a.fresh.began) < freshCooldown {
		defer a.mu.Unlock()
		return a.mirrored("cooldown")
	}
	gen := a.fresh.gen
	a.mu.Unlock()
	list, at, _ := a.readApp()
	a.mu.Lock()
	defer a.mu.Unlock()
	a.fresh.began = time.Now()
	a.keep(list, at, gen)
	return ok(map[string]any{"subjects": list, "current": true, "taken_at": at, "truncated": false})
}

func withField(body []byte, key string, value any) []byte {
	var doc map[string]any
	if json.Unmarshal(body, &doc) != nil {
		return body
	}
	doc[key] = value
	out, _ := json.Marshal(doc)
	return out
}

func codeOf(rep reply) string {
	var p struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(rep.body, &p)
	return p.Code
}

// statsDeviate breaks a /stats page the ways the stats defects say. Under a.mu.
func (a *Addon) statsDeviate(metric string, doc map[string]any) {
	d := a.cfg.Defects
	a.dev.reads++
	points := objectsOf(doc["points"])
	gaps := objectsOf(doc["gaps"])
	wm, _ := time.Parse(time.RFC3339, doc["watermark"].(string))
	// bucket is a point's day and dimensions, which a total shares with its people.
	bucket := func(p map[string]any) string {
		dims, _ := json.Marshal(p["dimensions"])
		return p["start"].(string) + string(dims)
	}
	people := map[string][]float64{}
	for _, p := range points {
		if p["subject"] != nil {
			people[bucket(p)] = append(people[bucket(p)], p["value"].(float64))
		}
	}
	kept := points[:0]
	for _, p := range points {
		end, _ := time.Parse(time.RFC3339, p["end"].(string))
		values, shared := people[bucket(p)]
		switch {
		case p["subject"] == nil && shared && d[StatsTotalMissing]:
			continue
		case p["subject"] == nil && shared && d[StatsSubjectsAboveTotal]:
			whole := slices.Max(values)
			if statsMetric(metric)["instrument"] == "counter" {
				whole = 0
				for _, v := range values {
					whole += v
				}
			}
			p["value"] = whole - 1
		case p["subject"] != nil && d[StatsLocalNameSubject]:
			p["subject"] = a.bindings[p["subject"].(string)]
		}
		if dims, _ := p["dimensions"].(map[string]any); d[StatsDimensionNamesPerson] && dims["printer"] == "mk4-a" && len(a.accounts) > 0 {
			dims["printer"] = a.accounts[slices.Min(slices.Collect(maps.Keys(a.accounts)))].username
		}
		if d[StatsUndeclaredDimension] {
			dims, _ := p["dimensions"].(map[string]any)
			if dims == nil {
				dims = map[string]any{}
			}
			dims["colour"] = "black"
			p["dimensions"] = dims
		}
		if d[StatsFinalMoves] && !end.After(wm) {
			p["value"] = p["value"].(float64) + float64(a.dev.reads)
		}
		kept = append(kept, p)
	}
	keptGaps := gaps[:0]
	for _, g := range gaps {
		if d[StatsNoBufferGap] && g["reason"] == bufferReason {
			continue
		}
		if dims := g["dimensions"]; d[StatsZeroForGap] && dims != nil {
			start, _ := time.Parse(time.RFC3339, g["start"].(string))
			kept = append(kept, map[string]any{"start": g["start"], "end": stamp3339(next(dayOf(start))), "dimensions": dims, "value": 0})
		}
		keptGaps = append(keptGaps, g)
	}
	if d[StatsWatermarkAhead] {
		doc["watermark"] = stamp3339(next(dayOf(time.Now())))
	}
	if c, _ := doc["next_cursor"].(string); d[StatsPageOverlap] && c != "" && len(kept) >= 2 {
		p := kept[len(kept)-2]
		dims := map[string]string{}
		for k, v := range objectOf(p["dimensions"]) {
			dims[k], _ = v.(string)
		}
		subject, _ := p["subject"].(string)
		doc["next_cursor"] = cursorOf(point{Start: p["start"].(string), Subject: subject, Dimensions: dims})
	}
	doc["points"], doc["gaps"] = kept, keptGaps
}

func objectsOf(v any) []map[string]any {
	out := []map[string]any{} // never null on the wire
	for _, e := range v.([]any) {
		out = append(out, e.(map[string]any))
	}
	return out
}

func objectOf(v any) map[string]any { m, _ := v.(map[string]any); return m }
