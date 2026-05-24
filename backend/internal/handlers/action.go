package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// ActionV2UserRef is the subset of the Zitadel Actions v2 user block MkAuth reads.
type ActionV2UserRef struct {
	ID string `json:"id"`
}

// ActionV2UserGrantRef is the subset of a Zitadel Actions v2 user grant row
// MkAuth reads. Zitadel may emit one entry per role, so callers MUST
// deduplicate by ProjectID before acting.
type ActionV2UserGrantRef struct {
	ProjectID string   `json:"projectId"`
	Roles     []string `json:"roles"`
}

// ActionV2Request is the acceptance shape for Zitadel Actions v2
// `function/preaccesstoken` and `function/preuserinfo` trigger payloads.
// Only the fields MkAuth needs are declared; all other fields Zitadel sends
// (org, userMetadata, userinfo, ...) are accepted and ignored via the
// lenient decoder, since the v2 payload surface is owned by Zitadel and
// expected to extend over time.
type ActionV2Request struct {
	Function   string                 `json:"function,omitempty"`
	User       ActionV2UserRef        `json:"user"`
	UserGrants []ActionV2UserGrantRef `json:"user_grants"`
}

// ActionV2Claim is one entry in the Zitadel Actions v2 append_claims list.
type ActionV2Claim struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

// ActionV2Response is the envelope Zitadel Actions v2 expects back from a
// custom-claim-injection handler. Only append_claims (and optionally
// append_log_claims) are emitted by MkAuth; set_user_metadata is intentionally
// omitted until a spec requires it.
type ActionV2Response struct {
	AppendClaims    []ActionV2Claim `json:"append_claims"`
	AppendLogClaims []string        `json:"append_log_claims,omitempty"`
}

// redisTimeout is the maximum time the data plane will wait for a Redis response.
// Zitadel Actions v2 have hard latency budgets (3s target timeout); this keeps
// the per-project Redis fetch well inside that envelope and prevents cascade
// failures.
const redisTimeout = 50 * time.Millisecond

// HandleActionInject is the DATA PLANE entrypoint for Zitadel Actions v2.
// Zitadel POSTs the function trigger payload (preaccesstoken or preuserinfo);
// MkAuth returns the pre-compiled per-project claim envelope.
//
// Project resolution:
//   - Zero unique projects in user_grants: emit empty append_claims.
//   - One unique project: flat claim keys (preserves the spec's "Printing
//     Portal only gets Printing roles" least-privilege scenario).
//   - Multiple projects: namespaced keys "mkauth.<projectID>.<claim>" so
//     claims from different projects cannot collide in the issued token.
//
// Degraded behavior is resolved per-project via dbGetClaimFailureMode:
//   - fail_closed (default): empty append_claims for that project
//   - minimal_safe: the configured minimal claim set, wrapped in the envelope
func HandleActionInject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonErrorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST is supported")
		return
	}

	var req ActionV2Request
	if err := decodeJSONLenient(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	if !trimmedNonEmpty(req.User.ID) {
		jsonValidationErrorResponse(w, "user.id is required", map[string]string{"user.id": "required"})
		return
	}

	projectIDs := dedupProjectIDs(req.UserGrants)

	switch len(projectIDs) {
	case 0:
		log.Printf("[DATA PLANE] No project grants in request for user=%s function=%s", req.User.ID, req.Function)
		jsonResponse(w, http.StatusOK, ActionV2Response{AppendClaims: []ActionV2Claim{}})
	case 1:
		jsonResponse(w, http.StatusOK, claimsForProject(r.Context(), req.User.ID, projectIDs[0], false))
	default:
		merged := ActionV2Response{AppendClaims: []ActionV2Claim{}}
		for _, pid := range projectIDs {
			resp := claimsForProject(r.Context(), req.User.ID, pid, true)
			merged.AppendClaims = append(merged.AppendClaims, resp.AppendClaims...)
		}
		jsonResponse(w, http.StatusOK, merged)
	}
}

// dedupProjectIDs extracts unique, trimmed project IDs from the grant list,
// preserving first-seen order so the output is deterministic for golden tests.
func dedupProjectIDs(grants []ActionV2UserGrantRef) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(grants))
	for _, g := range grants {
		pid := strings.TrimSpace(g.ProjectID)
		if pid == "" {
			continue
		}
		if _, dup := seen[pid]; dup {
			continue
		}
		seen[pid] = struct{}{}
		out = append(out, pid)
	}
	return out
}

// claimsForProject fetches the pre-compiled claim map for a (user, project)
// pair from Redis and converts it into the Zitadel Actions v2 append_claims
// envelope. When namespace=true, every emitted key is prefixed with
// "mkauth.<projectID>." so claims from different projects cannot collide
// in the issued token. Degraded paths (Redis miss/timeout, malformed cache,
// DB lookup failure) fall through to degradedResponse for that project.
func claimsForProject(ctx context.Context, userID, projectID string, namespace bool) ActionV2Response {
	cacheKey := fmt.Sprintf("mapping:%s:%s", userID, projectID)

	redisCtx, cancel := context.WithTimeout(ctx, redisTimeout)
	defer cancel()

	val, err := redisGetClaims(redisCtx, cacheKey)
	if err != nil {
		if redisCtx.Err() != nil {
			log.Printf("[DATA PLANE] Redis timeout for key=%s after %v", cacheKey, redisTimeout)
		} else {
			log.Printf("[DATA PLANE] Cache miss for key=%s: %v", cacheKey, err)
		}
		return degradedResponse(ctx, projectID, namespace)
	}

	var claims map[string]interface{}
	if err := json.Unmarshal([]byte(val), &claims); err != nil {
		log.Printf("[DATA PLANE] Malformed cache data for key=%s: %v", cacheKey, err)
		return degradedResponse(ctx, projectID, namespace)
	}

	log.Printf("[DATA PLANE] Cache hit for key=%s", cacheKey)
	return ActionV2Response{AppendClaims: claimsToEnvelope(claims, projectID, namespace)}
}

