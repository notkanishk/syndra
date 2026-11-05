# Rate limits that never get in the way

**Why.** Syndra has doors anyone on the internet can knock on: the Actions target and the event webhook. Nothing limited a source that knocks with a bad signature over and over. Each knock is logged, sends a "read the keyring again" nudge, and costs a Redis read for the rotation mark. A member's own password and storage writes had no ceiling either, so a stuck client could hammer the TrueNAS add-on.

The owner approved this on 5 Nov 2026 with one hard rule: **"it should never come in the way of legitimate use."**

**What.**
- **Only failures are counted on the signed doors.** A call whose Zitadel signature verifies is never counted. That covers every sign-in, which Zitadel sends from the same few addresses, and Syndra's own L1 and L2 probes, which sign theirs. A call that fails verification, after every key and the keyring refresh, takes a token from its source IP's bucket (default 30 a minute). Past that it is answered 429 instead of 401. This covers:
  - `POST /api/action/inject` in the claims container;
  - `POST /api/action/inject` on the backend;
  - `POST /api/webhooks/zitadel`.
- **A member's own writes have a ceiling no person typing reaches.** It is 10 a minute and 50 an hour, per person, per endpoint. This covers:
  - `POST /api/v1/me/targets/{target}/credential` (the storage password);
  - `DELETE /api/v1/users/{uid}/shadow-credential`.
- **Fail open.** If Redis is down or slower than 50ms, the request is allowed. The fail-open is counted and logged once per outage.
- **429 means the same thing everywhere.** The response has `Retry-After` in whole seconds and `{"error":"RATE_LIMITED","message":"Too many tries. Wait a minute and try again."}`. The UI already shows the backend's message, and the proxy already passes `Retry-After` through.
- **Configuration.** The defaults live in code. `SYNDRA_RATE_LIMIT_FAILURES` (`30/1m`) and `SYNDRA_RATE_LIMIT_SELF_SERVICE` (`10/1m,50/1h`) override them. `SYNDRA_RATE_LIMITS=off` turns every limit off.
- **Observable.** `/healthz/claims`, on both the backend and the claims container, gains `rate_limits`: limited requests per endpoint pattern, `fail_open`, and `off` when the limits are off.

**Not in scope (and why).**
- **Login and callback (`ui/src/app/auth/*`).** No secret is checked there; Google and Zitadel check it. The callback refuses before any outbound call unless the PKCE cookie and state match (`callback/route.ts:74`). Zitadel rate-limits its own token endpoint. Limiting these would add Redis to the UI for no abuse it stops.
- **Bad sessions on the backend.** `/api/v1/*` is not routed by Caddy. It is reached only through the UI's proxy, which refuses a request with no session before calling the backend (`route.ts:123`). Every backend auth failure therefore arrives from the UI container's one address, and most are genuine members whose token expired. A per-IP limit there would turn their 401 (which sends them to sign in) into a 429.
- **Health reads, access requests, and first sign-in.** Health is read by monitors. A request is human-paced and goes to review. First sign-in runs once per sign-in.
