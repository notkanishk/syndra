"use client";

import { useQuery } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

interface SystemMode {
  mode?: string;
  degraded?: boolean;
  reason?: string;
}

/**
 * Degraded — the fourth state, and the one that matters.
 *
 * GET /api/v1/system/mode reports degraded: true when Zitadel is configured
 * but the backend fell back to demo data. At that point every number on screen
 * is fiction, so the banner is persistent and cannot be dismissed, and the
 * content behind it is dimmed.
 *
 * Amber rather than red: the data is wrong, not dangerous. And the sentence
 * says so in words — colour is never the only signal.
 */
export function DegradedBanner() {
  const { data } = useQuery({
    queryKey: ["system", "mode"],
    queryFn: () => request<SystemMode>("/system/mode"),
    refetchInterval: 60_000,
    retry: false,
  });

  if (!data?.degraded) return null;

  return (
    <div
      role="alert"
      className="sticky top-0 z-40 flex items-start gap-3.5 bg-warn px-[26px] py-4 text-warn-ink"
    >
      <span
        aria-hidden
        className="mt-0.5 flex h-[22px] w-[22px] flex-none items-center justify-center rounded-pill bg-warn-ink text-[13px] font-bold text-warn"
      >
        !
      </span>
      <div>
        <div className="font-display text-[19px] font-bold">These numbers are not real.</div>
        <p className="max-w-[70ch] text-[14px] font-medium">
          The identity provider is configured but unreachable, so MkAuth is serving demo data.
          Don&rsquo;t grant or revoke anything until this clears.
          {data.reason ? ` (${data.reason})` : ""}
        </p>
      </div>
    </div>
  );
}

/**
 * Wraps page content while degraded so it reads as the fiction it is. Kept
 * separate from the banner because the banner is mounted once in the shell and
 * this is applied per page.
 */
export function useDegraded(): boolean {
  const { data } = useQuery({
    queryKey: ["system", "mode"],
    queryFn: () => request<SystemMode>("/system/mode"),
    refetchInterval: 60_000,
    retry: false,
  });
  return Boolean(data?.degraded);
}
