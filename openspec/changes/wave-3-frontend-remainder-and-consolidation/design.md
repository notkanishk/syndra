# Wave 3 — Design Decisions

Refactor/cleanup wave; few requirement changes. This records the non-obvious calls only.

## Decision 1 — U1 resolver: full-catalog fetch, interface preserved

The design doc says "single full-catalog fetch". Reality needs **two** fetches: `GET /api/v1/catalog` returns `{users, projects (roles nested), applications}` but **no bundles**; `GET /api/v1/bundles` supplies bundle names. Both run once on `<NameResolverProvider>` mount.

**They MUST be two independent React Query calls, not one `Promise.all`.** The provider is mounted globally (`providers.tsx:30`), so it runs for members too — and the proxy allowlist (`api/proxy/[...path]/route.ts:11-25`) permits member `GET /catalog` but **forbids member `GET /bundles`** (403). If the two fetches shared one query (or a combined `isLoading`), the bundles 403 would reject the whole query and members would lose user/project/role resolution on every page ("My Requests" renders `<ProjectName>`/`<RoleName>`). Independent queries isolate the failure:

- Users/projects/roles come from the catalog query; `resolveUser/Project/Role.resolved = !catalogQuery.isLoading`.
- Bundles come from a separate query with `retry: false`; `resolveBundle.resolved = !bundlesQuery.isLoading`. On the member 403 the bundles query settles to `isError` with `data: undefined`, so `resolveBundle` returns `{ value: undefined, resolved: true }` → the `<BundleName>` fallback renders. Members don't surface bundle names on their own pages (`<BundleName>` lives in admin operations/propagation surfaces), so this soft-fail is invisible to them and matches today's behavior.

Net effect for members: this change is a **net improvement** — the member-allowed `GET /catalog` gives them real user/project/role names where today's member-forbidden `POST /lookup` gave them fallback for everything. The proxy boundary is left untouched (no member `GET /bundles`, no new endpoint) — the resolver tolerates the boundary rather than widening it.

**The public contract is frozen.** Consumers (`UserName`, `RoleName`, `BundleName`, `ProjectName`, `GrantsClient`, `audit/page.tsx`) must not change. That means the replacement keeps:
- `ResolveResult<T> = { value: T | undefined; resolved: boolean }` tri-state.
- `resolved:false` **only** while the initial catalog load is in flight → consumers render skeletons.
- `resolved:true, value:undefined` once loaded but id absent → consumers render fallback.
- The no-provider fallback returning `{value:undefined, resolved:true}` for all resolvers.
- `prefetch(ids)` still exported, now a no-op.

**Why this is simpler, not just different:** for the makerspace audience (≤ ~200 users, ~10 projects, ~10 bundles) the entire name catalog is a few KB. Fetching it once and reading a `Map` deletes the rAF scheduler, the batch-coalescing, the per-batch cache keys, and the loading-race handling — ~409 lines collapse to ~120. The `POST /lookup` endpoint and its handler are left in place (still used server-side / by other potential callers); only the client resolver stops calling it.

**Invalidation:** the catalog query key is invalidated on user create and user delete (the two mutations that change the resolvable set operators notice immediately). Project/role/bundle churn is rare and picked up on next mount or manual refetch — not worth wiring extra invalidations (YAGNI).

## Decision 2 — U5: `preserveErrorBody` flag, not a second function

`apiGetDiagnostic` differs from `apiGet` in exactly one way: it returns the parsed JSON body on non-2xx instead of throwing (the health probe wants the error payload to render diagnostics). Rather than port a parallel diagnostic function, add one option to the existing `request<T>`:

```ts
// api-client.ts RequestInitJSON gains:
preserveErrorBody?: boolean; // when true, non-2xx returns parsed body instead of throwing ApiError
```

The branch lives at the existing throw site (`api-client.ts:81`): `if (!res.ok) { if (init?.preserveErrorBody) return parsed as T; throw new ApiError(...) }`. One flag, one branch — the diagnostic path and the normal path share all transport code. Only the health probe passes the flag.

## Decision 3 — U5 split boundaries follow the existing sections

The page already renders five self-contained sections (`HealthSection`, `RotationStatusSection`, `ProjectsSection`, `UsersSection`, `AllGrantsSection`). The split is mechanical: one section → one file under `components/zitadel/`, exporting a default (client) component; the sections own their data hooks and CRUD. This is "files that change together live together."

**The split also closes a live gating gap (found during P2 review).** Today `app/zitadel/page.tsx` is a monolithic `"use client"` component gated by *neither* middleware (it is not in `ADMIN_ONLY_PATHS`) *nor* a page-level guard — a member is stopped only by the proxy 403'ing its data calls, but the admin surface still hydrates. Every sibling admin page (`/grants`, `/operations`, `/governance/*`) uses a server-component `page.tsx` that guards then renders a client island. The U5 split makes the new `app/zitadel/page.tsx` follow that exact pattern:

```tsx
export default async function ZitadelPage() {
  const session = await getSession();
  if (!session) redirect("/login");
  if (session.role !== "admin") redirect("/");
  return <ZitadelDiagnostics />; // client island composing the five sections
}
```

This is a security fix (defense-in-depth: the surface never hydrates for members) folded into the file U5 already rewrites — not new scope. Its regression test lives in Task 8 (where the guard is introduced), not Task 6.

## Decision 4 — D6: full-vertical removal, keep title/team

The operator chose to drop `location` (not keep it despite Wave 1 rendering it). "Dropping from code" elegantly means removing the whole vertical so no dead field lingers: backend model + directory metadata read + demo + profile handlers, the OIDC `ProfileMetadata`/`fetchProfileMetadata`/callback write, the session cookie + `SessionUser` shape + demo data + legacy decode, and the two render sites. `title` and `team` travel the identical path and stay — they are the proof the path itself is sound, so removing `location` is a field-level excision, not a path teardown.

**Spec scope is bounded.** Only living docs are edited: this change's `user-management` spec delta (MODIFIED requirement), `feature-coverage.md`, and the field's owning change `live-directory-identity-completeness`. Shipped/archived intent (`wave-1-production-trust-hardening/specs/operational-readiness/spec.md`) is historical record and is NOT rewritten — retroactively editing what a shipped change claimed is dishonest bookkeeping.

## Decision 5 — R2: client-side join, no new query

The pending outbox list (`usePendingPropagations()` → `PendingRow[]`) is already fetched for the dashboard. Rendering the inline tag needs a membership test `(user_id, project_id, role_key) ∈ pending-adds`. Build that `Set` with a `useMemo` over the existing query — matching `PendingRow.role_keys[]` for `op_type:"add"` — rather than adding a per-grant server query. At this scale the aggregate list is tiny and already in cache. A dedicated query would be speculative server work for a UI hint (YAGNI).

The tag lands in the post-U5 `components/zitadel/Users.tsx` (the operator's per-user grant editor, where a just-queued grant is what §5.2's "affected grant" refers to). Sequenced after U5 so it targets the split file.

## Decision 6 — R3: extract the superset, keep TOLERATE_404

The two `zitadel_api()` copies are near-duplicates: `register.sh`'s has an extra `ZITADEL_API_TOLERATE_404` branch (idempotent `--remove` cleanup); `rotate.sh`'s lacks it. The shared `scripts/lib/zitadel-api.sh` keeps the superset — the TOLERATE_404 branch is inert unless the caller sets the env var, which `rotate.sh` never does. This is the correct de-dup: one implementation, behavior gated by a variable the caller opts into. Wave 2 · Part 3 deferred this (Decision 4, "single-consumer") — that rationale expired the moment `rotate.sh` grew its own copy.

## Decision 8 — Admin routes use two enforcement mechanisms; U7 tests both

The admin/member boundary on the frontend is enforced two ways, deliberately, and U7's regression coverage must exercise both (P2 review — the original plan only tested the middleware list):

| Mechanism | Routes | How |
|---|---|---|
| **Middleware** (`middleware.ts` `ADMIN_ONLY_PATHS`) | `/applications`, `/audit`, `/bundles`, `/graph`, `/policies`, `/projects`, `/users` | `role:"user"` → redirect `/` before the route renders |
| **Page-level server guard** (`page.tsx` `getSession()` → `redirect`) | `/operations`, `/operations/cascades`, `/governance/pending`, `/governance/drift`, `/grants`, and `/zitadel` (added by U5) | non-admin → `redirect("/")`, no session → `redirect("/login")`; the client island never hydrates |

Non-admin routes (`/`, `/login`, `/requests`) are intentionally ungated (member-reachable). The two mechanisms coexist because middleware can only match static path prefixes cheaply, while pages that already need a server component for data-gating fold the auth check into that server pass. New admin pages should pick one mechanism deliberately.

**U7 coverage:** Task 6 tests the middleware list (all 7 paths → member redirect) *and* the five existing page-gated routes (member session → `redirect("/")`, no session → `redirect("/login")`) by mocking `getSession` + `next/navigation`'s `redirect` and invoking each server-component `page.tsx`. `/zitadel`'s guard + guard-test are added in Task 8 alongside its introduction. This makes the boundary a tested invariant, not two conventions a future page could silently skip (as `/zitadel` did).

## Decision 7 — Task ordering

Size-ascending with dependency edges honored:
- `D6` touches `session.ts` (also touched by `U6`) — U6 lands first, D6 later; no conflict (different lines, sequential).
- `R2` depends on `U5` (targets the split `Users.tsx`) — R2 after U5.
- `U1` is isolated to `useNameResolver.tsx` — order-independent.
- Consolidation and the verification gate are last.
