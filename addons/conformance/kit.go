// Package conformance is the add-on conformance kit (design §7 of
// openspec/changes/addon-contract): it drives an add-on black-box over the real
// transport and says, check by check, whether it may be connected to Syndra.
//
// Without Mutate it sends reads only (GET routes, /plan, and signed
// POST /operations/{id} for each operation the registry gives effect: read,
// all of which mutate nothing by contract). With Mutate it creates and converges one throwaway subject named
// conformance-..., and leaves exactly one disabled, labelled account, which
// Report.Left names. MutateSettings also writes a target-wide setting back.
package conformance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	contractfiles "syndra/addons/contract"
)

// Config is one run.
type Config struct {
	URL     string // https://host:port of the add-on
	Target  string // the target name: the salt of the key derivation
	Secret  []byte // the per-target secret, trimmed
	Subject string // the throwaway subject; default conformance-<UTC time>
	Email   string // default <subject>@example.org
	Mutate  bool
	// MutateSettings writes the first setting's current value back. It touches
	// a target-wide setting, so it is its own opt-in; not in design §7's table.
	MutateSettings bool
	// DrainWait bounds how long the end of a -mutate run waits for the add-on
	// to report in_flight 0 before it reads the final plan. Default 60 s.
	DrainWait time.Duration
	// CurrentGap separates the two reads the current check compares; it must
	// exceed the resolution of taken_at. Default 1.1 s.
	CurrentGap time.Duration
	// FreshWait is the longest limits.fresh_read_cooldown the freshness checks
	// wait out (they wait it before each fresh read); a longer one is a skip.
	// Default 35 s, above constants.cooldown_default.
	FreshWait time.Duration
	// Item is a thing on the target the kit may check out to its subject with
	// a physical operation (feature calls), and leave checked out: it is
	// named by the operator, never guessed. Empty skips operations.physical.
	Item string
}

// spansTheRun are the checks judged across several steps or over every reply
// of the run; an interrupted run never passes them.
const interruptedNote = "not judged: the run was interrupted"

var spansTheRun = []string{"health", "settings", "strict-requests", "requests.exact-names", "error-envelope", "required-fields",
	"plan-equals-apply", "create-is-new", "bound-to-created", "replay.refusal", "operations.secret"}

var (
	// ErrSubjectExists refuses a -mutate run whose subject already has an
	// account or whose name is taken: the kit never touches what it did not create.
	ErrSubjectExists = errors.New("the kit's subject already exists on the target; choose another -subject")
	// ErrReadsFailed refuses a -mutate run against an add-on whose reads
	// already broke a MUST: the kit cannot trust it to say what a write did.
	ErrReadsFailed = errors.New("-mutate refused: the add-on's reads already failed a MUST, so no write was sent")
	// ErrInterrupted is a run stopped by its context (SIGINT, SIGTERM). The
	// kit still restored lifecycle active and disabled its account first.
	ErrInterrupted = errors.New("interrupted; the kit restored what it had changed (see left)")
)

// subjectPrefix is the only kind of subject the kit will write to, whatever
// the add-on says: a subject an operator could mistake for a member's never is.
const subjectPrefix = "conformance-"

type check struct {
	ran       bool
	notJudged string // why it is neither pass nor fail: interrupted, or the run stopped early
	fails     []string
	skips     []string
	halt      func() string // why nothing is judged any more, or ""
}

// mark records that an assertion is being made. Once the run is interrupted,
// or the kit has stopped writing to its subject, nothing more is judged: the
// check is marked not judged instead, so it cannot pass on what never ran.
func (c *check) mark() bool {
	if why := c.haltReason(); why != "" {
		if c.notJudged == "" {
			c.notJudged = why
		}
		return false
	}
	c.ran = true
	return true
}

func (c *check) haltReason() string {
	if c.halt == nil {
		return ""
	}
	return c.halt()
}

func (c *check) fail(format string, args ...any) {
	if c.mark() {
		c.fails = append(c.fails, fmt.Sprintf(format, args...))
	}
}

func (c *check) skip(format string, args ...any) {
	c.skips = append(c.skips, fmt.Sprintf(format, args...))
}

// unjudged is a check that reached nothing it asserts: the reads that led
// there are judged by their own checks, and its skips say why it could not
// look. It is a skip then, never a pass on the way in; a fail stays a fail.
func (c *check) unjudged() { c.ran = false }

// expect is one assertion: it counts as having run whether or not it holds.
func (c *check) expect(holds bool, format string, args ...any) bool {
	if !holds {
		c.fail(format, args...)
	} else {
		c.mark()
	}
	return holds
}

