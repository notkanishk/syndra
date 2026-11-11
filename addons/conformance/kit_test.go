package conformance

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"syndra/addons/reference"
)

// The kit's own tests (tasks 4.4). A kit nobody has watched fail is a kit that
// passes everything, so each defect the reference can be switched into is
// served in-process, over the real derived-key transport, and the check that
// exists to catch it must say fail.

const testSecret = "conformance-self-test-secret-not-a-credential"

// serve starts the reference over TLS with the key derived from secret, and
// records every request that got past the TLS handshake. hook, when given,
// sees each request (and its body) before the reference has it, answered
// false, and after, answered true.
func serve(t *testing.T, secret string, hook func(r *http.Request, body []byte, answered bool), defects ...reference.Defect) (string, func() []string) {
	t.Helper()
	set := map[reference.Defect]bool{}
	for _, d := range defects {
		set[d] = true
	}
	addon, err := reference.New(reference.Config{Target: "reference", Secret: []byte(secret), Defects: set})
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var seen []string
	h := addon.Handler()
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Method+" "+r.URL.Path)
		mu.Unlock()
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))
		if hook != nil {
			hook(r, body, false)
		}
		h.ServeHTTP(w, r)
		if hook != nil {
			hook(r, body, true)
		}
	}))
	srv.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{addon.Certificate()}}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv.URL, func() []string { mu.Lock(); defer mu.Unlock(); return slices.Clone(seen) }
}

func config(url string, mutate bool) Config {
	return Config{URL: url, Target: "reference", Secret: []byte(testSecret), Mutate: mutate, Subject: "conformance-self-test"}
}

