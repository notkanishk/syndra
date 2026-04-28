## Frontend Architecture

The Next.js admin console (`ui/`) standardizes on three Stage-1 / Stage-2 decisions captured here so future maintainers don't re-litigate them.

### Adopt TanStack Query as the canonical client data layer
- One `QueryClientProvider` mounts at the root via `ui/src/components/providers.tsx`.
- Every browser-side request goes through `request<T>(path, init)` in `ui/src/lib/api-client.ts`, which targets `/api/proxy/<path>` and throws typed `ApiError` on non-2xx so React Query's `error` state is structured.
- Per-resource hooks live under `ui/src/lib/queries/` (`useProjects`, `useBundles`, `useUsers`, `useAudit`, `useGovernance`, `useIntents`, `useDashboard`, `useNameResolver`, …). Stage 2 added the audit/users/governance/intents/dashboard slices; remaining hooks (`useApplications`, `useRoles`, `useRequests`, `useTopology`, `useOperations`, `useGrants`) are authored as their pages migrate in Stages 3–4.
- `getQueryClient()` returns a per-request client via `cache()` from React for RSC use; mutations invalidate by query-key family rather than by URL string.
- Defaults: `staleTime: 30s`, `gcTime: 5min`, `retry: 1`, `refetchOnWindowFocus: false`. Polling surfaces (`useIntents`) opt in via `refetchInterval`.
- Members never receive a 200 from `/lookup`, `/users`, `/audit`, or `/intents` — the proxy `isMemberAllowed` allowlist is the boundary; admin-only pages are the only consumers of these hooks today.

### Batch UID → name resolution via `POST /api/v1/lookup`
- The backend handler at `backend/internal/handlers/lookup.go` accepts `{user_ids, project_ids, role_keys[{project_id,role_key}], bundle_ids}`, caps each array at 256, and tolerates partial misses (missing IDs are absent from the response, never a 404).
- Client-side, `<UserName/>`, `<ProjectName/>`, `<RoleName/>`, and `<BundleName/>` enqueue ids into `NameResolverProvider`'s tick-batched queue. A `requestAnimationFrame` flush issues exactly one `useQuery(['lookup', sortedKey], …)` per tick — 50 components in one render produce one request.
- `ResolveResult<T>` is tri-state (`{value: T | undefined, resolved: boolean}`): components show a `<Skeleton/>` while `!value && !resolved`, and fall back to the `fallback` prop once `resolved === true` with no value. The `attempted` Set per type prevents missing ids from re-enqueuing forever.
- The actor `<select>` on `/audit` reads the resolver cache directly to build option label strings (native `<option>` cannot compose React children). This is acceptable as a Stage 2 affordance; a full combobox primitive is reserved for Stage 4.

### Obsidian Clarity design tokens (dark-first, light counterpart)
- `ui/src/app/globals.css` defines two complete token sets under `[data-theme="dark"]` and `[data-theme="light"]` — the light theme is a deliberate counterpart (desaturated indigo on warm-white surfaces), not an auto-inverted mirror.
- The Tailwind v4 `@theme {…}` block maps `--color-*` to the design tokens so utilities like `bg-surface-container-high`, `text-on-surface-variant`, `text-on-primary`, `border-outline-variant` resolve uniformly across themes.
- Core utilities: `bg-blob-hero` (atmospheric radial-gradient layer mounted once in `<body>`), `glass-card` (translucent surface-container + 28px backdrop-filter blur + ambient shadow), `pulse-dot` (animated status indicator).
- Typography: Inter (`--font-sans`) for body; Fraunces variable serif (`--font-display`) for h1/display surfaces. Both load via `next/font/google` with `display: 'swap'`.
- Primary buttons use a saturated `linear-gradient(135deg, var(--primary), var(--secondary))` (NOT the `*-container` pales) so `text-on-primary` meets WCAG AA in both themes — verified during Stage 1 review.