type exchange struct {
	method, target string
	request        []byte
	status         int
	header         http.Header
	body           []byte
	err            error
}

func (ex *exchange) String() string { return ex.method + " " + ex.target }

// doc decodes the reply as a JSON object, numbers as json.Number.
func (ex *exchange) doc() map[string]any {
	dec := json.NewDecoder(bytes.NewReader(ex.body))
	dec.UseNumber()
	var d map[string]any
	if dec.Decode(&d) != nil {
		return nil
	}
	return d
}

func (ex *exchange) code() string { s, _ := ex.doc()["code"].(string); return s }

type kit struct {
	// mu is held by send except while a request is on the wire, so the two
	// concurrent reads of freshness.coalesce share one kit; every other
	// request is sent one at a time.
	mu         sync.Mutex
	ctx        context.Context
	cleaning   bool   // the run's last requests, sent even when ctx is done
	wrote      bool   // a write route was called (whether or not it answered)
	unanswered bool   // a write was sent and no reply came back: it may have landed
	unknown    string // why the kit cannot say what the run left
	step       string // the -mutate step running, to say when something happened
	// foreign lists the account ids owner() judged not the kit's; the first
	// freezes every write to the subject. halting turns on with the first
	// request after that, from when nothing more is judged.
	foreign    []string
	halting    bool
	stopReason string         // why the kit stopped writing to its subject, when no foreign id did
	finalRead  bool           // the end-of-run plan for the subject was read
	writes     []subjectWrite // every write to the subject, for the last line
	cfg        Config
	c          *contract
	client     *http.Client
	key        []byte
	base       string
	checks     map[string]*check
	log        []*exchange
	calls      int
	halted     string // why nothing more is sent

	m        *manifest
	values   map[string][]string // the managed set per enumerable field, from /values
	skipped  []string            // manifest keys of features newer than this kit
	before   []string            // account ids before the first write; nil until then
	appeared []string            // account ids that appeared during the run, not the kit's own
	canaries []string            // secret values sent; none may come back
	// passwordCall and passwordReply are the first password.set's call_id
	// and reply, which /calls must answer recorded (calls.recorded).
	passwordCall  string
	passwordReply []byte
	lent          string // Config.Item, once a physical operation checked it out or may have
	lentUnsure    bool   // the confirm's answer was lost or unreadable
	since         int    // index into log of the first canary's request
	readOnly      bool   // the kit set read_only and has not yet set active
	created       struct {
		id, username string
		off          bool // /subjects confirmed the account is off
		none         bool // a plan confirmed there is no account for the subject
	}
}

// Run checks one add-on. Cancelling ctx stops it between requests; a -mutate
// run still puts back what it changed before it returns. The error is for a
// run that could not start, a -mutate run refused, or an interrupted run;
// check failures are in the Report.
func Run(ctx context.Context, cfg Config) (*Report, error) {
	k, err := newKit(ctx, cfg)
	if err != nil {
		return nil, err
	}
	k.reads()
	var refused error
	switch {
	case ctx.Err() != nil:
		k.halted, refused = "not reached: the run was interrupted", ErrInterrupted
	case !k.cfg.Mutate || k.halted != "":
	case k.mustFailures() > 0:
		k.halted = "not reached: " + ErrReadsFailed.Error()
		refused = ErrReadsFailed
	default:
		refused = k.mutate()
	}
	// Half of each of these is about replies to writes (log_records advancing,
	// what an apply answers): reads alone cannot pass them. Decided after the
	// run, by whether a write was sent, so a -mutate run refused before its
	// first write is marked like a reads-only one (4.4a).
	if !k.wrote {
		why := "which need -mutate"
		if k.cfg.Mutate {
			why = "and this -mutate run sent none (see the last line)"
		}
		for _, id := range []string{"health", "required-fields"} {
			if c := k.at(id); c.notJudged == "" {
				c.notJudged = "not judged: half of this check is about writes (log_records advancing, apply replies), " + why
			}
		}
	}
	if ctx.Err() != nil {
		refused = ErrInterrupted
		// A check judged across several steps, or over the whole run's replies,
		// was not fully judged: it says so rather than pass on half of it.
		for _, id := range spansTheRun {
			k.at(id).notJudged = interruptedNote
		}
	} else {
		k.afterwards()
	}
	return k.report(), refused
}

func (k *kit) mustFailures() int {
	n := 0
	for _, e := range catalogue {
		if !e.advisory && len(k.at(e.id).fails) > 0 {
			n++
		}
	}
	return n
}

