"use client";

import { useQuery } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

export interface CatalogUser {
  id: string;
  name: string;
  title: string;
  email: string;
}

/**
 * The persona list for pickers. /catalog is the one path members are allowed
 * to read for it; we take just the `users` slice and cache it under a shared
 * key so the few pages that need it make one network call between them.
 */
export function useCatalogUsers() {
  return useQuery({
    queryKey: ["catalog", "users"],
    queryFn: async (): Promise<CatalogUser[]> => {
      const data = await request<{ users?: CatalogUser[] }>("/catalog");
      return Array.isArray(data?.users) ? data.users : [];
    },
  });
}
