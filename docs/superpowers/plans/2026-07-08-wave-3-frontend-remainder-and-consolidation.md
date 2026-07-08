# Wave 3 — Frontend Remainder & Consolidation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the audit-resolution effort — finish the non-palette half of Theme 4 (UI refactors, security tests, one field removal), fold in three Wave 2 residuals, and true up cross-cutting docs to shipped reality.

**Architecture:** Refactor/cleanup wave. No new backend architecture. Simplify the name resolver to a full-catalog `Map`, split a 955-line page along its existing section seams, delete dead code, add the missing security-boundary tests, drop the `location` field vertically, and consolidate the docs.

**Tech Stack:** Go (backend), Next.js + Bun + React Query + Vitest (`ui/`), bash (scripts). Material/obsidian-clarity tokens for any UI touch.

## Global Constraints

- Backend module commands run from `backend/`: `go test ./... && go vet ./...`. Sync untouched this wave.
- UI commands run from `ui/`: `bun run lint && bun run test && bun run build`. Runner is **Vitest** (`vitest run`); node env by default, per-file `// @vitest-environment jsdom` for component/DOM tests; setup `src/test-setup.ts`.
- Proxy/fetch test idiom: `ui/src/test-utils/proxyFetch.ts` (`makeProxyFetch()` → `{fetchImpl, calls, register}`, `respondWith`, `UUID_REGEX`). Mock `next/headers` via `vi.mock`.
- Any UI visual touch uses Material tokens (`bg-tertiary-container`, `text-on-surface-variant`, etc.); no legacy palette classes (a canary test forbids them).
- Keep `title` and `team` everywhere `location` is removed.
- Commit at the end of each task with the audit ref in the message.

---

### Task 0: OpenSpec scaffolding

**Files:**
- Create: `openspec/changes/wave-3-frontend-remainder-and-consolidation/{proposal,design,tasks}.md` (done)
- Create: `.../specs/user-management/spec.md`, `.../specs/production-security-boundary/spec.md` (done)
- Create: this plan doc (done)

- [ ] **Step 1:** `openspec validate wave-3-frontend-remainder-and-consolidation --strict` from repo root. Expected: PASS.
- [ ] **Step 2:** Commit.

```bash
git add openspec/changes/wave-3-frontend-remainder-and-consolidation docs/superpowers/plans/2026-07-08-wave-3-frontend-remainder-and-consolidation.md
git commit -m "docs(openspec): scaffold wave-3 frontend remainder & consolidation plan"
```

---

### Task 1: ⌘K roadmap stub (D2)

**Files:** Modify `openspec/changes/mkauth-core-architecture/ROADMAP.md` (Phase 6, after the Google Workspace poller bullet ~line 81).

- [ ] **Step 1:** Add the bullet immediately after the Google Workspace poller line:

```markdown
- [ ] **Optional: ⌘K command palette**: Struck from the current spec during the May 2026 audit resolution (design.md §5 no longer lists it). Reserved here as an optional Phase 6 nicety if operator navigation demand materializes.
```

- [ ] **Step 2:** Commit.

```bash
git add openspec/changes/mkauth-core-architecture/ROADMAP.md
git commit -m "docs(roadmap): stub ⌘K command palette in Phase 6 (D2)"
```

---

### Task 2: Delete dead `HasPendingDrift` (R1)

**Files:** Modify `backend/internal/db/drift.go` (remove func + doc comment, lines ~121-140).

- [ ] **Step 1:** Confirm zero callers.

Run: `grep -rn "HasPendingDrift" backend/` — Expected: only the definition + its comment. If any caller appears, stop and reassess (it is not dead).

- [ ] **Step 2:** Delete the `HasPendingDrift` function and its doc comment. Leave `PendingOutboxAddExists` (in `propagations.go`) untouched — it is the wired successor.

- [ ] **Step 3:** Build/vet.

Run: `cd backend && go build ./... && go vet ./...` — Expected: clean.

- [ ] **Step 4:** Commit.

```bash
git add backend/internal/db/drift.go
git commit -m "refactor(db): delete dead HasPendingDrift; PendingOutboxAddExists is the successor (R1)"
```

---

### Task 3: Delete dead `lib/api.ts` fetchers (U4)

**Files:** Modify `ui/src/lib/api.ts` (delete lines 70/74/86/90 functions; trim line-2 type imports).

