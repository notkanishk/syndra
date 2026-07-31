## Why

Two problems, one change.

**The information architecture sorted by implementation, not by question.** Thirteen top-level admin links across six sidebar sections, with overlapping names (*Users & Access*, *Grants*, *God Mode*, *Operations*, *Policy Engine*). The rail also injected a Drift section at the top whenever its count went non-zero, pushing every other item down under the operator's cursor mid-click. Operators arrive to answer one of five questions; the navigation answered none of them.

**The token simulator previewed a token that did not exist.** `SimulateApplication` built `{iss, sub, aud, source, project, <claim_name>: formatRoles(...)}`. The real Zitadel Actions v2 path (`claimsForProject`) emitted the raw Redis cache map — `roles`, `user_id`, `project_id`, `compiled_at`, `source` — and never read `claim_name` or `format_type` at all. The screen operators use to debug "my app isn't seeing the roles it expects" was showing them a payload no application had ever received, and the claim-format fields in `claim_profiles` were decoration.

Targets **Phase 5** (operator experience) on the roadmap.

## What Changes

**Control plane.** The rail becomes two views of one product — Basic (the everyday surface) and Advanced (the machine) — persisted as `ui_view`, deliberately not `mode`, which is taken. Structure never moves: Advanced *appends* sections, a zero-count row keeps its seat with a hollow zero, and the view switch never changes the URL. Members get two destinations and no switch. Every route gets exactly one home; `/grants` 301s to Review › Unexplained access, its All-grants tab absorbed by People and role membership, which answer the same question with the access source attached.

**Data plane.** Token shaping moves out of the compiler and into a shared `internal/claims` package applied at read time by BOTH the Actions v2 handler and the simulator. The compiler now persists *facts* (roles plus profile attributes); the shape is an operator-editable profile resolved per token, so a claim-name or format edit lands on the very next token rather than on whichever users happened to be recompiled afterwards. Per-application overrides are added, with the honest constraint that Zitadel's function trigger carries no application identifier: a project's token therefore carries the project default and every override key on that project, each application reads its own, and keys are validated unique.

**Bridge plane.** Unchanged. Provisioning intents keep accruing behind an honest "not connected yet" state.

Three previously missing endpoints ship: `GET /governance/indicators` (four scalars, so the rail stops downloading every pending request to render a "3"), `GET /projects/{id}/roles/{key}/members` (role → members, with sources), and `DELETE /users/{id}/grants/{grantId}` (ledger delete plus revoke outbox in one transaction — deliberately not the Zitadel-side delete, which removes a different object and lets the next compile restore the access).

## Impact

- **Affected specs**: application-claims, access-governance, user-management, role-management
- **Affected code**: `backend/internal/{claims,cache,handlers,services,db}`, migration `000018`, and the whole of `ui/src`
- **Behaviour change**: issued tokens now carry operator-configured claim keys instead of the compiler's internal field names. Existing applications reading `roles` keep working — that is still the built-in default.
