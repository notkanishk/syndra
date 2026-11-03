"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

/** Welcome roles at a first sign-in (GET /config/welcome-claims). Who set it and when are null when it was never set. */
export interface WelcomeClaimsSetting {
  on: boolean;
  set_by?: string | null;
  set_at?: string | null;
}

const key = ["welcome-claims"] as const;

/** The switch, operator only. Its one write place is Makerspace settings. */
export function useWelcomeClaims() {
  return useQuery({ queryKey: key, queryFn: () => request<WelcomeClaimsSetting>("/config/welcome-claims") });
}

/** PUT /config/welcome-claims: the data plane follows within a second. */
export function useSaveWelcomeClaims() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (on: boolean) => request<WelcomeClaimsSetting>("/config/welcome-claims", { method: "PUT", body: { on } }),
    onSuccess: (saved) => qc.setQueryData(key, saved),
  });
}
