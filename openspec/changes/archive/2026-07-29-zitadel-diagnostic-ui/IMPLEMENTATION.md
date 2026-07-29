# Implementation — Zitadel Diagnostic UI

## Summary

Added an admin-only `/zitadel` page that exercises all 12 live Zitadel management endpoints plus the M2M health probe, aligned the health route with the rest of the `/zitadel/*` namespace (now `withOperatorAuth`), and added `DELETE` support to the Next.js proxy so the UI can revoke grants and delete roles.

## Files

### Created

| Path | Purpose |
|------|---------|
| `ui/src/app/zitadel/page.tsx` | Single client component with four sections: M2M Health, Projects & Roles, Users & Grants, All Grants |
| `openspec/changes/zitadel-diagnostic-ui/proposal.md` | Motivation & capability deltas |
| `openspec/changes/zitadel-diagnostic-ui/design.md` | Component layout, API paths, auth model |
| `openspec/changes/zitadel-diagnostic-ui/tasks.md` | Task breakdown (all complete) |

### Modified

| Path | Change |
|------|--------|
| `backend/internal/handlers/router.go` | `/api/v1/zitadel/health` swapped from `withAPIKeyAuth` to `withOperatorAuth` |
| `ui/src/app/api/proxy/[...path]/route.ts` | Added `DELETE` handler; narrowed body parsing to POST/PUT only; extended method union type |
| `ui/src/components/Sidebar.tsx` | New "Operations" section with "Zitadel Diagnostics" link in the admin branch |

## Key Design Choices

1. **Single-file page** — one `page.tsx` with four local components rather than spreading across a folder. Diagnostic tools don't need reusable composition; legibility beats abstraction here.

2. **Refetch on mutation** — after every POST/PUT/DELETE, refetch the affected list rather than trying to merge the mutation result into local state. Simpler; failures are immediately visible.

3. **Inline editors** — `editing` state holds the id of the currently-edited row; only one row is editable at a time per section. No modals, no separate edit pages.

4. **3-second flash messages** — inline `<p>` under each section's form with auto-clear. No toast framework.

5. **Proxy DELETE mirrors PUT** — same auth flow, same forwarding path. Only difference is the body parsing is gated to POST/PUT so DELETE requests don't fail on an empty body.

6. **Health auth unified with the rest of `/zitadel/*`** — all Zitadel management routes now run through `withOperatorAuth`. The cmdline API-key probe still works in dev mode (where `withUserAuth` falls through to `withAPIKeyAuth`).

## Verification

```bash
cd backend && go build ./... && go vet ./... && go test ./...
# → 196 passed in 11 packages

cd ui && bun run lint && bun run build
# → No ESLint warnings or errors
# → Compiled successfully; /zitadel at 4.16 kB (first load 106 kB)
```

Manual smoke test path (post-deploy against `auth.example.org`):

1. Log in as an admin at `http://198.51.100.14/`
2. Visit `/zitadel` via the sidebar → "Operations" → "Zitadel Diagnostics"
3. Click **Check connection** — expect green "ok" badge with domain + latency + projects total
4. Select a project in **Projects & Roles** — expect its roles to render; create/edit/delete a test role
5. Select a user in **Users & Grants** — expect their grants; assign a test grant, edit role keys, revoke
6. Click **Load all grants** — expect a table populated with every grant in the org

## What Remains

Nothing for this change. Future enhancements that were deliberately deferred:

- Pagination controls (list endpoints already report `total`, so truncation is visible)
- User / project CRUD (Zitadel owns that surface)
- Optimistic updates + toast framework (unnecessary for a diagnostic tool)
- Server-component admin gate (proxy enforces admin on `/zitadel/*` already; pattern matches every other admin page)