// claimsToEnvelope converts a raw claim map into the append_claims list.
// When namespace=true, keys are prefixed with "mkauth.<projectID>." so a
// multi-project response cannot have colliding claim keys across projects.
func claimsToEnvelope(claims map[string]interface{}, projectID string, namespace bool) []ActionV2Claim {
	out := make([]ActionV2Claim, 0, len(claims))
	for k, v := range claims {
		key := k
		if namespace {
			key = fmt.Sprintf("mkauth.%s.%s", projectID, k)
		}
		out = append(out, ActionV2Claim{Key: key, Value: v})
	}
	return out
}

// claimFailureModeCacheTTL returns the Redis TTL for the read-through cache
// of the per-project claim_failure_mode. Default 5 minutes; overridable via
// CLAIM_MODE_CACHE_TTL_SECONDS for environments that pin Redis to short
// retention.
func claimFailureModeCacheTTL() int {
	if v := os.Getenv("CLAIM_MODE_CACHE_TTL_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 300
}

// claimFailureModeRead resolves the per-project failure mode + minimal-safe
// claims via a Redis read-through cache.
//
//  1. Read claim_mode:<projectID> from Redis (bounded by redisTimeout). Hit → return.
//  2. Miss → call dbGetClaimFailureMode. Success → cache + return.
//  3. DB error after cache miss → log + return ("fail_closed", nil, nil).
//
// The cache exists so a transient DB outage cannot collapse degraded-mode
// behaviour into fail_closed for projects whose operator configured
// minimal_safe (audit ref C5). Errors are suppressed by design: the data
// plane MUST always have a fallback mode to return.
func claimFailureModeRead(ctx context.Context, projectID string) (string, map[string]interface{}, error) {
	readCtx, cancel := context.WithTimeout(ctx, redisTimeout)
	raw, rerr := redisGetClaimMode(readCtx, projectID)
	cancel()
	if rerr == nil && raw != "" {
		var payload struct {
			Mode              string                 `json:"mode"`
			MinimalSafeClaims map[string]interface{} `json:"minimal_safe_claims"`
		}
		if jerr := json.Unmarshal([]byte(raw), &payload); jerr == nil {
			return payload.Mode, payload.MinimalSafeClaims, nil
		}
		log.Printf("[CLAIM-MODE-CACHE] malformed cached value for project=%s; refreshing from DB", projectID)
	}

	mode, claims, err := dbGetClaimFailureMode(ctx, projectID)
	if err != nil {
		log.Printf("[CLAIM-MODE-CACHE] DB read failed for project=%s; defaulting to fail_closed: %v", projectID, err)
		return "fail_closed", nil, nil
	}

	encoded, jerr := json.Marshal(struct {
		Mode              string                 `json:"mode"`
		MinimalSafeClaims map[string]interface{} `json:"minimal_safe_claims"`
	}{Mode: mode, MinimalSafeClaims: claims})
	if jerr == nil {
		writeCtx, wcancel := context.WithTimeout(ctx, redisTimeout)
		serr := redisSetClaimMode(writeCtx, projectID, string(encoded), claimFailureModeCacheTTL())
		wcancel()
		if serr != nil {
			log.Printf("[CLAIM-MODE-CACHE] cache write failed for project=%s: %v (non-fatal)", projectID, serr)
		}
	}
	return mode, claims, nil
}

// degradedResponse returns the per-project fallback envelope when the Redis
// fetch or cache body for that project is unusable.
//
// fail_closed (default): empty append_claims — applications must cope with
// absent custom claims.
// minimal_safe: the configured minimal claim map from the DB, wrapped in the
// v2 envelope (namespacing preserved when namespace=true).
// DB lookup failure: defaulted to fail_closed and logged (handled inside
// claimFailureModeRead).
func degradedResponse(ctx context.Context, projectID string, namespace bool) ActionV2Response {
	mode, minimalClaims, _ := claimFailureModeRead(ctx, projectID)

	switch mode {
	case "minimal_safe":
		if minimalClaims == nil {
			minimalClaims = map[string]interface{}{}
		}
		log.Printf("[DATA PLANE] Degraded mode=minimal_safe for project=%s", projectID)
		return ActionV2Response{AppendClaims: claimsToEnvelope(minimalClaims, projectID, namespace)}
	default:
		log.Printf("[DATA PLANE] Degraded mode=fail_closed for project=%s", projectID)
		return ActionV2Response{AppendClaims: []ActionV2Claim{}}
	}
}
