# Tasks: claims delivery integrity

Slices from `assessment.md` §5. Each ships alone and has a named proof.

- [x] S0a: read `project_id` (PR #64, 3 Nov 2026). Proof still to record: prod logs show `Cache hit` after a real sign-in. #ready-for-human
- [ ] S0: stop the bleeding.
  - F-1: Redis client honours deadlines.
  - F-2: a 250ms request deadline and a bounded failure-mode read.
  - F-10: nil vs error.
  - F-11: recover in the background goroutines.
  - F-3/F-4: presence-aware `user_grants`.
  - F-6: warm freshness (TTL 7d, a skipped warm is an error, Zitadel 404 = gone).
  - F-7: repair on miss, deduplicated and rate-limited.
  - Log size caps.
  
  #ready-for-agent
- [ ] S1: see every call. Outcome-class telemetry with minute rollups, `claim_delivery_last`, payload fingerprint and fixture recorder, `GET /api/v1/claims/delivery`, `/healthz/claims`. #ready-for-agent
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
