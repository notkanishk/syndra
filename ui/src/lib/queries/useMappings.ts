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

/**
 * One difference between the working copy and the newest published version.
 *
 * Enumerated rather than counted, which is the whole argument for the version
 * band: "rolling back undoes work listed nowhere" stops being true the moment
 * something lists it.
 */
export interface MappingChange {
  kind: "added" | "changed" | "removed";
  project_id: string;
  role_key: string;
  field: string;
  /** What the working copy holds. Absent on a removal, which has no row. */
  value?: string;
  /** What the published version holds. Set on a change and on a removal. */
  was_value?: string;
  /**
   * Who last touched the row, and when.
   *
   * Both are absent on a REMOVAL and the surface must cope: a deleted row takes
   * its `updated_by` with it and nothing records the deletion, so a removal is
   * shown without attribution rather than credited to whoever published the
   * version — who did not remove it.
   */
  actor?: string;
  at?: string;
  /** How many people hold the role this change reaches. */
  holders: number;
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
  /** What differs. Empty when the working copy matches, never absent. */
  unpublished_changes: MappingChange[];
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
/**
 * A rehearsal, plus whether the value was actually checked.
 *
 * `value_checked: false` means the add-on could not be asked — it is not
 * answering, or it cannot enumerate this field — and the edit was allowed
 * through deliberately. Refusing an edit while a NAS reboots would make an
 * outage look like the operator's mistake, so the check fails open on
 * everything except a definite no.
 *
 * But a check that did not run must not read as one that passed, which is what
 * it did while success carried no distinction at all.
 */
export type MappingRehearsal = BulkPlan & { value_checked?: boolean };

export function rehearseMappingEdit(id: string, value: string, acknowledgeScope: boolean) {
  return request<MappingRehearsal>(`/targets/mappings/${id}/rehearse-edit`, {
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

/** What a new mapping would be, before there is a row to name it by. */
export interface NewMapping {
  target: string;
  projectId: string;
  roleKey: string;
  field: string;
  value: string;
}

/**
 * Who a new mapping would reach, before it exists.
 *
 * Creating one is an access change like editing one: entitlements are derived
 * from mappings, so the row alone changes what everybody holding that role is
 * entitled to. It is keyed on what WOULD be written rather than on a row id,
 * because there is no row yet to rehearse against.
 *
 * Carries `value_checked` for the same reason the edit's rehearsal does — a
 * check that could not run must not read as one that passed.
 */
export function rehearseMappingCreate(
  input: NewMapping,
  acknowledgeScope: boolean,
): Promise<MappingRehearsal> {
  return request<MappingRehearsal>("/targets/mappings/rehearse-create", {
    method: "POST",
    body: {
      target: input.target,
      project_id: input.projectId,
      role_key: input.roleKey,
      field: input.field,
      value: input.value,
      acknowledge_scope: acknowledgeScope,
    },
  });
}

/**
 * The result of writing one: the row, and how many people it moved.
 *
 * Queued, never applied. The mapping is written and the people it reaches are
 * converged by the drain.
 */
export interface MappingCreateResult {
  mapping: RoleMapping;
  queued_convergences: number;
}

export function useCreateMapping() {
  const client = useQueryClient();
  return useMutation({
    // The approval the rehearsal issued, cited whenever the role has holders.
    // A mapping on a role nobody holds is a definition and needs none — which
    // is why `planId` is optional here and required by the backend exactly when
    // it matters.
    mutationFn: (input: NewMapping & { planId?: string }) =>
      request<MappingCreateResult>("/targets/mappings", {
        method: "POST",
        body: {
          target: input.target,
          project_id: input.projectId,
          role_key: input.roleKey,
          field: input.field,
          value: input.value,
          ...(input.planId ? { plan_id: input.planId } : {}),
        },
      }),
    onSuccess: () => {
      client.invalidateQueries({ queryKey: ["targets", "mappings"] });
      // The convergences it queued show up as pending changes, the same as
      // every other mapping change.
      client.invalidateQueries({ queryKey: ["propagation"] });
    },
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

/**
 * What a rollback would do, before it does it.
 *
 * It was the one mapping change that did not rehearse — edit and delete both
 * state their cohort and wait, while reverting a publish, which can move more
 * people than either, went straight through.
 *
 * One plan for the whole version rather than one per mapping: two mappings on
 * one role reach the same people, so per-mapping counts cannot be added up, and
 * somebody whose mapping the rollback deletes appears in the working copy and
 * in no version at all. Only the whole-version plan computes distinct people
 * across the union.
 */
export async function rehearseMappingRollback(
  target: string,
  version: number,
  acknowledgeScope: boolean,
): Promise<BulkPlan> {
  return request<BulkPlan>(
    `/targets/${target}/mappings/versions/${version}/rehearse-rollback`,
    { method: "POST", body: { acknowledge_scope: acknowledgeScope } },
  );
}

export function useRollbackMappingVersion(target: string) {
  const client = useQueryClient();
  return useMutation({
    // The approval the rehearsal issued, cited on apply. It travels in the body
    // for the same reason the delete's does: an approval in a query parameter
    // ends up in browser history and access logs.
    mutationFn: ({ version, planId }: { version: number; planId: string }) =>
      request<MappingApplyResult & { version: number }>(
        `/targets/${target}/mappings/versions/${version}/rollback`,
        { method: "POST", body: { plan_id: planId } },
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
