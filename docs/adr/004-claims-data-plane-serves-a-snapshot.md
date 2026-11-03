## ADR-004: The claims data plane serves a snapshot

**Date:** 2026-11-03
**Status:** Accepted (owner: "prevention is always better than cure")

### Context

Zitadel calls Syndra's Actions v2 target on every token, with a 3s timeout and interrupt-on-error off. Any slow or failed answer silently issues the token without Syndra's claims. Today each answer reads Redis and, on a cache miss, Postgres. Every deploy restarts the process that answers. The signing key is env-only, while Zitadel rotates it with no grace period. The target scale is hundreds of active users and thousands in total. Detail: `openspec/changes/claims-delivery-integrity/assessment.md`.

### Decision

- At request time the data plane reads only memory and one bounded Redis read. Never Postgres, never Zitadel.
- Config (shapes, failure modes, the client→app map and the keyring) is an immutable, versioned snapshot. It is reloaded when `claims:gen` moves, and written through by the write side.
- A token's roles are the ones Zitadel lists for the project in the payload's `user_grants` (amended 4 Nov 2026, below). Syndra's facts only enrich them.
- Facts come from one `HGET facts:{u} {p}` with a real deadline. A last-known-good in-process copy answers while Redis is out, within a per-project staleness bound. With no facts, the shape is answered over Zitadel's roles alone.
- Every answer, every fallback rung included, is counted by outcome class. None is silent.
- The data plane has its own listener. At the design scale it moves to its own container, so backend deploys and admin load do not touch it.
- The signing key is read from Zitadel (the one place it is written) and followed on rotation.

### Consequences

- The hot path gets smaller and is reproducible: each answer records the snapshot `gen` it used.
- A shape or mode save must write through and bump `claims:gen`. A save that skips this is a bug the equivalence test catches.
- Interrupt-on-error stays off. Loudness comes from detection (the correlator, reconciler, probes and dead-man's switch), not from failing sign-ins.

### As built (S1b and S2, 3 Nov 2026)

- The equivalence property test (`services/claim_equivalence_test.go`) and a test over every write path (`TestEverySavePathMovesTheGeneration`) hold the two consequences above.
- A project absent from the snapshot answers fail closed, never the built-in shape: absence means "not published", and a guess could show a role an app's shape hides. The write side publishes every project Syndra or Zitadel knows, at start and every minute, so absence lasts at most a minute.
- The snapshot also reloads at least once a minute without a `claims:gen` move, so a write whose `INCR` was lost still lands.
- The last-known-good bound is per project (`claim_profiles.claim_lkg_max_age_seconds`, default 6h pending the owner), counted from when the copy was last read from Redis.
- The facts read falls back to the old `mapping:{u}:{p}` key on a miss until every person has moved to the hash; the move deletes a person's old keys in the same transaction that writes their hash.

### As built (S3 to S5, 3 Nov 2026)

- The keyring is part of the snapshot: the write side reads the claims target's key from Zitadel every minute (and at once, at most every 10s, after a call matches no key), publishes `claims:keyring` `{current, previous until +15 min}` and moves `claims:gen`. The data plane verifies with current, then previous, then the env key, which stays the bootstrap and the fallback. Each call records which (`key_slot`: current, previous, env). A denied read leaves the env key verifying and raises `CANNOT_CHECK`.
- Detection reads, never writes Zitadel: the correlator (sign-ins against the recorder's per-(person, client) spans), the reconciler (`internal/claimcheck`, pure over recorded JSON), and the dead-man ping share one findings table (`claim_findings`, migration 000074). `/healthz/claims` reads the open findings from memory, so the data plane's door still never touches Postgres.

### As built (S6 and S7, 3 Nov 2026)

- The data plane is its own process (`cmd/claims`, the `claims` container on `:8090`), built from a narrow Dockerfile, so a backend deploy does not recreate it. The route's code lives in `claimserve` (`Door`), and the backend's fallback on `:8080` serves the same code with its own hooks.
- The claims process has no Postgres: its telemetry, its calls (for the correlator) and its requests (recompile, keyring read, debug capture) go to Redis, and the backend folds and acts on them. The backend keeps its own snapshot for the detectors.
- The L1 probe exercises the whole public path every minute with a canary that exists only in Redis (published and compiled by the same code as everyone), counted apart from every SLI. L2, a real Zitadel token, is built and stays off until the owner creates its canary.
- Not yet: a Redis ACL user for the claims process.

### As built (welcome roles at a first sign-in, 3 Nov 2026)

- The snapshot also carries `claims:welcome`: Makerspace settings' switch (off by default), the welcome roles per project that wait for nobody, and every app of those projects by client id. It is written through like a shape save.
- With it on, a token naming no project for a person whose Zitadel account is under 10 minutes old (`user.creation_date`, from the payload) is answered from the snapshot and the same one facts read: no facts in the app's project means the app's shape over the welcome roles (`served_welcome`). Still no Postgres or Zitadel on the request path.

### Amended: the roles come from Zitadel's payload (4 Nov 2026)

Found on prod: the compiler builds facts only from Syndra's own records (the ledger, bundles, rules), so a role given in Zitadel directly (the operator's admin role in the Syndra project, grants from before Syndra, console grants) never reached a token. 9 of 23 people had no facts for their project; the operator's own sign-in to Syndra was answered `degraded_no_facts` though Zitadel's payload listed his grant. Actions v1, still on, read Zitadel's grants and hid it. The rule it broke: every holder is observed from Zitadel, and Syndra's records only explain.

- The token's roles are the union of `roles` across the payload's `user_grants` rows for the token's project, deduplicated and sorted (`claims.RoleSet`, via `Request.Roles`). The app's shape (include, rename, nesting, `if_role`) applies to them as before. A role Syndra holds that Zitadel did not list (queued, not yet sent) is not sent.
- `roles` is now read, so a value that is not a list of strings is a `contract_fault`, never a silent `[]`.
- Facts still enrich: names, username, title, team, bundles, end dates, the service-account mark. `role_ends` only ever names a listed role.
- A role past its end date that Zitadel still lists is kept. The expiry sweep ends a role by removing it from Zitadel, and Zitadel's own roles claim carries it until then, so dropping it here would hide the sweep's failure, not end the role (drift reports it). `claims.ShapeAt` and P9's serve-time drop are gone; the one shaper is `claims.Shape`.
- No facts (no entry for the project, or Redis out with no last-known-good copy): the shape over Zitadel's roles alone, without the rows that need facts (`Profile.RolesOnly`: other attributes, bundles, end dates), counted `served_roles_only`. It is delivered for the correlator, and soft in health only while it is persistent (at least 5 in the window and more than the full answers). The miss still asks for a repair. `degraded_no_facts` is no longer answered; malformed facts and a blown deadline still answer the project's failure mode.
- Welcome roles still apply only when the token lists no project at all.
- Previews (the explain step, the simulator) take the roles from Syndra's last observation of Zitadel's grants (`services.ObservedRoles`, the same store the People pages count), and a person with no stored facts is previewed over those roles alone, as served. The equivalence property now draws Zitadel's grants independently of Syndra's facts.
- The correlator's "holds a role here" is the observed grants too: none counts only from a complete, current sweep.
- The L1 canary carries a fact-only claim (`syndra_canary_team`), so a canary answered over its roles alone still reads as wrong claims.
- Still no Postgres or Zitadel on the request path.