- [ ] **Step 1:** Delete `fetchBundles`, `fetchMappingRules`, `fetchProjects`, `fetchAudit`. Keep `fetchApplications`, `fetchWithAuth`, `fetchSystemMode`, `fetchCatalog`, and internal `fetchServerJson`/`resolveAuthToken`.

- [ ] **Step 2:** Update the import on line 2 to drop now-unused types. Keep whatever the survivors need (`ApplicationView`, `CatalogResponse`):

```ts
import type { ApplicationView, CatalogResponse } from "@/lib/types";
```

- [ ] **Step 3:** Verify nothing references the deleted symbols.

Run: `cd ui && grep -rn "fetchBundles\|fetchMappingRules\|fetchProjects\|fetchAudit" src/` — Expected: no matches.

- [ ] **Step 4:** Lint + build.

Run: `cd ui && bun run lint && bun run build` — Expected: PASS (no unused-import or missing-symbol errors).

- [ ] **Step 5:** Commit.

```bash
git add ui/src/lib/api.ts
git commit -m "refactor(ui): delete dead lib/api.ts fetchers (U4)"
```

---

### Task 4: OIDC avatar fallback (U6)

**Files:**
- Modify: `ui/src/lib/session.ts` (avatar assignment ~line 224).
- Test: `ui/src/lib/__tests__/session.test.ts`.

**Interfaces:**
- Consumes: `nameToAvatar(name)` from `@/lib/oidc`, `payload: OidcSessionCookie` with `name`, `email`, `userId`.
- Produces: a non-empty avatar string for any OIDC payload.

- [ ] **Step 1: Write the failing test** — append to `session.test.ts`:

```ts
it("OIDC avatar falls back to email then userId when name is empty", () => {
  // build an OIDC session cookie payload with empty name, present email
  const s = decodeOidcForTest({ userId: "u-1", name: "", email: "jane.doe@x.edu", role: "user" });
  expect(s.avatar).not.toBe("");
  expect(s.avatar).toBe("JA"); // from email local-part "jane.doe"
});
```
(Use the file's existing helper for constructing a session from an OIDC payload; mirror the neighbouring tests' setup.)

- [ ] **Step 2: Run — verify it fails.** `cd ui && bun run test -- session` — Expected: FAIL (avatar is `""`).

- [ ] **Step 3: Implement.** Add a small seed helper and use it at the OIDC assignment site:

```ts
function avatarSeed(name: string, email: string, userId: string): string {
  if (name.trim()) return name;
  const local = email.split("@")[0] ?? "";
  if (local.trim()) return local;
  return userId;
}
// ...at the OIDC branch (was: avatar: nameToAvatar(payload.name)):
avatar: nameToAvatar(avatarSeed(payload.name, payload.email, payload.userId)),
```

- [ ] **Step 4: Run — verify it passes.** `cd ui && bun run test -- session` — Expected: PASS.

- [ ] **Step 5: Commit.**

```bash
git add ui/src/lib/session.ts ui/src/lib/__tests__/session.test.ts
git commit -m "fix(ui): OIDC avatar falls back name→email→userId (U6)"
```

---

### Task 5: Extract `scripts/lib/zitadel-api.sh` (R3, finishes S2)

**Files:**
- Create: `scripts/lib/zitadel-api.sh`.
- Modify: `zitadel/actions/register.sh` (delete inline def 140-175, add source), `zitadel/actions/rotate.sh` (delete inline def 118-150, add source).

- [ ] **Step 1:** Create `scripts/lib/zitadel-api.sh` containing the **superset** — register.sh's copy (it has the extra `ZITADEL_API_TOLERATE_404` branch). Preserve the 401/403 permission-hint block and curl/mktemp/trap core verbatim. Header comment notes it expects `API_BASE` and `TOKEN` in scope and honours optional `ZITADEL_API_TOLERATE_404`.

- [ ] **Step 2:** In `register.sh`, delete the inline `zitadel_api()` (lines 140-175) and add, after `API_BASE`/`TOKEN` are established (mirror the existing `load-env.sh` source at line 74-77):

```bash
# shellcheck source=../../scripts/lib/zitadel-api.sh
source "${REPO_ROOT}/scripts/lib/zitadel-api.sh"
```

