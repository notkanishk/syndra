## ADR-002: One project per token

**Date:** 2026-10-28
**Status:** Accepted (owner)

### Context

The Actions v2 target (`POST /api/action/inject`) shapes claims per Zitadel project. Zitadel lists in `user_grants` only the token's role audience (`prepareRoles` and `userinfo_by_id.sql` in Zitadel's `internal/api/oidc` and `internal/query`):
- the client's own project, when the app asserts roles or requests role scopes;
- plus any project the app asks for with `urn:zitadel:iam:org:project:id:{id}:aud` alongside a role scope.

So a token spans two projects only when an app asks for another project's audience.

Until now Syndra merged every listed project into one flat token, namespacing a key to `syndra.<projectID>.<key>` when two projects both emitted it. That made an app's claim key depend on the user's grants elsewhere: an app reading `roles` broke the day its user gained a second project that also emitted `roles`. The core spec, the basic-advanced-ia spec and the code each gave a different rule.

### Decision

- A token carries claims for exactly one project.
- When `user_grants` names more than one distinct project, the target returns `append_claims: []` with HTTP 200, so the token is still issued without Syndra claims. It logs the user, the client id and the projects, and reads nothing.
- Keys are never namespaced.
- Claim keys must be unique within a project (the default, built-in or configured, and every override), not across projects. Two projects may both use `roles`.
- An app that needs a role from another project gets it in its own project through a rule ("if someone holds A, they also get B").

### Consequences

- An app's claims depend only on its own project's state: deterministic, and what the per-app preview shows.
- An app that requests extra project audiences for another reason (say, to call another project's API) gets no Syndra claims. If that need appears, resolve `application.client_id` (Zitadel v4+) to its project and deliver that project alone.
- A cross-project request is refused, not served wrong: the log line names the client.
