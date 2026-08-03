"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { request } from "@/lib/api-client";
import type { BulkPlan } from "@/lib/queries/useBulkGrants";
import { bundlesQueryKeys, type BundleRoleRow } from "@/lib/queries/useBundles";

/**
 * Bundle versioning.
 *
 * Editing a bundle changes its WORKING COPY and reaches nobody. Publishing is
 * the separate, rehearsed step that can move the people already holding it —
 * which is why the publish hooks below speak the same `BulkPlan` every other
 * apply in the product speaks, and why `?apply=true` is a property of the
 * request rather than a field in the body.
 */

export interface BundleVersion {
  id: string;
  bundle_id: string;
  version: number;
  note: string;
  published_by: string;
  published_at: string;
  /** How many people are pinned to THIS version. */
  holder_count: number;
  roles?: BundleRoleRow[];
}

export interface BundleHolder {
  bundle_id: string;
  user_id: string;
  version_id: string;
  version: number;
  assigned_at: string;
}

/** The unpublished difference between the working copy and the latest version. */
export interface BundleDraft {
  bundle_id: string;
  latest_version: number;
  next_version: number;
  added: BundleRoleRow[];
  removed: BundleRoleRow[];
  holder_count: number;
}

/**
 * How many unpublished changes a draft carries.
 *
 * Every `?.` here is load-bearing, including the ones on the arrays. Optional chaining guards the
 * PARENT only: `draft?.added.length` short-circuits when `draft` is absent, and then throws on the
 * `.length` when `draft` is present but `added` is null — which is exactly what took the bundles
 * page down, with `?? 0` sitting right there looking like it handled the case.
 */
export function draftChangeCount(draft: BundleDraft | undefined): number {
  return (draft?.added?.length ?? 0) + (draft?.removed?.length ?? 0);
}

const KEYS = {
  versions: (bundleId: string) => ["bundles", bundleId, "versions"] as const,
  holders: (bundleId: string) => ["bundles", bundleId, "holders"] as const,
  draft: (bundleId: string) => ["bundles", bundleId, "draft"] as const,
};

export function useBundleVersions(bundleId: string | null | undefined) {
  return useQuery({
    queryKey: KEYS.versions(bundleId ?? ""),
    queryFn: async (): Promise<BundleVersion[]> => {
      const data = await request<unknown>(`/bundles/${bundleId}/versions`);
      return Array.isArray(data) ? (data as BundleVersion[]) : [];
    },
    enabled: Boolean(bundleId),
  });
}

export function useBundleHolders(bundleId: string | null | undefined) {
  return useQuery({
    queryKey: KEYS.holders(bundleId ?? ""),
    queryFn: async (): Promise<BundleHolder[]> => {
      const data = await request<unknown>(`/bundles/${bundleId}/holders`);
      return Array.isArray(data) ? (data as BundleHolder[]) : [];
    },
    enabled: Boolean(bundleId),
  });
}

export function useBundleDraft(bundleId: string | null | undefined) {
  return useQuery({
    queryKey: KEYS.draft(bundleId ?? ""),
    // Normalised here so `BundleDraft`'s non-nullable arrays are TRUE for everything downstream.
    // They were not: a Go nil slice marshals to `null`, five call sites called `.length` or `.map`
    // on it, and the bundles page threw on render for any bundle with nothing unpublished. The
    // backend no longer emits `null` either — but the type declared a guarantee this boundary was
    // not enforcing, and that is what made every consumer wrong at once.
    queryFn: async () => {
      const draft = await request<BundleDraft>(`/bundles/${bundleId}/draft`);
      return {
        ...draft,
        added: draft?.added ?? [],
        removed: draft?.removed ?? [],
      };
    },
    enabled: Boolean(bundleId),
  });
}

interface PublishInput {
  note: string;
  /** Move everyone currently holding the bundle onto the new version. */
  migrate: boolean;
}

/**
 * Rehearsing a publish returns the plan AND the draft, because the two answer
 * different halves of the same question: what the version will contain, and
 * what it will do to the fourteen people who hold the old one.
 */
export function useRehearsePublish(bundleId: string) {
  return useMutation({
    mutationFn: (input: PublishInput) =>
      request<{ plan: BulkPlan; draft: BundleDraft }>(`/bundles/${bundleId}/publish`, {
        method: "POST",
        body: input,
      }),
  });
}

export function useApplyPublish(bundleId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: PublishInput) =>
      request<{ plan: BulkPlan; version: BundleVersion }>(
        `/bundles/${bundleId}/publish?apply=true`,
        { method: "POST", body: input },
      ),
    onSuccess: () => invalidateBundle(qc, bundleId),
  });
}

interface MoveInput {
  version_id: string;
  user_ids: string[];
}

export function useRehearseMoveHolders(bundleId: string) {
  return useMutation({
    mutationFn: (input: MoveInput) =>
      request<BulkPlan>(`/bundles/${bundleId}/holders/move`, { method: "POST", body: input }),
  });
}

export function useApplyMoveHolders(bundleId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: MoveInput) =>
      request<BulkPlan>(`/bundles/${bundleId}/holders/move?apply=true`, {
        method: "POST",
        body: input,
      }),
    onSuccess: () => invalidateBundle(qc, bundleId),
  });
}

/**
 * Everything a version change touches. The People list is included because a
 * person's row carries which version of each bundle they hold, and a stale row
 * there would show somebody on a version they were just moved off.
 */
function invalidateBundle(qc: ReturnType<typeof useQueryClient>, bundleId: string) {
  qc.invalidateQueries({ queryKey: KEYS.versions(bundleId) });
  qc.invalidateQueries({ queryKey: KEYS.holders(bundleId) });
  qc.invalidateQueries({ queryKey: KEYS.draft(bundleId) });
  qc.invalidateQueries({ queryKey: bundlesQueryKeys.list });
  qc.invalidateQueries({ queryKey: ["users"] });
  qc.invalidateQueries({ queryKey: ["propagations"] });
}

export const bundleVersionKeys = KEYS;
