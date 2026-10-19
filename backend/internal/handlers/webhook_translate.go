package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
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
	case "user.token.added":
		// A user aggregate: the aggregate ID is the person, as for user.human.*.
		if ev.AggregateType != "" && ev.AggregateType != "user" {
			log.Printf("[WEBHOOK] %s on aggregate type %q — ignoring", ev.EventType, ev.AggregateType)
			return WebhookPayload{}
		}
		return mapSignInEvent(ev, ev.AggregateID)
	case "oidc_session.added":
		// The aggregate is the OIDC SESSION, not the person: the only place this
		// event names the user is its own payload. Trusted exactly as far as the
		// grant events' payload userId already is — the delivery is signed — and
		// bounded like every other id taken from a payload.
		var p signInPayload
		_ = json.Unmarshal(ev.EventPayload, &p)
		return mapSignInEvent(ev, p.UserID)
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

// signInPayload is what a sign-in event's event_payload may say about who and
// where. oidc_session.added: userID, clientID, audience. user.token.added:
// applicationId, audience. encoding/json matches keys case-insensitively, so
// `clientId` also takes `clientID`.
type signInPayload struct {
	UserID        string          `json:"userID"`
	ApplicationID string          `json:"applicationId"`
	ClientID      string          `json:"clientId"`
	ClientIDSnake string          `json:"client_id"`
	Audience      json.RawMessage `json:"audience"`
}

// Bounds for everything read out of a sign-in payload. The delivery is signed,
// but a bound costs nothing and keeps one odd event from becoming a row or a
// log line of arbitrary size.
const (
	maxSignInUserIDLen = 128
	maxSignInRefLen    = 256
	maxSignInAudience  = 8
)

// mapSignInEvent builds the user_signed_in payload. No user id, or one that
// fails the bounds, means "not usable": the event is ignored like an unknown
// one rather than recorded against a made-up person.
func mapSignInEvent(ev zitadelEventPayload, userID string) WebhookPayload {
	if !validSignInUserID(userID) {
		log.Printf("[WEBHOOK] %s without a usable user id aggregate=%s — ignoring", ev.EventType, ev.AggregateID)
		return WebhookPayload{}
	}
	var p signInPayload
	_ = json.Unmarshal(ev.EventPayload, &p)
	var refs []string
	add := func(s string) {
		if validSignInRef(s) {
			refs = append(refs, s)
		}
	}
	// The explicit application first; the audience only as a fallback, since a
	// token's audience can name more than the application that asked for it.
	add(p.ApplicationID)
	add(p.ClientID)
	add(p.ClientIDSnake)
	var aud []string
	if json.Unmarshal(p.Audience, &aud) != nil {
		var one string
		if json.Unmarshal(p.Audience, &one) == nil {
			aud = []string{one}
		}
	}
	for i, a := range aud {
		if i == maxSignInAudience {
			break
		}
		add(a)
	}
	return WebhookPayload{EventType: "user_signed_in", UserID: userID, ClientRefs: refs}
}

// validSignInUserID is a non-empty run of visible ASCII, at most 128 bytes.
func validSignInUserID(s string) bool {
	if s == "" || len(s) > maxSignInUserIDLen {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < 0x21 || s[i] > 0x7e {
			return false
		}
	}
	return true
}

// validSignInRef is a non-empty, valid-UTF-8 string without control characters,
// at most 256 bytes. Spaces are allowed: a client id ends in a project name.
func validSignInRef(s string) bool {
	if s == "" || len(s) > maxSignInRefLen || !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
