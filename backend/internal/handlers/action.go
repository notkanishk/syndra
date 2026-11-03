package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"syndra/internal/claimdebug"
	"syndra/internal/claims"
	"syndra/internal/claimtel"
	"syndra/internal/services"
)

// ActionV2UserRef is the subset of the Zitadel Actions v2 user block Syndra reads.
type ActionV2UserRef struct {
	ID string `json:"id"`
}

// ActionV2UserGrantRef is the subset of a Zitadel Actions v2 user grant row
// Syndra reads. Zitadel may emit one entry per role, so callers MUST
// deduplicate by ProjectID before acting.
//
// Zitadel serialises query.UserGrant with snake_case tags: the project is
// "project_id" (internal/query/user_grant.go, v3.4 and v4). Its docs show
// "projectId", and Syndra read only that until 3 Nov 2026, so every token
// carried no Syndra claims while Actions v1 hid it. Both spellings are read;
// snake_case wins when both are present.
//
// Roles is kept raw: only debug capture shows it, and a field Syndra does not
// need must never be the reason a token loses its claims (F-4).
type ActionV2UserGrantRef struct {
	ProjectID string          `json:"project_id"`
	Roles     json.RawMessage `json:"roles,omitempty"`
}

// errGrantContract is a grant row that names no project in either spelling.
var errGrantContract = errors.New("user grant names no project_id")

// UnmarshalJSON accepts "project_id" (what Zitadel sends) and "projectId"
// (what its docs show).
func (g *ActionV2UserGrantRef) UnmarshalJSON(b []byte) error {
	ref, ok := grantFromJSON(b)
	if !ok {
		return errGrantContract
	}
	*g = ref
	return nil
}

// grantFromJSON reads one grant row. Not ok when the row is not an object, or
// carries no project key (a rename, the F-0 class), or one that is not a string.
func grantFromJSON(b []byte) (ActionV2UserGrantRef, bool) {
	var raw map[string]json.RawMessage
	if json.Unmarshal(b, &raw) != nil || raw == nil {
		return ActionV2UserGrantRef{}, false
	}
	g := ActionV2UserGrantRef{Roles: raw["roles"]}
	found := false
	for _, k := range []string{"project_id", "projectId"} {
		v, ok := raw[k]
		if !ok {
			continue
		}
		var s string
		if json.Unmarshal(v, &s) != nil {
			return ActionV2UserGrantRef{}, false
		}
		found = true
		if g.ProjectID == "" {
			g.ProjectID = s
		}
	}
	return g, found
}

// ActionV2ApplicationRef names the client the token is for (Zitadel v4+).
// It picks the app's own shape (ADR-003); absent or unknown, the project's
// default applies.
type ActionV2ApplicationRef struct {
	ClientID string `json:"client_id"`
}

// ActionV2Request is what Syndra reads from a Zitadel Actions v2
// `function/preaccesstoken` or `function/preuserinfo` payload. It is decoded by
// decodeActionV2, field by field, so the payload Zitadel owns can grow or
// retype what Syndra does not need without costing a token its claims.
type ActionV2Request struct {
	Function    string                 `json:"function,omitempty"`
	User        ActionV2UserRef        `json:"user"`
	UserGrants  []ActionV2UserGrantRef `json:"user_grants"`
	Application ActionV2ApplicationRef `json:"application"`
}

// knownTopLevel is every top-level key Zitadel's ContextInfo has
// (internal/api/oidc/userinfo.go, v4.20.1 and main).
var knownTopLevel = map[string]bool{
	"function": true, "userinfo": true, "user": true, "user_metadata": true, "org": true,
	"user_grants": true, "application": true, "actor": true, "response": true,
}