func run(t *testing.T, cfg Config) *Report {
	t.Helper()
	r, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func failures(r *Report) []Result {
	var out []Result
	for _, res := range r.Results {
		if res.Status == "fail" {
			out = append(out, res)
		}
	}
	return out
}

func writes(seen []string) []string {
	return slices.DeleteFunc(seen, func(s string) bool { return strings.HasPrefix(s, "GET ") || s == "POST /plan" })
}

// subjects reads /subjects through a kit of its own: what the run left.
func subjects(t *testing.T, url string) []map[string]any {
	k, err := newKit(context.Background(), config(url, false))
	if err != nil {
		t.Fatal(err)
	}
	return objects(k.send(request{method: http.MethodGet, target: "/subjects"}).doc()["subjects"])
}

func lifecycleOf(t *testing.T, url string) string {
	k, _ := newKit(context.Background(), config(url, false))
	return str(k.send(request{method: http.MethodGet, target: "/health"}).doc(), "lifecycle")
}

const leftOne = `one disabled account "conformance-self-test" (account_id 1001) on target "reference"; removal disables, never deletes`

// The clean reference passes every core check, and skips only what this kit
// does not check yet. A check that quietly skipped against the reference
// would be a check nobody runs, so the skips are pinned too.
func TestTheCleanReferencePassesEveryCheck(t *testing.T) {
	url, seen := serve(t, testSecret, nil)
	r := run(t, config(url, true))
	for _, f := range failures(r) {
		t.Errorf("%s failed against the clean reference: %v", f.ID, f.Detail)
	}
	var skipped []string
	for _, res := range r.Results {
		if res.Status == "skip" {
			skipped = append(skipped, res.ID)
		}
	}
	want := []string{"subjects.paged", "replay.29d", "operations.physical", "freshness", "dangerous-scopes"}
	if !slices.Equal(skipped, want) {
		t.Errorf("skipped %v, want exactly %v", skipped, want)
	}
	if r.Failed() || r.Left != leftOne {
		t.Errorf("failed %v, left %q", r.Failed(), r.Left)
	}
	if list := subjects(t, url); len(list) != 1 || list[0]["state"].(map[string]any)["enabled"] != false {
		t.Errorf("the run left %v, want exactly one disabled account", list)
	}
	if slices.ContainsFunc(seen(), func(s string) bool { return strings.HasPrefix(s, "PUT /settings") }) {
		t.Error("a setting was written without -mutate-settings")
	}
}

// A setting is target-wide: it is written back only under its own flag.
func TestSettingsAreWrittenOnlyWithMutateSettings(t *testing.T) {
	url, seen := serve(t, testSecret, nil)
	cfg := config(url, true)
	cfg.MutateSettings = true
	r := run(t, cfg)
	if r.Status("settings") != "pass" || !slices.Contains(seen(), "PUT /settings/home_printer") {
		t.Errorf("settings %s, requests %v", r.Status("settings"), writes(seen()))
	}
	if _, err := Run(context.Background(), Config{URL: url, Target: "reference", Secret: []byte(testSecret), MutateSettings: true}); err == nil {
		t.Error("-mutate-settings without -mutate was accepted")
	}
}

// Without -mutate the kit sends nothing but reads: the GET routes, and /plan,
// which mutates nothing by contract.
func TestWithoutMutateTheKitSendsReadsOnly(t *testing.T) {
	url, seen := serve(t, testSecret, nil)
	r := run(t, config(url, false))
	for _, f := range failures(r) {
		t.Errorf("%s failed against the clean reference: %v", f.ID, f.Detail)
	}
	if w := writes(seen()); len(w) > 0 {
		t.Errorf("a reads-only run sent %v", w)
	}
	if r.Left != "nothing; reads only, no write was sent" || r.Status("fingerprint") != "skip" {
		t.Errorf("left %q, fingerprint %s", r.Left, r.Status("fingerprint"))
	}
}

// Every defect the reference can be switched into fails the check written to
// catch it. A defect with no row here fails the test, so the list cannot rot.
func TestEveryDefectFailsTheCheckThatCatchesIt(t *testing.T) {
	catches := map[reference.Defect]string{
		reference.IgnoresFingerprint:        "fingerprint",
		reference.CreatesOnWithdraw:         "withdraw-only.no-account",
		reference.EchoesSecret:              "operations.secret",
		reference.ReplaysAfterFingerprint:   "replay",
		reference.RemembersRefusal:          "replay.refusal",
		reference.RemembersPlanStale:        "replay.refusal",
		reference.OmitsTruncated:            "required-fields",
		reference.CurrentFromMirror:         "current",
		reference.AcceptsAlteredQuery:       "transport.query",
		reference.LimitIgnoresUnlimited:     "limit-combine",
		reference.WithdrawTouchesMembership: "withdraw-only.existing",
		reference.WithdrawStripsManaged:     "withdraw-only.existing",
		reference.NumericAccountID:          "read-back",
		reference.FailsAfterCreate:          "plan-equals-apply",
		reference.LogNeverMoves:             "health",
		// Not a check's to catch: what a run leaves. Each has its own test.
		reference.CommitsLate:         "(TestAnUnansweredCreateIsNeverReportedAsNothing)",
		reference.RebindsSubject:      "(TestTheEndTouchesOnlyTheKitsOwnAccount)",
		reference.AdoptsExisting:      "(TestACreateThatAdoptsAnExistingAccountStopsTheRun)",
		reference.BindsOnWithdraw:     "(TestABindingBeforeTheCreateStopsEveryWrite)",
		reference.OperationNamesOther: "(TestAPasswordSetNamingAnotherAccountFreezesTheSubject)",
		reference.RebindsOnOperation:  "(TestAPasswordSetBeforeARebindIsListed)",
		reference.PlanForgetsAccount:  "bound-to-created",
		reference.CreateAnswersMember: "(TestACreateAnsweredWithAMembersAccountIsNotClaimed)",
		reference.RebindsQuietly:      "(TestNoWriteFollowsAPlanThatNamesNoAccount)",
		reference.CreateOmitsAccount:  "(TestACreateNamingNoAccountIsListed)",
		reference.IgnoresReadOnly:     "lifecycle",
	}
	for _, d := range reference.Defects {
		id, ok := catches[d]
		if !ok {
			t.Errorf("defect %s has no check that is watched to catch it", d)
			continue
		}
		if strings.HasPrefix(id, "(") {
			continue
		}
		t.Run(string(d), func(t *testing.T) {
			t.Parallel()
			url, _ := serve(t, testSecret, nil, d)
			r, err := Run(context.Background(), config(url, true))
			if err != nil && !errors.Is(err, ErrReadsFailed) {
				t.Fatal(err)
			}
			if got := r.Status(id); got != "fail" {
				t.Errorf("check %s says %s against a reference that %s", id, got, d)
			}
			if !r.Failed() {
				t.Error("the run would exit 0")
			}
		})
	}
}

// -mutate never writes to a subject it cannot prove is unbound: the add-on's
// plan is read by presence, not by value, and a run whose reads already broke
// a MUST sends no write at all. A bound subject whose account_id comes back as
// a number would pass a check of the string; it must not.
func TestMutateRefusesBeforeAnyWriteWhenItCannotProveTheSubjectIsNew(t *testing.T) {
	url, seen := serve(t, testSecret, nil, reference.NumericAccountID)
	k, err := newKit(context.Background(), config(url, true))
	if err != nil {
		t.Fatal(err)
	}
	k.m = &manifest{account: "enabled"}
	plan := objects(k.send(request{method: http.MethodPost, target: "/plan", body: k.planBody(map[string]any{"enabled": true})}).doc()["outcomes"])[0]
	k.send(request{method: http.MethodPost, target: "/apply", body: k.applyBody("bind-it", str(plan, "fingerprint"), map[string]any{"enabled": true})})
	bound := len(seen())

	r, err := Run(context.Background(), config(url, true))
	if !errors.Is(err, ErrReadsFailed) {
		t.Errorf("Run answered %v, want ErrReadsFailed", err)
	}
	if w := writes(seen()[bound:]); len(w) > 0 {
		t.Errorf("the refused run sent %v", w)
	}
	if r.Left != "nothing; no write was sent" {
		t.Errorf("left %q", r.Left)
	}
	for _, pre := range []map[string]any{
		{"effect": "apply", "account_id": json.Number("1001")},
		{"effect": "apply", "standing": map[string]any{}},
		{"effect": "apply", "conflict": map[string]any{}},
		{"effect": "no_change"},
		{},
	} {
		if !exists(pre) {
			t.Errorf("exists(%v) is false", pre)
		}
	}
	if exists(map[string]any{"effect": "apply", "subject": "conformance-x", "fingerprint": "f"}) {
		t.Error("a plain create plan reads as an existing account")
	}
}

// A bound subject is refused, and so is any -subject not named as the kit's.
func TestMutateRefusesASubjectThatExistsOrIsNotTheKits(t *testing.T) {
	url, seen := serve(t, testSecret, nil)
	run(t, config(url, true))
	before := len(seen())
	r, err := Run(context.Background(), config(url, true))
	if !errors.Is(err, ErrSubjectExists) {
		t.Fatalf("a second run on the same subject answered %v", err)
	}
	if w := writes(seen()[before:]); len(w) > 0 || r.Left != "nothing; no write was sent" {
		t.Errorf("a refused run sent %v, left %q", w, r.Left)
	}
	before = len(seen())
	cfg := config(url, true)
	cfg.Subject = "seth-rollins"
	if _, err := Run(context.Background(), cfg); err == nil || len(seen()) != before {
		t.Errorf("subject seth-rollins: err %v, %d requests sent", err, len(seen())-before)
	}
}

// The creating apply answers 500 after the account was made, so the kit
// never comes to own it. The final plan names that account for the first
// time: it is foreign, nothing is sent to it, and the last line lists the
// create as what may have reached it.
func TestANewIdFirstSeenAtTheEndIsNeverAdopted(t *testing.T) {
	url, _ := serve(t, testSecret, nil, reference.FailsAfterCreate)
	r, err := Run(context.Background(), config(url, true))
	if err != nil {
		t.Fatal(err)
	}
	if r.Status("plan-equals-apply") != "fail" || r.Status("bound-to-created") != "fail" {
		t.Errorf("plan-equals-apply %s, bound-to-created %s", r.Status("plan-equals-apply"), r.Status("bound-to-created"))
	}
	if !strings.HasPrefix(r.Left, `no account of the kit's own; subject "conformance-self-test" was answered with account_id 1001, which the kit does not own (it may have been made by the writes listed); `) ||
		!strings.Contains(r.Left, `create: apply {`) || !strings.Contains(r.Left, `(answered 500 APPLY_FAILED); nothing after that: check it`) {
		t.Errorf("left %q", r.Left)
	}
	if list := subjects(t, url); len(list) != 1 || list[0]["state"].(map[string]any)["enabled"] != true {
		t.Errorf("the account first seen at the end was written to: %v", list)
	}
}

// Interrupted while the add-on is read_only, and in the worst place: the
// request that set read_only reached the add-on but its reply never came back.
// The kit must still set active again and turn its account off.
func TestAnInterruptedRunPutsBackWhatItChanged(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	url, _ := serve(t, testSecret, func(r *http.Request, _ []byte, answered bool) {
		if answered && r.URL.Path == "/lifecycle" {
			cancel()
		}
	})
	r, err := Run(ctx, config(url, true))
	if !errors.Is(err, ErrInterrupted) {
		t.Errorf("Run answered %v, want ErrInterrupted", err)
	}
	if got := lifecycleOf(t, url); got != "active" {
		t.Errorf("the add-on was left %s", got)
	}
	if r.Left != leftOne {
		t.Errorf("left %q", r.Left)
	}
}

// The last line is the operator's record of what the run left on a real
// target, so every case of it says something true and actionable.
func TestTheLastLineSaysWhatWasLeft(t *testing.T) {
	k := &kit{cfg: Config{Mutate: true, Target: "truenas", Subject: "conformance-x"}, wrote: true}
	if got := k.left(); !strings.HasPrefix(got, `UNKNOWN`) || !strings.Contains(got, `"conformance-x"`) {
		t.Errorf("an unconfirmed run says %q", got)
	}
	k.created.none = true
	if got := k.left(); got != "nothing; no account was created" {
		t.Errorf("left %q", got)
	}
	k.created.id, k.created.username, k.created.off, k.finalRead = "1001", "conformance-x", true, true
	if got := k.left(); got != `one disabled account "conformance-x" (account_id 1001) on target "truenas"; removal disables, never deletes` {
		t.Errorf("left %q", got)
	}
	k.created.off, k.appeared, k.readOnly = false, []string{"1002"}, true
	for _, want := range []string{"NOT confirmed disabled", "account_id [1002]", "still be read_only"} {
		if !strings.Contains(k.left(), want) {
			t.Errorf("left %q does not say %q", k.left(), want)
		}
	}
}

// A wrong secret is a different derived key: the handshake fails, nothing
// reaches a handler, and the kit says so once rather than failing everything.
func TestAWrongSecretStopsAtThePin(t *testing.T) {
	url, seen := serve(t, "a different secret", nil)
	r := run(t, config(url, true))
	if r.Status("transport.pin") != "fail" || r.Status("transport.vectors") != "pass" {
		t.Errorf("pin %s, vectors %s", r.Status("transport.pin"), r.Status("transport.vectors"))
	}
	if got := failures(r); len(got) != 1 {
		t.Errorf("%d checks failed, want only transport.pin: %v", len(got), got)
	}
	if n := len(seen()); n != 0 || r.Left != "nothing; no write was sent" {
		t.Errorf("%d requests reached the add-on through a pin that did not hold; left %q", n, r.Left)
	}
}

// The kit judges add-ons black-box: the reference is a test fixture, and the
// binary never links it.
func TestTheKitDoesNotImportTheReference(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "./cmd/syndra-addon-conform").CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, out)
	}
	if strings.Contains(string(out), "syndra/addons/reference") {
		t.Error("the kit binary links the reference add-on")
	}
}

