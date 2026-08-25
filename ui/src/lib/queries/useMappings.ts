"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { request } from "@/lib/api-client";
import type { BulkPlan } from "@/lib/queries/useBulkGrants";

/**
 * What a role reaches on a target, and the history of that answer.
 *
 * A mapping is the highest-leverage object in this system: editing one moves
 * access for everyone holding the role, and deleting one does it invisibly —
 * nothing on any screen says "these forty people just lost their storage", it
 * simply stops being true. So edit, delete and rollback all rehearse first, and
 * the rehearsal returns the same `BulkPlan` every other planned surface speaks.
 */

export interface RoleMapping {
  id: string;
  target: string;
  project_id: string;
  role_key: string;
  field: string;
  value: string;
  created_by: string;
  updated_by?: string;
}

export function useMappings(target?: string) {
  return useQuery({
    queryKey: ["targets", "mappings", target ?? "all"],
    queryFn: () =>
      request<{ mappings: RoleMapping[] }>(
        target ? `/targets/mappings?target=${encodeURIComponent(target)}` : "/targets/mappings",
      ),
    select: (data) => data.mappings ?? [],
  });
}

export interface MappingVersionEntry {
  project_id: string;
  role_key: string;
  field: string;
  value: string;
}

export interface MappingVersion {
  version: number;
  /** Why it was published. A rollback target with no reason attached is a guess. */
  note: string;
  published_by: string;
  published_at: string;
  entries: MappingVersionEntry[];
}

export interface MappingHistory {
  target: string;
  /** The newest published version, or 0. What the list tints. */
  current_version: number;
  /**
   * The working copy differs from that newest version.
   *
   * Publishing snapshots the working set and every edit afterwards moves the
   * set and not the snapshot — so "current version 4" on its own can mean
   * version 4 plus three edits nobody has published, and an operator rolling
   * back to 4 from there would be undoing work not listed anywhere.
   */
  unpublished: boolean;
  versions: MappingVersion[];
}

export function useMappingHistory(target: string | undefined) {
  return useQuery({
    queryKey: ["targets", target, "mapping-versions"],
    queryFn: () => request<MappingHistory>(`/targets/${target}/mappings/versions`),
    enabled: Boolean(target),
  });
}

export function useMappingHolders(id: string | undefined) {
  return useQuery({
    queryKey: ["targets", "mappings", id, "holders"],
    queryFn: () =>
      request<{ mapping: RoleMapping; holders: string[]; count: number }>(
        `/targets/mappings/${id}/holders`,
      ),
    enabled: Boolean(id),
  });
}

/**
 * Everything that changes a mapping, in the shape `RehearsalDialog` speaks.
 *
 * Rehearsals are plain functions rather than mutations: the dialog owns the
 * two-step flow and needs to await each leg, and a mutation's `mutateAsync`
 * would add a second copy of the pending state it already tracks.
 */
export function rehearseMappingEdit(id: string, value: string, acknowledgeScope: boolean) {
  return request<BulkPlan>(`/targets/mappings/${id}/rehearse-edit`, {
    method: "POST",
    body: { value, acknowledge_scope: acknowledgeScope },
  });
}

export function rehearseMappingDelete(id: string, acknowledgeScope: boolean) {
  return request<BulkPlan>(`/targets/mappings/${id}/rehearse-delete`, {
    method: "POST",
    body: { acknowledge_scope: acknowledgeScope },
  });
}

/**
 * What an apply reports back.
 *
 * `queued_convergences`, never "applied": the edit lands in Syndra and the
 * people it moves are converged by the drain afterwards. The dialog renders a
 * `BulkPlan`, so the caller adapts this into one — and the adaptation is where
 * "queued" has to survive, because a plan whose summary said `succeeded` would
 * put a tick on a change that has not reached the target.
 */
export interface MappingApplyResult {
  status: string;
  queued_convergences: number;
}

export function applyMappingEdit(id: string, value: string, planId: string) {
  return request<MappingApplyResult>(`/targets/mappings/${id}`, {
    method: "PATCH",
    body: { value, plan_id: planId },
  });
}

export function applyMappingDelete(id: string, planId: string) {
  return request<MappingApplyResult>(`/targets/mappings/${id}`, {
    method: "DELETE",
    body: { plan_id: planId },
  });
}

export function useCreateMapping() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (input: {
      target: string;
      projectId: string;
      roleKey: string;
      field: string;
      value: string;
    }) =>
      request<RoleMapping>("/targets/mappings", {
        method: "POST",
        body: {
          target: input.target,
          project_id: input.projectId,
          role_key: input.roleKey,
          field: input.field,
          value: input.value,
        },
      }),
    onSuccess: () => client.invalidateQueries({ queryKey: ["targets", "mappings"] }),
  });
}

export function usePublishMappingVersion(target: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (note: string) =>
      request<{ target: string; version: number }>("/targets/mappings/versions", {
        method: "POST",
        body: { target, note },
      }),
    onSuccess: () => client.invalidateQueries({ queryKey: ["targets", target, "mapping-versions"] }),
  });
}

export function useRollbackMappingVersion(target: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (version: number) =>
      request<MappingApplyResult & { version: number }>(
        `/targets/${target}/mappings/versions/${version}/rollback`,
        { method: "POST", body: {} },
      ),
    onSuccess: () => {
      client.invalidateQueries({ queryKey: ["targets", target, "mapping-versions"] });
      client.invalidateQueries({ queryKey: ["targets", "mappings"] });
      // The rollback queues a convergence per affected holder, so what changed
      // for an operator watching is the pending count.
      client.invalidateQueries({ queryKey: ["propagation"] });
    },
  });
}