// decodeActionV2 reads a payload. The outcome is "" when the payload is
// usable, bad_request when it is not a JSON object, and contract_fault when
// it is one that breaks the contract Syndra depends on.
//
// user_grants is presence-aware (F-3), and the fact that decides it is
// Zitadel's: ContextInfo.UserGrants is `json:"user_grants,omitempty"`, so a
// token with no role audience arrives with the key ABSENT, never as []. Absent
// is therefore no_grants, the normal case for an app that asserts no roles. It
// is a contract fault only when the payload carries an unknown top-level array
// of objects instead, which is what a renamed grants list looks like. A list
// whose rows name no project in either spelling is a contract fault too: that
// is exactly how F-0 read for months.
//
// Everything else is best-effort: a function, application or roles value of
// another type reads as empty, never as a 400 (F-4).
func decodeActionV2(body []byte) (ActionV2Request, claimtel.Outcome) {
	var top map[string]json.RawMessage
	if err := decodeJSONLenient(bytes.NewReader(body), &top); err != nil || top == nil {
		return ActionV2Request{}, claimtel.BadRequest
	}
	var req ActionV2Request
	_ = json.Unmarshal(top["function"], &req.Function)
	_ = json.Unmarshal(top["application"], &req.Application)
	var user struct {
		ID json.RawMessage `json:"id"`
	}
	_ = json.Unmarshal(top["user"], &user)
	_ = json.Unmarshal(user.ID, &req.User.ID)
	if !trimmedNonEmpty(req.User.ID) {
		return req, claimtel.ContractFault
	}

	raw, present := top["user_grants"]
	if !present {
		for k, v := range top {
			if !knownTopLevel[k] && isArrayOfObjects(v) {
				return req, claimtel.ContractFault
			}
		}
		return req, ""
	}
	if string(bytes.TrimSpace(raw)) == "null" {
		return req, ""
	}
	var rows []json.RawMessage
	if json.Unmarshal(raw, &rows) != nil {
		return req, claimtel.ContractFault
	}
	for _, row := range rows {
		g, ok := grantFromJSON(row)
		if !ok {
			return req, claimtel.ContractFault
		}
		req.UserGrants = append(req.UserGrants, g)
	}
	return req, ""
}

