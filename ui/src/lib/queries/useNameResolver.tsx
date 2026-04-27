"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

import { request } from "@/lib/api-client";
import {
  type LookupRequest,
  type LookupResponse,
  type ResolvedBundle,
  type ResolvedProject,
  type ResolvedRole,
  type ResolvedUser,
  roleCompositeKey,
} from "@/lib/lookup-types";

/**
 * Tick-batched UID→name resolver backed by POST /api/v1/lookup.
 *
 * Every <UserName/>/<ProjectName/>/<RoleName/>/<BundleName/> call enqueues
 * its id into a per-tick batch. After a single requestAnimationFrame, the
 * batch is flushed as ONE network request keyed by the sorted concatenation
 * of all queued ids (so 50 components in one tick → 1 fetch). Resolution
 * results live in the React Query cache for 5 minutes; subsequent mounts of
 * the same id hit the cache without a request.
 *
 * Implementation note: we keep our own `pending` and `resolved` maps in
 * React state alongside React Query so individual <Name/> components can
 * subscribe to a single id's resolution without writing custom selectors.
 */

interface BatchKey {
  u: string[];
  p: string[];
  r: { project_id: string; role_key: string }[];
  b: string[];
}

type ResolvedAll = {
  users: Map<string, ResolvedUser>;
  projects: Map<string, ResolvedProject>;
  roles: Map<string, ResolvedRole>;
  bundles: Map<string, ResolvedBundle>;
};

/**
 * Tri-state result for any resolve* call:
 * - `value !== undefined`: resolution succeeded and the entity is known.
 * - `value === undefined && resolved === false`: still loading (skeleton).
 * - `value === undefined && resolved === true`: lookup completed with no
 *   entry for this id (render fallback, never skeleton).
 *
 * Components use the `resolved` flag to decide between Skeleton and the
 * fallback prop. Without it, a missing id renders skeleton forever.
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
  /** Imperative prefetch — useful when a list page knows which ids it'll need. */
  prefetch: (ids: LookupRequest) => void;
}

const NameResolverContext = createContext<NameResolverContextValue | null>(null);

const STALE_TIME = 5 * 60_000; // 5 minutes

/**
 * Build a stable, sorted query key from the resolved set. JSON.stringify a
 * deterministic object so React Query treats {users: ['a','b']} and
 * {users: ['b','a']} as the same query.
 */
function makeQueryKey(resolved: ResolvedAll, queue: Required<LookupRequest>): readonly unknown[] {
  // The queryKey is the sorted UNION of resolved + queued ids. This way, when
  // a new id is added later, React Query treats it as a new query (rather than
  // refetching all previously resolved ids).
  const userIds = Array.from(
    new Set([...Array.from(resolved.users.keys()), ...(queue.user_ids ?? [])]),
  ).sort();
  const projectIds = Array.from(
    new Set([...Array.from(resolved.projects.keys()), ...(queue.project_ids ?? [])]),
  ).sort();
  const roleKeys = Array.from(
    new Set([
      ...Array.from(resolved.roles.keys()),
      ...queue.role_keys.map((rk) => roleCompositeKey(rk.project_id, rk.role_key)),
    ]),
  ).sort();
  const bundleIds = Array.from(
    new Set([...Array.from(resolved.bundles.keys()), ...(queue.bundle_ids ?? [])]),
  ).sort();

  return ["lookup", { u: userIds, p: projectIds, r: roleKeys, b: bundleIds }] as const;
}

