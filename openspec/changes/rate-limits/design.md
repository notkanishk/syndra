# Design: rate limits

Owner rule, 5 Nov 2026: "it should never come in the way of legitimate use." Every decision below follows from it.

## The surface

The surface was checked in code on 5 Nov 2026. Line numbers are at the commit that adds this change.

| Endpoint | Reachable without a Syndra session | Where | Limited |
|---|---|---|---|
| `POST /api/action/inject` (claims container, :8090) | yes, via Caddy `/api/action/*` | `backend/internal/claimserve/door.go:291` | failures per IP |
| `POST /api/action/inject` (backend fallback, :8080) | yes, if Caddy is pointed back at it | `backend/internal/handlers/router.go:437` | failures per IP |
| `POST /api/webhooks/zitadel` | yes, via Caddy `/api/webhooks/*` | `backend/internal/handlers/router.go:433` | failures per IP |
| `GET /healthz/claims` (both), `GET /healthz` | yes (backend's only on the LAN) | `door.go:292`, `router.go:24,27` | no: monitor reads |
| `GET /auth/zitadel`, `GET /auth/callback`, `POST /auth/login`, `POST /auth/logout` (UI) | yes | `ui/src/app/auth/*/route.ts` | no, see proposal |
| `/api/proxy/*` (UI) | needs a session; refused 401 with none, before any backend call | `ui/src/app/api/proxy/[...path]/route.ts:123` | no |
| `POST /api/v1/me/targets/{target}/credential` | member session | `router.go:323` | per person |
| `DELETE /api/v1/users/{uid}/shadow-credential` | member session | `router.go:209` | per person |

There is no separate "set shadow password" route. The Argon2id vault's set endpoint was removed with the LLDAP bridge (`vault.go`, addon-platform group 11). A member's storage password is now the per-target credential route.

## Only failures, decided after verification

The limiter sits inside `claimserve.SignatureCheck`, which all three signed doors share. It runs only after verification has failed against every key and after the keyring refresh. A call that verifies never reaches it. So a valid call from a source that is currently limited still passes. That is the case where an attacker shares an address with Zitadel, or a LAN peer spoofs `X-Forwarded-For` to drain Zitadel's bucket.

A limited failure:
- still lands in its claimtel outcome class (`sig_mismatch` / `sig_missing` / `sig_stale`), so a real key outage stays loud on `/healthz/claims`;
- still calls `onMismatch`, so the keyring nudge that recovers from a key change still fires;
- is answered 429 and is not logged line by line (the counter replaces the log flood).

For Zitadel, 429 and 401 lead to the same outcome: a token without Syndra's claims. So limiting Zitadel's own failures during a key outage changes nothing for a sign-in.

The limiter does not skip the keyring refresh for a limited source. Skipping it would save one Redis GET per forged call. But it would refuse a valid call signed with a key Syndra replaced a moment ago, if it came from a limited address, and that breaks the rule. The refresh is already single-flight per gap (`AwaitNewKeys`).

## Source address

`ratelimit.ClientIP`:
- When the peer is private or loopback (Caddy at the proxy host, or Docker's port mapping), the source is the last `X-Forwarded-For` entry, the one the nearest proxy wrote.
- From any other peer, the header is ignored.

A lying private peer can only move failed calls between buckets.

## Per person

`withSelfServiceLimit` sits behind `withUserAuth`. The bucket belongs to the authenticated subject (`getAdminUserID`), so nobody can spend another person's tokens. It is keyed per endpoint pattern, so the two routes are separate buckets. Every attempt counts, including ones the handler then refuses. At 10 a minute and 50 an hour, only a loop reaches the limit. A request with no subject is not counted; the handler refuses it anyway.

## Token bucket in Redis

- Each rule (`N/Per`) is a bucket of N tokens that refills at N per Per.
- One Lua script (`bucketScript`) checks every rule of a request at once. It takes a token from each only when all have one, and otherwise returns the wait.
- State is a hash `{t, ts}` under `ratelimit:<endpoint>:<ip:|person:><who>:<Per>`, which expires after Per. A bucket idle that long is full anyway.
- The caller supplies the time, so the bucket can be tested. Skew between processes is clamped (`max(now, ts)`).
- `ratelimit.Memory` is the same math in Go. The unit tests run on it, and `TestRedisMatchesMemory` (gated on `SYNDRA_TEST_REDIS_URL`) holds the Lua to it step for step.

## Fail open

Each counted request makes one Redis call, bounded at 50ms. An error or timeout allows the request, increments `fail_open`, and logs once per outage (and once on recovery). Signed calls make no Redis call for the limiter at all, so a Redis outage cannot slow a sign-in through this code.

## Config and kill switch

- Defaults: `ratelimit.DefaultFailures` (30/1m) and `ratelimit.DefaultSelfService` (10/1m, 50/1h).
- `SYNDRA_RATE_LIMIT_FAILURES` and `SYNDRA_RATE_LIMIT_SELF_SERVICE` take `N/duration[,N/duration]`. A value that does not parse keeps the default and logs.
- `SYNDRA_RATE_LIMITS=off` makes `FromEnv` return nil. A nil `*Limiter` allows everything and reports `off: true`.

## Observability

`claimserve.Health.RateLimits` is present on both `/healthz/claims` answers:
- `limited` per endpoint pattern (no ids: patterns, not paths);
- `fail_open`;
- `off`.

These are counts since process start. They change no status.
