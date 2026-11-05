# Rules on the pipeline

**Why.** Mapping rules ("whoever holds A also gets B") reached Zitadel by a path no other access change used. The owner approved the rework on 5 Nov 2026, before the first rule is created in production (production had 0 rules, checked read-only).
- **No trace.** `zitadel.EnforceMappingRules` and `RevokeMappingRules` called the Management API directly. They wrote no ledger row, no outbox row and no audit line, and a failure was logged and stepped over. That breaks CLAUDE.md's rule that a Syndra-mediated Zitadel mutation leaves its trace before the call.
- **The wrong trigger.** They ran only from the grant webhook, and the self-mutation guard drops Syndra's own events. So rules fired for grants made by hand in Zitadel, which is drift, and never for the ones Syndra made. A direct grant, an approved request or a bulk give reached Zitadel alone. The token and the person's page still showed the rule targets.
- **One hop.** On a chain A→B→C they granted B and never C, while the claims compiler followed the whole chain. The token promised C, and Zitadel never had it.
- **409 swallowed.** `AddUserGrant` on a person who already had a grant in the target project answered 409, and the error was only logged. The rule role was never written.
- **Add and remove did not match.** Removing a direct grant revoked its rule targets through the closure. Adding one did not grant them.

**What.**
- **One closure.** `models.RuleClosure` is the only implementation that follows rules. The claims compiler, every cascade (through `effectiveClosure`), the access views (`collectUserRoles`) and the mapping cohort (`db.MappingHolders`, moved from SQL to Go) all call it.
- **One chokepoint.** Every direct write (`db.enqueueWrites`) runs through `db.FollowRules`, wired by services. It reads the person's closure, runs the write, and reads the closure again, all in the write's own transaction and under the access lock. The difference, beyond the write's own roles, goes to the outbox as `source='rule'` rows.
  - These rows go through the same outbox and drain every bundle and rule cascade uses: trace first, idempotent, retried, and delivered with the write's own row.
  - The drain adds a role to an existing grant by updating it, so a 409 cannot drop a rule role.
- **Symmetric.** A removal follows the whole chain. It keeps any role still held another way (a bundle, a direct grant or another rule), because that role is still in the closure afterwards.
- **The webhook fires no rules.** A Zitadel-side change is drift to triage, as `ConvergeProjectChange` already said. Adopting the drift is a Syndra write, and adopting fires the rules. The orchestrator's rule functions, the `StillExpected` hook and the repo guard's KNOWN GAP entry are gone.
- **Folded in: `confirmation_mode`.** Under the roles model, a bundle's or rule's own mode decided nothing (see design). It is removed: the reads, the writes, the global default, the bulk endpoint, the one-time move on Roles that wait, and the UI that guessed outcomes from it.
- **Folded in: `app_claim_overrides`.** It had 0 rows in production. Its one reader (`seedApp`) is removed.
- **Migration 000078** drops both columns, the default setting and the table. It refuses unless `confirmation.model = 'roles'`, and refuses if the table has a row.

**Not in scope** (listed in tasks.md for the next wave): the other "who holds what" readers the audit found (family 3).
