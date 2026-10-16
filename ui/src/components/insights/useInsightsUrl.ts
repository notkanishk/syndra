"use client";

import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useCallback } from "react";

import { fromParams } from "@/components/insights/question";

export type Finding = "drift" | "send" | "single" | "unused" | "overlap";
export const FINDINGS: Finding[] = ["drift", "send", "single", "unused", "overlap"];
export const MAX_PINS = 3;

/**
 * Everything Insights remembers is in the link: the tab, the question (`q`
 * and the parts it names), Map or List, the pins and the finding. Nothing here is component state, so
 * a link opened on a desktop is the map as it was left on a phone.
 */
export function useInsightsUrl() {
  const router = useRouter();
  const pathname = usePathname();
  const params = useSearchParams();
  const raw = params.toString();

  // ponytail: ids are joined with commas; a node id with a comma would split. Real ids never carry one.
  const pins = Array.from(new Set((params.get("pin") ?? "").split(",").filter(Boolean))).slice(0, MAX_PINS);
  const finding = FINDINGS.find((f) => f === params.get("finding")) ?? null;

  /** One call changes everything it names, so a click is one history entry. */
  const set = useCallback(
    (changes: Record<string, string | null>) => {
      const next = new URLSearchParams(raw);
      for (const [key, value] of Object.entries(changes)) {
        if (value) next.set(key, value);
        else next.delete(key);
      }
      const qs = next.toString();
      router.replace(qs ? `${pathname}?${qs}` : pathname, { scroll: false });
    },
    [raw, pathname, router],
  );

  return {
    tab: params.get("tab") === "trends" ? ("trends" as const) : ("map" as const),
    question: fromParams((k) => params.get(k)),
    view: params.get("view") === "list" ? ("list" as const) : ("map" as const),
    pins,
    finding,
    set,
  };
}

export type InsightsUrl = ReturnType<typeof useInsightsUrl>;