- [ ] **Step 3:** Same deletion (lines 118-150) + source line in `rotate.sh`.

- [ ] **Step 4:** Syntax-check both.

Run: `bash -n zitadel/actions/register.sh && bash -n zitadel/actions/rotate.sh && bash -n scripts/lib/zitadel-api.sh` — Expected: no output (all valid).

- [ ] **Step 5:** Confirm no other inline copy remains.

Run: `grep -rn "^zitadel_api()" zitadel/actions/` — Expected: no matches (definition now only in `scripts/lib/`).

- [ ] **Step 6:** Commit.

```bash
git add scripts/lib/zitadel-api.sh zitadel/actions/register.sh zitadel/actions/rotate.sh
git commit -m "refactor(scripts): extract shared zitadel-api.sh, drop copy-paste (R3, S2)"
```

---

### Task 6: Security-boundary tests (U7)

The admin/member boundary is enforced **two ways** (design.md Decision 8) and both must be covered:
- **Middleware** gates `ADMIN_ONLY_PATHS`: `/applications`, `/audit`, `/bundles`, `/graph`, `/policies`, `/projects`, `/users`.
- **Page-level server guards** gate `/operations`, `/operations/cascades`, `/governance/pending`, `/governance/drift`, `/grants` (each `page.tsx` does `getSession()` → `redirect`). (`/zitadel`'s guard is added + tested in Task 8.)

**Files:**
- Create: `ui/src/middleware.test.ts` (node env), `ui/src/app/api/proxy/[...path]/route.test.ts` (node env), `ui/src/app/__tests__/admin-page-guards.test.ts` (node env — covers the page-gated routes).
- Reference (do not modify): `ui/src/middleware.ts`, `ui/src/app/api/proxy/[...path]/route.ts`, the six page-gated `page.tsx` files, `ui/src/test-utils/proxyFetch.ts`.

**Interfaces:**
- Consumes: `middleware(request: NextRequest)`, the proxy `GET/POST/PUT/DELETE` handlers, each admin `page.tsx` default export, `makeProxyFetch()`, `vi.mock("next/headers")` for `cookies()`, `vi.mock("@/lib/session")` for `getSession`, `vi.mock("next/navigation")` for `redirect`.
- Produces: regression coverage for the guarantees in `specs/production-security-boundary/spec.md`.

- [ ] **Step 1 (middleware): Write failing tests.** Cover, each as its own `it`:
  - `role:"user"` requesting **each** `ADMIN_ONLY_PATHS` entry → 307 redirect to `/` (iterate the list so a new entry is auto-covered).
  - demo/legacy cookie + `process.env.ZITADEL_DOMAIN` set → redirect `/login` with `Set-Cookie` maxAge 0 (assert the response clears the cookie).
  - expired-OIDC (`expiresAt` past) not on `/login` → redirect `/login`.
  - `valid` session on `/login` → redirect `/`.
  - `valid` admin on an admin path → `NextResponse.next()` (no redirect).

Build `NextRequest` with the target URL and a session cookie encoded exactly as `readSession` decodes it (base64url JSON `{type,userId,role,expiresAt}`). Set/clear `process.env.ZITADEL_DOMAIN` per-test in `beforeEach`/`afterEach`.

- [ ] **Step 2 (page guards): Write failing tests** in `admin-page-guards.test.ts`. For each page-gated route, import its `page.tsx` default export and:
  - member session (`getSession` → `{role:"user"}`) → invoking the page calls `redirect("/")`.
  - no session (`getSession` → `null`) → calls `redirect("/login")`.
  - admin session → does NOT redirect (renders its client island).

```ts
import { redirect } from "next/navigation";
vi.mock("next/navigation", () => ({ redirect: vi.fn(() => { throw new Error("REDIRECT"); }) }));
vi.mock("@/lib/session", () => ({ getSession: vi.fn() }));
import { getSession } from "@/lib/session";
import OperationsPage from "@/app/operations/page";
// ...one describe-block per route, driven by a table of [route, importedPage]
it("operations redirects members to /", async () => {
  (getSession as Mock).mockResolvedValue({ id: "u1", role: "user" });
  await expect(OperationsPage()).rejects.toThrow("REDIRECT");
  expect(redirect).toHaveBeenCalledWith("/");
});
```
Cover `/operations`, `/operations/cascades`, `/governance/pending`, `/governance/drift`, `/grants`.

- [ ] **Step 3: Run — verify fail.** `cd ui && bun run test -- middleware admin-page-guards` — Expected: FAIL (assertions written before mocks wired; if any passes immediately, confirm it exercises the branch, not a vacuous assertion).

- [ ] **Step 4 (proxy): Write tests** using `makeProxyFetch()` + mocked `getSession()`:
  - member GET `users/{ownId}/grants` → forwarded (200); `users/{otherId}/grants` → 403 (no fetch issued — assert `proxy.calls` empty).
  - member GET `catalog`/`applications`/`requests` → allowed; member GET `bundles` → 403.
  - member POST `requests` → allowed with `requester_id` forced to session id (assert the forwarded body); member POST/PUT/DELETE anything else → 403.
  - member GET `requests` list → response filtered to `requester_id === session.id`.
  - no session → 401; backend unreachable → 502.

- [ ] **Step 5: Run — verify pass.** `cd ui && bun run test -- proxy middleware admin-page-guards` — Expected: PASS.

- [ ] **Step 6: Commit.**

```bash
git add ui/src/middleware.test.ts "ui/src/app/api/proxy/[...path]/route.test.ts" ui/src/app/__tests__/admin-page-guards.test.ts
git commit -m "test(ui): cover admin/member boundary — middleware, proxy, page guards (U7)"
```

---

### Task 7: Full-catalog name resolver (U1)

**Files:**
- Modify (rewrite): `ui/src/lib/queries/useNameResolver.tsx`.
- Test: `ui/src/lib/queries/__tests__/useNameResolver.test.tsx`.
- Reference: `ui/src/lib/lookup-types.ts` (`ResolvedUser/Project/Role/Bundle`, `roleCompositeKey`), `ui/src/lib/types.ts` (`CatalogResponse`), catalog endpoint `GET /api/v1/catalog`, `GET /api/v1/bundles`.

**Interfaces (FROZEN — must match current exports byte-for-byte in shape):**
- Produces: `NameResolverProvider({children})`, `useNameResolver(): NameResolverContextValue`.
- `NameResolverContextValue = { resolveUser(id): ResolveResult<ResolvedUser>; resolveProject(id): ResolveResult<ResolvedProject>; resolveRole(projectId, roleKey): ResolveResult<ResolvedRole>; resolveBundle(id): ResolveResult<ResolvedBundle>; prefetch(ids: LookupRequest): void }`.
- `ResolveResult<T> = { value: T | undefined; resolved: boolean }`.
- No-provider fallback: every resolver returns `{ value: undefined, resolved: true }`; `prefetch` is a no-op.

- [ ] **Step 1: Confirm the catalog shapes** the resolver will read (do this before coding, not as a placeholder):

Run: `cd ui && grep -n "roles\|role_key\|display_name\|name" src/lib/types.ts | head` and inspect `CatalogResponse.projects[].` for nested roles + `GET /api/v1/bundles` row shape (`useBundles` hook / `Bundle` type). Note the exact fields feeding `ResolvedUser.display_name`/`.email`, `ResolvedProject.name`, `ResolvedRole.display_name`, `ResolvedBundle.name`.

- [ ] **Step 2: Write failing tests** in `useNameResolver.test.tsx` (jsdom env), rendering a probe component under `<NameResolverProvider>` with React Query + a mocked fetch (use `test-utils/proxyFetch.ts` to register `GET catalog` and `GET bundles` separately):
  - while the catalog query is loading → `resolveUser("u1").resolved === false`.
  - after load, known id → `{ value: {display_name,...}, resolved: true }`.
  - after load, unknown id → `{ value: undefined, resolved: true }`.
  - `roleCompositeKey` lookups resolve nested project roles.
  - **member boundary:** with `GET bundles` registered to return **403** (member context) but `GET catalog` returning 200 → `resolveUser`/`resolveProject`/`resolveRole` still resolve to real values, while `resolveBundle(id)` returns `{ value: undefined, resolved: true }` (fallback). This is the regression guard for the P1 proxy-boundary interaction.
  - invalidating the catalog query key (simulate user-create) refetches and newly-present ids resolve.

- [ ] **Step 3: Run — verify fail.** `cd ui && bun run test -- useNameResolver` — Expected: FAIL.

- [ ] **Step 4: Implement.** Replace the rAF batcher with **two independent queries** (NOT a `Promise.all` — a shared query would let the member `GET /bundles` 403 sink user/project/role resolution; see design.md Decision 1):

```tsx
// Catalog: users + projects + nested roles. Member-allowed via proxy.
const catalogQ = useQuery({
  queryKey: ["name-catalog"],
  queryFn: () => request<CatalogResponse>("catalog"),
  staleTime: 5 * 60_000,
});
// Bundles: separate + non-retrying. Member GET /bundles is 403 by design;
// on failure data is undefined → resolveBundle yields fallback, isolated
// from catalog. Members don't surface bundle names on their own pages.
const bundlesQ = useQuery({
  queryKey: ["name-bundles"],
  queryFn: () => request<BundleRow[]>("bundles"),
  staleTime: 5 * 60_000,
  retry: false,
});

const catalogMaps = useMemo(() => buildCatalogMaps(catalogQ.data), [catalogQ.data]); // {users, projects, roles}
const bundleMap = useMemo(() => buildBundleMap(bundlesQ.data), [bundlesQ.data]);      // Map<id, ResolvedBundle>

const ctx: NameResolverContextValue = useMemo(() => ({
  resolveUser:    (id)      => ({ value: catalogMaps.users.get(id),                       resolved: !catalogQ.isLoading }),
  resolveProject: (id)      => ({ value: catalogMaps.projects.get(id),                    resolved: !catalogQ.isLoading }),
  resolveRole:    (pid, rk) => ({ value: catalogMaps.roles.get(roleCompositeKey(pid, rk)), resolved: !catalogQ.isLoading }),
  resolveBundle:  (id)      => ({ value: bundleMap.get(id),                               resolved: !bundlesQ.isLoading }),
  prefetch: () => {}, // no-op: catalog is already complete
}), [catalogMaps, bundleMap, catalogQ.isLoading, bundlesQ.isLoading]);
```

`buildCatalogMaps` maps `catalog.users → ResolvedUser`, `catalog.projects → ResolvedProject` + iterates nested roles into the role Map keyed by `roleCompositeKey`. `buildBundleMap` maps the `/bundles` rows → `ResolvedBundle` (empty Map when `data` is undefined). Keep the exported types (`ResolveResult`, `ResolvedUser`, etc.) — re-export from `lookup-types` where they already live. Note the per-source `resolved` flags: `resolveBundle` keys off `bundlesQ.isLoading`, the other three off `catalogQ.isLoading`.

- [ ] **Step 5: Wire invalidation.** In the user create + delete mutations (`ui/src/lib/queries/useUsers.ts` or wherever `useCreateUser`/`useDeleteUser` invalidate), add `queryClient.invalidateQueries({ queryKey: ["name-catalog"] })`. Run: `grep -rn "invalidateQueries" src/lib/queries/useUsers.ts` to find the existing pattern and mirror it.

- [ ] **Step 6: Run — verify pass + no consumer breakage.** `cd ui && bun run test -- useNameResolver && bun run lint && bun run build` — Expected: PASS. Consumers (`UserName`/`RoleName`/`BundleName`/`ProjectName`, `GrantsClient`, `audit/page.tsx`) untouched and green.

- [ ] **Step 7: Commit.**

```bash
git add ui/src/lib/queries/useNameResolver.tsx ui/src/lib/queries/__tests__/useNameResolver.test.tsx ui/src/lib/queries/useUsers.ts
git commit -m "refactor(ui): full-catalog name resolver, synchronous Map lookup (U1)"
```

---

### Task 8: Split `zitadel/page.tsx` + `preserveErrorBody` (U5)

**Files:**
- Modify: `ui/src/lib/api-client.ts` (add flag).
- Create: `ui/src/components/zitadel/{Health,Rotation,Projects,Users,AllGrants}.tsx` + `ui/src/components/zitadel/ZitadelDiagnostics.tsx` (client island composing the five sections).
- Modify: `ui/src/app/zitadel/page.tsx` → **server component with admin guard** (was a monolithic `"use client"` with NO guard — closes the P2 gating gap; see design.md Decision 3/8).
- Test: `ui/src/lib/__tests__/api-client.test.ts`; add the `/zitadel` guard case to `ui/src/app/__tests__/admin-page-guards.test.ts` (created in Task 6); keep existing zitadel page tests green.

**Interfaces:**
- Produces: `request<T>(path, init?: RequestInitJSON & { preserveErrorBody?: boolean })`. When `preserveErrorBody` is true and `!res.ok`, return the parsed body as `T` instead of throwing `ApiError`.
- Each `components/zitadel/X.tsx` default-exports a client component owning its section's hooks + CRUD, matching the JSX currently in the corresponding `*Section`. `ZitadelDiagnostics.tsx` (`"use client"`) composes them. `app/zitadel/page.tsx` becomes an async server component that guards then renders `<ZitadelDiagnostics/>`.

- [ ] **Step 1: Write failing test** for the flag in `api-client.test.ts`:

```ts
it("preserveErrorBody returns parsed body on non-2xx instead of throwing", async () => {
  global.fetch = respondWith(500, { error: "boom", detail: "x" });
  const body = await request<{ error: string }>("zitadel/health", { preserveErrorBody: true });
  expect(body.error).toBe("boom");
});
it("throws ApiError on non-2xx without the flag", async () => {
  global.fetch = respondWith(500, { error: "boom" });
  await expect(request("zitadel/health")).rejects.toBeInstanceOf(ApiError);
});
```

- [ ] **Step 2: Run — verify fail.** `cd ui && bun run test -- api-client` — Expected: FAIL.

- [ ] **Step 3: Implement the flag.** In `RequestInitJSON` add `preserveErrorBody?: boolean;`. At the non-2xx site (~line 81):

```ts
if (!res.ok) {
  if (init?.preserveErrorBody) return parsed as T;
  throw new ApiError(res.status, parsed);
}
```

- [ ] **Step 4: Run — verify pass.** `cd ui && bun run test -- api-client` — Expected: PASS.

- [ ] **Step 5: Extract sections.** For each of the five sections, move its JSX + local hooks/handlers into `components/zitadel/<Name>.tsx` as a default-exported component. Replace the three local helpers at their call sites:
  - `apiGet<T>(p)` → `request<T>(p)`.
  - `apiSend<T>(m, p, b)` → `request<T>(p, { method: m, body: b })`.
  - `apiGetDiagnostic<T>(p)` (health probe, line 345) → `request<T>(p, { preserveErrorBody: true })`.
  Carry `grantFlash` (async `.status` handling) into `Users.tsx` unchanged.

- [ ] **Step 6: Compose the client island.** Move the current `"use client"` composition into `components/zitadel/ZitadelDiagnostics.tsx` (composing `<Health/>`, `<Rotation/>`, `<Projects/>`, `<Users/>`, `<AllGrants/>`). Delete the three local helper functions.

- [ ] **Step 7: Guard the page.** Rewrite `app/zitadel/page.tsx` as an async server component (drop `"use client"`), mirroring `app/grants/page.tsx`:

```tsx
import { redirect } from "next/navigation";
import ZitadelDiagnostics from "@/components/zitadel/ZitadelDiagnostics";
import { getSession } from "@/lib/session";

export default async function ZitadelPage() {
  const session = await getSession();
  if (!session) redirect("/login");
  if (session.role !== "admin") redirect("/");
  return <ZitadelDiagnostics />;
}
```

- [ ] **Step 8: Guard test.** Add `/zitadel` to the table in `admin-page-guards.test.ts` (Task 6): member → `redirect("/")`, no session → `redirect("/login")`. Run: `cd ui && bun run test -- admin-page-guards` — Expected: PASS (was the untested gap).

- [ ] **Step 9: Run full UI suite.** `cd ui && bun run lint && bun run test && bun run build` — Expected: PASS; existing zitadel page tests still green (behaviour unchanged apart from the added guard).

- [ ] **Step 10: Commit.**

```bash
git add ui/src/lib/api-client.ts ui/src/components/zitadel ui/src/app/zitadel/page.tsx ui/src/lib/__tests__/api-client.test.ts ui/src/app/__tests__/admin-page-guards.test.ts
git commit -m "refactor(ui): split zitadel page, unify on request<T>, add admin guard (U5)"
```

---

### Task 9: Inline §5.2 pending tag (R2)

**Files:**
- Modify: `ui/src/components/zitadel/Users.tsx` (from Task 8).
- Reference: `ui/src/lib/queries/usePropagation.ts` (`usePendingPropagations()` → `PendingRow[]`).
- Test: `ui/src/components/zitadel/__tests__/Users.test.tsx` (new) or extend the existing zitadel page test.

**Interfaces:**
- Consumes: `usePendingPropagations()`, `PendingRow = { op_type, user_id, project_id, role_keys[], status, ... }`.
- Produces: an inline `[⏱ Awaiting Zitadel]` tag + Resume link rendered next to grants whose `(user, project, role)` is a pending add.

- [ ] **Step 1: Write failing test** (jsdom): render `Users.tsx` with a mocked `usePendingPropagations` returning one `op_type:"add"` row for `(u1, p1, roleA)`; assert the tag appears on that grant row and not on an unrelated grant.

- [ ] **Step 2: Run — verify fail.** `cd ui && bun run test -- Users` — Expected: FAIL.

- [ ] **Step 3: Implement.** Build a memoized key set and render the tag:

```tsx
const { data: pending } = usePendingPropagations();
const pendingAdds = useMemo(() => {
  const s = new Set<string>();
  for (const r of pending ?? []) {
    if (r.op_type !== "add") continue;
    for (const rk of r.role_keys) s.add(`${r.user_id}|${r.project_id}|${rk}`);
  }
  return s;
}, [pending]);
// next to a grant row for (userId, projectId, roleKey):
{pendingAdds.has(`${userId}|${projectId}|${roleKey}`) && (
  <span className="inline-flex items-center gap-1 rounded px-2 py-0.5 text-xs
                   bg-tertiary-container text-on-tertiary-container">
    ⏱ Awaiting Zitadel
    <Link href="/" className="underline">Resume</Link>
  </span>
)}
```

Resume points at the dashboard pending callout (`/`). No animation (or `motion-reduce:` guarded) per §5.2 (single-pulse is on the count badge, not the inline tag).

- [ ] **Step 4: Run — verify pass.** `cd ui && bun run test -- Users` — Expected: PASS.

- [ ] **Step 5: Commit.**

```bash
git add ui/src/components/zitadel/Users.tsx ui/src/components/zitadel/__tests__/Users.test.tsx
git commit -m "feat(ui): inline 'Awaiting Zitadel' pending tag on grants (R2, §5.2)"
```

---

### Task 10: Drop `location` end-to-end (D6)

**Files (backend):** `models/models.go:69`, `directory/zitadel.go:197-198,491`, `demo/catalog.go:10-14`, `handlers/profile.go`, `directory/directory_test.go:492-493`, `handlers/profile_test.go:46,63`.
**Files (frontend):** `lib/types.ts:44`, `lib/session.ts` (:14,:40, demo :68/80/92/104/116, :165, :223), `lib/oidc.ts:227,242,253`, `app/auth/callback/route.ts:139`, `app/page.tsx:56`, `app/login/page.tsx:127`, `lib/__tests__/session.test.ts`.
**Files (specs):** `openspec/changes/mkauth-core-architecture/specs/feature-coverage.md:24`, `openspec/changes/live-directory-identity-completeness/{proposal,design,tasks}.md`.

- [ ] **Step 1 (backend TDD):** Update `directory_test.go` and `profile_test.go` first — remove `Location` assertions / fixtures so they encode the new truth; run `cd backend && go test ./internal/directory/... ./internal/handlers/...` — Expected: FAIL to compile (Location field still referenced in prod).

- [ ] **Step 2 (backend impl):** Delete `Location` from `models.UserProfile`; remove the `"location"` metadata merge + empty-init in `directory/zitadel.go`; drop `Location` from `demo/catalog.go` users; remove any `Location` passthrough in `handlers/profile.go`. Keep `Title`/`Team`.

- [ ] **Step 3 (backend verify):** `cd backend && go build ./... && go vet ./... && go test ./...` — Expected: PASS.

- [ ] **Step 4 (frontend impl):** Remove `location` from `types.ts`, the `OidcSessionCookie` field + `SessionUser` field + demo data + legacy-decode default + OIDC assignment in `session.ts`, `ProfileMetadata`/default/`fetchProfileMetadata` read in `oidc.ts`, the callback write in `auth/callback/route.ts`, and both render sites (`app/page.tsx:56` `{session.team}` only; `login/page.tsx:127` `{user.team} • {user.email}`). Update `session.test.ts` (drop `location` fixtures/assertions at the listed lines).

- [ ] **Step 5 (frontend verify):** `cd ui && bun run lint && bun run test && bun run build` — Expected: PASS. Grep guard: `grep -rn "\.location\b\|location:" ui/src/lib ui/src/app/page.tsx ui/src/app/login` — Expected: no `UserProfile`/session `location` (only `window.location` if any).

- [ ] **Step 6 (specs):** Edit `feature-coverage.md:24` user-management identity row to read Title/Team only. Edit `live-directory-identity-completeness` proposal/design/tasks to drop `location` from the well-known key set and outcomes (leave the change's history intact otherwise). Do NOT edit shipped `wave-1` specs.

- [ ] **Step 7: Commit.**

```bash
git add backend/internal/models/models.go backend/internal/directory backend/internal/demo/catalog.go backend/internal/handlers/profile.go backend/internal/handlers/profile_test.go ui/src/lib ui/src/app/auth/callback/route.ts ui/src/app/page.tsx ui/src/app/login/page.tsx openspec/changes/mkauth-core-architecture/specs/feature-coverage.md openspec/changes/live-directory-identity-completeness
git commit -m "refactor: drop location field end-to-end, keep title/team (D6)"
```

---

### Task 11: Verification gate

- [ ] **Step 1:** `cd backend && go test ./... && go vet ./...` — Expected: all pass.
- [ ] **Step 2:** `cd ui && bun run lint && bun run test && bun run build` — Expected: all pass.
- [ ] **Step 3:** `bash -n zitadel/actions/register.sh && bash -n zitadel/actions/rotate.sh && bash -n scripts/lib/zitadel-api.sh` — Expected: valid.
- [ ] **Step 4:** `mcp__codebase-memory-mcp__detect_changes` then reindex the affected scope (`ui/src`, `backend/internal`, `scripts`). Update any ADR via `manage_adr` only if an architectural decision shifted (none expected — this is cleanup).
- [ ] **Step 5:** `openspec validate wave-3-frontend-remainder-and-consolidation --strict` — Expected: PASS.
- [ ] **Step 6:** Tick all boxes in `tasks.md`.

---

### Task 12: Consolidation pass (§7)

**Files:** `openspec/INDEX.md`, `openspec/changes/mkauth-core-architecture/specs/feature-coverage.md`, `ROADMAP.md`/`design.md` (confirm only).

- [ ] **Step 1:** Append a Wave 3 change-log row to `INDEX.md` (mirror the Wave 2 Part rows' format; Phase 5.5).
- [ ] **Step 2:** Confirm `feature-coverage.md` reflects: `location` dropped (Task 10 did the row), Drift Control Integrated (already there), versioned-policies Removed (already there). Fix any residual `location` mention.
- [ ] **Step 3:** Read `ROADMAP.md` Phase 5.5 + Phase 6 and `mkauth-core-architecture/design.md §5` — confirm ⌘K stub present (Task 1), no live ⌘K claim, two-layer doctrine intact. No edits unless a drift is found.
- [ ] **Step 4:** Commit.

```bash
git add openspec/INDEX.md openspec/changes/mkauth-core-architecture/specs/feature-coverage.md
git commit -m "docs(openspec): wave-3 consolidation — INDEX row, feature-coverage trued to shipped reality (§7)"
```

---

## Self-Review

**Spec coverage:** U1→T7, U4→T3, U5→T8, U6→T4, U7→T6, D2→T1, D6→T10, R1→T2, R2→T9, R3→T5, consolidation→T12, verification→T11. All audit refs mapped.

**Placeholder scan:** Step 1 of T7 ("confirm catalog shapes") and T0/T11 validation are verification steps, not TODOs — each names the exact command and what to read. No "add error handling"/"TBD"/"similar to Task N".

**Type consistency:** `ResolveResult<T>`, `NameResolverContextValue`, `RequestInitJSON.preserveErrorBody`, `PendingRow` fields (`op_type`, `role_keys`), and the `user|project|role` key format are used identically across T7/T8/T9. `avatarSeed` (T4) is self-contained.

**Sequencing check:** T4 (session.ts avatar) before T10 (session.ts location) — different lines, no conflict. T9 (Users.tsx tag) after T8 (creates Users.tsx). T7 isolated.