// An add-on ahead of the kit is skipped, never failed (design §2): a feature
// the kit does not know, and the manifest key it brings, are not judged. The
// same key with no unknown feature to own it is a typo, and fails.
func TestAnUnknownFeatureIsSkippedNeverFailed(t *testing.T) {
	for _, tc := range []struct {
		features []any
		manifest string
	}{
		{[]any{"operations", "settings", "inventory.loans"}, "pass"},
		{[]any{"operations", "settings"}, "fail"},
	} {
		addon, err := reference.New(reference.Config{Target: "reference", Secret: []byte(testSecret)})
		if err != nil {
			t.Fatal(err)
		}
		h := addon.Handler()
		srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/capabilities" {
				h.ServeHTTP(w, r)
				return
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, r)
			var doc map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &doc); err == nil && rec.Code == 200 {
				doc["features"], doc["loan_period"] = tc.features, "P14D"
			}
			w.WriteHeader(rec.Code)
			_ = json.NewEncoder(w).Encode(doc)
		}))
		srv.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{addon.Certificate()}}
		srv.StartTLS()
		r := run(t, config(srv.URL, false))
		srv.Close()
		if got := r.Status("manifest"); got != tc.manifest {
			t.Errorf("features %v with an undeclared key: manifest %s, want %s", tc.features, got, tc.manifest)
		}
		if tc.manifest == "pass" && r.Status("feature.inventory.loans") != "skip" {
			t.Errorf("an unknown feature says %q, want skip", r.Status("feature.inventory.loans"))
		}
	}
}

// "Nothing was written" is a claim about log_records; when the count cannot be
// read the claim is unverified, and that is a failure, never a pass.
func TestAnUnreadableLogIsNeverAPass(t *testing.T) {
	addon, err := reference.New(reference.Config{Target: "reference", Secret: []byte(testSecret)})
	if err != nil {
		t.Fatal(err)
	}
	h := addon.Handler()
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		h.ServeHTTP(w, r)
	}))
	srv.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{addon.Certificate()}}
	srv.StartTLS()
	defer srv.Close()
	k, err := newKit(context.Background(), config(srv.URL, true))
	if err != nil {
		t.Fatal(err)
	}
	for _, assert := range []func(*check){
		func(c *check) { k.unwritten(c, 0, true, "a probe") },
		func(c *check) { n, ok := k.logRecords(c); k.unwritten(c, n, ok, "a probe") },
		func(c *check) { k.advanced(c, 0, true, "a write") },
	} {
		c := &check{}
		if assert(c); len(c.fails) == 0 {
			t.Error("an unreadable log_records passed")
		}
	}
}

// wrapped serves the reference behind edit, which may answer a request itself.
func wrapped(t *testing.T, edit func(w http.ResponseWriter, r *http.Request, h http.Handler) bool, defects ...reference.Defect) (string, func() []string) {
	t.Helper()
	set := map[reference.Defect]bool{}
	for _, d := range defects {
		set[d] = true
	}
	addon, err := reference.New(reference.Config{Target: "reference", Secret: []byte(testSecret), Defects: set})
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var seen []string
	h := addon.Handler()
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Method+" "+r.URL.Path)
		mu.Unlock()
		if !edit(w, r, h) {
			h.ServeHTTP(w, r)
		}
	}))
	srv.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{addon.Certificate()}}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv.URL, func() []string { mu.Lock(); defer mu.Unlock(); return slices.Clone(seen) }
}

// The run is cancelled while the apply that creates the kit's account is in
// the add-on, which commits it 300 ms later. The kit never saw that create
// answered, so it never owns the account: after the drain the final plan
// names it, it is foreign, nothing is sent to it, and the last line names the
// unanswered create, never "no account was created".
func TestAnUnansweredCreateIsNeverReportedAsNothing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	url, _ := serve(t, testSecret, func(r *http.Request, body []byte, answered bool) {
		if !answered && r.URL.Path == "/apply" && bytes.Contains(body, []byte(`"enabled":true`)) &&
			bytes.Contains(body, []byte(`"fingerprint":"`)) && !bytes.Contains(body, []byte("conformance-wrong-fingerprint")) {
			cancel()
		}
	}, reference.CommitsLate)
	r, err := Run(ctx, config(url, true))
	if !errors.Is(err, ErrInterrupted) {
		t.Errorf("Run answered %v, want ErrInterrupted", err)
	}
	if !strings.Contains(r.Left, "account_id 1001, which the kit does not own (it may have been made by the writes listed); what of the kit's may have reached it: ") ||
		!strings.Contains(r.Left, "create: apply {") || !strings.Contains(r.Left, "(unanswered)") {
		t.Errorf("left %q", r.Left)
	}
	if list := subjects(t, url); len(list) != 1 || list[0]["state"].(map[string]any)["enabled"] != true {
		t.Errorf("the late account was written to after it was judged foreign: %v", list)
	}
}

// What the end of a run believes about "no account": only a plan read after
// the add-on drained, and only when every write was answered.
func TestNoAccountIsBelievedOnlyWhenEveryWriteWasAnswered(t *testing.T) {
	stuck := false
	url, _ := wrapped(t, func(w http.ResponseWriter, r *http.Request, _ http.Handler) bool {
		if stuck && r.URL.Path == "/health" {
			rec := httptest.NewRecorder()
			addonHealthOnce(rec)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(rec.Body.Bytes())
			return true
		}
		return false
	})
	for _, tc := range []struct {
		unanswered, stuck bool
		none              bool
	}{{false, false, true}, {true, false, false}, {false, true, false}} {
		cfg := config(url, true)
		cfg.DrainWait = 300 * time.Millisecond
		k, err := newKit(context.Background(), cfg)
		if err != nil {
			t.Fatal(err)
		}
		k.m, k.before, k.unanswered, k.wrote, k.cleaning, stuck = &manifest{account: "enabled"}, []string{}, tc.unanswered, true, true, tc.stuck
		k.disableOwn()
		if k.created.none != tc.none || (!tc.none && !strings.HasPrefix(k.left(), "UNKNOWN")) {
			t.Errorf("unanswered %v, never drains %v: none %v, left %q", tc.unanswered, tc.stuck, k.created.none, k.left())
		}
	}
}

// addonHealthOnce writes a valid health reply that never drains.
func addonHealthOnce(w http.ResponseWriter) {
	_ = json.NewEncoder(w).Encode(map[string]any{"reachable": true, "product": "reference", "product_version": "1.0.0",
		"version_tested": true, "credential_expiry": "none", "circuit_open": false, "lifecycle": "active",
		"in_flight": 1, "drained": false, "log_head": "", "log_records": 0, "checked_at": "2026-11-10T00:00:00Z"})
}

// Mid-run, the add-on starts answering for the kit's subject with an account
// that was there before the run. The kit stops writing to the subject at the
// first plan that names it: no write reaches that account, every check the
// freeze cut short is not judged, and the last line lists what may have reached it.
func TestARebindMidRunStopsEveryWriteToTheSubject(t *testing.T) {
	url, seen := serve(t, testSecret, nil, reference.RebindsSubject)
	bystander(t, url, false) // off: a write that reached it would show
	before := account(t, url, "1001")
	r := run(t, config(url, true))
	if after := account(t, url, "1001"); after != before {
		t.Errorf("the rebound account changed:\n before %s\n after  %s", before, after)
	}
	if r.Status("bound-to-created") != "fail" {
		t.Errorf("bound-to-created %s", r.Status("bound-to-created"))
	}
	notJudged(t, r, "absent-unmanaged", "lifecycle", "operations", "plan-equals-apply", "withdraw-only.existing")
	if slices.Contains(seen(), "POST /operations/password.set") {
		t.Error("an operation was sent after the subject was rebound")
	}
	if !strings.Contains(r.Left, "account_id 1001, which the kit does not own (it may have been made by the writes listed); what of the kit's may have reached it: ") ||
		!strings.Contains(r.Left, "; NOT confirmed: ") || !strings.Contains(r.Left, `"conformance-self-test" (account_id 1002)`) {
		t.Errorf("left %q", r.Left)
	}
}

