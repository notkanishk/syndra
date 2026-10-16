"use client";

import { useQuery } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

/** One change to the caller's own access (GET /me/changes, B10). */
export interface MyChange {
  kind: "request_approved" | "request_declined" | "request_withdrawn" | "given" | "taken_away";
  at: string;
  project_id: string;
  role_key: string;
  /** Who decided. Absent when it was automatic. */
  by?: string;
  /** The decider's reason, or the grant's. */
  note?: string;
  via: { bundle_id?: string; rule_id?: string } | null;
  until?: string | null;
  /** An approval not on their account yet (N19), and why: role_waits or halted:<reason>. */
  waiting?: boolean;
  wait_reason?: string;
}

/** The last 30 days of changes to your own access, newest first. */
export function useMyChanges() {
  return useQuery({
    queryKey: ["me", "changes"],
    queryFn: () => request<{ items: MyChange[] }>("/me/changes?days=30"),
    select: (data) => data.items ?? [],
  });
}
