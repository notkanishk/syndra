"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";

import { CommandBlock } from "@/components/ui/CommandBlock";
import { Notice } from "@/components/ui/Notice";
import { Relative } from "@/components/ui/Time";
import { Unfold } from "@/components/ui/Unfold";
import { request } from "@/lib/api-client";
import { formatWhen } from "@/lib/format";
import { useUiView } from "@/lib/ui-view";

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
  /** When Zitadel last answered a directory read; null when it never has. */
  last_read_at?: string | null;
  reason?: string;
}

/**
 * Degraded — the fourth state, and the one that matters.
 *
 * GET /api/v1/system/mode reports degraded: true when Zitadel is configured
 * and isn't answering. Syndra never shows sample data in its place (N23): with
 * a last good read (`last_read_at`) the page shows that, real and labelled
 * with its age; with none it shows nothing. The Notice sits in the content
 * column above the page (proposed/Notice.md, Motion 6a), the page is not
 * dimmed, and it leaves the same way when Zitadel answers.
 *
 * Amber rather than red: the data is old, not wrong. And the sentence says so
 * in words — colour is never the only signal.
 */
export function DegradedBanner() {
  // An operator's read: a member's proxy refuses /system/mode, every minute.
  const { isOperator } = useUiView();
  const { data } = useQuery({
    queryKey: ["system", "mode"],
    queryFn: () => request<SystemMode>("/system/mode"),
    enabled: isOperator,
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
  const [serverOpen, setServerOpen] = useState(false);
  const client = useQueryClient();
  const degraded = Boolean(data?.degraded);
  const wasDegraded = useRef(degraded);
  // Zitadel answering again: every list read while it was down asks again,
  // rather than waiting out its stale time.
  useEffect(() => {
    if (wasDegraded.current && !degraded) void client.invalidateQueries();
    wasDegraded.current = degraded;
  }, [client, degraded]);
  const residue = data?.seed_residue ?? 0;
  const seededOverLive = residue > 0 && data?.directory === "zitadel";
  const open = Boolean(data?.degraded || seededOverLive);

  return (
    <Unfold open={open}>
      <div className="pb-4">
        {data?.degraded ? (
          data.last_read_at ? (
            <Notice
              tone="warn"
              live
              title={`Zitadel isn't answering. This is what it held at ${formatWhen(data.last_read_at)}.`}
            >
              Everything below is real, read <Relative iso={data.last_read_at} />, and may have
              changed since. Giving and revoking access is paused until Zitadel answers, because a
              decision made on an old read can be wrong.
            </Notice>
          ) : (
            <Notice
              tone="warn"
              live
              blocking
              title="Zitadel isn't answering, and Syndra hasn't read it yet."
            >
              There is nothing to show until it answers. This page tries again every minute on its
              own, and giving and revoking access waits until then.
            </Notice>
          )
        ) : (
          <Notice tone="warn" title={`${residue} things on these screens are sample data.`}>
            People and projects are real. Some bundles, rules, access and history are sample data,
            and nothing else on screen tells them apart, so treat anything you can&apos;t confirm
            as sample until it&apos;s cleared.
            <button
              type="button"
              aria-expanded={serverOpen}
              onClick={() => setServerOpen((v) => !v)}
              className="mt-2 flex min-h-[44px] items-center gap-1.5 text-[13.5px] font-semibold text-ink motion-tint hover:text-accent-text desktop:min-h-[32px]"
            >
              <span
                aria-hidden
                className={`inline-block motion-press ${serverOpen ? "rotate-90" : ""}`}
              >
                ›
              </span>
              For whoever runs the Syndra server
            </button>
            <Unfold open={serverOpen}>
              <div className="flex max-w-[86ch] flex-col gap-2 pt-1">
                <p className="text-[13.5px] text-muted">
                  {data?.seed_active
                    ? "Sample data is still switched on, so a restart would put it back after you clear it. Set SYNDRA_SEED_DEMO=false first."
                    : "Sample data is switched off, so what remains is left over from an earlier run. Switching it off never removes what was already written."}
                </p>
                <CommandBlock
                  tone="warn"
                  command={data?.reset_command ?? "make reset-demo-data"}
                  caption="Run this on the server that hosts Syndra. It prints what it would delete and stops. Add APPLY=1 to delete."
                  steps={[
                    "Only sample data is deleted. Real people, real projects and every decision you made stay exactly as they are.",
                    "Zitadel is not changed. Anything left over there is reported under Drift at the next check.",
                    "For a blank slate instead, with no bundles, rules or history, use make reset-all-data APPLY=1.",
                  ]}
                />
              </div>
            </Unfold>
          </Notice>
        )}
      </div>
    </Unfold>
  );
}

export function useDegraded(): boolean {
  const { data } = useQuery({
    queryKey: ["system", "mode"],
    queryFn: () => request<SystemMode>("/system/mode"),
    refetchInterval: 60_000,
    retry: false,
  });
  return Boolean(data?.degraded);
}