// The accounts there before the first write are what tells the kit's account
// from anyone else's: if they cannot be read, nothing is written.
func TestAnUnreadableBeforeRefusesBeforeAnyWrite(t *testing.T) {
	url, seen := wrapped(t, func(w http.ResponseWriter, r *http.Request, _ http.Handler) bool {
		if r.URL.Path == "/subjects" {
			w.WriteHeader(http.StatusInternalServerError)
			return true
		}
		return false
	})
	k, err := newKit(context.Background(), config(url, true))
	if err != nil {
		t.Fatal(err)
	}
	k.reads()
	if err := k.mutate(); !errors.Is(err, ErrReadsFailed) {
		t.Errorf("mutate answered %v, want ErrReadsFailed", err)
	}
	if w := writes(seen()); len(w) > 0 {
		t.Errorf("sent %v", w)
	}
}

// An interrupt during the reads is an interrupt: exit 130, not a refusal, and
// what was never answered because of it is not counted against the add-on.
func TestAnInterruptDuringReadsIsAnInterrupt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	url, seen := serve(t, testSecret, func(r *http.Request, _ []byte, answered bool) {
		if !answered && strings.HasPrefix(r.URL.Path, "/values/") {
			cancel()
		}
	})
	r, err := Run(ctx, config(url, true))
	if !errors.Is(err, ErrInterrupted) {
		t.Errorf("Run answered %v, want ErrInterrupted", err)
	}
	if r.MustFailures != 0 {
		t.Errorf("an interrupted clean run reports %d MUST failures: %v", r.MustFailures, failures(r))
	}
	if w := writes(seen()); len(w) > 0 {
		t.Errorf("sent %v", w)
	}
}

// bystander makes account 1001, bound to someone else, before the kit runs,
// and turns it off again unless on.
func bystander(t *testing.T, url string, on bool) {
	t.Helper()
	k, err := newKit(context.Background(), config(url, true))
	if err != nil {
		t.Fatal(err)
	}
	k.cfg.Subject, k.cfg.Email = "conformance-bystander", "randy.orton@example.org"
	k.m = &manifest{account: "enabled"}
	plan := objects(k.send(request{method: http.MethodPost, target: "/plan", body: k.planBody(map[string]any{"enabled": true})}).doc()["outcomes"])[0]
	k.send(request{method: http.MethodPost, target: "/apply", body: k.applyBody("bystander", str(plan, "fingerprint"), map[string]any{"enabled": true})})
	if !on {
		plan = objects(k.send(request{method: http.MethodPost, target: "/plan", body: k.planBody(map[string]any{"enabled": false})}).doc()["outcomes"])[0]
		k.send(request{method: http.MethodPost, target: "/apply", body: k.applyBody("bystander-off", str(plan, "fingerprint"), map[string]any{"enabled": false})})
	}
}

// Once a foreign account has been seen, no apply or operation for the
// subject or any other leaves the kit, whoever asks, the end of the run
// included; reads still do.
func TestAFrozenKitSendsNoWriteToItsSubject(t *testing.T) {
	url, seen := serve(t, testSecret, nil)
	k, err := newKit(context.Background(), config(url, true))
	if err != nil {
		t.Fatal(err)
	}
	k.foreign, k.cleaning = []string{"1001"}, true
	for _, target := range []string{"/apply", "/operations/password.set"} {
		if ex := k.send(request{method: http.MethodPost, target: target, body: map[string]any{"subject": k.cfg.Subject}}); ex.err == nil {
			t.Errorf("%s was sent", target)
		}
	}
	// Any subject: the -other replay probe must not go out after a freeze either.
	if ex := k.send(request{method: http.MethodPost, target: "/apply", body: map[string]any{"subject": k.cfg.Subject + "-other"}}); ex.err == nil {
		t.Error("an apply for another subject was sent after a freeze")
	}
	k.send(request{method: http.MethodGet, target: "/health"})
	if got := seen(); !slices.Equal(got, []string{"GET /health"}) {
		t.Errorf("reached the add-on: %v", got)
	}
}

func account(t *testing.T, url, id string) string {
	for _, s := range subjects(t, url) {
		if s["account_id"] == id {
			raw, _ := json.Marshal(s)
			return string(raw)
		}
	}
	return ""
}

// The add-on answers the kit's create with an account that was already there.
// The kit stops at once: it never converges, replays or disables that account,
// and the last line names it as untouched, never "disable it by hand".
func TestACreateThatAdoptsAnExistingAccountStopsTheRun(t *testing.T) {
	url, _ := serve(t, testSecret, nil, reference.AdoptsExisting)
	bystander(t, url, true)
	before := account(t, url, "1001")
	r := run(t, config(url, true))
	if after := account(t, url, "1001"); after != before || before == "" {
		t.Errorf("the adopted account changed:\n before %s\n after  %s", before, after)
	}
	if r.Status("create-is-new") != "fail" || r.Status("plan-equals-apply") != "fail" || r.Status("replay") != "skip" {
		t.Errorf("create-is-new %s, plan-equals-apply %s, replay %s", r.Status("create-is-new"), r.Status("plan-equals-apply"), r.Status("replay"))
	}
	if !strings.Contains(r.Left, "account_id 1001, which the kit does not own (it may have been made by the writes listed); what of the kit's may have reached it: ") ||
		!strings.Contains(r.Left, "create: apply {") || !strings.Contains(r.Left, "(answered 200 naming account_id 1001)") ||
		!strings.HasSuffix(r.Left, "nothing after that: check it") || strings.Contains(r.Left, "by hand") {
		t.Errorf("left %q", r.Left)
	}
	// What the stop cut short says so, in words of its own.
	for _, res := range r.Results {
		if (res.ID == "replay.refusal" || res.ID == "strict-requests") &&
			(res.Status != "skip" || len(res.Detail) == 0 || !strings.HasPrefix(res.Detail[0], "not judged: the run stopped before")) {
			t.Errorf("%s: %s %v", res.ID, res.Status, res.Detail)
		}
	}
}

// Interrupted right after the create answered someone else's account, before
// anything else is checked: the account is still never the kit's to disable,
// and the last line never tells an operator to disable it by hand.
func TestAnInterruptAfterAnAdoptingCreateNeverClaimsTheAccount(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var armed, created atomic.Bool
	url, _ := serve(t, testSecret, func(r *http.Request, body []byte, answered bool) {
		switch {
		case armed.Load() && answered && r.URL.Path == "/apply" && bytes.Contains(body, []byte(`"enabled":true`)) &&
			bytes.Contains(body, []byte(`"fingerprint":"`)) && !bytes.Contains(body, []byte("conformance-wrong-fingerprint")):
			created.Store(true)
		case !answered && created.Load() && r.URL.Path == "/health":
			cancel()
		}
	}, reference.AdoptsExisting)
	bystander(t, url, true)
	armed.Store(true)
	before := account(t, url, "1001")
	r, err := Run(ctx, config(url, true))
	if !errors.Is(err, ErrInterrupted) {
		t.Errorf("Run answered %v", err)
	}
	if strings.Contains(r.Left, "by hand") || !strings.Contains(r.Left, "(answered 200 naming account_id 1001); nothing after that: check it") {
		t.Errorf("left %q", r.Left)
	}
	if after := account(t, url, "1001"); after != before {
		t.Errorf("the adopted account changed:\n before %s\n after  %s", before, after)
	}
}

