package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"syndra/internal/claimdebug"
	"syndra/internal/claimserve"
	"syndra/internal/claimtel"
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
type ActionV2Claim = claimserve.Claim

// ActionV2Response is the envelope Zitadel Actions v2 expects back from a
// custom-claim-injection handler. Only append_claims (and optionally
// append_log_claims) are emitted by Syndra; set_user_metadata is intentionally
// omitted until a spec requires it.
type ActionV2Response struct {
	AppendClaims    []ActionV2Claim `json:"append_claims"`
	AppendLogClaims []string        `json:"append_log_claims,omitempty"`
}

// actionDeadline bounds a whole call (assessment §4e, P7 kept as a belt).
// Zitadel gives a call 3s including connect and TLS; under the snapshot the
// answer is memory plus one Redis read bounded at claimserve.FactsTimeout, so
// this is never reached unless something is badly wrong.
const actionDeadline = 250 * time.Millisecond

// actionAnswer is the token's answer and what it came to.
type actionAnswer struct {
	resp        ActionV2Response
	projectID   string // "" when the grants name none, or several and it is refused
	outcome     claimtel.Outcome
	clientKnown bool
	gen         int64
}

func emptyAnswer(o claimtel.Outcome) actionAnswer {
	return actionAnswer{resp: ActionV2Response{AppendClaims: []ActionV2Claim{}}, outcome: o}
}

// HandleActionInject is the DATA PLANE entrypoint for Zitadel Actions v2.
// Zitadel POSTs the function trigger payload (preaccesstoken or preuserinfo);
// Syndra answers from the claims snapshot (claimserve, ADR-004): config from
// memory, facts from one bounded Redis read, never Postgres or Zitadel.
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
// Every answer is counted by outcome class (claimtel), through the Call the
// route's wrapper put in the context, with the snapshot generation it used.
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
		call.Outcome, call.ProjectID, call.ClientKnown, call.Gen = ans.outcome, ans.projectID, ans.clientKnown, ans.gen
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

	res := claimPlane.Answer(ctx, req.User.ID, projectIDs[0], req.Application.ClientID, req.Function)
	if res.Miss {
		// The first sign that a person was never compiled: answer degraded
		// now, recompile off the hot path so the next token finds them (F-7).
		claimRepair.enqueue(req.User.ID)
	}
	return actionAnswer{
		resp:      ActionV2Response{AppendClaims: res.Claims},
		projectID: projectIDs[0], outcome: res.Outcome, clientKnown: res.ClientKnown, gen: res.Gen,
	}
}
