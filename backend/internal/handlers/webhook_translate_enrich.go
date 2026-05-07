package handlers

import (
	"context"
	"errors"
	"log"

	"mkauth/internal/db"
)

// enrichGrantPayload fills in the projectId (and roleKeys for grant_removed)
// that Zitadel omits from grant.changed and grant.removed event payloads.
//
// Strategy:
//  1. Look up the grant aggregate in the local zitadel_grants_index table
//     (populated by prior grant.added events).
//  2. On a miss, fall back to a synchronous Zitadel ListUserGrants call.
//  3. If both fail, return the payload unmodified — handler validation will
//     either reject it (grant.removed without source_project) or process it
//     with whatever fields are present. Best-effort: never bounce a 4xx back
//     to Zitadel from an enrichment miss; that triggers redelivery storms.
//
// Roles from the event itself always win for grant_changed (the event
// represents the new state). For grant_removed, the event has no roles, so
// the cached roles fill in.
func enrichGrantPayload(ctx context.Context, p WebhookPayload) WebhookPayload {
	if p.EventType != "grant_changed" && p.EventType != "grant_removed" {
		return p
	}
	if p.GrantID == "" {
		log.Printf("[WEBHOOK] enrich: missing GrantID, skipping enrichment event_type=%s user=%s", p.EventType, p.UserID)
		return p
	}
	needsProject := p.SourceProject == ""
	needsRoles := len(p.RoleKeys) == 0
	if !needsProject && !needsRoles {
		return p
	}

	// 1) Local index.
	idx, err := dbGetGrantIndex(ctx, p.GrantID)
	if err == nil {
		return applyEnrichment(p, idx.ProjectID, idx.RoleKeys, needsProject, needsRoles)
	}
	if !errors.Is(err, db.ErrGrantIndexNotFound) {
		log.Printf("[WEBHOOK] enrich: index lookup failed grant=%s: %v — falling back to Zitadel", p.GrantID, err)
	}

	// 2) Zitadel API fallback.
	live, lerr := dbListUserGrantsLive(ctx, p.UserID, p.GrantID)
	if lerr != nil {
		log.Printf("[WEBHOOK] enrich: index miss + zitadel lookup failed grant=%s user=%s: %v — payload left unenriched", p.GrantID, p.UserID, lerr)
		return p
	}
	return applyEnrichment(p, live.ProjectID, live.RoleKeys, needsProject, needsRoles)
}

// applyEnrichment writes the supplied projectID/roleKeys onto p only for the
// fields the caller marked as needing enrichment, preserving the event's
// own values otherwise.
func applyEnrichment(p WebhookPayload, projectID string, roleKeys []string, needsProject, needsRoles bool) WebhookPayload {
	if needsProject && projectID != "" {
		p.SourceProject = projectID
		p.ProjectIDs = []string{projectID}
	}
	if needsRoles && len(roleKeys) > 0 {
		p.RoleKeys = roleKeys
		p.RoleKey = roleKeys[0]
	}
	return p
}
