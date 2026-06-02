package handlers

import (
	"context"
	"fmt"
	"log"

	"mkauth/internal/zitadel"
)

// grantLookupMaxPages bounds the pagination loop in the fallback enrichment
// path. At 100 grants per page (Zitadel DefaultSearchLimit), 10 pages cover
// 1000 grants per user — already an order of magnitude beyond what the
// makerspace audience generates (p99 ≈ 100 grants/user). If a future
// deployment regularly hits the cap, the right fix is a more selective
// Zitadel query (search by grantID), not a higher page count.
const grantLookupMaxPages = 10

// listUserGrantsViaZitadel is the fallback path when the local grants_index
// has no row for a grant aggregate ID. Walks pages of Zitadel's
// ListUserGrants until it finds the matching grant (early exit) or the
// listing is exhausted.
//
// Used by enrichGrantPayload to recover projectId/roleKeys for grant.changed
// and grant.removed events whose Zitadel payload omits those fields. The
// caller MUST handle errors gracefully — a missed enrichment must never
// 4xx the webhook back to Zitadel (that would trigger redelivery storms).
func listUserGrantsViaZitadel(ctx context.Context, userID, grantID string) (zitadel.UserGrant, error) {
	if userID == "" || grantID == "" {
		return zitadel.UserGrant{}, fmt.Errorf("listUserGrantsViaZitadel: user_id and grant_id are required")
	}
	scanned := 0
	offset := 0
	drained := false
	for page := 0; page < grantLookupMaxPages; page++ {
		res, err := zitadelListUserGrants(ctx, userID, zitadel.SearchParams{
			Limit:  zitadel.DefaultSearchLimit,
			Offset: offset,
		})
		if err != nil {
			return zitadel.UserGrant{}, fmt.Errorf("list user grants for user=%s offset=%d: %w", userID, offset, err)
		}
		for _, g := range res.Items {
			if g.ID == grantID {
				return g, nil
			}
		}
		scanned += len(res.Items)
		// Mirror the directory.paginate exit conditions: drained when we've
		// caught up to the reported Total or the page came back empty.
		if len(res.Items) == 0 || (res.Total > 0 && scanned >= res.Total) {
			drained = true
			break
		}
		offset += len(res.Items)
	}
	// Distinguish a genuine miss (listing exhausted) from a truncated search
	// (hit the page cap with more grants to go). The latter is invisible in the
	// returned error, so surface it: the grant may exist beyond the cap.
	if !drained {
		log.Printf("[WEBHOOK] grant lookup hit the %d-page cap for user=%s (scanned %d grants) without draining the listing; grant=%s may exist beyond the cap and was not enriched", grantLookupMaxPages, userID, scanned, grantID)
	}
	return zitadel.UserGrant{}, fmt.Errorf("grant %s not found among %d grants for user %s", grantID, scanned, userID)
}
