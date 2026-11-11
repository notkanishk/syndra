package main

import (
	"encoding/json"
	"log"
	"maps"
	"net/http"
	"strings"
)

// The `code`s this add-on writes on the wire. Every non-2xx reply is one
// RFC 9457 problem (application/problem+json, contract 2, decision 25) written
// by writeError, and nothing else writes a refusal.
//
// Named here, once, so the set the add-on can say is a list a test can compare
// against `addons/contract/v2/registry/errors.json`. A code written inline at its
// call site is a code nothing checks: the registry would be a document, not a
// contract. The values are the wire and never change with the names.
const (
	CodeApplyBlocked             = "APPLY_BLOCKED"
	CodeApplyFailed              = "APPLY_FAILED"
	CodeBadRequest               = "BAD_REQUEST"
	CodeBindingConflict          = "BINDING_CONFLICT"
	CodeCallIDReused             = "CALL_ID_REUSED"
	CodeCallInProgress           = "CALL_IN_PROGRESS"
	CodeContractVersionMismatch  = "CONTRACT_VERSION_MISMATCH"
	CodeFingerprintRequired      = "FINGERPRINT_REQUIRED"
	CodeFreshReadCooldown        = "FRESH_READ_COOLDOWN"
	CodeInvalidBody              = "INVALID_BODY"
	CodeLifecycleRefusal         = "LIFECYCLE_REFUSAL"
	CodeNoCurrentRead            = "NO_CURRENT_READ"
	CodeNoSuchSetting            = "NO_SUCH_SETTING"
	CodeNotAHomePlace            = "NOT_A_HOME_PLACE"
	CodeOperationFailed          = "OPERATION_FAILED"
	CodeOperationMismatch        = "OPERATION_MISMATCH"
	CodePlanStale                = "PLAN_STALE"
	CodePlanTooLarge             = "PLAN_TOO_LARGE"
	CodeReachNeverRead           = "REACH_NEVER_READ"
	CodeReasonRequired           = "REASON_REQUIRED"
	CodeSettingNotSaved          = "SETTING_NOT_SAVED"
	CodeSettingsUnreadable       = "SETTINGS_UNREADABLE"
	CodeStoreUnreadable          = "STORE_UNREADABLE"
	CodeTargetUnreachable        = "TARGET_UNREACHABLE"
	CodeTargetUnreadable         = "TARGET_UNREADABLE"
	CodeTargetVersionUnsupported = "TARGET_VERSION_UNSUPPORTED"
	CodeUnauthenticated          = "UNAUTHENTICATED"
	CodeUnknownField             = "UNKNOWN_FIELD"
	CodeUnknownLifecycleState    = "UNKNOWN_LIFECYCLE_STATE"
	CodeUnknownMetric            = "UNKNOWN_METRIC"
	CodeUnknownRequestField      = "UNKNOWN_REQUEST_FIELD"
	CodeUnknownRoute             = "UNKNOWN_ROUTE"
)

// problemKind is a code's registry entry as this add-on needs it: the constant
// title, and the retry class (`by_status` is resolved per reply).
type problemKind struct{ title, retry string }

