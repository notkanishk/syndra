## ADR-006: Mapping rules fire from Syndra's record, through the outbox

**Date:** 2026-11-05
**Status:** Accepted (owner approved the rework, 5 Nov 2026). Change: `openspec/changes/rules-on-the-pipeline`.

### Context

Every Syndra-mediated Zitadel mutation leaves a trace before the call (CLAUDE.md). Mapping rules did not:
- The webhook orchestrator called the Management API directly.
- It walked one hop where the token compiler walked the whole chain.
- It fired only for changes made outside Syndra, because the webhook drops Syndra's own.

Bundle and rule cascades already followed rules through the closure and the outbox. Direct writes (a grant, an approval, a bulk give, an adopted drift) did not follow them at all.

### Decision

- **One closure.** `access.Closure` / `access.Ends` (`internal/access`, one-path W2a; was `models.RuleClosure`) is the only code that follows rules. A rule role ends when its source ends. The token compiler, every cascade, the access views and the mapping cohort call it.
- **Rules fire on changes to Syndra's own record.** Every direct write goes through `db.enqueueWrites`, which runs it through `db.FollowRules` (wired by services). In the write's transaction, the hook diffs the person's closure before and after. The difference, beyond the write's own roles, is enqueued as `source='rule'` outbox rows. They are delivered with the write, by the same drain, under the same rule (only a role that waits holds a row).
- **Rules do not fire on webhooks.** A Zitadel-side change with no Syndra trace is drift. It is triaged, and when adopted it is a Syndra write, which fires the rules.
- **Removal is the mirror.** The same closure diff follows the chain and keeps what another source still gives.
- The repo guard `TestOnlyTheTracedPathWritesZitadelGrants` allows one writer of Zitadel grants: the outbox drain.

### Consequences

- What Zitadel is sent for a person is the closure the token is compiled from. A live test asserts it.
- A rule cannot fire from untrusted state, and cannot loop: its rows write no ledger, and their echo is Syndra's own.
- With no rules (production today), a direct write costs one extra read, the active rules, and nothing else.
- `confirmation_mode` (bundle and rule) and `app_claim_overrides` are removed in the same change (migration 000078). Under roles the mode decided nothing.
