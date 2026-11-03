## ADR-004: The claims data plane serves a snapshot

**Date:** 2026-11-03
**Status:** Accepted (owner: "prevention is always better than cure")

### Context

Zitadel calls Syndra's Actions v2 target on every token, with a 3s timeout and interrupt-on-error off. Any slow or failed answer silently issues the token without Syndra's claims. Today each answer reads Redis and, on a cache miss, Postgres. Every deploy restarts the process that answers. The signing key is env-only, while Zitadel rotates it with no grace period. The target scale is hundreds of active users and thousands in total. Detail: `openspec/changes/claims-delivery-integrity/assessment.md`.

### Decision

- At request time the data plane reads only memory and one bounded Redis read. Never Postgres, never Zitadel.
- Config (shapes, failure modes, the client→app map and the keyring) is an immutable, versioned snapshot. It is reloaded when `claims:gen` moves, and written through by the write side.
- Facts come from one `HGET facts:{u} {p}` with a real deadline. A last-known-good in-process copy answers while Redis is out, within a per-project staleness bound. Then the failure mode answers.
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
