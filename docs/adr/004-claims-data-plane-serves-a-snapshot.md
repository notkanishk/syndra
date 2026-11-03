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
