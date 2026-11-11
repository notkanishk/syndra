"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { ApiError, request } from "@/lib/api-client";
import { invalidateAfter } from "./invalidation";
import type { ReadView } from "./useMyStorage";

/**
 * A member acting on the world (a physical operation such as item.checkout),
 * previewed and then confirmed. Backend: handlers/member_operations.go.
 * The subject is always the signed-in member; the backend mints the confirm's
 * record id, so the page never sends one, and never sends a confirm twice.
 */
export type ActionParams = Record<string, string>;

export type ActionAnswer =
  /** Nothing was sent but the preview: `done` is what a confirm would do, `no_change` that they already have it. */
  | { state: "preview"; outcome: "done" | "no_change"; view: ReadView }
  | { state: "done" | "no_change"; operation: string; view: ReadView | null }
  /** Sent, the answer lost: poll `operation`. */
  | { state: "checking"; operation: string; sent_at: string };

/** Where a confirm stands (GET .../operations/{id}). `view` only on the poll that settled it. */
export interface OperationState {
  operation: string;
  sent_at: string;
  state: "checking" | "done" | "no_change" | "not_done" | "ask_desk";
  view?: ReadView | null;
}

/** Whether a state is final: nothing left to poll for. */
export const settled = (state: OperationState["state"] | undefined) => state !== undefined && state !== "checking";

/** How often an unsettled confirm is asked about: well inside the member poll's limit (60 a minute). */
export const POLL_MS = 5000;

const path = (system: string, tail: string) => `/me/targets/${encodeURIComponent(system)}/operations/${encodeURIComponent(tail)}`;

/** Preview or confirm one action. A mutation is never retried, so a confirm is sent once. */
export function useMemberAction(system: string, action: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: ({ params, mode }: { params: ActionParams; mode: "preview" | "confirm" }) =>
      request<ActionAnswer>(path(system, action), { method: "POST", body: { params, [mode]: true } }),
    // A preview writes nothing; a confirm moves what they hold and what is unsettled.
    onSettled: (_data, _error, { mode }) => {
      if (mode === "confirm") invalidateAfter(client, "memberAction");
    },
  });
}

/**
 * Where one confirm stands, asked every POLL_MS until it settles. A settled
 * record is kept as it came (the settling poll alone carries the record), so it
 * is never asked again, and a 4xx other than a rate limit ends the asking.
 */
export function useMemberOperation(system: string, id: string | undefined) {
  return useQuery({
    queryKey: ["me", "operations", id],
    enabled: Boolean(id),
    queryFn: () => request<OperationState>(path(system, id ?? "")),
    staleTime: Infinity,
    refetchInterval: (query) => {
      const error = query.state.error;
      if (error instanceof ApiError && error.status >= 400 && error.status < 500 && error.status !== 429) return false;
      return settled(query.state.data?.state) ? false : POLL_MS;
    },
  });
}