func newKit(ctx context.Context, cfg Config) (*kit, error) {
	u, err := url.Parse(cfg.URL)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return nil, fmt.Errorf("-url must be https://host:port, got %q", cfg.URL)
	}
	if strings.TrimSpace(cfg.Target) == "" || len(bytes.TrimSpace(cfg.Secret)) == 0 {
		return nil, errors.New("-target and a non-empty -secret-file are required")
	}
	c, err := loadContract(contractfiles.FS())
	if err != nil {
		return nil, err
	}
	if cfg.Subject == "" {
		cfg.Subject = subjectPrefix + time.Now().UTC().Format("20060102150405")
	}
	if !strings.HasPrefix(cfg.Subject, subjectPrefix) || len(cfg.Subject) == len(subjectPrefix) {
		return nil, fmt.Errorf("-subject must begin %q: the kit writes only to a subject named as its own", subjectPrefix)
	}
	if local, _, _ := strings.Cut(cfg.Email, "@"); cfg.Email != "" && !strings.HasPrefix(local, subjectPrefix) {
		return nil, fmt.Errorf("-email must begin %q: an add-on may name the account after it, and the kit owns only an account labelled as its own", subjectPrefix)
	}
	if cfg.MutateSettings && !cfg.Mutate {
		return nil, errors.New("-mutate-settings is a write and needs -mutate")
	}
	if cfg.Email == "" {
		cfg.Email = cfg.Subject + "@example.org"
	}
	if cfg.DrainWait == 0 {
		cfg.DrainWait = time.Minute
	}
	if cfg.CurrentGap == 0 {
		cfg.CurrentGap = 1100 * time.Millisecond
	}
	if cfg.FreshWait == 0 {
		cfg.FreshWait = 35 * time.Second
	}
	pub, key, err := deriveKeys(bytes.TrimSpace(cfg.Secret), cfg.Target)
	if err != nil {
		return nil, err
	}
	return &kit{ctx: ctx, cfg: cfg, c: c, client: pinnedClient(pub), key: key, base: strings.TrimRight(u.String(), "/"),
		checks: map[string]*check{}, values: map[string][]string{}}, nil
}

func (k *kit) at(id string) *check {
	if k.checks[id] == nil {
		k.checks[id] = &check{halt: k.notJudgedNow}
	}
	return k.checks[id]
}

func (k *kit) report() *Report {
	r := &Report{Kit: "syndra-addon-conform", ContractVersion: Major, URL: k.cfg.URL, Target: k.cfg.Target,
		Mode: "reads-only", Subject: k.cfg.Subject}
	if k.cfg.Mutate {
		r.Mode = "mutate"
	}
	add := func(res Result) {
		r.Results = append(r.Results, res)
		switch res.Status {
		case "pass":
			r.Pass++
		case "fail":
			r.Fail++
			if res.Level == "MUST" {
				r.MustFailures++
			}
		default:
			r.Skip++
		}
	}
	for _, e := range catalogue {
		res := Result{ID: e.id, Must: e.must + " (AUTHORING.md#" + e.guide + ")", Level: "MUST"}
		if e.advisory {
			res.Level = "ADVISORY"
		}
		c := k.at(e.id)
		switch {
		case len(c.fails) > 0:
			res.Status, res.Detail = "fail", c.fails
		case c.notJudged != "":
			res.Status, res.Detail = "skip", []string{c.notJudged}
		case c.ran:
			// What it skipped rides beside the pass, so a partial run never
			// reads as a whole one.
			res.Status, res.Detail = "pass", c.skips
		case len(c.skips) > 0:
			res.Status, res.Detail = "skip", c.skips
		case k.halted != "":
			res.Status, res.Detail = "skip", []string{k.halted}
		case e.mutate && !k.cfg.Mutate:
			res.Status, res.Detail = "skip", []string{"sends writes: needs -mutate"}
		default:
			res.Status, res.Detail = "skip", []string{"not reached"}
		}
		add(res)
	}
	if k.m != nil {
		for _, f := range k.m.features {
			id := "feature." + f
			switch _, known := k.c.features[f]; {
			case f == "operations" || f == "settings" || f == "freshness" || f == "calls" || f == "stats":
				continue // a row of its own above
			case known:
				add(Result{ID: id, Status: "skip", Level: "MUST", Must: "feature " + f, Detail: []string{"declared; this kit checks the core and does not check this feature yet"}})
			default:
				detail := []string{"unknown feature: newer than this kit's contract, so skip, never fail (AUTHORING.md#14-evolving)"}
				if len(k.skipped) > 0 {
					detail = append(detail, fmt.Sprintf("manifest keys not judged: %v", k.skipped))
				}
				add(Result{ID: id, Status: "skip", Level: "MUST", Must: "feature " + f, Detail: detail})
			}
		}
	}
	r.Left = k.left()
	return r
}

