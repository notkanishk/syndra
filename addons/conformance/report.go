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
	{id: "date", guide: "observations-taken_at-current-stale", must: "a read (/subjects) carries a Date header from the clock that stamps taken_at, at the moment it answers, and never one more than a second before its taken_at (RFC 9110 section 6.6.1)"},
	{id: "values", guide: "get-valuesfield", must: "every schema field answers /values; account and channel answer false,true; a membership answers enumerable: true; an unknown field is 404 UNKNOWN_FIELD"},
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
	{id: "operations.result", guide: "read-operations-and-result", must: "every read operation answers one result valid against the schema the registry names for it, and under no other key", mutate: true},
	{id: "operations.secret", guide: "8-security-rules", must: "a secret parameter never appears in any later byte of any reply", mutate: true},
	{id: "calls.unknown", guide: "physical-operations-and-get-callscall_id-feature-calls", must: "GET /calls on a call_id never sent answers unknown, with no reply (feature calls)"},
	{id: "calls.recorded", guide: "physical-operations-and-get-callscall_id-feature-calls", must: "GET /calls on a write's call_id answers recorded with its first reply, and never a secret parameter (feature calls)", mutate: true},
	{id: "operations.physical", guide: "physical-operations-and-get-callscall_id-feature-calls", must: "a physical operation is declared only with feature calls and confirm; a preview answers the would-be result and writes nothing; the confirm under a fresh call_id runs once and /calls answers it recorded; the same item again is no_change and writes nothing (feature calls)", mutate: true},
	{id: "calls.unsettled", guide: "physical-operations-and-get-callscall_id-feature-calls", must: "while a physical call is in doubt, every call for that subject is 409 CALL_UNSETTLED and none runs (feature calls)", mutate: true},
	{id: "calls.serialised", guide: "physical-operations-and-get-callscall_id-feature-calls", must: "physical operations are serialised per subject: two at once cannot both pass a limit (feature calls)", mutate: true},
	{id: "freshness.cached", guide: "fresh-reads-feature-freshness", must: "a read without fresh answered from the mirror says stale cached and reads the app not at all; current: true only from a read of the app (feature freshness)"},
	{id: "freshness.cooldown", guide: "fresh-reads-feature-freshness", must: "fresh reads the app once; a second inside the cooldown answers stale cooldown with retry_after, or 429 FRESH_READ_COOLDOWN with Retry-After, and reads nothing; /plan straight after is current, never cooled down (feature freshness)"},
	{id: "freshness.coalesce", guide: "fresh-reads-feature-freshness", must: "two concurrent fresh reads read the app once, and at least one is current (feature freshness)"},
	{id: "freshness.unreachable", guide: "fresh-reads-feature-freshness", must: "with the app unreachable a read answers the last value with stale unreachable, not an error (feature freshness)"},
	{id: "stats.descriptors", guide: "statistics-get-stats-feature-stats", must: "stats_zone is an IANA zone; limits.stats_buffer is at most P400D; metric and dimension names are unique; against names a limit field in the metric's unit (feature stats)"},
	{id: "stats.points", guide: "statistics-get-stats-feature-stats", must: "every /stats page validates and is for the metric asked; every point is one day in stats_zone, start a midnight and end the next; it names only declared dimensions, and a subject only on a per_subject metric (feature stats)"},
	{id: "stats.totals", guide: "statistics-get-stats-feature-stats", must: "every value is at least 0; a day and dimensions with a per-person point has a total; a counter's people sum to no more than it, and each of a gauge's people is no more than it (feature stats)"},
	{id: "stats.since", guide: "statistics-get-stats-feature-stats", must: "no point or gap ends at or before since; the watermark is a midnight in stats_zone, no later than the start of taken_at's day, and never moves back; an undeclared metric is 404 UNKNOWN_METRIC (feature stats)"},
	{id: "stats.idempotent", guide: "statistics-get-stats-feature-stats", must: "one metric read twice from one since answers the same final points and gaps, those ending at or before the watermark (feature stats)"},
	{id: "stats.gaps", guide: "statistics-get-stats-feature-stats", must: "every gap runs from a midnight to a later one, with a reason and only declared dimensions, and no point lies inside a gap that covers it: a window not observed is never a value (feature stats)"},
	{id: "stats.unbound", guide: "statistics-get-stats-feature-stats", must: "a per-person point never names a target account by its id or name; a counter total above its people's sum comes with unbound.accounts (feature stats)"},
	{id: "stats.buffer", guide: "statistics-get-stats-feature-stats", must: "a since older than limits.stats_buffer is answered with a gap with a reason covering that day, and the points still held (feature stats)"},
	{id: "stats.page-size", guide: "statistics-get-stats-feature-stats", must: "every page fits constants.max_reply_bytes; points come in (start, subject, dimensions) order across pages, none twice and none skipped, and a cursor read again gives the same page (feature stats)"},
	{id: "stats.no-people", guide: "statistics-get-stats-feature-stats", must: "no dimension value is the username or account_id of the account the kit created: dimensions name resources, never people (feature stats)"},
	{id: "stats.no-people.ids", guide: "statistics-get-stats-feature-stats", must: "no dimension value is the username or account_id of another account /subjects lists (advisory: a service account may share a resource's name, a scanner share and a scanner user) (feature stats)", advisory: true},
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
