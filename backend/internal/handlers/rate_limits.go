package handlers

import (
	"net/http"

	"syndra/internal/ratelimit"
)

// rateLimiter is this process's limiter (rate-limits): failed signatures on
// the Actions target and the event webhook, per source, and a member's own
// password and storage writes, per person. nil limits nothing, which is what
// SYNDRA_RATE_LIMITS=off gives.
var rateLimiter *ratelimit.Limiter

// SetRateLimiter installs l; main calls it before the server starts.
func SetRateLimiter(l *ratelimit.Limiter) { rateLimiter = l }

func limiter() *ratelimit.Limiter { return rateLimiter }

// withSelfServiceLimit counts a member's own write against their own bucket,
// at limits no person typing meets (ten a minute, fifty an hour by default):
// it stops a stuck client, never a person. Behind withUserAuth, so the bucket
// is the authenticated subject's and nobody can spend somebody else's.
func withSelfServiceLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if wait := rateLimiter.Person(r, getAdminUserID(r.Context())); wait > 0 {
			ratelimit.Refuse(w, wait)
			return
		}
		next(w, r)
	}
}