// problems is registry/errors.json for the codes above, held equal to it by
// TestEveryCodeTheAddOnWritesIsInTheRegistry.
var problems = map[string]problemKind{
	CodeApplyBlocked:             {"Apply blocked", "never"},
	CodeApplyFailed:              {"Apply failed", "by_status"},
	CodeBadRequest:               {"Bad request", "never"},
	CodeBindingConflict:          {"Binding conflict", "never"},
	CodeCallIDReused:             {"Call id reused", "never"},
	CodeCallInProgress:           {"Call in progress", "retry_after"},
	CodeContractVersionMismatch:  {"Contract version mismatch", "never"},
	CodeFingerprintRequired:      {"Fingerprint required", "never"},
	CodeFreshReadCooldown:        {"Fresh read cooldown", "retry_after"},
	CodeInvalidBody:              {"Invalid body", "never"},
	CodeLifecycleRefusal:         {"Lifecycle refusal", "retry_after"},
	CodeNoCurrentRead:            {"No current read", "backoff"},
	CodeNoSuchSetting:            {"No such setting", "never"},
	CodeNotAHomePlace:            {"Not a home place", "never"},
	CodeOperationFailed:          {"Operation failed", "by_status"},
	CodeOperationMismatch:        {"Operation mismatch", "never"},
	CodePlanStale:                {"Plan stale", "replan"},
	CodePlanTooLarge:             {"Plan too large", "never"},
	CodeReachNeverRead:           {"Reach never read", "backoff"},
	CodeReasonRequired:           {"Reason required", "never"},
	CodeSettingNotSaved:          {"Setting not saved", "backoff"},
	CodeSettingsUnreadable:       {"Settings unreadable", "backoff"},
	CodeStoreUnreadable:          {"Store unreadable", "backoff"},
	CodeTargetUnreachable:        {"Target unreachable", "retry_after"},
	CodeTargetUnreadable:         {"Target unreadable", "backoff"},
	CodeTargetVersionUnsupported: {"Target version unsupported", "never"},
	CodeUnauthenticated:          {"Unauthenticated", "never"},
	CodeUnknownField:             {"Unknown field", "never"},
	CodeUnknownLifecycleState:    {"Unknown lifecycle state", "never"},
	CodeUnknownMetric:            {"Unknown metric", "never"},
	CodeUnknownRequestField:      {"Unknown request field", "never"},
	CodeUnknownRoute:             {"Unknown route", "never"},
}

// retryAfterSeconds is the Retry-After a `retry_after` code carries: a
// lifecycle refusal (a maintenance window is minutes, and the backend polls),
// a call still in flight (the backend's next pass re-sends it), and a write
// that could not reach the NAS (the breaker's cooldown is longer; the backend
// keeps the change queued either way).
const retryAfterSeconds = "30"

// writeError is the one refusal body (design §7, decision 25): `type`, `title`,
// `status`, `detail`, and the extensions `code` and `retry`, plus whatever ext
// carries (a blocked apply's outcome, both version numbers, the plan limit).
// detail is this add-on's own sentence, never the target's text: that text is
// where a submitted password comes back.
func writeError(w http.ResponseWriter, status int, code, detail string, ext map[string]any) {
	kind, known := problems[code]
	if !known {
		// Unreachable while the guard test holds; a refusal is still sent.
		log.Printf("[HTTP] unregistered error code %s", code)
		kind = problemKind{title: code, retry: "never"}
	}
	retry := kind.retry
	if retry == "by_status" {
		// What a caller may do about THIS reply: a throttle or a failing target
		// may clear, a refusal of the request itself will not.
		retry = "never"
		if status == http.StatusTooManyRequests || status >= 500 {
			retry = "backoff"
		}
	}
	body := maps.Clone(ext)
	if body == nil {
		body = map[string]any{}
	}
	maps.Copy(body, map[string]any{
		"type": "urn:syndra:addon:error:" + code, "title": kind.title,
		"status": status, "code": code, "retry": retry,
	})
	if strings.TrimSpace(detail) != "" {
		body["detail"] = detail
	} else {
		delete(body, "detail")
	}
	w.Header().Set("Content-Type", "application/problem+json")
	// A caller that knows the wait (a cooldown's seconds left) has set it.
	if retry == "retry_after" && w.Header().Get("Retry-After") == "" {
		w.Header().Set("Retry-After", retryAfterSeconds)
	}
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("[HTTP] could not write response: %v", err)
	}
}

// writeOutcomeRefusal answers a blocked or conflicting apply: a problem whose
// extension members are the outcome's fields (RFC 9457 §3.2), so the backend
// reads the same outcome it reads from a 200.
func writeOutcomeRefusal(w http.ResponseWriter, status int, code string, out ApplyOutcome) {
	raw, _ := json.Marshal(out)
	var ext map[string]any
	_ = json.Unmarshal(raw, &ext)
	delete(ext, "detail")
	writeError(w, status, code, out.Detail, ext)
}
