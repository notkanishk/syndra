"use client";

import { useQuery } from "@tanstack/react-query";

import { request } from "@/lib/api-client";
import type { NavSystem } from "@/lib/nav";

/** One system the signed-in person meets: its kind is always known. */
export type MySystem = Required<NavSystem>;

/**
 * The systems the signed-in person meets, by name (GET /me/systems): the one
 * source for the member's rail, the touch bar, Go to, the system page's gate
 * and an operator's account sheet. The registry and the database only, never
 * an add-on call, so navigation is drawn from one cheap read and does not wait
 * on every add-on answering (that is GET /me/targets, the cards' read). A
 * system not read yet is never in it (operators-only), and the backend sorts
 * it the way lib/sort does.
 */
export function useMySystems(enabled = true) {
  return useQuery({
    queryKey: ["me", "systems"],
    enabled,
    queryFn: () => request<{ systems: MySystem[] }>("/me/systems"),
    select: (data) => data.systems ?? [],
    staleTime: 60_000,
  });
}