func isArrayOfObjects(raw json.RawMessage) bool {
	var rows []map[string]json.RawMessage
	return json.Unmarshal(raw, &rows) == nil && len(rows) > 0
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

// Time budgets (assessment §4e, the interim rung before the snapshot). Zitadel
// gives a call 3s including connect and TLS; Syndra answers within
// actionDeadline whatever its dependencies do, and every read below derives
// from that one deadline.
const (
	actionDeadline = 250 * time.Millisecond
	// redisTimeout bounds one Redis read. Real since the client honours
	// context deadlines (db.NewRedisClient, F-1).
	redisTimeout = 50 * time.Millisecond
	// shapeReadTimeout bounds the Postgres read of a project's shapes on a
	// cache miss.
	shapeReadTimeout = 150 * time.Millisecond
	// failureModeReadTimeout bounds the Postgres read of a project's failure
	// mode, which runs exactly when something is already failing (F-2).
	failureModeReadTimeout = 100 * time.Millisecond
)

// actionAnswer is the token's answer and what it came to.
type actionAnswer struct {
	resp        ActionV2Response
	projectID   string // "" when the grants name none, or several and it is refused
	outcome     claimtel.Outcome
	clientKnown bool
}

func emptyAnswer(o claimtel.Outcome) actionAnswer {
	return actionAnswer{resp: ActionV2Response{AppendClaims: []ActionV2Claim{}}, outcome: o}
}

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
// Every answer is counted by outcome class (claimtel), through the Call the
// route's wrapper put in the context. Per-token logs are gone: a hit or a
// no-grants is a number, not a line.
//
// Debug mode (claimdebug) is asked once, after the answer is written: only the
// one account being watched on that client is captured, by a non-blocking
// send, so the latency path is untouched.
func HandleActionInject(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	call := claimtel.From(r.Context())
	if r.Method != http.MethodPost {
		jsonErrorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST is supported")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		call.Set(claimtel.BadRequest)
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	req, fault := decodeActionV2(body)
	if fault == claimtel.BadRequest {
		call.Set(claimtel.BadRequest)
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": "not a JSON object"})
		return
	}
	if call != nil {
		call.Function = req.Function
		if call.Function == "" {
			call.Function = claimtel.None
		}
		call.ClientID, call.UserID, call.Body = req.Application.ClientID, req.User.ID, body
	}
	if fault == claimtel.ContractFault {
		call.Set(claimtel.ContractFault)
		if !trimmedNonEmpty(req.User.ID) {
			jsonValidationErrorResponse(w, "user.id is required", map[string]string{"user.id": "required"})
			return
		}
		log.Printf("[DATA PLANE] Contract fault: the payload's user_grants do not read as Zitadel's (client=%q function=%s); answering no claims", req.Application.ClientID, req.Function)
		jsonResponse(w, http.StatusOK, ActionV2Response{AppendClaims: []ActionV2Claim{}})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), actionDeadline)
	defer cancel()
	ans := answerAction(ctx, req)
	if call != nil {
		call.Outcome, call.ProjectID, call.ClientKnown = ans.outcome, ans.projectID, ans.clientKnown
	}
	jsonResponse(w, http.StatusOK, ans.resp)

	if claimDebug.Wants(req.Application.ClientID, req.User.ID) {
		answer, _ := json.Marshal(ans.resp)
		claimDebug.Record(claimdebug.Capture{
			At: start, Function: req.Function, ClientID: req.Application.ClientID, UserID: req.User.ID,
			ProjectID: ans.projectID, Grants: req.UserGrants, Request: body, Response: answer, Duration: time.Since(start),
		})
	}
}

// answerAction is the token's answer, the one project it is for, and its outcome.
func answerAction(ctx context.Context, req ActionV2Request) actionAnswer {
	grantProjects := make([]string, len(req.UserGrants))
	for i, g := range req.UserGrants {
		grantProjects[i] = g.ProjectID
	}
	projectIDs := dedupeNonEmpty(grantProjects)

	if len(projectIDs) == 0 {
		return emptyAnswer(claimtel.NoGrants)
	}

	if len(projectIDs) > 1 {
		log.Printf("[DATA PLANE] Refusing cross-project claims for user=%s client=%q function=%s: the token spans projects %v. "+
			"Syndra delivers one project per token; drop the extra :aud scopes, or give the role in the app's own project with a rule.",
			req.User.ID, req.Application.ClientID, req.Function, projectIDs)
		return emptyAnswer(claimtel.RefusedCrossProject)
	}

	a := claimsForProject(ctx, req.User.ID, projectIDs[0], req.Application.ClientID, req.Function)
	a.projectID = projectIDs[0]
	return a
}

// knownFunction is a function string Zitadel is known to send. Anything else
// is still answered (LandsIn treats it as both: missing claims are worse than
// over-delivery to the same app), and counted as unknown_function (F-5).
func knownFunction(f string) bool {
	return f == claims.FunctionPreAccessToken || f == claims.FunctionPreUserinfo
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
// Degraded paths (Redis miss/timeout, malformed cache) answer the project's
// failure mode. A miss is also a repair signal: the person's facts are
// recompiled off the hot path (F-7), so the next token finds them.
func claimsForProject(ctx context.Context, userID, projectID, clientID, function string) actionAnswer {
	shape, known, ok := claimShapeFor(ctx, projectID, clientID)
	if !ok {
		// The shapes could not be read. A guessed shape could show a role the
		// app's own shape hides, so the project's failure mode answers instead.
		return degraded(ctx, projectID, claimtel.DegradedShapeUnreadable, known)
	}
	if !shape.Profile.LandsIn(function) {
		a := emptyAnswer(claimtel.NotLanding)
		a.clientKnown = known
		return a
	}

	cacheKey := fmt.Sprintf("mapping:%s:%s", userID, projectID)

	redisCtx, cancel := context.WithTimeout(ctx, redisTimeout)
	defer cancel()

	val, err := redisGetClaims(redisCtx, cacheKey)
	if err != nil {
		switch {
		case errors.Is(err, redis.Nil):
			claimRepair.enqueue(userID)
		case redisCtx.Err() != nil:
			log.Printf("[DATA PLANE] Redis timeout for key=%s after %v", cacheKey, redisTimeout)
		default:
			log.Printf("[DATA PLANE] Facts unreadable for key=%s: %v", cacheKey, err)
		}
		return degraded(ctx, projectID, claimtel.DegradedNoFacts, known)
	}

	var facts claims.Facts
	if err := json.Unmarshal([]byte(val), &facts); err != nil {
		log.Printf("[DATA PLANE] Malformed cache data for key=%s: %v", cacheKey, err)
		return degraded(ctx, projectID, claimtel.DegradedMalformed, known)
	}
	if facts.ProjectID == "" {
		facts.ProjectID = projectID
	}
	if facts.UserID == "" {
		facts.UserID = userID
	}

	outcome := claimtel.Served
	if !knownFunction(function) {
		outcome = claimtel.UnknownFunction
	}
	return actionAnswer{
		resp:    ActionV2Response{AppendClaims: objectToEnvelope(shapeClaims(shape.Profile, facts))},
		outcome: outcome, clientKnown: known,
	}
}

// degraded answers the project's failure mode. When the request's deadline
// has passed by then, the outcome is deadline_exceeded whatever rung failed:
// the answer is whatever the failure mode could still be read in time
// (fail_closed when nothing could).
func degraded(ctx context.Context, projectID string, o claimtel.Outcome, known bool) actionAnswer {
	resp := degradedResponse(ctx, projectID)
	if ctx.Err() != nil {
		o = claimtel.DeadlineExceeded
	}
	return actionAnswer{resp: resp, outcome: o, clientKnown: known}
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
// The count per client is in claim_delivery_last's client_known.
var unmatchedClients sync.Map

// claimShapeFor is the shape for one sign-in: the app's own when the client id
// maps to an app with a shape on this project, else the project's default
// (then the built-in). An absent client id (Zitadel before v4) or one with no
// shape of its own gets the default, which is logged once per client. known
// is whether the app had a shape of its own.
func claimShapeFor(ctx context.Context, projectID, clientID string) (shape services.AppShape, known, ok bool) {
	shapes, err := projectShapesRead(ctx, projectID)
	if err != nil {
		return services.AppShape{}, false, false
	}
	shape, own := shapes.For(clientID)
	if !own {
		if _, seen := unmatchedClients.LoadOrStore(projectID+"\x00"+clientID, true); !seen {
			if clientID == "" {
				log.Printf("[DATA PLANE] Project %s: a request named no application.client_id; using the project's default shape", projectID)
			} else {
				log.Printf("[DATA PLANE] Project %s: client %q has no shape of its own; using the project's default shape", projectID, clientID)
			}
		}
	}
	return shape, own, true
}

// errShapesUnreadable is Redis answering with something other than a miss.
var errShapesUnreadable = errors.New("claim shape cache unreadable")

// projectShapesRead resolves a project's shapes via a Redis read-through
// cache, so the data plane does not hit Postgres per token.
//
// Only a miss (redis.Nil) or a malformed entry reads Postgres. Redis down or
// timing out is an error here (F-10): with Redis gone, every token would
// otherwise pay a Postgres read and a failed write-back, on exactly the day
// latency matters most.
//
// A failed read is an error, never a guessed shape: the caller answers with
// the project's failure mode, so a role an app's shape hides is never sent.
func projectShapesRead(ctx context.Context, projectID string) (services.ProjectShapes, error) {
	redisCtx, cancel := context.WithTimeout(ctx, redisTimeout)
	defer cancel()

	raw, err := redisGetKey(redisCtx, claimShapeCacheKey(projectID))
	switch {
	case err == nil:
		var cached services.ProjectShapes
		if jerr := json.Unmarshal([]byte(raw), &cached); jerr == nil && cached.Default.Lands != "" {
			return cached, nil
		}
		log.Printf("[DATA PLANE] Malformed claim shape cache for project=%s", projectID)
	case errors.Is(err, redis.Nil):
	default:
		log.Printf("[DATA PLANE] Claim shape cache unreadable for project=%s, answering with its failure mode: %v", projectID, err)
		return services.ProjectShapes{}, fmt.Errorf("%w: %v", errShapesUnreadable, err)
	}

	dbCtx, dbCancel := context.WithTimeout(ctx, shapeReadTimeout)
	defer dbCancel()
	shapes, err := dbResolveProjectShapes(dbCtx, projectID)
	if err != nil {
		log.Printf("[DATA PLANE] Claim shape lookup failed for project=%s, answering with its failure mode: %v", projectID, err)
		return services.ProjectShapes{}, err
	}

	if encoded, jerr := json.Marshal(shapes); jerr == nil {
		writeCtx, writeCancel := context.WithTimeout(ctx, redisTimeout)
		defer writeCancel()
		if serr := redisSetKey(writeCtx, claimShapeCacheKey(projectID), string(encoded), claimFailureModeCacheTTL()); serr != nil {
			log.Printf("[DATA PLANE] Claim shape cache write failed for project=%s: %v", projectID, serr)
		}
	}
	return shapes, nil
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
//  2. Miss → call dbGetClaimFailureMode, bounded by failureModeReadTimeout
//     and the request's own deadline. Success → cache + return.
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

	dbCtx, dbCancel := context.WithTimeout(ctx, failureModeReadTimeout)
	mode, minimalClaims, err := dbGetClaimFailureMode(dbCtx, projectID)
	dbCancel()
	if err != nil {
		log.Printf("[CLAIM-MODE-CACHE] DB read failed for project=%s; defaulting to fail_closed: %v", projectID, err)
		return "fail_closed", nil, nil
	}
	if rerr != nil && !errors.Is(rerr, redis.Nil) {
		// Redis is erroring, not missing: a write-back would only spend the
		// request's remaining budget on the same fault.
		return mode, minimalClaims, nil
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