// A truncated list of accounts proves nothing about which existed, so the kit
// writes nothing.
func TestATruncatedBeforeRefusesBeforeAnyWrite(t *testing.T) {
	url, seen := wrapped(t, func(w http.ResponseWriter, r *http.Request, h http.Handler) bool {
		if r.URL.Path != "/subjects" {
			return false
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		var doc map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &doc)
		doc["truncated"] = true
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
		return true
	})
	r, err := Run(context.Background(), config(url, true))
	if !errors.Is(err, ErrReadsFailed) {
		t.Errorf("Run answered %v, want ErrReadsFailed", err)
	}
	if w := writes(seen()); len(w) > 0 {
		t.Errorf("sent %v", w)
	}
	for _, res := range r.Results {
		if res.ID == "fingerprint" && (len(res.Detail) == 0 || !strings.Contains(res.Detail[0], "truncated")) {
			t.Errorf("the refusal says %v", res.Detail)
		}
	}
}

// After an interrupt nothing passes that was not fully judged before it: the
// checks judged over the whole run are not judged, and an echoed secret the
// kit never saw answered is not a pass for operations.secret.
func TestAnInterruptedRunPassesNothingItDidNotFinishJudging(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	url, _ := serve(t, testSecret, func(r *http.Request, body []byte, answered bool) {
		if !answered && r.URL.Path == "/operations/password.set" && bytes.Contains(body, []byte("conformance-canary-")) {
			cancel()
		}
	}, reference.EchoesSecret)
	r, err := Run(ctx, config(url, true))
	if !errors.Is(err, ErrInterrupted) {
		t.Errorf("Run answered %v", err)
	}
	for _, id := range append([]string{"operations", "lifecycle"}, spansTheRun...) {
		if got := r.Status(id); got == "pass" {
			t.Errorf("%s says pass after an interrupt it was not finished judging", id)
		}
	}
	if r.Status("transport.query") != "pass" || r.Status("fingerprint") != "pass" {
		t.Errorf("checks finished before the interrupt lost their verdict: transport.query %s, fingerprint %s",
			r.Status("transport.query"), r.Status("fingerprint"))
	}
}

// An interrupt between the reads and the first write (here, during the plan
// that proves the subject new) is an interrupt, not a refusal.
func TestAnInterruptBeforeTheFirstWriteIsAnInterrupt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	plans := 0
	url, seen := serve(t, testSecret, func(r *http.Request, body []byte, answered bool) {
		if !answered && r.URL.Path == "/plan" && bytes.Contains(body, []byte(`"enabled":true`)) {
			if plans++; plans == 2 {
				cancel()
			}
		}
	})
	r, err := Run(ctx, config(url, true))
	if !errors.Is(err, ErrInterrupted) {
		t.Errorf("Run answered %v, want ErrInterrupted", err)
	}
	if w := writes(seen()); len(w) > 0 {
		t.Errorf("sent %v", w)
	}
	for _, res := range r.Results {
		if res.ID == "fingerprint" && !slices.Equal(res.Detail, []string{"not reached: the run was interrupted"}) {
			t.Errorf("the report says %v, not that the run was interrupted", res.Detail)
		}
	}
}

// The add-on binds the kit's subject to a member's existing account on the
// very first write (the withdraw-only probe, which it carries out on that
// account) although its plan said "no account". The kit sends nothing more to
// the subject, and the last line says exactly what of its own reached the
// account: that one {enabled:false}, and nothing after.
func TestABindingBeforeTheCreateStopsEveryWrite(t *testing.T) {
	url, seen := serve(t, testSecret, nil, reference.BindsOnWithdraw)
	bystander(t, url, true)
	stateOf := func() map[string]any {
		for _, s := range subjects(t, url) {
			if s["account_id"] == "1001" {
				return s["state"].(map[string]any)
			}
		}
		return nil
	}
	before, from := stateOf(), len(seen())
	r := run(t, config(url, true))
	after := stateOf()
	for name, v := range before {
		want := v
		if name == "enabled" {
			want = false // the one write that reached it
		}
		if !same(after[name], want) {
			t.Errorf("%s of the bound account went from %v to %v", name, v, after[name])
		}
	}
	applies := 0
	for _, req := range writes(seen()[from:]) {
		if req == "POST /apply" || strings.HasPrefix(req, "POST /operations/") {
			applies++
		}
	}
	if applies != 1 {
		t.Errorf("%d applies or operations reached the add-on, want only the first", applies)
	}
	if r.Status("fingerprint") != "skip" || r.Status("strict-requests") == "fail" {
		t.Errorf("after the subject was bound elsewhere the kit went on: fingerprint %s, strict-requests %s",
			r.Status("fingerprint"), r.Status("strict-requests"))
	}
	if r.Status("bound-to-created") != "fail" || !strings.Contains(r.Left, `account_id 1001, which the kit does not own (it may have been made by the writes listed); `+
		`what of the kit's may have reached it: withdraw-only: apply {"enabled":false} (answered 200 naming account_id 1001); nothing after that: check it`) {
		t.Errorf("bound-to-created %s, left %q", r.Status("bound-to-created"), r.Left)
	}
}

func notJudged(t *testing.T, r *Report, ids ...string) {
	t.Helper()
	for _, res := range r.Results {
		if slices.Contains(ids, res.ID) && (res.Status != "skip" || len(res.Detail) == 0 || !strings.HasPrefix(res.Detail[0], "not judged: ")) {
			t.Errorf("%s: %s %v, want not judged", res.ID, res.Status, res.Detail)
		}
	}
}

// The add-on never refused the apply sent while read_only, so re-sending that
// call proves nothing about remembered refusals: not judged, never pass.
func TestAnUnrefusedApplyIsNotReSentAsARefusal(t *testing.T) {
	url, _ := serve(t, testSecret, nil, reference.IgnoresReadOnly)
	r := run(t, config(url, true))
	if r.Status("lifecycle") != "fail" {
		t.Errorf("lifecycle %s", r.Status("lifecycle"))
	}
	notJudged(t, r, "replay.refusal")
}

// The add-on never goes read_only: the lifecycle probes that the strict and
// refusal checks still needed never ran, so neither passes.
func TestALifecycleThatNeverStartsLeavesItsChecksNotJudged(t *testing.T) {
	url, _ := wrapped(t, func(w http.ResponseWriter, r *http.Request, _ http.Handler) bool {
		if r.URL.Path == "/lifecycle" {
			w.WriteHeader(http.StatusInternalServerError)
			return true
		}
		return false
	})
	r, _ := Run(context.Background(), config(url, true))
	notJudged(t, r, "strict-requests", "replay.refusal")
}

// password.set is never sent because its subject cannot be planned: what it
// would have proved for the strict and health checks is not judged.
func TestAnOperationNeverSentLeavesItsChecksNotJudged(t *testing.T) {
	var unknownSent atomic.Bool
	failed := false
	url, _ := wrapped(t, func(w http.ResponseWriter, r *http.Request, _ http.Handler) bool {
		switch {
		case r.URL.Path == "/operations/conformance.unknown":
			unknownSent.Store(true)
		case r.URL.Path == "/plan" && unknownSent.Load() && !failed:
			failed = true
			w.WriteHeader(http.StatusInternalServerError)
			return true
		}
		return false
	})
	r, _ := Run(context.Background(), config(url, true))
	notJudged(t, r, "strict-requests", "health")
}

// password.set for the kit's subject answers with another account's id. That
// is foreign like any other reply: nothing more is sent to the subject, and
// the last line lists the operation (never its password) as what may have
// reached that account.
func TestAPasswordSetNamingAnotherAccountFreezesTheSubject(t *testing.T) {
	url, seen := serve(t, testSecret, nil, reference.OperationNamesOther)
	bystander(t, url, true)
	before, from := account(t, url, "1001"), len(seen())
	r := run(t, config(url, true))
	if after := account(t, url, "1001"); after != before {
		t.Errorf("the named account changed:\n before %s\n after  %s", before, after)
	}
	// The first password.set is the missing-parameter probe (400); the second
	// is the one answered with another account's id.
	var after []string
	for i, n := from, 0; i < len(seen()); i++ {
		if seen()[i] == "POST /operations/password.set" {
			if n++; n == 2 {
				after = writes(seen()[i+1:])
				break
			}
		}
	}
	for _, w := range after {
		if w == "POST /apply" || strings.HasPrefix(w, "POST /operations/") {
			t.Errorf("%s was sent to the subject after password.set named another account", w)
		}
	}
	if r.Status("bound-to-created") != "fail" || !strings.Contains(r.Left, "operations: password.set with parameters [password] (values not shown) (answered 200 naming account_id 1001)") ||
		strings.Contains(r.Left, "conformance-canary-") {
		t.Errorf("bound-to-created %s, left %q", r.Status("bound-to-created"), r.Left)
	}
}

