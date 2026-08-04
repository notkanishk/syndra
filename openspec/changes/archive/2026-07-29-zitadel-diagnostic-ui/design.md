## Design

### Page shape

`ui/src/app/zitadel/page.tsx` is a single `"use client"` component — matching `/users/page.tsx` and `/projects/page.tsx`. Four sections, each a `Card` with its own local state. No shared global store, no URL state. The page is a tool, not a hub.

```
┌── M2M Health ─────────────────────────────┐
│ [Check connection]  status badge · latency │
│ {raw JSON response}                         │
└────────────────────────────────────────────┘
┌── Projects & Roles ────────────────────────┐
│ Project: [dropdown]      total: 3           │
│ Roles:                                       │
│   admin  · Administrator · ops [Edit][Del] │
│   member · Member        · users [Edit][Del]│
│   + add role  key [_] display [_] group [_] │
└────────────────────────────────────────────┘
┌── Users & Grants ──────────────────────────┐
│ User: [search / dropdown]   total: 12       │
│ Grants:                                      │
│   syndra:admin  [Edit][Revoke]              │
│   lab-3d:member [Edit][Revoke]              │
│   + assign grant  project [_] roles [_]    │
└────────────────────────────────────────────┘
┌── All Grants ──────────────────────────────┐
│ [Refresh]   total: 42                       │
│ user │ project │ roles                      │
└────────────────────────────────────────────┘
```

All inline forms are small — one row of inputs per action. Success/error shows as a small message line under the form (3-second auto-clear) so the operator never has to open devtools.

### API client

All calls go through the existing `/api/proxy/*` — no direct access to `BACKEND_URL` from the browser. The proxy reads the session, attaches the Zitadel JWT (OIDC) or `SYNDRA_API_KEY` (demo), and forwards.

Concrete paths:

| UI action | Proxy path | Method |
|-----------|-----------|--------|
| Health probe | `/api/proxy/zitadel/health` | GET |
| List projects | `/api/proxy/zitadel/projects?limit=500` | GET |
| List roles | `/api/proxy/zitadel/projects/{id}/roles?limit=500` | GET |
| Create role | `/api/proxy/zitadel/projects/{id}/roles` | POST |
| Update role | `/api/proxy/zitadel/projects/{id}/roles/{key}` | PUT |
| Delete role | `/api/proxy/zitadel/projects/{id}/roles/{key}` | DELETE |
| List users | `/api/proxy/zitadel/users?limit=500` | GET |
| List user grants | `/api/proxy/zitadel/users/{id}/grants?limit=500` | GET |
| Assign grant | `/api/proxy/zitadel/users/{id}/grants` | POST |
| Update grant | `/api/proxy/zitadel/users/{id}/grants/{gid}` | PUT |
| Remove grant | `/api/proxy/zitadel/users/{id}/grants/{gid}` | DELETE |
| All grants | `/api/proxy/zitadel/grants?limit=500` | GET |

Pagination is intentionally sidelined in this PoC — `limit=500` matches `zitadel.DefaultSearchLimit` and is enough for makerspace scale. The underlying endpoints return `{items, total, limit, offset}`, and the UI surfaces `total` next to each list so truncation is visible.

### Proxy DELETE

Mirror the existing `PUT` handler verbatim, adjusting only the method. Reuses the same auth path (`isMemberAllowed` already returns `false` for anything that isn't GET/POST, so non-admins are blocked by default — exactly right for destructive operations).

### Health auth unification

`/api/v1/zitadel/health` currently uses `withAPIKeyAuth` because the first use case was a cmdline smoke test pre-UI. The UI routes bearer tokens through the proxy; those are Zitadel JWTs, not the shared key. Switching to `withOperatorAuth` keeps the UI working and aligns health with the rest of `/zitadel/*` (operators only). Dev-mode cmdline tests continue to work because `withUserAuth` still falls through to API key when `ZITADEL_DOMAIN` is unset.

### Admin gating

The proxy already rejects non-admin requests to `/zitadel/*` (paths not in `isMemberAllowed` default to `403` for non-admins). The sidebar link is rendered only inside the `isAdmin` branch. The page itself doesn't duplicate the check — consistent with every other admin page in the app.

### Styling

Only existing primitives:
* `Card`, `CardHeader`, `CardTitle` from `@/components/ui/Card`
* `Badge` for status
* Tailwind theme tokens (`bg-surface`, `text-muted`, `border-border`, `bg-primary`)
* No new fonts, no new colors, no new icons

### What is intentionally NOT built

* User create / update / delete — Zitadel has its own user-management console; Syndra doesn't own that surface.
* Project create / update / delete — same reason.
* Pagination controls — defer until scale demands it.
* Search / filter UI beyond the project / user dropdown.
* Optimistic updates — after every mutation, refetch the affected list. Simpler, and failures are visible.
* Toast framework — small inline message lines are enough for a diagnostic tool.
