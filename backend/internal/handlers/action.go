package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"

	"syndra/internal/claims"
)

// ActionV2UserRef is the subset of the Zitadel Actions v2 user block Syndra reads.
type ActionV2UserRef struct {
	ID string `json:"id"`
}

// ActionV2UserGrantRef is the subset of a Zitadel Actions v2 user grant row
// Syndra reads. Zitadel may emit one entry per role, so callers MUST
// deduplicate by ProjectID before acting.
type ActionV2UserGrantRef struct {
	ProjectID string   `json:"projectId"`
	Roles     []string `json:"roles"`
}

// ActionV2Request is the acceptance shape for Zitadel Actions v2
// `function/preaccesstoken` and `function/preuserinfo` trigger payloads.
// Only the fields Syndra needs are declared; all other fields Zitadel sends
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
// append_log_claims) are emitted by Syndra; set_user_metadata is intentionally
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
// Syndra returns the pre-compiled per-project claim envelope.
//
// Project resolution: every unique project in user_grants is shaped through
// its own claim profiles and the results merged into one flat envelope. Keys
// are operator-authored and validated unique across projects at save time, so
// a multi-project token cannot collide with itself — the old
// "syndra.<projectID>." prefixing that guaranteed this is gone, along with the
// unreadable keys it produced.
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

	grantProjects := make([]string, len(req.UserGrants))
	for i, g := range req.UserGrants {
		grantProjects[i] = g.ProjectID
	}
	projectIDs := dedupeNonEmpty(grantProjects)

	if len(projectIDs) == 0 {
		log.Printf("[DATA PLANE] No project grants in request for user=%s function=%s", req.User.ID, req.Function)
		jsonResponse(w, http.StatusOK, ActionV2Response{AppendClaims: []ActionV2Claim{}})
		return
	}

	perProject := make(map[string][]ActionV2Claim, len(projectIDs))
	for _, pid := range projectIDs {
		perProject[pid] = claimsForProject(r.Context(), req.User.ID, pid).AppendClaims
	}

	jsonResponse(w, http.StatusOK, ActionV2Response{
		AppendClaims: mergeProjectClaims(projectIDs, perProject),
	})
}

