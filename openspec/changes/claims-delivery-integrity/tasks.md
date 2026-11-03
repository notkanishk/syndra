# Tasks: claims delivery integrity

Slices from `assessment.md` §5. Each ships alone and has a named proof.

- [x] S0a: read `project_id` (PR #64, 3 Nov 2026). Proof still to record: after a real sign-in, `GET /healthz/claims` shows `outcomes_10m.served` > 0 (the per-token `Cache hit` log line is gone since S0/S1; a hit is a count now). #ready-for-human
- [x] S0: stop the bleeding (branch `feat/integrity-s0-s1`, 3 Nov 2026).
  - [x] F-1: `db.NewRedisClient` is the one constructor: `ContextTimeoutEnabled`, dial/read/write 200ms, pool 100ms.
  - [x] F-2: one 250ms deadline per call; shapes from Postgres 150ms, failure mode 100ms, both under it. A degraded answer past the deadline counts `deadline_exceeded`.
  - [x] F-10: only `redis.Nil` (or a malformed entry) reads Postgres for shapes; Redis erroring answers the failure mode, and the failure-mode read skips its write-back.
  - [x] F-11: `periodic.Recover` guards the claimdebug worker, add-on refresh goroutines, repair worker and every runner; recovered panics are counted (`background_panics`).
  - [x] F-3/F-4: `decodeActionV2` reads field by field. **Decision:** Zitadel's `user_grants` is `omitempty` (ContextInfo, v4.20.1), so *absent* is the normal no-role-audience case and reads `no_grants`. `contract_fault` is absent next to an unknown top-level array of objects (a renamed list), rows naming no project in either spelling (the F-0 class), a non-list, or no `user.id`. `roles`, `function`, `application` of any type never cost a token its claims.
  - [x] F-6: TTL 7d; a skipped or partly failed warm logs `ERROR` with counts and is kept as `cache.LastWarm()`; a Zitadel 404 is gone, counted apart and not asked again this process.
  - [x] F-7: a facts miss enqueues a recompile: deduplicated, queue 4096 (full drops, counted), one worker at 20/s.
  - [x] Every compose service logs `json-file` 10m × 5.
  - Proof: `TestClaimDelivery_FaultInjection` (each through the real router, budget 300ms): Redis hung (accepts, never replies) 104–107ms → `degraded_shape_unreadable` with the project's `minimal_safe` claims; Redis refused 103–105ms, same; Postgres slow 253–256ms → `deadline_exceeded`; Postgres failing 2–3ms → `degraded_shape_unreadable`; panic in the shaper 2ms → 500, `panic`, process alive (three runs). Mutation check: with `ContextTimeoutEnabled` off the hung case takes 407ms and fails.
- [x] S1: see every call (same branch).
  - [x] `withClaimDelivery` wraps the whole route, signature check included: 16 exhaustive classes (`claimtel.Outcomes`), dimensions project, client (or `-`), function, key slot; a refused signature names nothing (its body is nobody's word).
  - [x] Fixed 12-bucket latency histogram; 16-way sharded in-memory counters; (person, client) last-call map capped at 65,536 and pruned at 20 min.
  - [x] Flush every 60s to `claim_delivery_bucket` (5-minute rows, migration 000072), `claim_delivery_last`; a failed flush keeps counters (capped 20,000 rows, drops counted) and shows `telemetry_flush_failures`. Never a per-call row.
  - [x] Nightly roll (>48h) into `claim_delivery_hour`, kept 90 days, on the usage-prune scheduler; one statement, so nothing is counted twice.
  - [x] Payload fingerprint (key paths; full depth for `user_grants` and `application`, one level for `user`, names elsewhere) with the first body of each shape stored redacted in `claim_payload_fingerprints`; a later shape is the informational finding `PAYLOAD_SHAPE_CHANGED`.
  - [x] `GET /api/v1/claims/delivery?window=` (operator): totals by class, rows with p50/p99 bounds, last per app, shapes, findings, telemetry and repair stats.
  - [x] `GET /healthz/claims` (no auth, no ids): `ok | degraded | failing` (503) with reasons. Hard: `sig_stale`, `sig_mismatch`, `contract_fault`, `bad_request`, `panic` in the last 10 min, Redis down. Soft: degraded paths, `deadline_exceeded`, `unknown_function`, a skipped/failing/stale warm, a failing flush, a recovered panic. `sig_missing` is not a reason: Zitadel always signs, and the deploy check sends one.
  - [x] The deploy job checks `/healthz/claims` answers (200 or 503 with a status); the unsigned-401 assertion stays.
  - Proof: `TestClaimDelivery_EveryPathHasItsOutcome`, `TestClaimDelivery_SignatureOutcomes`, `TestClaimsHealth`, `FuzzActionV2Decode`, `FuzzZitadelSignature`, `-race` on `claimtel`, and `TestClaimDelivery_FlushRollWindow` on a real Postgres 15. Still to record in prod: "No project grants" as a per-app `no_grants` number. #ready-for-human
  - Not yet (later slices own them): `served_lkg` is never set until S2; `key_slot` is always `current` until S5; `gen` per call until S2; measured skew on `sig_stale` is in the log line only.
- [ ] S1b: the write side scales. `facts:{u}` hash, compile context once per pass, one directory snapshot per warm, incremental warm, pool size, Redis `maxmemory noeviction`. #ready-for-agent
- [ ] S2: the hot path stands alone. `claimserve` snapshot, write-through config with `claims:gen`, last-known-good, serve-time role-end guard, and the dedicated `:8090` listener started first. #ready-for-agent
- [ ] S2-caddy: Caddy routes `/api/action/*` to `:8090` with `lb_try_duration 2s`. #ready-for-human
- [ ] S3: the correlator, the findings table, the Home line, System › Zitadel "Live", and the dead-man ping. #ready-for-agent
- [ ] S3-alert: create the dead-man's-switch check and give Syndra its ping URL. #ready-for-human
- [ ] S4: the configuration reconciler (R1–R9), System › Zitadel "Configuration", and explain check 6. #ready-for-agent
- [ ] S4-perm: grant Syndra's service account `IAM_OWNER_VIEWER` (read-only) in Zitadel. #ready-for-human
- [ ] S5: a keyring read from Zitadel; rotation without an outage. #ready-for-agent
- [ ] S6: the L1 probe, then the L2 canary; the deploy job checks the probe. #ready-for-agent
- [ ] S6-canary: create the canary project, role and machine user (JWT) in Zitadel. #ready-for-human
- [ ] S7: process split (`cmd/claims` container, narrow Dockerfile, Redis ACL). #ready-for-agent
- [ ] Chaos drill on the dev box (assessment §4h), recorded here. #ready-for-human
