"use client";

import { useQuery } from "@tanstack/react-query";
import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";

import { request } from "@/lib/api-client";
import {
  type LookupResponse,
  type ResolvedBundle,
  type ResolvedProject,
  type ResolvedRole,
  type ResolvedUser,
  roleCompositeKey,
} from "@/lib/lookup-types";
import type { Bundle, CatalogResponse } from "@/lib/types";

/**
 * UID→name resolver backed by the full catalog.
 *
 * Two independent React Query fetches populate synchronous lookup Maps:
 *   - GET /catalog  → users + projects + nested project roles (member-allowed)
 *   - GET /bundles  → bundle names (admin-only; members get 403)
 *
 * Every <UserName/>/<ProjectName/>/<RoleName/>/<BundleName/> then resolves via
 * an in-memory Map hit — no per-id network round-trip, no rAF batching. The
 * two queries are deliberately separate (design.md Decision 1): a shared query
 * would let the member `GET /bundles` 403 sink user/project/role resolution.
 * Bundles fail in isolation → resolveBundle yields a fallback while the other
 * three keep resolving. Results are cached for 5 minutes; the catalog query is
 * invalidated on user create/delete so new ids resolve without a reload.
 */

/**
 * Tri-state result for any resolve* call:
 * - `value !== undefined`: resolution succeeded and the entity is known.
 * - `value === undefined && resolved === false`: still loading (skeleton).
 * - `value === undefined && resolved === true`: lookup completed with no
 *   entry for this id (render fallback, never skeleton).
 */
export interface ResolveResult<T> {
  value: T | undefined;
  resolved: boolean;
}

interface NameResolverContextValue {
  resolveUser: (id: string) => ResolveResult<ResolvedUser>;
  resolveProject: (id: string) => ResolveResult<ResolvedProject>;
  resolveRole: (projectId: string, roleKey: string) => ResolveResult<ResolvedRole>;
  resolveBundle: (id: string) => ResolveResult<ResolvedBundle>;
}

const NameResolverContext = createContext<NameResolverContextValue | null>(null);

const STALE_TIME = 5 * 60_000; // 5 minutes

interface CatalogMaps {
  users: Map<string, ResolvedUser>;
  projects: Map<string, ResolvedProject>;
  roles: Map<string, ResolvedRole>;
}

function buildCatalogMaps(data: CatalogResponse | undefined): CatalogMaps {
  const users = new Map<string, ResolvedUser>();
  const projects = new Map<string, ResolvedProject>();
  const roles = new Map<string, ResolvedRole>();
  if (!data) return { users, projects, roles };
  for (const u of data.users ?? []) {
    users.set(u.id, { display_name: u.name, email: u.email });
  }
  for (const p of data.projects ?? []) {
    projects.set(p.id, { name: p.name });
    for (const role of p.roles ?? []) {
      roles.set(roleCompositeKey(p.id, role.key), { display_name: role.label });
    }
  }
  return { users, projects, roles };
}

function buildBundleMap(data: Bundle[] | undefined): Map<string, ResolvedBundle> {
  const map = new Map<string, ResolvedBundle>();
  for (const b of data ?? []) {
    map.set(b.id, { name: b.name });
  }
  return map;
}

/**
 * Ids the catalog didn't know, collected during render and resolved in one
 * batched POST /lookup.
 *
 * The catalog is the directory as it stands now, so it cannot contain an
 * account that has since been deleted, one created in the seconds since the
 * last fetch, or a machine principal that never appears in a user list. Those
 * ids show up in audit rows and old grants and used to resolve to nothing —
 * which is how a screen ends up displaying a raw Zitadel id. The backend's
 * FindUser falls through to a direct Zitadel read, so a miss here is usually
 * recoverable; asking once and caching the answer makes it permanent.
 */
function useMissResolver(known: (id: string) => boolean, enabled: boolean) {
  // `queue` holds only ids that have NOT yet been asked about. Ids leave it
  // when their lookup settles, which is what lets the next batch through: a
  // queue that kept settled ids would re-slice the same first LOOKUP_MAX_BATCH
  // forever, and every id past that ceiling would never be requested at all.
  const [queue, setQueue] = useState<ReadonlySet<string>>(() => new Set());
  // Answers accumulate across batches. Reading them off the current query
  // instead would drop every earlier batch's names the moment the query key
  // advanced — the ids would be marked asked and resolve to nothing.
  const [resolved, setResolved] = useState<ReadonlyMap<string, ResolvedUser>>(() => new Map());
  const pending = useRef<Set<string>>(new Set());
  const asked = useRef<Set<string>>(new Set());

  const note = useCallback(
    (id: string) => {
      // No session, nothing to ask on behalf of. Refused before the queue
      // rather than before the request, so a disabled provider does no state
      // churn either.
      if (!enabled) return;
      if (!id || known(id) || asked.current.has(id) || pending.current.has(id)) return;
      pending.current.add(id);
      // Defer the state write out of the render that discovered the miss.
      queueMicrotask(() => {
        if (pending.current.size === 0) return;
        const batch = Array.from(pending.current);
        pending.current.clear();
        setQueue((prev) => {
          const next = new Set(prev);
          batch.forEach((value) => next.add(value));
          return next.size === prev.size ? prev : next;
        });
      });
    },
    [known, enabled],
  );

  const ids = useMemo(() => Array.from(queue).sort().slice(0, LOOKUP_MAX_BATCH), [queue]);

  const lookupQ = useQuery({
    queryKey: ["name-lookup", ids],
    queryFn: () =>
      request<LookupResponse>("lookup", { method: "POST", body: { user_ids: ids } }),
    // Gated twice on purpose. `note()` keeps ids out of the queue while
    // disabled; this keeps a queue filled *before* the gate closed — a session
    // expiring under a mounted tree — from draining after it.
    enabled: enabled && ids.length > 0,
    staleTime: STALE_TIME,
    retry: false,
  });

  const settled = lookupQ.isSuccess || lookupQ.isError;
  const answers = lookupQ.data?.users;

  useEffect(() => {
    if (!settled || ids.length === 0) return;

    if (answers) {
      setResolved((prev) => {
        const next = new Map(prev);
        for (const [id, user] of Object.entries(answers)) next.set(id, user);
        return next;
      });
    }

    // Every id in the batch is now answered, resolved or not. Marking them
    // asked stops an id the backend also can't place from re-firing a lookup
    // for the life of the session; dropping them from the queue lets whatever
    // is behind them become the next batch.
    ids.forEach((id) => asked.current.add(id));
    setQueue((prev) => {
      const next = new Set(prev);
      ids.forEach((id) => next.delete(id));
      return next.size === prev.size ? prev : next;
    });
  }, [ids, settled, answers]);

  return { note, resolved, pendingLookup: lookupQ.isFetching };
}

