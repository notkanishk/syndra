# Tasks: rate limits

Branch `feat/rate-limits`, 5 Nov 2026. Proposal: [proposal.md](proposal.md). Design: [design.md](design.md).

## Backend

- [x] `internal/ratelimit`: rules, env, kill switch, `ClientIP`, `Refuse`, `Stats`, the Redis token bucket (Lua), and `Memory`. Proof:
  - `TestFailuresLimitPerSourceAndRefill`
  - `TestPersonHasBothWindows`
  - `TestFailsOpen`: down and hung, each under 5×50ms.
  - `TestKillSwitchAndEnv`
  - `TestParseRules`
  - `TestClientIP`
  - `TestRefuse`: Retry-After rounds up, minimum 1.
- [x] The Lua bucket matches `Memory` against a real Redis. `TestRedisMatchesMemory` ran green on 5 Nov 2026 against the dev box's legacy `mkauth_redis`, DB 15, through an ssh tunnel. It is skipped without `SYNDRA_TEST_REDIS_URL`.
- [x] `SignatureCheck` takes a `limit` hook and counts only a call that failed after every key and the refresh. It is wired into the claims container (`Process.Limit`, `cmd/claims`), the backend's claims door, and the event webhook. Proof (claimserve):
  - `TestRateLimit_SignedCallsAreNeverLimited`: 1,000 signed calls from one IP in a burst, all 200, none counted.
  - `TestRateLimit_FailuresAreLimitedAndSignedStillPass`: 401 up to the limit, then 429 with Retry-After. A signed call from that IP still gets 200. Outcomes are still recorded, and health shows the count.
  - `TestRateLimit_RedisDownAllowsEverything`: down and hung.
  - `TestRateLimit_KillSwitch`
  - Mutation-checked: counting before verification fails all three of the first.
- [x] `withSelfServiceLimit` on `POST /me/targets/{target}/credential` and `DELETE /users/{uid}/shadow-credential`. Proof: `TestRateLimit_SelfServiceWritesPerPerson`, through the real router: 10 pass, the 11th is 429 with Retry-After and the copy, another person passes, and the stats are per pattern.
- [x] Event webhook wiring and the kill switch through the router: `TestRateLimit_WebhookFailures`.
- [x] `rate_limits` on both `/healthz/claims`.
- [x] The three env vars are passed into the backend and claims services in `docker-compose.yml`, which lists env explicitly, so the kill switch reaches both. They are documented in `.env.example`.
- [x] `Dockerfile.claims` copies `internal/ratelimit` (`TestTheImageCopiesExactlyWhatTheBinaryImports`), and `[RATE LIMIT]` is in the log vocabulary.

## Open

- [ ] Read `rate_limits` on production's `/healthz/claims` after deploy. `fail_open` should be 0 and `limited` empty or small. #ready-for-human
- [ ] Confirm production's Caddy writes the client's address as the last `X-Forwarded-For` entry. Caddy's default does, and DEPLOY.md's site block sets nothing that changes it, but this has not been read off the live box. Without it, every failure would share Caddy's bucket. Signed calls would be unaffected either way. #ready-for-human