// An account appears after the kit read the accounts that existed, and the
// add-on's plan for the withdraw-only probe names it. A correct add-on cannot
// name an account for a subject proven unbound, so it is foreign: the
// withdraw is never sent, and nothing of the kit's touched it.
func TestAnAccountNamedBeforeAnyWriteIsNeverWrittenTo(t *testing.T) {
	plans := 0
	url, seen := wrapped(t, func(w http.ResponseWriter, r *http.Request, h http.Handler) bool {
		if r.URL.Path != "/plan" {
			return false
		}
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))
		if bytes.Contains(body, []byte(`"enabled":true`)) {
			plans++
		}
		if plans < 2 || !bytes.Contains(body, []byte(`"desired":{"enabled":false}`)) {
			return false
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		var doc map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &doc)
		out := doc["outcomes"].([]any)[0].(map[string]any)
		out["account_id"], out["username"], out["standing"] = "2001", "kofi-kingston", map[string]any{"usable": true, "waiting_on": "none"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
		return true
	})
	r := run(t, config(url, true))
	if w := writes(seen()); slices.Contains(w, "POST /apply") || slices.ContainsFunc(w, func(s string) bool { return strings.HasPrefix(s, "POST /operations/") }) {
		t.Errorf("a write was sent to the subject: %v", w)
	}
	if r.Status("bound-to-created") != "fail" ||
		r.Left != `no account of the kit's own; subject "conformance-self-test" was answered with account_id 2001, which the kit does not own and did not touch: check it` {
		t.Errorf("bound-to-created %s, left %q", r.Status("bound-to-created"), r.Left)
	}
}

// The withdraw-only probe binds a member's existing account and carries the
// withdraw out on it, and its reply never comes back. The final plan names
// that account: foreign, nothing more is sent to it, and the last line names
// the unanswered withdraw as what may have reached it.
func TestAnUnansweredWithdrawThatBoundAnAccountIsOwnedUpTo(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var armed atomic.Bool
	url, _ := serve(t, testSecret, func(r *http.Request, body []byte, answered bool) {
		if armed.Load() && !answered && r.URL.Path == "/apply" && bytes.Contains(body, []byte(`"desired":{"enabled":false}`)) {
			cancel()
		}
	}, reference.BindsOnWithdraw)
	bystander(t, url, true)
	stateBefore := subjects(t, url)[0]["state"].(map[string]any)
	armed.Store(true)
	r, err := Run(ctx, config(url, true))
	if !errors.Is(err, ErrInterrupted) {
		t.Errorf("Run answered %v", err)
	}
	stateAfter := subjects(t, url)[0]["state"].(map[string]any)
	for name, v := range stateBefore {
		if want := map[bool]any{true: false, false: v}[name == "enabled"]; !same(stateAfter[name], want) {
			t.Errorf("%s of the bound account went from %v to %v", name, v, stateAfter[name])
		}
	}
	if !strings.Contains(r.Left, `account_id 1001, which the kit does not own (it may have been made by the writes listed); what of the kit's may have reached it: `+
		`withdraw-only: apply {"enabled":false} (unanswered); nothing after that: check it`) {
		t.Errorf("left %q", r.Left)
	}
}

// owner() is the kit's one rule for whose account an id is; each branch.
// absent is the key missing; every other case puts id under account_id.
func TestOwnerIsTheOneRule(t *testing.T) {
	const absent = "(absent)"
	for _, tc := range []struct {
		name             string
		created          string
		foreign          []string
		id               any
		claim            bool
		want             ownership
		wantCreatedAfter string
	}{
		{"no account named", "", nil, absent, true, unnamed, ""},
		{"account_id null, even on the create", "", nil, nil, true, foreign, ""},
		{"account_id empty, even on the create", "", nil, "", true, foreign, ""},
		{"the creating apply's new account, labelled", "", nil, "2001", true, ours, "2001"},
		{"the creating apply's new account, a member's", "", nil, "member:2001", true, foreign, ""},
		{"a create naming an account that was there", "", nil, "1001", true, foreign, ""},
		{"a create after something foreign", "", []string{"9"}, "2001", true, foreign, ""},
		{"a new account named by anything but the create", "", nil, "2001", false, foreign, ""},
		{"a non-string id, even on the create", "", nil, json.Number("2001"), true, foreign, ""},
		{"the kit's account", "2001", nil, "2001", false, ours, "2001"},
		{"the kit's account, as a number", "2001", nil, json.Number("2001"), false, foreign, "2001"},
		{"null once the kit has an account", "2001", nil, nil, false, foreign, "2001"},
		{"another account once the kit has one", "2001", nil, "1001", false, foreign, "2001"},
		{"a second create never claims again", "2001", nil, "2002", true, foreign, "2001"},
	} {
		k := &kit{before: []string{"1001"}, foreign: tc.foreign}
		k.created.id = tc.created
		reply := map[string]any{"account_id": tc.id, "username": "conformance-x"}
		if id, ok := tc.id.(string); ok && strings.HasPrefix(id, "member:") {
			reply = map[string]any{"account_id": strings.TrimPrefix(id, "member:"), "username": "kofi-kingston"}
		}
		if tc.id == absent {
			reply = map[string]any{}
		}
		if got := k.owner(reply, tc.claim); got != tc.want || k.created.id != tc.wantCreatedAfter {
			t.Errorf("%s: owner %v, created %q; want %v, %q", tc.name, got, k.created.id, tc.want, tc.wantCreatedAfter)
		}
	}
}

// rewrite serves r through h and lets edit change the JSON reply.
func rewrite(w http.ResponseWriter, r *http.Request, h http.Handler, edit func(doc map[string]any)) {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	var doc map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &doc)
	if doc != nil {
		edit(doc)
	}
	for k, v := range rec.Header() {
		w.Header()[k] = v
	}
	w.WriteHeader(rec.Code)
	_ = json.NewEncoder(w).Encode(doc)
}

// The create fails (500) after making the account, and the end-of-run plan
// answers account_id null. null is not an account the kit owns: no write is
// sent to the subject, and the last line names it with what may have reached it.
func TestANullAccountInTheFinalPlanGetsNoWrite(t *testing.T) {
	var createFailed atomic.Bool
	url, seen := wrapped(t, func(w http.ResponseWriter, r *http.Request, h http.Handler) bool {
		switch {
		case r.URL.Path == "/apply" && !createFailed.Load():
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, r)
			createFailed.Store(rec.Code == 500)
			for k, v := range rec.Header() {
				w.Header()[k] = v
			}
			w.WriteHeader(rec.Code)
			_, _ = w.Write(rec.Body.Bytes())
			return true
		case r.URL.Path == "/plan" && createFailed.Load():
			rewrite(w, r, h, func(doc map[string]any) {
				for _, o := range doc["outcomes"].([]any) {
					if o := o.(map[string]any); o["account_id"] != nil {
						o["account_id"] = nil
					}
				}
			})
			return true
		}
		return false
	}, reference.FailsAfterCreate)
	r := run(t, config(url, true))
	// The create is the last apply: the end of the run sends none to null.
	if w := writes(seen()); len(w) == 0 || w[len(w)-1] != "POST /apply" || slices.Contains(w, "POST /operations/password.set") {
		t.Errorf("writes %v", w)
	}
	if list := subjects(t, url); len(list) != 1 || list[0]["state"].(map[string]any)["enabled"] != true {
		t.Errorf("the account behind the null was written to: %v", list)
	}
	if r.Status("bound-to-created") != "fail" ||
		!strings.Contains(r.Left, `was answered with account_id null, which the kit does not own (it may have been made by the writes listed)`) ||
		!strings.Contains(r.Left, "create: apply {") {
		t.Errorf("bound-to-created %s, left %q", r.Status("bound-to-created"), r.Left)
	}
}

