package conformance

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// The checks, each with the MUST it enforces and the guide section that
// states it (AUTHORING.md, which the author has; TestEveryCheckCitesAGuideSection
// holds each anchor to a heading). A check the kit cannot run says skip and
// why; it never passes by default. mutate marks a check that sends writes and
// so needs -mutate.
var catalogue = []struct {
	id, must, guide  string
	mutate, advisory bool
}{
	{id: "transport.vectors", guide: "transport", must: "the kit's own derivation and signer reproduce every vector in transport_derivation.json"},
	{id: "transport.pin", guide: "transport", must: "the add-on serves the TLS key derived from the per-target secret (TLS 1.3, Ed25519, pinned)"},
	{id: "transport.refusals", guide: "transport", must: "unsigned, stale and wrong-key requests are 401 UNAUTHENTICATED, alike, never saying which half failed"},
	{id: "transport.query", guide: "transport", must: "the raw query is inside the MAC as sent; a query altered, reordered or added after signing is 401"},
	{id: "manifest", guide: "3-the-manifest-get-capabilities", must: "the manifest validates; major 2; exactly one account field; a closed kind; registered operation ids; features fit the kind"},
	{id: "health", guide: "get-health", must: "health validates; log_head never empty; across every read, log_records never falls, the same count keeps the same head and a higher count moves it; log_records advancing on a write; target_reads when freshness is declared"},
	{id: "subjects", guide: "get-subjects", must: "subjects validates; every state key is a schema field in its role's wire shape; current false iff stale"},
	{id: "subjects.paged", guide: "get-subjects", must: "snapshot cursor paging (feature subjects.paged)"},
	{id: "unlisted-hidden", guide: "get-subjects", must: "a membership in state or observed holds only the managed set: no value /values leaves out"},
	{id: "current", guide: "observations-taken_at-current-stale", must: "current: true means a read of the app that began after the request arrived, never a mirror (subjects, plan, and the storage profile's reach and homes when declared)"},
	{id: "values", guide: "get-valuesfield", must: "every schema field answers /values; account and channel answer false,true; an unknown field is 404 UNKNOWN_FIELD"},
	{id: "settings", guide: "settings-get-settings-and-put-settingsname-feature-settings", must: "GET /settings validates; with -mutate-settings, writing the current value back is accepted"},
	{id: "plan", guide: "post-plan", must: "plan validates; one outcome with a fingerprint per subject; above limits.subjects_per_plan 422 PLAN_TOO_LARGE with the limit"},
	{id: "strict-requests", guide: "every-request-and-every-reply", must: "an unknown request field is 400 UNKNOWN_REQUEST_FIELD; a wrong or absent contract_version 422 CONTRACT_VERSION_MISMATCH with both numbers"},
	{id: "error-envelope", guide: "6-errors", must: "every non-2xx is application/problem+json valid against error.json, with a registered code and the registry's status, title, retry and Retry-After"},
	{id: "required-fields", guide: "every-request-and-every-reply", must: "absence is never a value: taken_at, current, truncated where the schema requires them; effect on plan and apply; standing.usable wherever an account exists"},
	{id: "fingerprint", guide: "post-apply", must: "an apply without a fingerprint is 400 FINGERPRINT_REQUIRED; with a wrong one 409 PLAN_STALE, and nothing is written", mutate: true},
	{id: "plan-equals-apply", guide: "5-plan-and-apply-agree", must: "the plan's effect and consequence for a subject are what the apply then does", mutate: true},
	{id: "create-is-new", guide: "the-one-create", must: "an apply that creates never binds an account that was already there (adopting one is account.adopt's, confirmed)", mutate: true},
	{id: "bound-to-created", guide: "the-one-create", must: "a subject proven unbound names no account until its create answers one; then it names that account and no other; a binding is authoritative", mutate: true},
	{id: "replay", guide: "every-write-once-per-call_id", must: "rules 1 and 3: same call_id and identity: the byte-identical recorded reply, before the fingerprint check, nothing written; other identity: 422 CALL_ID_REUSED", mutate: true},
	{id: "replay.refusal", guide: "every-write-once-per-call_id", must: "rule 2: only a 2xx is recorded; a call refused with PLAN_STALE, or while read_only, is executed when re-sent", mutate: true},
	{id: "replay.29d", guide: "every-write-once-per-call_id", must: "rule 5: a call is remembered for constants.call_memory (P30D)", mutate: true},
	{id: "read-back", guide: "post-apply", must: "an apply's observed equals the subject's state on the next /subjects", mutate: true},
	{id: "withdraw-only.no-account", guide: "what-desired-means", must: "a withdraw-only apply for a subject with no account is no_change and creates nothing", mutate: true},
	{id: "withdraw-only.existing", guide: "what-desired-means", must: "a withdraw-only apply turns the account off and leaves every membership, flag and limit as it was", mutate: true},
	{id: "channel-follows-account", guide: "what-desired-means", must: "every channel is off whenever the account is off", mutate: true},
	{id: "absent-unmanaged", guide: "what-desired-means", must: "a field absent from desired is not touched", mutate: true},
	{id: "limit-combine", guide: "what-desired-means", must: "a limit combines by max and unlimited dominates: max(unlimited, 5 GiB) is unlimited", mutate: true},
	{id: "limit-empty", guide: "what-desired-means", must: "a limit given [] sets nothing: the account keeps the limit it holds", mutate: true},
	{id: "lifecycle", guide: "9-lifecycle-and-degradation", must: "read_only refuses writes with 503 LIFECYCLE_REFUSAL and Retry-After while reads answer; no reason 400 REASON_REQUIRED; unknown state 422 UNKNOWN_LIFECYCLE_STATE", mutate: true},
	{id: "operations", guide: "post-operationsname-feature-operations", must: "an unknown operation is 404; a missing parameter 400; a re-send with another secret is the recorded reply, nothing written", mutate: true},
	{id: "operations.reads", guide: "every-write-once-per-call_id", must: "an effect: read operation ignores call_id: two reads under one call_id, and a read of another operation under it, all answer 200", mutate: true},
	{id: "operations.secret", guide: "8-security-rules", must: "a secret parameter never appears in any later byte of any reply", mutate: true},
	{id: "operations.physical", guide: "a3-physical-operations-and-get-callscall_id-feature-calls", must: "physical operations: confirm, preview, /calls, CALL_UNSETTLED (feature calls)"},
	{id: "freshness", guide: "a1-fresh-reads-feature-freshness", must: "fresh reads, cached and cooldown answers, single-flight (feature freshness)"},
	{id: "dangerous-scopes", guide: "2-before-you-write-a-line", must: "the credential holds no scope the kind's denylist names (advisory: the list is self-reported)", advisory: true},
}