func (k *kit) left() string {
	s := k.leftAccount()
	if len(k.foreign) > 0 && k.created.id != "" {
		s += "; NOT confirmed: " + k.foreignLine()
	}
	if len(k.appeared) > 0 {
		s += fmt.Sprintf("; ALSO new since the run began: account_id %v, which the kit did not mean to create: check them", k.appeared)
	}
	if k.readOnly {
		s += "; and the add-on may still be read_only: set it active"
	}
	switch {
	case k.lentUnsure:
		s += fmt.Sprintf("; item %q may be checked out to subject %q (the confirm's answer was lost): check it and return it", k.lent, k.cfg.Subject)
	case k.lent != "":
		s += fmt.Sprintf("; item %q is checked out to subject %q: return it", k.lent, k.cfg.Subject)
	}
	return s
}

func (k *kit) leftAccount() string {
	switch {
	case !k.wrote && !k.cfg.Mutate:
		return "nothing; reads only, no write was sent"
	case k.created.id == "" && len(k.foreign) > 0:
		return "no account of the kit's own; " + k.foreignLine()
	case !k.wrote:
		return "nothing; no write was sent"
	case k.created.id != "" && k.created.off && k.finalRead && k.unknown == "":
		return fmt.Sprintf("one disabled account %q (account_id %s) on target %q; removal disables, never deletes",
			k.created.username, k.created.id, k.cfg.Target)
	case k.created.id != "":
		why := k.unknown
		if why == "" && !k.finalRead {
			why = "the end-of-run plan was never read"
		}
		if why != "" {
			why = " (" + why + ")"
		}
		return fmt.Sprintf("account %q (account_id %s) on target %q, NOT confirmed disabled%s: disable it by hand",
			k.created.username, k.created.id, k.cfg.Target, why)
	case k.created.none:
		return "nothing; no account was created"
	}
	why := k.unknown
	if why == "" {
		why = "it could not read the subject's plan"
	}
	return fmt.Sprintf("UNKNOWN: the kit cannot confirm what subject %q holds on target %q (%s); check that subject",
		k.cfg.Subject, k.cfg.Target, why)
}

type signing int

const (
	signed signing = iota
	unsigned
	staleClock
	wrongKey
)

// request is one call. schema names the request schema the kit's own body must
// satisfy; it is empty only for a deliberate violation.
type request struct {
	method, target string
	body           any
	schema         string
	sign           signing
	signAs         string // a different target for the MAC than the one sent
	creates        bool   // the apply that creates the kit's account (owner's claim)
	owned          bool   // came through toOwned: a plan naming the kit's account was read just before
}

// fields is the body as a map, whichever way it was built: a body sent as raw
// JSON (requests.exact-names) is decoded, so the subject it is about and the
// write it is are seen like any other's.
func (r request) fields() map[string]any {
	if m, ok := r.body.(map[string]any); ok {
		return m
	}
	raw, _ := json.Marshal(r.body)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	return m
}

func (k *kit) send(r request) *exchange {
	k.mu.Lock()
	ex, req, write := k.prepare(r)
	k.mu.Unlock()
	if req == nil {
		return ex
	}
	resp, err := k.client.Do(req)
	defer k.observe(r, ex) // answered or not; outside the lock, since it may send
	k.mu.Lock()
	defer k.mu.Unlock()
	if err != nil {
		ex.err = err
		k.unanswered = k.unanswered || write
		return ex
	}
	defer resp.Body.Close()
	ex.status, ex.header = resp.StatusCode, resp.Header
	ex.body, ex.err = io.ReadAll(io.LimitReader(resp.Body, k.c.maxReplyBytes+1))
	if int64(len(ex.body)) > k.c.maxReplyBytes {
		ex.err = fmt.Errorf("reply larger than constants.max_reply_bytes (%d)", k.c.maxReplyBytes)
	}
	return ex
}