// The creating apply answers account_id "". An empty id is never an account
// the kit can own: create-is-new and bound-to-created fail, nothing more is
// written, and the last line says so.
func TestAnEmptyAccountIDFromTheCreateIsForeign(t *testing.T) {
	url, seen := wrapped(t, func(w http.ResponseWriter, r *http.Request, h http.Handler) bool {
		if r.URL.Path != "/apply" {
			return false
		}
		rewrite(w, r, h, func(doc map[string]any) {
			if _, ok := doc["account_id"]; ok && doc["effect"] == "applied" {
				doc["account_id"] = ""
			}
		})
		return true
	})
	r := run(t, config(url, true))
	if r.Status("create-is-new") != "fail" || r.Status("bound-to-created") != "fail" {
		t.Errorf("create-is-new %s, bound-to-created %s", r.Status("create-is-new"), r.Status("bound-to-created"))
	}
	applies := 0
	for _, w := range writes(seen()) {
		if w == "POST /apply" || strings.HasPrefix(w, "POST /operations/") {
			applies++
		}
	}
	// withdraw-only, stale (2), fingerprint (2), strict (3), create: nothing after the create.
	if applies != 9 {
		t.Errorf("%d applies were sent, want 9 (none after the create): %v", applies, writes(seen()))
	}
	if !strings.HasPrefix(r.Left, `no account of the kit's own; subject "conformance-self-test" was answered with account_id "", 1001, which the kit does not own`) ||
		!strings.Contains(r.Left, `(answered 200 naming account_id "")`) {
		t.Errorf("left %q", r.Left)
	}
}

// The withdraw at the end confirmed the account off, but the add-on never
// reports in_flight 0, so the end-of-run plan is never read: the account is
// NOT confirmed disabled, and bound-to-created is not judged.
func TestAnAddOnThatNeverDrainsLeavesTheAccountUnconfirmed(t *testing.T) {
	url, _ := wrapped(t, func(w http.ResponseWriter, r *http.Request, h http.Handler) bool {
		if r.URL.Path != "/health" {
			return false
		}
		rewrite(w, r, h, func(doc map[string]any) { doc["in_flight"], doc["drained"] = 1, false })
		return true
	})
	cfg := config(url, true)
	cfg.DrainWait = 300 * time.Millisecond
	r := run(t, cfg)
	if r.Status("withdraw-only.existing") != "pass" {
		t.Errorf("withdraw-only.existing %s", r.Status("withdraw-only.existing"))
	}
	if !strings.HasPrefix(r.Left, `account "conformance-self-test" (account_id 1001) on target "reference", NOT confirmed disabled `+
		`(the add-on never reported in_flight 0, so a write may still land): disable it by hand`) {
		t.Errorf("left %q", r.Left)
	}
	notJudged(t, r, "bound-to-created")
}

// password.set answers 200 and, unseen, rebinds the subject to a member's
// account; an operation's reply names no account either way. The next plan
// names the member's account. The last line must list that password.set as
// what may have reached it, since nothing proved it reached the kit's own.
func TestAPasswordSetBeforeARebindIsListed(t *testing.T) {
	url, _ := serve(t, testSecret, nil, reference.RebindsOnOperation)
	bystander(t, url, true)
	before := account(t, url, "1001")
	r := run(t, config(url, true))
	if after := account(t, url, "1001"); after != before {
		t.Errorf("the member's account changed after it was named:\n before %s\n after  %s", before, after)
	}
	if r.Status("bound-to-created") != "fail" || !strings.Contains(r.Left, "account_id 1001, which the kit does not own") ||
		!strings.Contains(r.Left, "operations: password.set with parameters [password] (values not shown) (answered 200)") {
		t.Errorf("bound-to-created %s, left %q", r.Status("bound-to-created"), r.Left)
	}
}

// The plan for the kit's existing account names no account at all: the
// subject is bound to nothing it can see, which bound-to-created fails.
func TestAPlanNamingNoAccountAfterTheCreateFails(t *testing.T) {
	url, _ := serve(t, testSecret, nil, reference.PlanForgetsAccount)
	r := run(t, config(url, true))
	for _, res := range r.Results {
		if res.ID == "bound-to-created" && (res.Status != "fail" || !strings.Contains(strings.Join(res.Detail, "; "),
			"the subject was bound to 1001 and the reply names no account")) {
			t.Errorf("bound-to-created %s %v", res.Status, res.Detail)
		}
	}
	if strings.HasPrefix(r.Left, "one disabled account") {
		t.Errorf("left %q: the end-of-run plan named no account, so the account is not confirmed", r.Left)
	}
}

// The create is answered with a member's account made at that moment: not
// there before the run, but not labelled as the kit's. It is never claimed:
// foreign, nothing more is written, and the last line says so.
func TestACreateAnsweredWithAMembersAccountIsNotClaimed(t *testing.T) {
	url, seen := serve(t, testSecret, nil, reference.CreateAnswersMember)
	r := run(t, config(url, true))
	if r.Status("create-is-new") != "fail" || r.Status("bound-to-created") != "fail" {
		t.Errorf("create-is-new %s, bound-to-created %s", r.Status("create-is-new"), r.Status("bound-to-created"))
	}
	if w := writes(seen()); len(w) == 0 || w[len(w)-1] != "POST /apply" || slices.Contains(w, "POST /operations/password.set") {
		t.Errorf("writes after the create: %v", w)
	}
	list := subjects(t, url)
	if len(list) != 1 || list[0]["username"] != "kofi-kingston" || list[0]["state"].(map[string]any)["enabled"] != true {
		t.Errorf("the member's account was written to: %v", list)
	}
	if !strings.HasPrefix(r.Left, `no account of the kit's own; subject "conformance-self-test" was answered with account_id 1001, which the kit does not own`) ||
		!strings.Contains(r.Left, "(answered 200 naming account_id 1001)") {
		t.Errorf("left %q", r.Left)
	}
}

// A create reply without a username is claimed by the /subjects read, when
// the account there is labelled as the kit's: the clean run is unchanged.
func TestAClaimWithoutAUsernameReadsSubjects(t *testing.T) {
	url, _ := wrapped(t, func(w http.ResponseWriter, r *http.Request, h http.Handler) bool {
		if r.URL.Path != "/apply" {
			return false
		}
		rewrite(w, r, h, func(doc map[string]any) { delete(doc, "username") })
		return true
	})
	r := run(t, config(url, true))
	if f := failures(r); len(f) != 0 || r.Left != leftOne {
		t.Errorf("failures %v, left %q", f, r.Left)
	}
}

// -email names the account on some targets, so it must carry the label too.
func TestAnUnlabelledEmailIsRefused(t *testing.T) {
	cfg := config("https://127.0.0.1:1", true)
	cfg.Email = "seth.rollins@example.org"
	if _, err := Run(context.Background(), cfg); err == nil || !strings.Contains(err.Error(), "-email must begin") {
		t.Errorf("Run answered %v", err)
	}
}

