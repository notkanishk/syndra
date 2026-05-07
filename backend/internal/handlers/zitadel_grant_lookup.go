package handlers

import (
	"context"
	"fmt"

	"mkauth/internal/zitadel"
)

// grantLookupMaxPages bounds the pagination loop so a bug in Zitadel's Total
// reporting (or an exotic mock) cannot spin the lookup forever. A user with
// > maxPages * DefaultSearchLimit grants is well outside the makerspace
// scale this code targets; if it ever matters, switch to a more selective
// API at that point.
const grantLookupMaxPages = 100

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
			break
		}
		offset += len(res.Items)
	}
	return zitadel.UserGrant{}, fmt.Errorf("grant %s not found among %d grants for user %s", grantID, scanned, userID)
}
