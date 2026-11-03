# Claims delivery integrity

**Why.** When Syndra's Actions v2 answer fails, apps still sign people in. They just get no claims from Syndra, and nothing is loud (Zitadel calls the target with interrupt-on-error off). The owner (3 Nov 2026): "outside of an LXC shutdown, it should really not have a situation where it just stops working or a bad config somewhere else causes it to silently fail partially or fully… prevention is always better than cure." The scale target is hundreds of active users, thousands in total, and more apps in development.

The assessment (`assessment.md`) found that v2 had **never** delivered: Zitadel sends `user_grants[].project_id`, and Syndra read only `projectId`. That was fixed and deployed in PR #64. It also found timing defects (a Redis client that ignores deadlines, no deadline on the failure-mode read), a claim-less window on every deploy, key rotation that breaks every call until the env is updated, and a cache warm that is O(N²) at scale.

**What.** Prevention first, then detection.
- **Prevent** (ADR-004):
  - The data plane serves an in-memory snapshot, with no Postgres or Zitadel at request time, a last-known-good answer, and a fallback ladder where every rung is counted.
  - It has its own listener, then its own container, so deploys and admin load can't touch it.
  - The signing key is read from Zitadel.
  - The write side scales to thousands of users.
- **Detect:**
  - telemetry of every call, by outcome class;
  - a correlator that checks every sign-in against a served v2 call;
  - a read-only reconciler of Zitadel's targets, executions and app settings;
  - L1 and L2 end-to-end probes;
  - `/healthz/claims`;
  - an external dead-man's switch;
  - a fixed Home line and a Claims delivery section on System › Zitadel.

**Not in scope.** A second host, a message bus or a metrics stack (assessment §4.0b explains why). Interrupt-on-error stays off.