// Result is one check's verdict.
type Result struct {
	ID     string   `json:"id"`
	Status string   `json:"status"` // pass | fail | skip
	Level  string   `json:"level"`  // MUST | ADVISORY
	Must   string   `json:"must"`
	Detail []string `json:"detail,omitempty"`
}

// Report is a whole run.
type Report struct {
	Kit             string   `json:"kit"`
	ContractVersion int      `json:"contract_version"`
	URL             string   `json:"url"`
	Target          string   `json:"target"`
	Mode            string   `json:"mode"` // reads-only | mutate
	Subject         string   `json:"subject"`
	Results         []Result `json:"results"`
	Pass            int      `json:"pass"`
	Fail            int      `json:"fail"`
	Skip            int      `json:"skip"`
	MustFailures    int      `json:"must_failures"`
	Left            string   `json:"left"`
}

// Failed reports whether any MUST failed: the kit's exit status.
func (r *Report) Failed() bool { return r.MustFailures > 0 }

// Status returns the status of one check, "" when there is no such check.
func (r *Report) Status(id string) string {
	for _, res := range r.Results {
		if res.ID == id {
			return res.Status
		}
	}
	return ""
}

func (r *Report) WriteText(w io.Writer) {
	fmt.Fprintf(w, "syndra-addon-conform: contract %d, %s, target %q, %s, subject %s\n\n", r.ContractVersion, r.URL, r.Target, r.Mode, r.Subject)
	for _, res := range r.Results {
		tag := strings.ToUpper(res.Status)
		if res.Status == "fail" && res.Level == "ADVISORY" {
			tag = "WARN"
		}
		fmt.Fprintf(w, "%-4s  %-25s %s\n", tag, res.ID, res.Must)
		for _, d := range res.Detail {
			fmt.Fprintf(w, "      - %s\n", d)
		}
	}
	fmt.Fprintf(w, "\nsummary: %d pass, %d fail (%d MUST), %d skip\n", r.Pass, r.Fail, r.MustFailures, r.Skip)
	fmt.Fprintf(w, "left: %s\n", r.Left)
}

func (r *Report) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}
