package handlers

import (
	"encoding/json"
	"log"
	"os"
)

// zitadelEventPayload is a lenient struct mirroring the Zitadel Actions v2
// event-trigger payload. Field paths verified empirically — capture a real
// payload via dev-mode pass-through (ZITADEL_EVENT_SIGNING_KEY unset) before
// flipping signature verification on. Unknown fields are ignored to immunize
// the translator against future Zitadel additions.
//
// Editor identity may appear at three locations across Zitadel response
// shapes: top-level editorUserId, aggregate.editorUserId, or editor.userId
// (see design.md note 2). The self-mutation guard probes all three.
type zitadelEventPayload struct {
	Aggregate struct {
		ID            string `json:"id"`
		Type          string `json:"type"`
		ResourceOwner string `json:"resourceOwner"`
		EditorUserID  string `json:"editorUserId"`
	} `json:"aggregate"`
	Event        string          `json:"event"`
	EditorUserID string          `json:"editorUserId"`
	Editor       struct {
		UserID string `json:"userId"`
	} `json:"editor"`
	Payload json.RawMessage `json:"payload"`
}

// editorID returns the first non-empty editor user ID across the documented
// Zitadel payload shapes. Empty string means no editor was reported.
func (e zitadelEventPayload) editorID() string {
	return firstNonEmpty(e.EditorUserID, firstNonEmpty(e.Aggregate.EditorUserID, e.Editor.UserID))
}

// userGrantPayload covers user.grant.* events.
type userGrantPayload struct {
	UserID    string   `json:"userId"`
	ProjectID string   `json:"projectId"`
	RoleKeys  []string `json:"roleKeys"`
}

// translateZitadelEvent inspects a request body. If it has a top-level
// "aggregate" object (the Zitadel-shape signal), it translates to a
// WebhookPayload and returns ok=true. Otherwise returns ok=false to let the
// caller fall back to internal-shape strict decoding.
//
// Self-mutation loop guard: when ZITADEL_M2M_USER_ID is set and matches
// payload.editorUserId, returns (zero, true, errSelfMutation) — caller MUST
// short-circuit with 200 OK and no dispatch.
func translateZitadelEvent(body []byte) (WebhookPayload, bool, error) {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(body, &probe); err != nil {
		return WebhookPayload{}, false, nil
	}
	if _, hasAgg := probe["aggregate"]; !hasAgg {
		return WebhookPayload{}, false, nil
	}

	var ev zitadelEventPayload
	if err := json.Unmarshal(body, &ev); err != nil {
		return WebhookPayload{}, true, err
	}

	if m2mID := os.Getenv("ZITADEL_M2M_USER_ID"); m2mID != "" {
		if editor := ev.editorID(); editor == m2mID {
			log.Printf("[WEBHOOK] dropped self-mutation event=%s editor=%s", ev.Event, editor)
			return WebhookPayload{}, true, errSelfMutation
		}
	}

	return translateEventName(ev), true, nil
}

var errSelfMutation = sentinelError("zitadel event triggered by MkAuth's own M2M user — dropped")

type sentinelError string

func (e sentinelError) Error() string { return string(e) }

// translateEventName dispatches per-event mapping. Unknown events return a
// zero-value WebhookPayload with EventType="" — the caller MUST treat this as
// "200 OK no-op" (matches the unknown-event passthrough scenario).
func translateEventName(ev zitadelEventPayload) WebhookPayload {
	base := WebhookPayload{UserID: ev.Aggregate.ID}
	switch ev.Event {
	case "user.human.added", "user.human.selfregistered":
		base.EventType = "user_created"
	case "user.human.deactivated":
		base.EventType = "user_deactivated"
	case "user.human.locked":
		base.EventType = "user_locked"
	case "user.grant.added", "user.user.grant.added":
		return mapGrantEvent("grant_added", ev)
	case "user.grant.changed", "user.user.grant.changed":
		return mapGrantEvent("grant_changed", ev)
	case "user.grant.removed", "user.user.grant.removed":
		return mapGrantEvent("grant_removed", ev)
	default:
		log.Printf("[WEBHOOK] unknown event=%s aggregate=%s — ignoring", ev.Event, ev.Aggregate.ID)
		return WebhookPayload{}
	}
	return base
}

func mapGrantEvent(eventType string, ev zitadelEventPayload) WebhookPayload {
	var grant userGrantPayload
	_ = json.Unmarshal(ev.Payload, &grant)
	out := WebhookPayload{
		EventType:     eventType,
		UserID:        firstNonEmpty(grant.UserID, ev.Aggregate.ID),
		SourceProject: grant.ProjectID,
		RoleKeys:      grant.RoleKeys,
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
