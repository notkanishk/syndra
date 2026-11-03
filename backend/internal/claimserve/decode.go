package claimserve

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"syndra/internal/claimtel"
)

// UserRef is the subset of the Zitadel Actions v2 user block Syndra reads.
//
// The block is Zitadel's query.User (internal/query/user.go): "creation_date"
// is when the account was made; a person carries "human", a machine user
// "machine".
type UserRef struct {
	ID string `json:"id"`
	// CreatedAt is zero when absent or unreadable.
	CreatedAt time.Time `json:"-"`
	// Human: the block carries "human" and no "machine".
	Human bool `json:"-"`
}

// GrantRef is the subset of a Zitadel Actions v2 user grant row Syndra reads.
// Zitadel may emit one entry per role, so callers MUST deduplicate by
// ProjectID before acting.
//
// Zitadel serialises query.UserGrant with snake_case tags: the project is
// "project_id" (internal/query/user_grant.go, v3.4 and v4). Its docs show
// "projectId", and Syndra read only that until 3 Nov 2026, so every token
// carried no Syndra claims while Actions v1 hid it. Both spellings are read;
// snake_case wins when both are present.
//
// Roles is kept raw: only debug capture shows it, and a field Syndra does not
// need must never be the reason a token loses its claims (F-4).
type GrantRef struct {
	ProjectID string          `json:"project_id"`
	Roles     json.RawMessage `json:"roles,omitempty"`
}

// errGrantContract is a grant row that names no project in either spelling.
var errGrantContract = errors.New("user grant names no project_id")

// UnmarshalJSON accepts "project_id" (what Zitadel sends) and "projectId"
// (what its docs show).
func (g *GrantRef) UnmarshalJSON(b []byte) error {
	ref, ok := grantFromJSON(b)
	if !ok {
		return errGrantContract
	}
	*g = ref
	return nil
}

// grantFromJSON reads one grant row. Not ok when the row is not an object, or
// carries no project key (a rename, the F-0 class), or one that is not a string.
func grantFromJSON(b []byte) (GrantRef, bool) {
	var raw map[string]json.RawMessage
	if json.Unmarshal(b, &raw) != nil || raw == nil {
		return GrantRef{}, false
	}
	g := GrantRef{Roles: raw["roles"]}
	found := false
	for _, k := range []string{"project_id", "projectId"} {
		v, ok := raw[k]
		if !ok {
			continue
		}
		var s string
		if json.Unmarshal(v, &s) != nil {
			return GrantRef{}, false
		}
		found = true
		if g.ProjectID == "" {
			g.ProjectID = s
		}
	}
	return g, found
}

// ApplicationRef names the client the token is for (Zitadel v4+). It picks
// the app's own shape (ADR-003); absent or unknown, the project's default
// applies.
type ApplicationRef struct {
	ClientID string `json:"client_id"`
}

// Request is what Syndra reads from a Zitadel Actions v2
// `function/preaccesstoken` or `function/preuserinfo` payload. It is decoded
// by Decode, field by field, so the payload Zitadel owns can grow or retype
// what Syndra does not need without costing a token its claims.
type Request struct {
	Function    string         `json:"function,omitempty"`
	User        UserRef        `json:"user"`
	UserGrants  []GrantRef     `json:"user_grants"`
	Application ApplicationRef `json:"application"`
}

// KnownTopLevel is every top-level key Zitadel's ContextInfo has
// (internal/api/oidc/userinfo.go, v4.20.1 and main).
var KnownTopLevel = map[string]bool{
	"function": true, "userinfo": true, "user": true, "user_metadata": true, "org": true,
	"user_grants": true, "application": true, "actor": true, "response": true,
}

// Decode reads a payload. The outcome is "" when the payload is usable,
// bad_request when it is not a JSON object, and contract_fault when it is one
// that breaks the contract Syndra depends on.
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
func Decode(body []byte) (Request, claimtel.Outcome) {
	var top map[string]json.RawMessage
	if err := decodeOne(bytes.NewReader(body), &top); err != nil || top == nil {
		return Request{}, claimtel.BadRequest
	}
	var req Request
	_ = json.Unmarshal(top["function"], &req.Function)
	_ = json.Unmarshal(top["application"], &req.Application)
	var user struct {
		ID      json.RawMessage `json:"id"`
		Created json.RawMessage `json:"creation_date"`
		Human   json.RawMessage `json:"human"`
		Machine json.RawMessage `json:"machine"`
	}
	_ = json.Unmarshal(top["user"], &user)
	_ = json.Unmarshal(user.ID, &req.User.ID)
	_ = json.Unmarshal(user.Created, &req.User.CreatedAt)
	req.User.Human = bytes.HasPrefix(bytes.TrimSpace(user.Human), []byte("{")) &&
		(len(user.Machine) == 0 || string(bytes.TrimSpace(user.Machine)) == "null")
	if strings.TrimSpace(req.User.ID) == "" {
		return req, claimtel.ContractFault
	}

	raw, present := top["user_grants"]
	if !present {
		for k, v := range top {
			if !KnownTopLevel[k] && isArrayOfObjects(v) {
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

// decodeOne decodes exactly one JSON value; a second one is an error.
func decodeOne(body io.Reader, dst any) error {
	dec := json.NewDecoder(body)
	if err := dec.Decode(dst); err != nil {
		return err
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("multiple JSON values in body")
	}
	return nil
}

func isArrayOfObjects(raw json.RawMessage) bool {
	var rows []map[string]json.RawMessage
	return json.Unmarshal(raw, &rows) == nil && len(rows) > 0
}

// Projects is the distinct, non-empty projects a token's grants name, in order.
func (r Request) Projects() []string {
	seen := map[string]bool{}
	var out []string
	for _, g := range r.UserGrants {
		id := strings.TrimSpace(g.ProjectID)
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