// prepare logs the exchange and builds the signed request, or refuses it
// (nil request, ex.err saying why). Under k.mu.
func (k *kit) prepare(r request) (ex *exchange, _ *http.Request, write bool) {
	ex = &exchange{method: r.method, target: r.target}
	k.log = append(k.log, ex)
	if r.body != nil {
		ex.request, _ = json.Marshal(r.body)
	}
	if r.schema != "" {
		var doc any
		dec := json.NewDecoder(bytes.NewReader(ex.request))
		dec.UseNumber()
		_ = dec.Decode(&doc)
		if err := k.c.validate(r.schema, doc); err != nil {
			ex.err = fmt.Errorf("kit defect, its own request %s", err)
			return ex, nil, false
		}
	}
	ctx := k.ctx
	if k.cleaning {
		ctx = context.WithoutCancel(ctx) // putting things back is never cut short
	}
	if ctx.Err() != nil {
		ex.err = errors.New("not sent: the run was interrupted")
		return ex, nil, false
	}
	k.halting = k.halting || k.frozen()
	if path, _, _ := strings.Cut(r.target, "?"); k.frozen() && (path == "/apply" || strings.HasPrefix(path, "/operations/")) {
		// Every apply and operation, whatever subject it names: once an
		// account the kit does not own has answered, the kit writes no more.
		ex.err = errors.New("not sent: the kit stopped writing (see bound-to-created)")
		return ex, nil, false
	}
	if about, write := k.about(r); about && write && k.created.id != "" && !r.owned {
		// Once the kit owns an account, toOwned is the only way to its subject.
		ex.err = errors.New("not sent: no plan naming the kit's account was read just before it")
		return ex, nil, false
	}
	req, err := http.NewRequestWithContext(ctx, r.method, k.base+r.target, bytes.NewReader(ex.request))
	if err != nil {
		ex.err = err
		return ex, nil, false
	}
	write = k.c.writes(r.method, r.target)
	k.wrote = k.wrote || write // before it is sent: a write that never answers may still have landed
	if r.body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	target, ts, key := signedTarget(req.URL), time.Now(), k.key
	if r.signAs != "" {
		target = r.signAs
	}
	switch r.sign {
	case staleClock:
		ts = ts.Add(-10 * time.Minute)
	case wrongKey:
		sum := sha256.Sum256([]byte("conformance: not the derived key"))
		key = sum[:]
	}
	if r.sign != unsigned {
		req.Header.Set(signatureHeader, fmt.Sprintf("t=%d,v1=%s", ts.Unix(), mac(ts.Unix(), r.method, target, ex.request, key)))
	}
	return ex, req, write
}

// ok asserts a 200 that validates against schema and returns it decoded.
func (k *kit) ok(c *check, ex *exchange, schema string) map[string]any {
	if !c.mark() {
		return nil
	}
	if ex.err != nil {
		c.fail("%s: %v", ex, ex.err)
		return nil
	}
	if ex.status != http.StatusOK {
		c.fail("%s answered %d %s, want 200", ex, ex.status, ex.code())
		return nil
	}
	doc := ex.doc()
	if doc == nil {
		c.fail("%s: the reply is not a JSON object", ex)
		return nil
	}
	if err := k.c.validate(schema, doc); err != nil {
		c.fail("%s: the reply %v", ex, err)
	}
	return doc
}

// refused asserts a refusal with this status and code (the envelope itself is
// error-envelope's) and returns the problem decoded.
func (k *kit) refused(c *check, ex *exchange, status int, code, why string) map[string]any {
	if !c.mark() {
		return nil
	}
	if ex.err != nil {
		c.fail("%s (%s): %v", ex, why, ex.err)
		return nil
	}
	if ex.status != status || ex.code() != code {
		c.fail("%s (%s) answered %d %s, want %d %s", ex, why, ex.status, ex.code(), status, code)
		return nil
	}
	return ex.doc()
}

func (k *kit) callID() string {
	k.calls++
	return fmt.Sprintf("%s-call-%d", k.cfg.Subject, k.calls)
}

// ---- small readers of decoded JSON ----------------------------------------

func str(m map[string]any, key string) string { s, _ := m[key].(string); return s }

func num(v any) (int64, bool) {
	n, ok := v.(json.Number)
	if !ok {
		return 0, false
	}
	i, err := n.Int64()
	return i, err == nil
}

func objects(v any) []map[string]any {
	list, _ := v.([]any)
	out := []map[string]any{}
	for _, e := range list {
		if m, ok := e.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func stringList(v any) ([]string, bool) {
	list, ok := v.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(list))
	for _, e := range list {
		s, ok := e.(string)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

func same(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return bytes.Equal(x, y)
}

// notJudgedNow is why nothing more is judged: the run was interrupted, or the
// kit has stopped writing to its subject and sent something since (what it
// says about the reply that exposed the foreign account is still judged).
func (k *kit) notJudgedNow() string {
	switch {
	case k.ctx.Err() != nil:
		return interruptedNote
	case k.halting && !k.cleaning:
		return "not judged: the kit stopped writing to its subject (see bound-to-created)"
	}
	return ""
}