export function NameResolverProvider({ children }: { children: React.ReactNode }) {
  // What's been resolved so far. Lives in React state because UI must re-render
  // when a name lands; React Query data is the upstream source.
  const [resolved, setResolved] = useState<ResolvedAll>(() => ({
    users: new Map(),
    projects: new Map(),
    roles: new Map(),
    bundles: new Map(),
  }));

  // Every id that has been looked up (whether successfully or not). Used by
  // the resolve* methods to distinguish "still loading" from "looked up but
  // missing" so components can swap skeleton for fallback at the right time.
  const [attempted, setAttempted] = useState<{
    users: Set<string>;
    projects: Set<string>;
    roles: Set<string>;
    bundles: Set<string>;
  }>(() => ({
    users: new Set(),
    projects: new Set(),
    roles: new Set(),
    bundles: new Set(),
  }));

  // What ids are in the current pending batch. Mutated by request* calls
  // and flushed on rAF. Stored as Sets to dedupe automatically.
  const queueRef = useRef<{
    users: Set<string>;
    projects: Set<string>;
    roles: Map<string, { project_id: string; role_key: string }>;
    bundles: Set<string>;
  }>({
    users: new Set(),
    projects: new Set(),
    roles: new Map(),
    bundles: new Set(),
  });
  const flushScheduledRef = useRef(false);
  const [batchKey, setBatchKey] = useState<BatchKey | null>(null);
  const queryClient = useQueryClient();

  // Flush the queue → bump the batch key → trigger a single useQuery fetch.
  const flush = useCallback(() => {
    flushScheduledRef.current = false;
    const q = queueRef.current;
    if (q.users.size === 0 && q.projects.size === 0 && q.roles.size === 0 && q.bundles.size === 0) {
      return;
    }
    const next: BatchKey = {
      u: Array.from(q.users).sort(),
      p: Array.from(q.projects).sort(),
      r: Array.from(q.roles.values()).sort((a, b) =>
        roleCompositeKey(a.project_id, a.role_key).localeCompare(
          roleCompositeKey(b.project_id, b.role_key),
        ),
      ),
      b: Array.from(q.bundles).sort(),
    };
    queueRef.current = {
      users: new Set(),
      projects: new Set(),
      roles: new Map(),
      bundles: new Set(),
    };
    setBatchKey(next);
  }, []);

  const scheduleFlush = useCallback(() => {
    if (flushScheduledRef.current) return;
    flushScheduledRef.current = true;
    if (typeof window === "undefined") {
      // Server: flush synchronously (no rAF). Tests using fake timers also
      // benefit from this path being deterministic.
      queueMicrotask(flush);
    } else {
      window.requestAnimationFrame(flush);
    }
  }, [flush]);

  // The actual network call. queryKey is the cumulative resolved + new-batch
  // set; this means cached resolutions never trigger a refetch and only newly
  // queued ids force a network round-trip.
  const queryKey = useMemo(() => {
    if (!batchKey) return null;
    return makeQueryKey(resolved, {
      user_ids: batchKey.u,
      project_ids: batchKey.p,
      role_keys: batchKey.r,
      bundle_ids: batchKey.b,
    });
  }, [batchKey, resolved]);

  const { data } = useQuery({
    queryKey: queryKey ?? ["lookup", "noop"],
    queryFn: async (): Promise<LookupResponse> => {
      if (!batchKey) {
        return { users: {}, projects: {}, roles: {}, bundles: {} };
      }
      // Send only the newly-queued ids — already-resolved entries stay in
      // local state and don't need to be requested again.
      const body: LookupRequest = {};
      if (batchKey.u.length > 0) body.user_ids = batchKey.u;
      if (batchKey.p.length > 0) body.project_ids = batchKey.p;
      if (batchKey.r.length > 0) body.role_keys = batchKey.r;
      if (batchKey.b.length > 0) body.bundle_ids = batchKey.b;
      return await request<LookupResponse>("/lookup", { method: "POST", body });
    },
    enabled: !!batchKey,
    staleTime: STALE_TIME,
  });

  // Merge fetched results into the resolved state, AND mark every id from the
  // request batch as attempted (whether it appeared in the response or not).
  // The latter is what lets components distinguish loading from miss.
  useEffect(() => {
    if (!data || !batchKey) return;
    setResolved((prev) => {
      const next: ResolvedAll = {
        users: new Map(prev.users),
        projects: new Map(prev.projects),
        roles: new Map(prev.roles),
        bundles: new Map(prev.bundles),
      };
      let changed = false;
      for (const [id, u] of Object.entries(data.users)) {
        if (!next.users.has(id)) {
          next.users.set(id, u);
          changed = true;
        }
      }
      for (const [id, p] of Object.entries(data.projects)) {
        if (!next.projects.has(id)) {
          next.projects.set(id, p);
          changed = true;
        }
      }
      for (const [key, r] of Object.entries(data.roles)) {
        if (!next.roles.has(key)) {
          next.roles.set(key, r);
          changed = true;
        }
      }
      for (const [id, b] of Object.entries(data.bundles)) {
        if (!next.bundles.has(id)) {
          next.bundles.set(id, b);
          changed = true;
        }
      }
      return changed ? next : prev;
    });
    setAttempted((prev) => {
      const next = {
        users: new Set(prev.users),
        projects: new Set(prev.projects),
        roles: new Set(prev.roles),
        bundles: new Set(prev.bundles),
      };
      for (const id of batchKey.u) next.users.add(id);
      for (const id of batchKey.p) next.projects.add(id);
      for (const rk of batchKey.r) next.roles.add(roleCompositeKey(rk.project_id, rk.role_key));
      for (const id of batchKey.b) next.bundles.add(id);
      return next;
    });
  }, [data, batchKey]);

  // request* methods enqueue an id ONLY when it has not yet been attempted.
  // Once a lookup has completed for an id (whether or not the entity was found),
  // the id is in `attempted` and we MUST short-circuit before scheduling another
  // flush — otherwise a missing id would re-enqueue on every render of every
  // mounted Name component, producing an infinite rAF/setState loop.
  const enqueueUser = useCallback(
    (id: string): ResolveResult<ResolvedUser> => {
      if (!id) return { value: undefined, resolved: true };
      const cached = resolved.users.get(id);
      if (cached) return { value: cached, resolved: true };
      if (attempted.users.has(id)) return { value: undefined, resolved: true };
      if (!queueRef.current.users.has(id)) {
        queueRef.current.users.add(id);
        scheduleFlush();
      }
      return { value: undefined, resolved: false };
    },
    [resolved.users, attempted.users, scheduleFlush],
  );

  const enqueueProject = useCallback(
    (id: string): ResolveResult<ResolvedProject> => {
      if (!id) return { value: undefined, resolved: true };
      const cached = resolved.projects.get(id);
      if (cached) return { value: cached, resolved: true };
      if (attempted.projects.has(id)) return { value: undefined, resolved: true };
      if (!queueRef.current.projects.has(id)) {
        queueRef.current.projects.add(id);
        scheduleFlush();
      }
      return { value: undefined, resolved: false };
    },
    [resolved.projects, attempted.projects, scheduleFlush],
  );

  const enqueueRole = useCallback(
    (projectId: string, roleKey: string): ResolveResult<ResolvedRole> => {
      if (!projectId || !roleKey) return { value: undefined, resolved: true };
      const composite = roleCompositeKey(projectId, roleKey);
      const cached = resolved.roles.get(composite);
      if (cached) return { value: cached, resolved: true };
      if (attempted.roles.has(composite)) return { value: undefined, resolved: true };
      if (!queueRef.current.roles.has(composite)) {
        queueRef.current.roles.set(composite, { project_id: projectId, role_key: roleKey });
        scheduleFlush();
      }
      return { value: undefined, resolved: false };
    },
    [resolved.roles, attempted.roles, scheduleFlush],
  );

  const enqueueBundle = useCallback(
    (id: string): ResolveResult<ResolvedBundle> => {
      if (!id) return { value: undefined, resolved: true };
      const cached = resolved.bundles.get(id);
      if (cached) return { value: cached, resolved: true };
      if (attempted.bundles.has(id)) return { value: undefined, resolved: true };
      if (!queueRef.current.bundles.has(id)) {
        queueRef.current.bundles.add(id);
        scheduleFlush();
      }
      return { value: undefined, resolved: false };
    },
    [resolved.bundles, attempted.bundles, scheduleFlush],
  );

  const prefetch = useCallback(
    (ids: LookupRequest) => {
      let touched = false;
      for (const id of ids.user_ids ?? []) {
        if (id && !resolved.users.has(id) && !queueRef.current.users.has(id)) {
          queueRef.current.users.add(id);
          touched = true;
        }
      }
      for (const id of ids.project_ids ?? []) {
        if (id && !resolved.projects.has(id) && !queueRef.current.projects.has(id)) {
          queueRef.current.projects.add(id);
          touched = true;
        }
      }
      for (const rk of ids.role_keys ?? []) {
        const composite = roleCompositeKey(rk.project_id, rk.role_key);
        if (!resolved.roles.has(composite) && !queueRef.current.roles.has(composite)) {
          queueRef.current.roles.set(composite, rk);
          touched = true;
        }
      }
      for (const id of ids.bundle_ids ?? []) {
        if (id && !resolved.bundles.has(id) && !queueRef.current.bundles.has(id)) {
          queueRef.current.bundles.add(id);
          touched = true;
        }
      }
      if (touched) scheduleFlush();
      // Also let React Query pre-populate via direct queryClient.fetchQuery
      // when caller wants to await. We don't await here — flush is async and
      // the caller doesn't need the result synchronously in this code path.
      void queryClient;
    },
    [resolved, scheduleFlush, queryClient],
  );

  const value = useMemo<NameResolverContextValue>(
    () => ({
      resolveUser: enqueueUser,
      resolveProject: enqueueProject,
      resolveRole: enqueueRole,
      resolveBundle: enqueueBundle,
      prefetch,
    }),
    [enqueueUser, enqueueProject, enqueueRole, enqueueBundle, prefetch],
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
      prefetch: () => {},
    };
  }
  return ctx;
}
