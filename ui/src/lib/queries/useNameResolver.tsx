"use client";

import { useQuery } from "@tanstack/react-query";
import { createContext, useContext, useMemo } from "react";

import { request } from "@/lib/api-client";
import {
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

export function NameResolverProvider({ children }: { children: React.ReactNode }) {
  // Catalog: users + projects + nested roles. Member-allowed via proxy.
  const catalogQ = useQuery({
    queryKey: ["name-catalog"],
    queryFn: () => request<CatalogResponse>("catalog"),
    staleTime: STALE_TIME,
  });
  // Bundles: separate + non-retrying. Member GET /bundles is 403 by design;
  // on failure data is undefined → resolveBundle yields fallback, isolated
  // from catalog. Members don't surface bundle names on their own pages.
  const bundlesQ = useQuery({
    queryKey: ["name-bundles"],
    queryFn: () => request<Bundle[]>("bundles"),
    staleTime: STALE_TIME,
    retry: false,
  });

  const catalogMaps = useMemo(() => buildCatalogMaps(catalogQ.data), [catalogQ.data]);
  const bundleMap = useMemo(() => buildBundleMap(bundlesQ.data), [bundlesQ.data]);

  const value = useMemo<NameResolverContextValue>(
    () => ({
      resolveUser: (id) => ({ value: catalogMaps.users.get(id), resolved: !catalogQ.isLoading }),
      resolveProject: (id) => ({ value: catalogMaps.projects.get(id), resolved: !catalogQ.isLoading }),
      resolveRole: (pid, rk) => ({
        value: catalogMaps.roles.get(roleCompositeKey(pid, rk)),
        resolved: !catalogQ.isLoading,
      }),
      resolveBundle: (id) => ({ value: bundleMap.get(id), resolved: !bundlesQ.isLoading }),
    }),
    [catalogMaps, bundleMap, catalogQ.isLoading, bundlesQ.isLoading],
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
