"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

/**
 * The shadow password vault.
 *
 * Every endpoint here is self-only — the backend refuses any {uid} that is not the authenticated
 * subject, for reads as well as writes. So `userId` is always the caller's own, and these hooks
 * take it as an argument only because the route does.
 *
 * There is deliberately no read of the credential itself. The plaintext is never stored and the
 * Argon2id hash is served on a separate, API-key-only route the browser cannot reach. Status is
 * the most the owner can be told about their own password, which is the correct amount.
 */

export interface ShadowCredentialStatus {
  has_credential: boolean;
  algorithm?: string;
  created_at?: string | null;
  updated_at?: string | null;
  rotated_at?: string | null;
  expires_at?: string | null;
}

/** One lifecycle event. `failed_validation` is recorded too — a vault with no failed attempts in
 *  it is a vault that is not telling you when somebody tried. */
export interface ShadowCredentialAuditEntry {
  id: string;
  user_id: string;
  action: "set" | "rotated" | "cleared" | "failed_validation" | string;
  actor_id: string;
  ip_address?: string;
  created_at: string;
}

const KEYS = {
  status: (uid: string) => ["shadow-credential", uid, "status"] as const,
  audit: (uid: string) => ["shadow-credential", uid, "audit"] as const,
};

export function useShadowCredentialStatus(userId: string) {
  return useQuery({
    queryKey: KEYS.status(userId),
    queryFn: async (): Promise<ShadowCredentialStatus> =>
      request<ShadowCredentialStatus>(`/users/${userId}/shadow-credential/status`),
    enabled: Boolean(userId),
  });
}

export function useShadowCredentialAudit(userId: string) {
  return useQuery({
    queryKey: KEYS.audit(userId),
    queryFn: async (): Promise<ShadowCredentialAuditEntry[]> => {
      const data = await request<unknown>(`/users/${userId}/shadow-credential/audit`);
      return Array.isArray(data) ? (data as ShadowCredentialAuditEntry[]) : [];
    },
    enabled: Boolean(userId),
  });
}

/**
 * Set or rotate the password. The backend judges complexity and returns the failing requirements
 * composed into one message — this does not re-implement those rules, so there is exactly one
 * authority on what a strong enough password is and no chance of the two drifting apart.
 */
export function useSetShadowCredential(userId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (password: string) =>
      request<{ message: string }>(`/users/${userId}/shadow-credential`, {
        method: "PUT",
        body: { password },
      }),
    // A rejected attempt is recorded as `failed_validation`, so the trail is refetched on
    // settle rather than on success: a failure changes the log too.
    onSettled: () => {
      qc.invalidateQueries({ queryKey: KEYS.status(userId) });
      qc.invalidateQueries({ queryKey: KEYS.audit(userId) });
    },
  });
}

export function useClearShadowCredential(userId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async () =>
      request<{ message: string }>(`/users/${userId}/shadow-credential`, { method: "DELETE" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: KEYS.status(userId) });
      qc.invalidateQueries({ queryKey: KEYS.audit(userId) });
    },
  });
}

export const shadowCredentialQueryKeys = KEYS;
