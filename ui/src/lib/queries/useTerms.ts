"use client";

import { useQuery } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

/** One term, both days inclusive, as YYYY-MM-DD (GET /terms). */
export interface Term {
  name: string;
  start: string;
  end: string;
}

/** The term dates, one setting any signed-in person may read. */
export function useTerms() {
  return useQuery({
    queryKey: ["terms"],
    queryFn: () => request<{ terms: Term[] }>("/terms"),
    select: (data) => data.terms ?? [],
    staleTime: 5 * 60_000,
  });
}