// mergeProjectClaims flattens per-project claim lists into one envelope,
// disambiguating any key two projects both want.
//
// Configured keys cannot collide — they are validated unique across projects
// at save time. What can collide is the BUILT-IN default: two projects nobody
// has opened the Token format panel for both emit "roles", and a user holding
// grants in both would otherwise receive one project's roles under a key the
// other project's application is also reading. Prefixing the colliding keys
// keeps both sets present and wrong-for-nobody, rather than silently dropping
// one; the operator fixes it properly by naming the claims.
func mergeProjectClaims(projectIDs []string, perProject map[string][]ActionV2Claim) []ActionV2Claim {
	owners := map[string]int{}
	for _, pid := range projectIDs {
		seen := map[string]bool{}
		for _, c := range perProject[pid] {
			if !seen[c.Key] {
				seen[c.Key] = true
				owners[c.Key]++
			}
		}
	}

	out := make([]ActionV2Claim, 0, len(owners))
	for _, pid := range projectIDs {
		for _, c := range perProject[pid] {
			if owners[c.Key] > 1 {
				log.Printf("[DATA PLANE] Claim key %q is claimed by %d projects; namespacing project=%s. "+
					"Give each project a distinct claim name in Token format.", c.Key, owners[c.Key], pid)
				c.Key = fmt.Sprintf("syndra.%s.%s", pid, c.Key)
			}
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// claimsForProject reads the compiled access facts for a (user, project) pair
// from Redis, shapes them through the project's operator-configured claim
// profiles, and converts the result into the Zitadel Actions v2 append_claims
// envelope.
//
// Shaping happens HERE, on read, not at compile time. A claim-name or format
// edit therefore lands on the very next token for every user, instead of only
// for those whose cache happened to be rebuilt afterwards.
//
// There is no key namespacing: claim keys are explicit, operator-owned, and
// validated unique across projects at save time. The old
// "syndra.<projectID>.<claim>" prefixing existed to stop collisions between
// per-project claim maps that all used the same generic key names — with
// explicit keys the collision cannot arise, and the prefix only made the token
// unreadable to the application that asked for a specific name.
//
// Degraded paths (Redis miss/timeout, malformed cache, DB lookup failure) fall
// through to degradedResponse for that project.
func claimsForProject(ctx context.Context, userID, projectID string) ActionV2Response {
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
		return degradedResponse(ctx, projectID)
	}

	var facts claims.Facts
	if err := json.Unmarshal([]byte(val), &facts); err != nil {
		log.Printf("[DATA PLANE] Malformed cache data for key=%s: %v", cacheKey, err)
		return degradedResponse(ctx, projectID)
	}
	if facts.ProjectID == "" {
		facts.ProjectID = projectID
	}
	if facts.UserID == "" {
		facts.UserID = userID
	}

	log.Printf("[DATA PLANE] Cache hit for key=%s", cacheKey)
	profiles := claimProfilesRead(ctx, projectID)
	return ActionV2Response{AppendClaims: claimsToEnvelope(claims.Shape(profiles, facts))}
}

// claimsToEnvelope converts a shaped claim map into the append_claims list,
// key-sorted so a token's claim order is stable across issues (it makes
// diffing two captured tokens during an incident possible).
func claimsToEnvelope(shaped map[string]interface{}) []ActionV2Claim {
	keys := make([]string, 0, len(shaped))
	for k := range shaped {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]ActionV2Claim, 0, len(shaped))
	for _, k := range keys {
		out = append(out, ActionV2Claim{Key: k, Value: shaped[k]})
	}
	return out
}

// claimShapeCacheKey is the Redis read-through key for a project's resolved
// claim profiles. Written by claimProfilesRead, deleted by the claim-shaping
// handlers on every save so an operator's edit is visible immediately rather
// than up to one TTL later.
func claimShapeCacheKey(projectID string) string {
	return "claim_shape:" + projectID
}

// claimProfilesRead resolves the profile set for a project — the project
// default plus every application override on it — via a Redis read-through
// cache, so the data plane does not hit Postgres per token.
//
// Every failure path returns the built-in default profile rather than an empty
// set: emitting the roles under the default key is a degraded token, emitting
// nothing is a locked door.
func claimProfilesRead(ctx context.Context, projectID string) []claims.Profile {
	fallback := []claims.Profile{{
		ProjectID:  projectID,
		ClaimName:  claims.DefaultClaimName,
		FormatType: claims.DefaultFormat,
	}}

	redisCtx, cancel := context.WithTimeout(ctx, redisTimeout)
	defer cancel()

	if raw, err := redisGetKey(redisCtx, claimShapeCacheKey(projectID)); err == nil {
		var cached []claims.Profile
		if jerr := json.Unmarshal([]byte(raw), &cached); jerr == nil {
			if len(cached) == 0 {
				return fallback
			}
			return cached
		}
		log.Printf("[DATA PLANE] Malformed claim shape cache for project=%s: %v", projectID, err)
	}

	profiles, err := dbResolveClaimProfiles(ctx, projectID)
	if err != nil {
		log.Printf("[DATA PLANE] Claim profile lookup failed for project=%s, using default shape: %v", projectID, err)
		return fallback
	}
	if len(profiles) == 0 {
		profiles = fallback
	}

	if encoded, jerr := json.Marshal(profiles); jerr == nil {
		writeCtx, writeCancel := context.WithTimeout(context.WithoutCancel(ctx), redisTimeout)
		defer writeCancel()
		if serr := redisSetKey(writeCtx, claimShapeCacheKey(projectID), string(encoded), claimFailureModeCacheTTL()); serr != nil {
			log.Printf("[DATA PLANE] Claim shape cache write failed for project=%s: %v", projectID, serr)
		}
	}
	return profiles
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

	mode, minimalClaims, err := dbGetClaimFailureMode(ctx, projectID)
	if err != nil {
		log.Printf("[CLAIM-MODE-CACHE] DB read failed for project=%s; defaulting to fail_closed: %v", projectID, err)
		return "fail_closed", nil, nil
	}

	encoded, jerr := json.Marshal(struct {
		Mode              string                 `json:"mode"`
		MinimalSafeClaims map[string]interface{} `json:"minimal_safe_claims"`
	}{Mode: mode, MinimalSafeClaims: minimalClaims})
	if jerr == nil {
		writeCtx, wcancel := context.WithTimeout(ctx, redisTimeout)
		serr := redisSetClaimMode(writeCtx, projectID, string(encoded), claimFailureModeCacheTTL())
		wcancel()
		if serr != nil {
			log.Printf("[CLAIM-MODE-CACHE] cache write failed for project=%s: %v (non-fatal)", projectID, serr)
		}
	}
	return mode, minimalClaims, nil
}

// degradedResponse returns the per-project fallback envelope when the Redis
// fetch or cache body for that project is unusable.
//
// fail_closed (default): empty append_claims — applications must cope with
// absent custom claims.
// minimal_safe: the configured minimal claim map from the DB, wrapped in the
// v2 envelope verbatim — it is an explicit operator-authored safety net, not
// something the shaper should reinterpret.
// DB lookup failure: defaulted to fail_closed and logged (handled inside
// claimFailureModeRead).
func degradedResponse(ctx context.Context, projectID string) ActionV2Response {
	mode, minimalClaims, _ := claimFailureModeRead(ctx, projectID)

	switch mode {
	case "minimal_safe":
		if minimalClaims == nil {
			minimalClaims = map[string]interface{}{}
		}
		log.Printf("[DATA PLANE] Degraded mode=minimal_safe for project=%s", projectID)
		return ActionV2Response{AppendClaims: claimsToEnvelope(minimalClaims)}
	default:
		log.Printf("[DATA PLANE] Degraded mode=fail_closed for project=%s", projectID)
		return ActionV2Response{AppendClaims: []ActionV2Claim{}}
	}
}
