package conformance

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// The checks of design §7, in its table's order. Each names the MUST it
// enforces. A check the kit cannot run says skip and why; it never passes by
// default. mutate marks a check that sends writes and so needs -mutate.
var catalogue = []struct {
	id, must         string
	mutate, advisory bool
}{
	{id: "transport.vectors", must: "§4: the kit's own derivation and signer reproduce every vector in transport_derivation.json"},
	{id: "transport.pin", must: "§7: the add-on serves the TLS key derived from the per-target secret (TLS 1.3, Ed25519, pinned)"},
	{id: "transport.refusals", must: "§7: unsigned, stale and wrong-key requests are 401 UNAUTHENTICATED, alike, never saying which half failed"},
	{id: "transport.query", must: "§4, decision 24: the raw query is inside the MAC as sent; a query altered, reordered or added after signing is 401"},
	{id: "manifest", must: "§2-§4: the manifest validates; major 2; exactly one account field; a closed kind; registered operation ids; features fit the kind"},
	{id: "health", must: "§6: health validates; log_head never empty; across every read, log_records never falls, the same count keeps the same head and a higher count moves it; log_records advancing on a write; target_reads when freshness is declared"},
	{id: "subjects", must: "§4, §6: subjects validates; every state key is a schema field in its role's wire shape; current false iff stale"},
	{id: "subjects.paged", must: "§11: snapshot cursor paging (feature subjects.paged)"},
	{id: "current", must: "§4: current: true means a read of the app that began after the request arrived, never a mirror (subjects, plan, and the storage profile's reach and homes when declared)"},
	{id: "values", must: "§3: every schema field answers /values; account and channel answer false,true; an unknown field is 404 UNKNOWN_FIELD"},
	{id: "settings", must: "feature settings: GET /settings validates; with -mutate-settings, writing the current value back is accepted (not in design §7's table)"},
	{id: "plan", must: "§4, §11: plan validates; one outcome with a fingerprint per subject; above limits.subjects_per_plan 422 PLAN_TOO_LARGE with the limit"},
	{id: "strict-requests", must: "§2: an unknown request field is 400 UNKNOWN_REQUEST_FIELD; a wrong or absent contract_version 422 CONTRACT_VERSION_MISMATCH with both numbers"},
	{id: "error-envelope", must: "§7, decision 25: every non-2xx is application/problem+json valid against error.json, with a registered code and the registry's status, title, retry and Retry-After"},
	{id: "required-fields", must: "§2(b): absence is never a value: taken_at, current, truncated where §4 lists them; effect on plan and apply; standing.usable wherever an account exists"},
	{id: "fingerprint", must: "§5: an apply without a fingerprint is 400 FINGERPRINT_REQUIRED; with a wrong one 409 PLAN_STALE, and nothing is written", mutate: true},
	{id: "plan-equals-apply", must: "§7: the plan's effect and consequence for a subject are what the apply then does", mutate: true},
	{id: "create-is-new", must: "§5: an apply that creates never binds an account that was already there (adopting one is admin.adopt's, confirmed)", mutate: true},
	{id: "bound-to-created", must: "§5: a subject proven unbound names no account until its create answers one; then it names that account and no other; a binding is authoritative", mutate: true},
	{id: "replay", must: "§5 rules 1, 3: same call_id and identity: the byte-identical recorded reply, before the fingerprint check, nothing written; other identity: 422 CALL_ID_REUSED", mutate: true},
	{id: "replay.refusal", must: "§5 rule 2: only a 2xx is recorded; a call refused with PLAN_STALE, or while read_only, is executed when re-sent", mutate: true},
	{id: "replay.29d", must: "§4: a call is remembered for constants.call_memory (P30D)", mutate: true},
	{id: "read-back", must: "§7: an apply's observed equals the subject's state on the next /subjects", mutate: true},
	{id: "withdraw-only.no-account", must: "§3: a withdraw-only apply for a subject with no account is no_change and creates nothing", mutate: true},
	{id: "withdraw-only.existing", must: "§3: a withdraw-only apply turns the account off and leaves every membership, flag and limit as it was", mutate: true},
	{id: "channel-follows-account", must: "§3: every channel is off whenever the account is off", mutate: true},
	{id: "absent-unmanaged", must: "§3: a field absent from desired is not touched", mutate: true},
	{id: "unlisted-preserved", must: "§3, decision 9: a membership value outside the managed set survives an apply that empties the field", mutate: true},
	{id: "limit-combine", must: "§3: a limit combines by max and unlimited dominates: max(unlimited, 5 GiB) is unlimited", mutate: true},
	{id: "limit-empty", must: "§3: a limit given [] sets nothing: the account keeps the limit it holds", mutate: true},
	{id: "lifecycle", must: "§7: read_only refuses writes with 503 LIFECYCLE_REFUSAL and Retry-After while reads answer; no reason 400 REASON_REQUIRED; unknown state 422 UNKNOWN_LIFECYCLE_STATE", mutate: true},
	{id: "operations", must: "§4, §5: an unknown operation is 404; a missing parameter 400; a re-send with another secret is the recorded reply, nothing written", mutate: true},
	{id: "operations.reads", must: "§5: an effect: read operation ignores call_id: two reads under one call_id, and a read of another operation under it, all answer 200", mutate: true},
	{id: "operations.secret", must: "§5, §8: a secret parameter never appears in any later byte of any reply", mutate: true},
	{id: "operations.physical", must: "§4: physical operations: confirm, preview, /calls, CALL_UNSETTLED (feature calls)"},
	{id: "freshness", must: "§4: fresh reads, cached and cooldown answers, single-flight (feature freshness)"},
	{id: "dangerous-scopes", must: "§4: the credential holds no scope the kind's denylist names (advisory: the list is self-reported)", advisory: true},
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
