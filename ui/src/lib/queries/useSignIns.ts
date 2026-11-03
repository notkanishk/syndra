"use client";

import { useMutation, useQuery } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

/**
 * Sign-ins (A-10 Part 2; owner, 3 Nov: "1a with statistics").
 *
 * Syndra keeps each person's last sign-in per app and each app's daily total,
 * naming nobody. "Who signed in, and when" is read from Zitadel's own log only
 * through Look up sign-ins: with a reason, audited, and kept nowhere.
 */

export interface SignInRow {
  project_id: string;
  /** "" is "app unknown": older rows, or an app that didn't resolve. */
  app_id: string;
  kind: "sign_in" | "badge";
  last_at: string;
}

export interface PersonSignIns {
  usage: SignInRow[];
  /** A sign-in reached Syndra in the last 90 days. Without it, say nothing. */
  receiving: boolean;
  /** The earliest Syndra is known to have been reading sign-ins. */
  receiving_since?: string;
}

/** One person's last sign-in per app (operators). */
export function usePersonSignIns(userId: string, enabled = true) {
  return useQuery({
    queryKey: ["users", userId, "usage"],
    enabled,
    queryFn: async (): Promise<PersonSignIns> => {
      const d = await request<PersonSignIns>(`/users/${encodeURIComponent(userId)}/usage`);
      return { usage: d?.usage ?? [], receiving: Boolean(d?.receiving), receiving_since: d?.receiving_since };
    },
  });
}

export interface AppSignIns {
  receiving: boolean;
  people_7d?: number;
  people_30d?: number;
  last_at?: string | null;
  /** Daily totals, server-local days, oldest first; days with none are absent. */
  days?: Array<{ day: string; count: number }>;
}

/** One app's sign-ins, naming nobody (operators). */
export function useAppSignIns(appId: string, enabled = true) {
  return useQuery({
    queryKey: ["applications", appId, "usage"],
    enabled,
    queryFn: () => request<AppSignIns>(`/applications/${encodeURIComponent(appId)}/usage`),
  });
}

export type LookupRange = 7 | 30 | 90;

export interface LookedUpSignIn {
  user_id: string;
  app_id: string;
  project_id: string;
  at: string;
}

export interface SignInLookup {
  sign_ins: LookedUpSignIn[];
  /** Zitadel's log held more in the range than one lookup reads. */
  truncated: boolean;
  since: string;
  until: string;
}

/** Look up sign-ins in Zitadel's own log. Never cached: each one is audited. */
export function useLookUpSignIns() {
  return useMutation({
    mutationFn: (ask: { user_id?: string; app_id?: string; days: LookupRange; reason: string }) =>
      request<SignInLookup>("/sign-ins/lookup", { method: "POST", body: ask }),
  });
}
