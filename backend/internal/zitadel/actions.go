package zitadel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// Read-only views of Zitadel's Actions configuration, for the claims
// configuration reconciler and the signing keyring (claims-delivery-integrity
// S4, S5). Needs action.target.read and action.execution.read, which
// IAM_OWNER_VIEWER grants; without them every call here is a 403 and the
// caller says it could not check. Nothing here writes.

// ActionTarget is one Actions v2 target as /v2/actions/targets/search returns
// it. SigningKey comes back decrypted (assessment Z-l): never log it, store it
// only in the data plane's keyring.
type ActionTarget struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Endpoint    string          `json:"endpoint"`
	Timeout     string          `json:"timeout"`
	PayloadType string          `json:"payloadType"`
	ChangeDate  string          `json:"changeDate"`
	SigningKey  string          `json:"signingKey"`
	RestCall    *RestCall       `json:"restCall"`
	RestWebhook json.RawMessage `json:"restWebhook"`
	RestAsync   json.RawMessage `json:"restAsync"`
}

// RestCall is the restCall target type's settings.
type RestCall struct {
	InterruptOnError bool `json:"interruptOnError"`
}

// Type is restCall, restWebhook, restAsync or "".
func (t ActionTarget) Type() string {
	switch {
	case t.RestCall != nil:
		return "restCall"
	case len(t.RestWebhook) > 0 && string(t.RestWebhook) != "null":
		return "restWebhook"
	case len(t.RestAsync) > 0 && string(t.RestAsync) != "null":
		return "restAsync"
	}
	return ""
}

// ActionExecution is one execution: a condition and the targets it calls, in
// order.
type ActionExecution struct {
	Condition struct {
		Function *struct {
			Name string `json:"name"`
		} `json:"function"`
		Event *struct {
			Event string `json:"event"`
			Group string `json:"group"`
			All   bool   `json:"all"`
		} `json:"event"`
	} `json:"condition"`
	// Targets are ids. v2 lists them as strings; v2beta as {"target": id}.
	Targets []json.RawMessage `json:"targets"`
}

// FunctionName is the function condition's name without its "function/"
// prefix, or "".
func (e ActionExecution) FunctionName() string {
	if e.Condition.Function == nil {
		return ""
	}
	return strings.TrimPrefix(e.Condition.Function.Name, "function/")
}

// EventName is the event condition's event, or "".
func (e ActionExecution) EventName() string {
	if e.Condition.Event == nil {
		return ""
	}
	return e.Condition.Event.Event
}

// TargetIDs are the execution's target ids, in order.
func (e ActionExecution) TargetIDs() []string {
	out := make([]string, 0, len(e.Targets))
	for _, raw := range e.Targets {
		var id string
		if json.Unmarshal(raw, &id) == nil {
			out = append(out, id)
			continue
		}
		var obj struct {
			Target string `json:"target"`
		}
		if json.Unmarshal(raw, &obj) == nil && obj.Target != "" {
			out = append(out, obj.Target)
			continue
		}
		out = append(out, string(raw)) // something else: kept, so it never compares equal
	}
	return out
}

// TokenAction is one Actions v1 script in the Complement Token flow.
type TokenAction struct {
	Name   string `json:"name"`
	Script string `json:"script"`
	State  string `json:"state"`
}

// ActionsReader is the read side of Zitadel's Actions. The live client is one;
// a separate interface so the many ZitadelClient fakes need not grow.
type ActionsReader interface {
	ListActionTargets(ctx context.Context) ([]ActionTarget, error)
	ListActionExecutions(ctx context.Context) ([]ActionExecution, error)
	// TokenFlowActions is every v1 action on the Complement Token flow.
	TokenFlowActions(ctx context.Context) ([]TokenAction, error)
}

// Actions is the live reader, or nil without a Management client.
func Actions() ActionsReader {
	r, _ := MgmtClient.(ActionsReader)
	return r
}

const (
	actionsPageSize = 100
	actionsMaxPages = 50 // 5,000 targets or executions: far past any instance
)

// searchAll pages a v2 search endpoint whose results are under "result".
// searchAll pages a v2 actions search. The list sits under a key named for
// the resource ("targets", "executions"), not "result": reading "result" saw
// an empty list on Zitadel v4 and reported the claims target missing.
func searchAll[T any](ctx context.Context, c *managementClient, path, listKey string) ([]T, error) {
	var out []T
	for page := 0; page < actionsMaxPages; page++ {
		body := map[string]any{"pagination": map[string]any{"offset": strconv.Itoa(page * actionsPageSize), "limit": actionsPageSize, "asc": true}}
		resp, err := c.doRequest(ctx, http.MethodPost, path, body)
		if err != nil {
			return nil, err
		}
		raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		var res map[string]json.RawMessage
		if err := json.Unmarshal(raw, &res); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		var pagination struct {
			TotalResult string `json:"totalResult"`
		}
		if p, ok := res["pagination"]; ok {
			_ = json.Unmarshal(p, &pagination)
		}
		list, ok := res[listKey]
		if !ok {
			list, ok = res["result"] // older v2beta responses
		}
		total, _ := strconv.Atoi(pagination.TotalResult)
		if !ok {
			if total > 0 {
				// Zitadel says there are rows but under a key we don't know: a
				// contract change, never "none exist".
				return nil, fmt.Errorf("%s: %d rows under an unknown key", path, total)
			}
			return out, nil
		}
		var page []T
		if err := json.Unmarshal(list, &page); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		out = append(out, page...)
		if len(page) < actionsPageSize || (total > 0 && len(out) >= total) {
			return out, nil
		}
	}
	return out, fmt.Errorf("%s: more than %d pages", path, actionsMaxPages)
}

func (c *managementClient) ListActionTargets(ctx context.Context) ([]ActionTarget, error) {
	return searchAll[ActionTarget](ctx, c, "/v2/actions/targets/search", "targets")
}

func (c *managementClient) ListActionExecutions(ctx context.Context) ([]ActionExecution, error) {
	return searchAll[ActionExecution](ctx, c, "/v2/actions/executions/search", "executions")
}

// flowTypeComplementToken is FLOW_TYPE_CUSTOMISE_TOKEN in the v1 Management API.
const flowTypeComplementToken = "2"

func (c *managementClient) TokenFlowActions(ctx context.Context) ([]TokenAction, error) {
	var res struct {
		Flow struct {
			TriggerActions []struct {
				Actions []TokenAction `json:"actions"`
			} `json:"triggerActions"`
		} `json:"flow"`
	}
	if err := c.getJSON(ctx, "/management/v1/flows/"+flowTypeComplementToken, &res); err != nil {
		return nil, fmt.Errorf("complement token flow: %w", err)
	}
	var out []TokenAction
	for _, t := range res.Flow.TriggerActions {
		out = append(out, t.Actions...)
	}
	return out, nil
}
