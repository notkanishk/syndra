package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// zitadelEventPayload mirrors Zitadel's ContextInfoEvent wire format
// (zitadel/zitadel:internal/repository/execution/queue.go). The JSON tags
// are deliberately mixed-case — Zitadel emits snake_case for some fields
// and camelCase-with-uppercase-ID for others. Match the source exactly.
//
// UserID at the top level is the EDITOR (the user who triggered the event),
// NOT the subject. The subject of grant events lives in event_payload; the
// subject of user.human.* / user.deactivated / user.locked events is the
// aggregateID itself.
type zitadelEventPayload struct {
	AggregateID   string          `json:"aggregateID"`
	AggregateType string          `json:"aggregateType"`
	ResourceOwner string          `json:"resourceOwner"`
	InstanceID    string          `json:"instanceID"`
	Version       string          `json:"version"`
	Sequence      uint64          `json:"sequence"`
	EventType     string          `json:"event_type"`
	CreatedAt     string          `json:"created_at"`
	UserID        string          `json:"userID"` // editor — see comment above
	EventPayload  json.RawMessage `json:"event_payload"`
}

// editorID returns the user ID Zitadel attributes the event to. Used by
// the self-mutation guard. Empty string means no editor was reported.
func (e zitadelEventPayload) editorID() string {
	return e.UserID
}

// userGrantPayload covers user.grant.* event_payload bodies. ProjectID is
// absent on grant.changed; RoleKeys is absent on grant.removed — the
// enrichment pass fills those from the local index / Zitadel API.
type userGrantPayload struct {
	UserID    string   `json:"userId"`
	ProjectID string   `json:"projectId"`
	GrantID   string   `json:"grantId"`
	RoleKeys  []string `json:"roleKeys"`
}

// translateZitadelEvent inspects a request body. If it has a top-level
// "aggregateID" field (the Zitadel ContextInfoEvent shape signal), it
// translates to a WebhookPayload and returns ok=true. Otherwise returns
// ok=false to let the caller fall back to internal-shape strict decoding.
//
// Self-mutation loop guard: when ZITADEL_M2M_USER_ID is set and matches
// payload.userID (the editor), returns (zero, true, errSelfMutation) —
// caller MUST short-circuit with 200 OK and no dispatch.
func translateZitadelEvent(body []byte) (WebhookPayload, bool, error) {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(body, &probe); err != nil {
		return WebhookPayload{}, false, nil
	}
	if _, hasAgg := probe["aggregateID"]; !hasAgg {
		return WebhookPayload{}, false, nil
	}

	var ev zitadelEventPayload
	if err := json.Unmarshal(body, &ev); err != nil {
		return WebhookPayload{}, true, err
	}

	m2mID := os.Getenv("ZITADEL_M2M_USER_ID")
	if m2mID == "" {
		warnSelfMutationGuardDisabled()
	} else if editor := ev.editorID(); editor == m2mID {
		log.Printf("[WEBHOOK] dropped self-mutation event=%s aggregate=%s editor=%s", ev.EventType, ev.AggregateID, editor)
		return WebhookPayload{}, true, errSelfMutation
	}

	out := translateEventName(ev)
	if out.EventType != "" {
		// Stable across Zitadel redeliveries — the dedup key for this event (SC5).
		out.DedupKey = fmt.Sprintf("%s:%s:%d", ev.AggregateID, ev.EventType, ev.Sequence)
		// Evidence for drift triage: who Zitadel says did this, and when it
		// recorded it. Both stay empty when the event omits them.
		out.EditorID = ev.editorID()
		out.EventCreatedAt = parseEventTime(ev.CreatedAt)
	}
	return out, true, nil
}

// parseEventTime accepts Zitadel's RFC3339 created_at and returns nil on
// anything it cannot parse. A missing or malformed timestamp is not an error
// worth failing a webhook over — it only means the drift row cannot say when
// the upstream change happened, which it then doesn't claim to know.
func parseEventTime(raw string) *time.Time {
	if raw == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil
	}
	return &t
}

// warnSelfMutationGuardDisabled emits a one-time process-lifetime warning
// when ZITADEL_M2M_USER_ID is unset on the first Zitadel-shape event. Without
// the guard, backend-initiated Zitadel mutations echo back through Actions v2
// and re-trigger orchestration. Acceptable in local-dev; never in production.
var selfMutationGuardWarnOnce sync.Once

func warnSelfMutationGuardDisabled() {
	selfMutationGuardWarnOnce.Do(func() {
		log.Printf("[WEBHOOK] ZITADEL_M2M_USER_ID unset — self-mutation guard DISABLED (dev mode); backend-initiated mutations may loop")
	})
}

var errSelfMutation = sentinelError("zitadel event triggered by Syndra's own M2M user — dropped")

type sentinelError string

func (e sentinelError) Error() string { return string(e) }

// translateEventName dispatches per-event mapping. Unknown events return a
// zero-value WebhookPayload with EventType="" — the caller MUST treat this as
// "200 OK no-op" (matches the unknown-event passthrough scenario).
func translateEventName(ev zitadelEventPayload) WebhookPayload {
	base := WebhookPayload{UserID: ev.AggregateID}
	switch ev.EventType {
	case "user.human.added", "user.human.selfregistered":
		base.EventType = "user_created"
	case "user.deactivated":
		base.EventType = "user_deactivated"
	case "user.locked":
		base.EventType = "user_locked"
	case "user.grant.added", "user.user.grant.added":
		return mapGrantEvent("grant_added", ev)
	case "user.grant.changed", "user.user.grant.changed":
		return mapGrantEvent("grant_changed", ev)
	case "user.grant.removed", "user.user.grant.removed":
		return mapGrantEvent("grant_removed", ev)
	default:
		log.Printf("[WEBHOOK] unknown event=%s aggregate=%s — ignoring", ev.EventType, ev.AggregateID)
		return WebhookPayload{}
	}
	return base
}

// mapGrantEvent unpacks user.grant.* events. The grant aggregate ID is
// always the top-level AggregateID; we surface it via WebhookPayload.GrantID
// so the enrichment step can correlate index lookups for grant.changed
// (no projectId in payload) and grant.removed (no roleKeys in payload).
func mapGrantEvent(eventType string, ev zitadelEventPayload) WebhookPayload {
	var grant userGrantPayload
	_ = json.Unmarshal(ev.EventPayload, &grant)
	out := WebhookPayload{
		EventType:     eventType,
		UserID:        firstNonEmpty(grant.UserID, ev.AggregateID),
		SourceProject: grant.ProjectID,
		RoleKeys:      grant.RoleKeys,
		GrantID:       firstNonEmpty(grant.GrantID, ev.AggregateID),
	}
	if len(out.RoleKeys) > 0 {
		out.RoleKey = out.RoleKeys[0]
	}
	if grant.ProjectID != "" {
		out.ProjectIDs = []string{grant.ProjectID}
	}
	return out
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
