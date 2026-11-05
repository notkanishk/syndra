"use client";

import { useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { useEffect } from "react";

import { prefetchAppClaimShape } from "@/lib/queries/useClaimShape";
import { prefetchUserAccess } from "@/lib/queries/useUsers";

/** Hover has to rest this long before it counts as intent (A-19 P9). */
export const INTENT_MS = 80;

type Connection = { saveData?: boolean; effectiveType?: string };

/** Data Saver on, or a connection that slow: the next page is asked for when it is opened, not before. */
export function savingData(): boolean {
  const c = (navigator as Navigator & { connection?: Connection }).connection;
  return Boolean(c?.saveData) || c?.effectiveType === "slow-2g" || c?.effectiveType === "2g";
}

/**
 * Anticipating the next step (A-19 P9). Resting the pointer on a link, or
 * focusing it, warms its page: Next's route code, and for a person or an app
 * the read the page opens with, so the page is usually complete on its first
 * frame. Nothing is visible, nothing moves. Each address is warmed once per
 * visit; a pointer that only passes over (under 80ms) warms nothing; and with
 * Data Saver or a slow connection nothing is warmed at all.
 */
export function IntentPrefetchHost() {
  const qc = useQueryClient();
  const router = useRouter();

  useEffect(() => {
    const warmed = new Set<string>();
    let timer: ReturnType<typeof setTimeout> | undefined;

    const warm = (href: string) => {
      if (warmed.has(href) || savingData()) return;
      warmed.add(href);
      router.prefetch(href);
      const [, kind, id] = href.split("?")[0].split("/");
      if (!id) return;
      if (kind === "users") void prefetchUserAccess(qc, decodeURIComponent(id));
      if (kind === "applications") void prefetchAppClaimShape(qc, decodeURIComponent(id));
    };

    const linkOf = (event: Event) => {
      const link = (event.target as Element | null)?.closest?.("a[href]");
      const href = link?.getAttribute("href");
      // This app's own pages only: a path, never `//host`, never a download or a new tab.
      if (!href || !href.startsWith("/") || href.startsWith("//")) return null;
      if (link?.getAttribute("target") === "_blank" || link?.hasAttribute("download")) return null;
      return href;
    };

    const intend = (event: Event) => {
      if ((event as PointerEvent).pointerType === "touch") return; // a touch is a press, not a hover
      const href = linkOf(event);
      clearTimeout(timer);
      if (href) timer = setTimeout(() => warm(href), INTENT_MS);
    };
    const drop = () => clearTimeout(timer);

    document.addEventListener("pointerover", intend);
    document.addEventListener("focusin", intend);
    document.addEventListener("pointerout", drop);
    document.addEventListener("focusout", drop);
    return () => {
      clearTimeout(timer);
      document.removeEventListener("pointerover", intend);
      document.removeEventListener("focusin", intend);
      document.removeEventListener("pointerout", drop);
      document.removeEventListener("focusout", drop);
    };
  }, [qc, router]);

  return null;
}
