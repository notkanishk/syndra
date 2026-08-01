> **Status:** Complete | Phase 5 | [< Index](../../INDEX.md)

## Why

Four things, one of which is a defect and three of which are the same defect in a different form: the product knew something and didn't put it on the screen.

**The signed-in operator's name rendered as their Zitadel subject id.** In the shell header, beside Sign out, and in the Today greeting — the two places on the whole product where a string is unambiguously supposed to be a person's name. The cause was one line: `extractSessionFields` parsed the **access token**, which on a default Zitadel instance carries no `name` or `preferred_username` claim, and fell back to `claims.sub`. The callback then stored that as `session.name`. Worse, the same callback already fetched `/api/v1/me/profile`, which returns the directory's `name` and `email`, and `fetchProfileMetadata` discarded both while keeping title/team/status. The real name was fetched at login and thrown away.

The same class of gap existed one level down: `useNameResolver` resolves ids from `/catalog`, which is the directory *as it stands now*. An account deleted since, created seconds ago, or a machine principal that never appears in a user list resolved to nothing and rendered a fallback — even though `POST /api/v1/lookup` would have named it, because the backend's `FindUser` falls through to a direct Zitadel read.

**Assigning a role to a cohort took one navigation per person.** The only grant endpoint was `POST /users/{id}/grants`. Onboarding six new members to the laser lab meant six round trips through People → person → Grant direct access, and there was no way to express "everyone whose access expires this month" as an operation at all.

**"Requests" and "Activity" on a person were signposts, not surfaces.** Both rendered an empty card pointing elsewhere. The Requests link went to `/requests`, which is the queue of everybody's *pending* work — so the one question the tab is opened to answer, "what has this person asked for and what did we decide?", was the one question the destination could not answer, because decided requests are not in the queue. Activity pointed at the global audit log with no way to scope it to the person.

**Today went blank on a good day.** Its contract read "actionable work only — no counts you cannot act on, no charts", which was right about the top of the page and assumed a non-empty queue. Most days the queue is empty, and an operator landing on a hero-sized "Nothing needs you." learned nothing about the space they run — so they went hunting through the nav, which is the navigation the landing page exists to prevent.

Targets **Phase 5** (operator experience). Follows `operator-runbook-surfaces`.

## What Changes

### Track 1 — A name is never an id

- `extractSessionFields` stops falling back to `claims.sub`. It returns `""`, and the sub-as-name path is deleted so it cannot come back.
- `fetchProfileMetadata` carries `name` and `email` through from `/me/profile` instead of dropping them.
- `resolveDisplayName` layers the sources in descending authority: token claim → directory profile → email local-part → nothing. Never the id.
- The shell header falls back to the email address; the Today greeting drops the address entirely rather than greeting a subject id.
- `useNameResolver` batches catalog misses into one `POST /lookup` and caches the result, asking about any given id exactly once. `UserName` renders "Unknown account" with the id on `title` when even that misses.

### Track 2 — Bulk access changes, rehearsed

- `POST /api/v1/grants/bulk` — five operations (`assign_role`, `remove_role`, `assign_bundle`, `remove_bundle`, `extend`) across up to 500 people. Rehearsal is the default; `?apply=true` executes.
- The rehearsal returns a per-person verdict computed against live state: *will change · already in that state · refused · what they keep anyway*. Apply re-rehearses server-side and never trusts a client-supplied plan.
- Per-person failure is isolated and reported; the batch does not abort.
- Every write still flows through `EnqueueDirectGrantPropagation` / the cascade services. No bulk path calls the Zitadel Management API from a handler.
- People gains a **Select** toggle. Off by default: no checkboxes, no bar, the list reads exactly as before. On: select-all spans the whole filter with a stated count and a "select only the N shown" escape hatch.

### Track 3 — Filters in the URL, and the connections they enable

- `UserListItem` gains `key_project_ids`, so a project filter can be exact and link-addressable rather than matching on a display name that can be renamed.
- People reads `?q &project &role &bundle &attention &bulk` from the URL. Every count elsewhere becomes a link into a pre-narrowed, shareable view.
- Role detail gains exactly one outbound link — "Add people to this role" → People in bulk mode, pre-armed. It otherwise stays read-only and source-aware, because it knows *why* each person holds the role and a people list cannot.
- Bundle chips link to the people in that bundle. The person header links to their full audit trail.
- `GET /api/v1/audit?user_id=` filters at the source (actor OR target). `/audit?user=` scopes the page, with a chip out.

### Track 4 — Person tabs, and Today's second zone

- Requests: the person's full history including decisions, with inline approve/deny and no navigation.
- Activity: their audit trail, both directions, grouped by day, admitting when it hits the 200-row cap.
- Today keeps the work zone on top, unchanged and never displaced, and appends **The makerspace**: gaps, health, where access lives, and what happened lately. The half of the old contract worth keeping is enforced — every number is a link into the thing it counts, and there are still no charts.

## Impact

- **Affected specs:** `user-management` (bulk operations, person surfaces, identity rendering), `operational-readiness` (Today's two-zone contract).
- **Affected code:** `ui/src/lib/oidc.ts`, `ui/src/app/auth/callback/route.ts`, `ui/src/lib/queries/useNameResolver.tsx`, `ui/src/lib/people-filters.ts` (new), `ui/src/lib/audit-vocabulary.ts` (new), `ui/src/app/users/page.tsx`, `ui/src/components/people/*`, `ui/src/components/today/Makerspace.tsx` (new), `backend/internal/services/bulk.go` (new), `backend/internal/handlers/bulk.go` (new), `backend/internal/db/validation.go`, `backend/internal/services/views.go`, `backend/internal/models/models.go`.
- **Breaking:** none. `key_project_ids` is additive; `?user_id=` and `?apply=` default to prior behaviour.