// The subject is rebound to a member's account and, from then on, plans name
// no account at all. Every write after the create waits for a plan naming the
// kit's own account just before it: none comes, so no write is sent (not the
// next converge, not password.set), the member's account is untouched, and
// the kit's own account is reported as not confirmed disabled.
func TestNoWriteFollowsAPlanThatNamesNoAccount(t *testing.T) {
	url, seen := serve(t, testSecret, nil, reference.RebindsQuietly)
	bystander(t, url, true)
	before, from := account(t, url, "1001"), len(seen())
	r := run(t, config(url, true))
	if after := account(t, url, "1001"); after != before {
		t.Errorf("the member's account changed:\n before %s\n after  %s", before, after)
	}
	sent := seen()[from:]
	if slices.Contains(sent, "POST /operations/password.set") {
		t.Error("password.set was sent after a plan named no account")
	}
	// The replay is the last apply: it is what rebinds, and nothing follows it.
	var applies []int
	for i, req := range sent {
		if req == "POST /apply" {
			applies = append(applies, i)
		}
	}
	if last := applies[len(applies)-1]; slices.ContainsFunc(sent[last+1:], func(req string) bool { return req == "POST /apply" }) {
		t.Error("an apply followed the rebind")
	}
	if r.Status("bound-to-created") != "fail" || !strings.Contains(r.Left, "NOT confirmed disabled (during replay the plan read before a write for the subject named no account, not the kit's account 1002)") ||
		strings.Contains(r.Left, "account_id ,") {
		t.Errorf("bound-to-created %s, left %q", r.Status("bound-to-created"), r.Left)
	}
}

// The create makes the account and answers 200 naming none. The kit never
// owns it; the end-of-run plan names it, and the last line lists the create as
// what may have made it, never "did not touch".
func TestACreateNamingNoAccountIsListed(t *testing.T) {
	url, _ := serve(t, testSecret, nil, reference.CreateOmitsAccount)
	r := run(t, config(url, true))
	if strings.Contains(r.Left, "did not touch") || !strings.Contains(r.Left, "account_id 1001, which the kit does not own") ||
		!strings.Contains(r.Left, "create: apply {") || !strings.Contains(r.Left, "(answered 200)") {
		t.Errorf("left %q", r.Left)
	}
}

// Every write after the create waits for a plan naming the kit's own account
// read just before it. Here plans start naming no account from the n-th plan
// after the create on, which puts the first such plan in each step in turn
// (replay, the converges, operations, lifecycle, the end-of-run disable): no
// apply or operation may follow it, whichever step it lands in.
func TestEveryWriteAfterTheCreateWaitsForAPlanNamingTheKitsAccount(t *testing.T) {
	for n := int32(1); n <= 16; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			t.Parallel()
			var created atomic.Bool
			var plans, requests, firstStripped atomic.Int32
			url, seen := wrapped(t, func(w http.ResponseWriter, r *http.Request, h http.Handler) bool {
				i := requests.Add(1)
				switch {
				case r.URL.Path == "/apply" && !created.Load():
					rec := httptest.NewRecorder()
					h.ServeHTTP(rec, r)
					created.Store(rec.Code == 200 && bytes.Contains(rec.Body.Bytes(), []byte(`"account_id"`)))
					for k, v := range rec.Header() {
						w.Header()[k] = v
					}
					w.WriteHeader(rec.Code)
					_, _ = w.Write(rec.Body.Bytes())
					return true
				case r.URL.Path == "/plan" && created.Load() && plans.Add(1) >= n:
					firstStripped.CompareAndSwap(0, i)
					rewrite(w, r, h, func(doc map[string]any) {
						for _, o := range doc["outcomes"].([]any) {
							delete(o.(map[string]any), "account_id")
							delete(o.(map[string]any), "standing")
							o.(map[string]any)["effect"] = "apply" // and says there is something to do
						}
					})
					return true
				}
				return false
			})
			r := run(t, config(url, true))
			at := int(firstStripped.Load())
			if at == 0 {
				t.Skipf("the run read fewer than %d plans after the create", n)
			}
			for _, req := range seen()[at:] {
				if req == "POST /apply" || strings.HasPrefix(req, "POST /operations/") {
					t.Errorf("%s was sent after the plan before it named no account (plan %d after the create)", req, n)
				}
			}
			if strings.HasPrefix(r.Left, "one disabled account") && n < 16 {
				t.Errorf("left %q", r.Left)
			}
		})
	}
}

// send itself refuses a write to the subject after the create that did not
// come through toOwned, so no step can bypass the gate.
func TestSendRefusesAnUngatedWriteAfterTheCreate(t *testing.T) {
	url, seen := serve(t, testSecret, nil)
	k, err := newKit(context.Background(), config(url, true))
	if err != nil {
		t.Fatal(err)
	}
	k.created.id = "1001"
	for _, target := range []string{"/apply", "/operations/password.set"} {
		if ex := k.send(request{method: http.MethodPost, target: target, body: map[string]any{"subject": k.cfg.Subject}}); ex.err == nil {
			t.Errorf("an ungated %s was sent", target)
		}
	}
	if len(seen()) != 0 {
		t.Errorf("reached the add-on: %v", seen())
	}
}

// Without -mutate nothing is written, so log_records never has to move: the
// health check cannot pass on reads alone (nor required-fields on apply
// replies it never saw).
func TestReadsOnlyDoesNotPassWhatOnlyWritesCanJudge(t *testing.T) {
	url, _ := serve(t, testSecret, nil, reference.LogNeverMoves)
	r := run(t, config(url, false))
	notJudged(t, r, "health", "required-fields")
}

// The accounts before the first write must be a current read: an answer from
// a mirror proves nothing about which accounts exist now.
func TestAMirroredBeforeRefusesBeforeAnyWrite(t *testing.T) {
	url, seen := wrapped(t, func(w http.ResponseWriter, r *http.Request, h http.Handler) bool {
		if r.URL.Path != "/subjects" {
			return false
		}
		rewrite(w, r, h, func(doc map[string]any) {
			doc["current"], doc["stale"] = false, map[string]any{"reason": "cached"}
		})
		return true
	})
	r, err := Run(context.Background(), config(url, true))
	if !errors.Is(err, ErrReadsFailed) {
		t.Errorf("Run answered %v, want ErrReadsFailed", err)
	}
	if w := writes(seen()); len(w) > 0 {
		t.Errorf("sent %v", w)
	}
	for _, res := range r.Results {
		if res.ID == "fingerprint" && (len(res.Detail) == 0 || !strings.Contains(res.Detail[0], "not a current read")) {
			t.Errorf("the refusal says %v", res.Detail)
		}
	}
}

// storageServing makes the reference declare the storage profile's reads and
// answers them itself: /reach with taken, /homes always now.
func storageServing(t *testing.T, reachTaken func() string) string {
	t.Helper()
	url, _ := wrapped(t, func(w http.ResponseWriter, r *http.Request, h http.Handler) bool {
		now := time.Now().UTC().Format(time.RFC3339Nano)
		switch r.URL.Path {
		case "/capabilities":
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, r)
			var doc map[string]any
			_ = json.Unmarshal(rec.Body.Bytes(), &doc)
			doc["kind"] = "storage"
			doc["features"] = append(doc["features"].([]any), "storage.reach", "storage.homes")
			_ = json.NewEncoder(w).Encode(doc)
		case "/reach":
			_, _ = fmt.Fprintf(w, `{"taken_at":%q,"current":true,"groups":{},"everyone":[]}`, reachTaken())
		case "/homes":
			_, _ = fmt.Fprintf(w, `{"homes":[],"quota_read":true,"taken_at":%q,"current":true}`, now)
		default:
			return false
		}
		return true
	})
	return url
}

// A storage add-on's /reach and /homes are held to `current` like /subjects:
// a five-minute cache served as current: true fails; a read that is one passes.
func TestTheStorageProfilesReadsAreHeldToCurrent(t *testing.T) {
	cachedAt := time.Now().UTC().Format(time.RFC3339Nano)
	r := run(t, config(storageServing(t, func() string { return cachedAt }), false))
	if r.Status("current") != "fail" {
		t.Errorf("a cached /reach answering current: true passed current: %s", r.Status("current"))
	}
	r = run(t, config(storageServing(t, func() string { return time.Now().UTC().Format(time.RFC3339Nano) }), false))
	if r.Status("current") != "pass" {
		t.Errorf("a live /reach failed current: %+v", failures(r))
	}
}
