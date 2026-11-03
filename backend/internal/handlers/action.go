package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"syndra/internal/claimdebug"
	"syndra/internal/claims"
	"syndra/internal/services"
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

// ActionV2ApplicationRef names the client the token is for (Zitadel v4+).
// It picks the app's own shape (ADR-003); absent or unknown, the project's
// default applies.
type ActionV2ApplicationRef struct {
	ClientID string `json:"client_id"`
}

// ActionV2Request is the acceptance shape for Zitadel Actions v2
// `function/preaccesstoken` and `function/preuserinfo` trigger payloads.
// Only the fields Syndra needs are declared; all other fields Zitadel sends
// (org, userMetadata, userinfo, ...) are accepted and ignored via the
// lenient decoder, since the v2 payload surface is owned by Zitadel and
// expected to extend over time.
type ActionV2Request struct {
	Function    string                 `json:"function,omitempty"`
	User        ActionV2UserRef        `json:"user"`
	UserGrants  []ActionV2UserGrantRef `json:"user_grants"`
	Application ActionV2ApplicationRef `json:"application"`
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
// One project per token. Zitadel lists in user_grants only the token's role
// audience: the client's own project, plus any project the app asked for with
// an `urn:zitadel:iam:org:project:id:{id}:aud` scope. Syndra delivers claims
// for exactly one project and refuses cross-project delivery outright: a token
// whose grants span several projects carries no Syndra claims, and the log says
// which client asked. An app that needs a role from elsewhere gets it in its
// own project through a rule. ponytail: refuse rather than pick the client's
// own project; resolve application.client_id to its project if an app ever
// needs extra audiences for another reason.
//
// Degraded behavior is resolved per-project via dbGetClaimFailureMode:
//   - fail_closed (default): empty append_claims for that project
//   - minimal_safe: the configured minimal claim set, wrapped in the envelope
//
// Debug mode (claimdebug) is asked once, after the answer is written: only the
// one account being watched on that client is captured, by a non-blocking
// send, so the latency path is untouched.
func HandleActionInject(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if r.Method != http.MethodPost {
		jsonErrorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST is supported")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	var req ActionV2Request
	if err := decodeJSONLenient(bytes.NewReader(body), &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	if !trimmedNonEmpty(req.User.ID) {
		jsonValidationErrorResponse(w, "user.id is required", map[string]string{"user.id": "required"})
		return
	}

	resp, projectID := answerAction(r.Context(), req)
	jsonResponse(w, http.StatusOK, resp)

	if claimDebug.Wants(req.Application.ClientID, req.User.ID) {
		answer, _ := json.Marshal(resp)
		claimDebug.Record(claimdebug.Capture{
			At: start, Function: req.Function, ClientID: req.Application.ClientID, UserID: req.User.ID,
			ProjectID: projectID, Grants: req.UserGrants, Request: body, Response: answer, Duration: time.Since(start),
		})
	}
}

// answerAction is the token's answer and the one project it is for ("" when
// the grants name none, or several and it is refused).
func answerAction(ctx context.Context, req ActionV2Request) (ActionV2Response, string) {
	grantProjects := make([]string, len(req.UserGrants))
	for i, g := range req.UserGrants {
		grantProjects[i] = g.ProjectID
	}
	projectIDs := dedupeNonEmpty(grantProjects)

	if len(projectIDs) == 0 {
		log.Printf("[DATA PLANE] No project grants in request for user=%s function=%s", req.User.ID, req.Function)
		return ActionV2Response{AppendClaims: []ActionV2Claim{}}, ""
	}

	if len(projectIDs) > 1 {
		log.Printf("[DATA PLANE] Refusing cross-project claims for user=%s client=%q function=%s: the token spans projects %v. "+
			"Syndra delivers one project per token; drop the extra :aud scopes, or give the role in the app's own project with a rule.",
			req.User.ID, req.Application.ClientID, req.Function, projectIDs)
		return ActionV2Response{AppendClaims: []ActionV2Claim{}}, ""
	}

	return claimsForProject(ctx, req.User.ID, projectIDs[0], req.Application.ClientID, req.Function), projectIDs[0]
}

// claimsForProject reads the compiled access facts for a (user, project) pair
// from Redis, shapes them through the signing-in app's shape (its own, else the
// project's default), and converts the result into the Actions v2
// append_claims envelope.
//
// Shaping happens HERE, on read, not at compile time. An edit therefore lands
// on the very next token for every user, instead of only for those whose
// cache happened to be rebuilt afterwards.
//
// A shape that doesn't land in this function (an access-token-only shape asked
// for userinfo, say) answers nothing, before any facts are read.
//
// Degraded paths (Redis miss/timeout, malformed cache) fall through to
// degradedResponse for that project.
func claimsForProject(ctx context.Context, userID, projectID, clientID, function string) ActionV2Response {
	shape := claimShapeFor(ctx, projectID, clientID)
	if !shape.Profile.LandsIn(function) {
		return ActionV2Response{AppendClaims: []ActionV2Claim{}}
	}

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
	return ActionV2Response{AppendClaims: objectToEnvelope(claims.Shape(shape.Profile, facts))}
}

// objectToEnvelope converts a shaped token into the append_claims list in row
// order: the token keeps the order the operator added the rows in.
func objectToEnvelope(shaped *claims.Object) []ActionV2Claim {
	out := make([]ActionV2Claim, 0, len(shaped.Keys()))
	for _, k := range shaped.Keys() {
		v, _ := shaped.Get(k)
		out = append(out, ActionV2Claim{Key: k, Value: v})
	}
	return out
}

// claimsToEnvelope converts a claim map (the operator's minimal-safe set) into
// the append_claims list, key-sorted so the order is stable across issues.
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

// claimShapeCacheKey is the Redis read-through key for a project's shapes
// (services.ProjectShapes). Written by projectShapesRead, deleted by the
// claim-shaping handlers on every save so an edit is visible at once rather
// than up to one TTL later.
func claimShapeCacheKey(projectID string) string {
	return "claim_shapes:" + projectID
}

// unmatchedClients remembers which clients have been logged as using their
// project's default, so the log says it once per client, not once per token.
var unmatchedClients sync.Map

// claimShapeFor is the shape for one sign-in: the app's own when the client id
// maps to an app with a shape on this project, else the project's default
// (then the built-in). An absent client id (Zitadel before v4) or one with no
// shape of its own gets the default, which is logged once per client.
func claimShapeFor(ctx context.Context, projectID, clientID string) services.AppShape {
	shape, own := projectShapesRead(ctx, projectID).For(clientID)
	if !own {
		if _, seen := unmatchedClients.LoadOrStore(projectID+"\x00"+clientID, true); !seen {
			if clientID == "" {
				log.Printf("[DATA PLANE] Project %s: a request named no application.client_id; using the project's default shape", projectID)
			} else {
				log.Printf("[DATA PLANE] Project %s: client %q has no shape of its own; using the project's default shape", projectID, clientID)
			}
		}
	}
	return shape
}

// projectShapesRead resolves a project's shapes via a Redis read-through
// cache, so the data plane does not hit Postgres per token.
//
// A failed read returns the built-in default rather than nothing: the roles
// under the default key is a degraded token, no claims is a locked door. That
// fallback is never cached.
func projectShapesRead(ctx context.Context, projectID string) services.ProjectShapes {
	fallback := services.ProjectShapes{Default: claims.Builtin(), DefaultSource: services.SourceBuiltin}

	redisCtx, cancel := context.WithTimeout(ctx, redisTimeout)
	defer cancel()

	if raw, err := redisGetKey(redisCtx, claimShapeCacheKey(projectID)); err == nil {
		var cached services.ProjectShapes
		if jerr := json.Unmarshal([]byte(raw), &cached); jerr == nil && cached.Default.Lands != "" {
			return cached
		}
		log.Printf("[DATA PLANE] Malformed claim shape cache for project=%s", projectID)
	}

	shapes, err := dbResolveProjectShapes(ctx, projectID)
	if err != nil {
		log.Printf("[DATA PLANE] Claim shape lookup failed for project=%s, using the built-in shape: %v", projectID, err)
		return fallback
	}

	if encoded, jerr := json.Marshal(shapes); jerr == nil {
		writeCtx, writeCancel := context.WithTimeout(context.WithoutCancel(ctx), redisTimeout)
		defer writeCancel()
		if serr := redisSetKey(writeCtx, claimShapeCacheKey(projectID), string(encoded), claimFailureModeCacheTTL()); serr != nil {
			log.Printf("[DATA PLANE] Claim shape cache write failed for project=%s: %v", projectID, serr)
		}
	}
	return shapes
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