/** Matches the backend's `lookupMaxBatchSize`. */
const LOOKUP_MAX_BATCH = 256;

/**
 * `enabled` is "is there someone to resolve names for". The provider mounts in
 * the root layout, which wraps the unauthenticated `/login` as well as every
 * signed-in route, so without it both queries fire at a stranger and take a
 * 401 — four wasted round trips and four console errors on the one screen
 * somebody sees before they trust this software.
 *
 * It gates all three requests this provider can make — the catalog and bundle
 * warm-ups, and the per-miss `POST /lookup` that a descendant triggers merely by
 * calling `resolveUser` on an id the catalog does not carry. Gating only the
 * first two leaves the guarantee to whether anything happens to render a name,
 * which is not a guarantee.
 *
 * A disabled query in TanStack v5 is `pending` with `fetchStatus: "idle"`, so
 * `isLoading` is false and `resolved` reads true: callers render their fallback
 * rather than an eternal skeleton. Defaults to `true` — a caller who forgets it
 * gets working name resolution, not silently blank names.
 */
export function NameResolverProvider({
  children,
  enabled = true,
}: {
  children: React.ReactNode;
  enabled?: boolean;
}) {
  // Catalog: users + projects + nested roles. Member-allowed via proxy.
  const catalogQ = useQuery({
    queryKey: ["name-catalog"],
    queryFn: () => request<CatalogResponse>("catalog"),
    enabled,
    staleTime: STALE_TIME,
  });
  // Bundles: separate + non-retrying. Member GET /bundles is 403 by design;
  // on failure data is undefined → resolveBundle yields fallback, isolated
  // from catalog. Members don't surface bundle names on their own pages.
  const bundlesQ = useQuery({
    queryKey: ["name-bundles"],
    queryFn: () => request<Bundle[]>("bundles"),
    enabled,
    staleTime: STALE_TIME,
    retry: false,
  });

  const catalogMaps = useMemo(() => buildCatalogMaps(catalogQ.data), [catalogQ.data]);
  const bundleMap = useMemo(() => buildBundleMap(bundlesQ.data), [bundlesQ.data]);

  const catalogHas = useCallback(
    (id: string) => catalogMaps.users.has(id),
    [catalogMaps],
  );
  const { note, resolved: lateUsers, pendingLookup } = useMissResolver(catalogHas, enabled);

  const value = useMemo<NameResolverContextValue>(
    () => ({
      resolveUser: (id) => {
        const hit = catalogMaps.users.get(id) ?? lateUsers.get(id);
        if (hit) return { value: hit, resolved: true };
        if (catalogQ.isLoading) return { value: undefined, resolved: false };
        // The catalog has answered and doesn't know this id. Ask the backend
        // once; report "still resolving" while that is in flight so the caller
        // shows a skeleton rather than settling on a fallback it will replace.
        note(id);
        return { value: undefined, resolved: !pendingLookup };
      },
      resolveProject: (id) => ({ value: catalogMaps.projects.get(id), resolved: !catalogQ.isLoading }),
      resolveRole: (pid, rk) => ({
        value: catalogMaps.roles.get(roleCompositeKey(pid, rk)),
        resolved: !catalogQ.isLoading,
      }),
      resolveBundle: (id) => ({ value: bundleMap.get(id), resolved: !bundlesQ.isLoading }),
    }),
    [catalogMaps, bundleMap, catalogQ.isLoading, bundlesQ.isLoading, lateUsers, note, pendingLookup],
  );

  return <NameResolverContext.Provider value={value}>{children}</NameResolverContext.Provider>;
}

export function useNameResolver(): NameResolverContextValue {
  const ctx = useContext(NameResolverContext);
  if (!ctx) {
    // Permissive fallback so components that mount outside the provider
    // (tests, login screen) don't crash. Always reports resolved=true with no
    // value so components fall straight through to their fallback prop.
    const miss = { value: undefined, resolved: true } as const;
    return {
      resolveUser: () => miss,
      resolveProject: () => miss,
      resolveRole: () => miss,
      resolveBundle: () => miss,
    };
  }
  return ctx;
}
