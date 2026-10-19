"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

/** One term, both days inclusive, as YYYY-MM-DD (GET /terms). */
export interface Term {
  name: string;
  start: string;
  end: string;
}

/** The setting as read: who last set it and when are null for a member, or when it was never set. */
export interface TermSetting {
  terms: Term[];
  set_by?: string | null;
  set_at?: string | null;
}

const termsKey = ["terms"] as const;
const readTerms = () => request<TermSetting>("/terms");

/** The term dates, one setting any signed-in person may read. */
export function useTerms() {
  return useQuery({
    queryKey: termsKey,
    queryFn: readTerms,
    select: (data) => data.terms ?? [],
    staleTime: 5 * 60_000,
  });
}

/** The same read with its provenance, for Makerspace settings. */
export function useTermSetting() {
  return useQuery({ queryKey: termsKey, queryFn: readTerms, staleTime: 5 * 60_000 });
}

/** PUT /terms (operator only). Every reader of the dates follows: Trends included. */
export function useSaveTerms() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (terms: Term[]) => request("/terms", { method: "PUT", body: { terms } }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: termsKey });
      qc.invalidateQueries({ queryKey: ["insights"] });
    },
  });
}
