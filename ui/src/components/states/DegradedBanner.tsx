"use client";

import { useQuery } from "@tanstack/react-query";

import { CommandBlock } from "@/components/ui/CommandBlock";
import { request } from "@/lib/api-client";

interface SystemMode {
  /** "zitadel" when the live directory is answering; "demo" when it is not. */
  directory?: string;
  /**
   * True when demo fixtures were seeded into THIS process's database. In pure
   * local dev that is expected. Anywhere else it means seeded rows — bundles,
   * rules, audit entries — are mixed in with real ones, and no other signal
   * on the screen distinguishes them.
   */
  seed_active?: boolean;
  /**
   * How many stored rows still reference a demo fixture, whichever process
   * wrote them. This is the number that matters: `seed_active` goes false the
   * moment MKAUTH_SEED_DEMO is unset and the backend restarts, while every row
   * the seeder already wrote stays in the database and keeps being served.
   */
  seed_residue?: number;
  /** The command that clears the residue, supplied by the backend. */
  reset_command?: string;
  zitadel_configured?: boolean;
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

  // Two different lies, two different banners.
  //
  // `degraded` means the directory itself fell back — every person, project and
  // role on screen is fiction. The second case is narrower and much easier to
  // miss: real people and projects, with demo bundles, rules and audit rows
  // sitting underneath them.
  //
  // That second case keys off `seed_residue`, not `seed_active`, and the
  // difference is the whole point. `seed_active` reports whether THIS process
  // seeded. An operator who notices demo data, sets MKAUTH_SEED_DEMO=false and
  // restarts gets a backend that stops seeding, keeps serving every row it
  // already seeded, and now reports itself as clean — the banner disappearing
  // reads as confirmation that the fix worked. Counting the rows is the only
  // signal that survives the restart.
  const residue = data?.seed_residue ?? 0;
  const seededOverLive = residue > 0 && data?.directory === "zitadel";

  if (!data?.degraded && !seededOverLive) return null;

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
        {data?.degraded ? (
          <>
            <div className="font-display text-[19px] font-bold">These numbers are not real.</div>
            <p className="max-w-[70ch] text-[14px] font-medium">
              The identity provider is configured but unreachable, so MkAuth is serving demo data.
              Don&rsquo;t grant or revoke anything until this clears.
              {data.reason ? ` (${data.reason})` : ""}
            </p>
          </>
        ) : (
          <>
            <div className="font-display text-[19px] font-bold">
              {residue} rows here came from the demo seeder.
            </div>
            <p className="max-w-[74ch] text-[14px] font-medium">
              People and projects are real. Some bundles, rules, grants and audit entries are
              fixtures, and nothing else on screen tells them apart.{" "}
              {data?.seed_active
                ? "Seeding is still switched on, so a restart would put back anything you delete — set MKAUTH_SEED_DEMO=false first."
                : "Seeding is already off, so these are leftovers from an earlier run; turning the flag off never removed the rows it had already written."}
            </p>
            <div className="mt-3 max-w-[86ch]">
              <CommandBlock
                tone="onWarn"
                command={data?.reset_command ?? "make reset-demo-data"}
                caption="Run this on the deployment host. It prints what it would delete and stops — add APPLY=1 to commit."
                steps={[
                  "Only rows referencing a demo fixture go. Real people, real projects and every decision you actually made stay exactly as they are.",
                  "Nothing upstream is touched. Zitadel keeps whatever it holds; the next reconciliation sweep reports anything unaccounted for as unexplained access.",
                  "For a genuine blank slate instead — no bundles, no rules, no history — use make reset-all-data APPLY=1.",
                ]}
              />
            </div>
          </>
        )}
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
