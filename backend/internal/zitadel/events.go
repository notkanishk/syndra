package zitadel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Who made a grant, read from Zitadel's own event log.
//
// The reconciliation sweep compares grant SETS, so it can see that a grant
// exists with nothing to explain it and cannot see who created it. "Unknown
// actor" is the honest rendering of that, and it is also the least useful
// sentence on the triage queue.
//
// The add-on targets solve the same problem with a recorded merge base — keep
// what you last saw, infer who moved from the difference. Zitadel does not
// need inferring: it is event-sourced, and the event that created the grant
// carries its editor. Given the choice between deducing an actor from a
// snapshot delta and reading the actor from a log, the log wins, and a merge
// base here would be a worse approximation of something the target can answer
// exactly.
//
// Read ON DEMAND, per row, when somebody opens a finding. Not from the sweep:
// the sweep sees every unexplained grant in the deployment and would turn one
// pass into one API call per row.
//
// ── UNOBSERVED ──────────────────────────────────────────────────────────────
// Written against Zitadel's published schema for `POST /admin/v1/events/_search`
// and NOT against a recorded response from a live instance. Everything this
// package learned the hard way about that distinction applies: the decoding
// below is deliberately tolerant, `type` and `aggregate.type` are OBJECTS
// rather than strings (a naive string field there fails the whole response),
// and every field is optional. First contact with a real Zitadel is expected
// to correct something here.
// ────────────────────────────────────────────────────────────────────────────

// GrantOrigin is the creation event of one user grant.
type GrantOrigin struct {
	// ActorID is the Zitadel user id of whoever made the change, or empty when
	// the event records a service rather than a person.
	ActorID   string
	ActorName string
	// Service names the machine actor when a human one is absent — an Action, a
	// service account, a migration. Distinguishing "a person did this in the
	// console" from "an integration did this" is most of the triage decision.
	Service   string
	EventType string
	At        time.Time
}

// Attributed says the event named somebody. An origin that resolved but
// carries no actor is a real answer — Zitadel recorded the change without an
// editor — and it must not read as a failed lookup.
func (o GrantOrigin) Attributed() bool {
	return o.ActorID != "" || o.ActorName != "" || o.Service != ""
}

// eventsEpoch is the `from` bound. The field is required by the API and the
// question here is "what is the EARLIEST event for this grant", so the bound
// has to precede any deployment rather than being a recent window.
const eventsEpoch = "1970-01-01T00:00:00Z"

// listEventsResponse mirrors the documented shape. Every level is a pointer or
// a struct with optional fields: this response is decoded whole, and one
// unexpected type anywhere in it fails the lot.
type listEventsResponse struct {
	Events []struct {
		Editor *struct {
			UserID      string `json:"userId"`
			DisplayName string `json:"displayName"`
			Service     string `json:"service"`
		} `json:"editor"`
		Type *struct {
			Type string `json:"type"`
		} `json:"type"`
		Aggregate *struct {
			ID string `json:"id"`
		} `json:"aggregate"`
		CreationDate string `json:"creationDate"`
	} `json:"events"`
}

// GrantOriginByID reads the earliest recorded event for one grant aggregate.
//
// Filtered by `aggregateId` alone, deliberately. Zitadel names its aggregate
// types and event types in its own vocabulary, and hardcoding "usergrant" or
// "user.grant.added" here would be this package guessing at a constant it has
// never seen — the exact move that kept the TrueNAS audit read broken for its
// whole life. The grant id is already stored on the drift row, it identifies
// the aggregate uniquely, and asking by it needs no such guess.
func (c *managementClient) GrantOriginByID(ctx context.Context, grantID string) (*GrantOrigin, error) {
	if strings.TrimSpace(grantID) == "" {
		return nil, fmt.Errorf("a grant id is required to read its origin")
	}

	resp, err := c.doRequest(ctx, http.MethodPost, "/admin/v1/events/_search", map[string]any{
		"from":        eventsEpoch,
		"asc":         true,
		"limit":       10,
		"aggregateId": grantID,
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read events response: %w", err)
	}
	var out listEventsResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode events response: %w", err)
	}
	if len(out.Events) == 0 {
		// The aggregate has no events Zitadel will show us. Not an error: the
		// grant may predate the retained event history, and "the log does not
		// go back that far" is an answer.
		return nil, nil
	}

	// Ascending, so the first is the earliest — the creation. Taking the first
	// rather than matching an event-type name for the same reason the filter
	// does not: this package has not seen Zitadel's event vocabulary.
	first := out.Events[0]
	origin := &GrantOrigin{}
	if first.Editor != nil {
		origin.ActorID = first.Editor.UserID
		origin.ActorName = first.Editor.DisplayName
		origin.Service = first.Editor.Service
	}
	if first.Type != nil {
		origin.EventType = first.Type.Type
	}
	if first.CreationDate != "" {
		// Tolerated rather than required: a timestamp this cannot parse costs
		// the timestamp, not the attribution, and the actor is the half that
		// changes what an operator does.
		if at, err := time.Parse(time.RFC3339, first.CreationDate); err == nil {
			origin.At = at.UTC()
		}
	}
	return origin, nil
}
