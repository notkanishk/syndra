## ADR-003: One shape per app, picked by client id

**Date:** 2026-11-03
**Status:** Accepted (owner, A-14)

### Context

A token for a project used to carry the project's default shape plus every app override on that project. Claim keys had to be unique across all of a project's apps, because the Actions v2 payload was read as not naming the client. So one app's override reached every app beside it, and two apps on one project could never be told different things: NetBird `groups` in its own words, and an in-house app `roles` in Syndra's.

The payload does carry `application.client_id` (Zitadel v4+, the board's debug frames and ADR-002's consequences). Each app's OIDC client id is fixed when Zitadel creates the app.

### Decision

- A shape belongs to one app. The data plane maps `application.client_id` to the app with that client id on the token's project, and shapes with that app's latest saved version only.
- Absent client id, unknown client, or an app with no shape of its own: the project's default shape. With no default, the built-in `roles` list. The first time this happens for each client, it is logged.
- Each saved version stores its app's client id, so the lookup is one cached read per project and never a Zitadel call on the token path.
- Keys are unique within one shape, not across a project's apps.
- ADR-002 still holds: a token carries one project's roles.

### Consequences

- An app's claims depend only on its own shape and its project's roles, so the preview of one app is the token of that app.
- On a Zitadel that sends no client id, every app on a project gets the project default. That is the old behaviour minus the siblings' keys. Debug mode's first capture shows whether prod sends it.
- An app that existed under Actions v1 is seeded with v1's exact shape, so moving off v1 changes nothing. An app made later starts on its project's default.
