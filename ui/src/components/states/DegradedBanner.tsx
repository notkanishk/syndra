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
   * moment SYNDRA_SEED_DEMO is unset and the backend restarts, while every row
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
  // seeded. An operator who notices demo data, sets SYNDRA_SEED_DEMO=false and
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
      className="flex items-start gap-3.5 bg-warn px-4 py-4 text-warn-ink tablet:px-[26px]"
    >
      {/* The mark breathes, not the banner. This is one of the product's two
          licensed loops and it means "still happening" — a whole amber field
          pulsing behind text would be unreadable, and would be decoration
          rather than the statement that the provider is still unreachable. */}
      <span
        aria-hidden
        className="breathe mt-0.5 flex h-[22px] w-[22px] flex-none items-center justify-center rounded-pill bg-warn-ink text-[13px] font-bold text-warn"
      >
        !
      </span>
      <div>
        {data?.degraded ? (
          <>
            <div className="font-display text-[19px] font-bold">These numbers are not real.</div>
            <p className="max-w-[70ch] text-[14px] font-medium">
              Syndra cannot reach Zitadel (the service everyone signs in through), so it is showing
              sample data instead of real people and access. Do not give or revoke (end) any access
              until this banner goes away.
            </p>
            {data.reason ? (
              <p className="mt-1.5 max-w-[70ch] text-[13px]">
                For the person who runs the Syndra server: {data.reason}
              </p>
            ) : null}
          </>
        ) : (
          <>
            <div className="font-display text-[19px] font-bold">
              {residue} items on these screens are sample data.
            </div>
            <p className="max-w-[74ch] text-[14px] font-medium">
              People and projects are real. Some bundles, rules, access and history entries are
              sample data, and nothing else on screen tells them apart. Ask the person who runs
              the Syndra server to clear the sample data. Until then, treat anything you cannot
              confirm as sample.
            </p>
            {/* Marked as theirs, so staff know the paragraph below is not for them. */}
            <div className="mt-3 max-w-[86ch]">
              <div className="mb-1.5 type-label">For the person who runs the Syndra server</div>
              <p className="mb-2 max-w-[74ch] text-[13.5px]">
                {data?.seed_active
                  ? "Sample data is still switched on, so a restart would put it back after you clear it. Set SYNDRA_SEED_DEMO=false first."
                  : "Sample data is switched off, so what remains is left over from an earlier run. Switching it off never removes what was already written."}
              </p>
              <CommandBlock
                tone="onWarn"
                command={data?.reset_command ?? "make reset-demo-data"}
                caption="Run this on the server that hosts Syndra. It prints what it would delete and stops. Add APPLY=1 to delete."
                steps={[
                  "Only sample data is deleted. Real people, real projects and every decision you made stay exactly as they are.",
                  "Zitadel is not changed. Anything left over there is reported under Unexplained access at the next check.",
                  "For a blank slate instead — no bundles, no rules, no history — use make reset-all-data APPLY=1.",
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
