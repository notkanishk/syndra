"use client";

import { useEffect, useState } from "react";

import { CheckTrail, type TrailCheck } from "@/components/ui/CheckTrail";
import { formatClock } from "@/lib/format";
import { useUserAccess } from "@/lib/queries/useUsers";
import type { Standing } from "@/lib/standing";
import { isRefused } from "@/lib/wait-reason";

export type ChangeAct = "give" | "revoke";

/** The trail's name, read before its steps. */
export const TRAIL_LABEL = "On its way to Zitadel";

/** While the last step is open, the person is read again this often (the page's own query)... */
export const TRAIL_REREAD_MS = 15_000;
/** ...and for no longer than this: a change still not read back by then is the row's to say. */
export const TRAIL_WATCH_MS = 6 * 60_000;

/** What the trail needs from an act's answer: whether Zitadel took it, and when the answer came. */
export interface TrailStart {
  sent: boolean;
  at: number;
}

/**
 * The one rule for whether an act's answer has a trail (A-24 §5). `status` is
 * the status of the act's OWN outbox row, or empty when it queued none (a give
 * that only cancelled a waiting removal; a role something else still gives):
 * then nothing went toward Zitadel and there is no trail. A refusal has none
 * either: its outcome says what happened, and nothing travels on.
 */
export function changeTrailFor(status: string | undefined, waitReason: string | undefined): TrailStart | undefined {
  if (!status || status === "failed" || isRefused(waitReason)) return undefined;
  return { sent: status === "applied" && !waitReason, at: Date.now() };
}

/**
 * A change's trail (A-24 §5, Q8): three steps in Syndra's own words, and each
 * one settles only on what was observed, never ahead of it.
 *
 *  - **Recorded in Syndra.** The act answered: its ledger and outbox rows are
 *    written before it returns (the doctrine seam), so a result means this.
 *  - **Sent to Zitadel.** The act's own answer said Zitadel took the call
 *    (`sent`: status applied, nothing waiting); for a give, also a later read
 *    whose standing is `sent` (Zitadel accepted it, from Syndra's own rows); or
 *    the last step, which cannot be seen without it.
 *  - **In Zitadel** (give) or **No longer in Zitadel** (revoke): only from a
 *    Zitadel read made after the act (`zitadel`), whose standing for the role
 *    is `held` or `not_held`. Until then a ring that says what is known.
 *
 * `standing` is from a read of the person fetched after the act, or undefined.
 */
export function trailSteps(
  act: ChangeAct,
  sent: boolean,
  standing: Standing,
  zitadel?: { readAt: string },
): TrailCheck[] {
  const there = zitadel !== undefined && (act === "give" ? standing === "held" : standing === "not_held");
  const went = sent || there || (act === "give" && standing === "sent");
  const done = (sentence: string, faint?: string[]): TrailCheck => ({ tone: "healthy", sentence, faint });
  const open = (sentence: string): TrailCheck => ({ tone: "neutral", ring: true, sentence });
  return [
    done("Recorded in Syndra."),
    went ? done("Sent to Zitadel.") : open("Not sent to Zitadel yet."),
    there
      ? done(act === "give" ? "In Zitadel." : "No longer in Zitadel.", [`Read back at ${formatClock(zitadel.readAt)}.`])
      : went
        ? open("Not read back from Zitadel yet.")
        : open(act === "give" ? "Not in Zitadel yet." : "Not removed from Zitadel yet."),
  ];
}

/**
 * The trail under a give's or a revoke's result. It reads the person's access,
 * the same read the sheet and the page already hold (the act invalidates it),
 * and trusts two clocks, never the browser's against the server's:
 *  - a fetch counts once it landed after the act (`at`, both the browser's);
 *  - a Zitadel read counts only once it is newer than the one the person's
 *    access carried when the trail appeared (both the server's). A standing
 *    from an older Zitadel read says nothing about this act, however recently
 *    it was fetched. A read that did not complete (`observation.current`
 *    false) is not a read of Zitadel at all, and settles nothing.
 * While its last step is open it reads the person again every
 * `TRAIL_REREAD_MS`, through the page's own query, for at most
 * `TRAIL_WATCH_MS`, and stops once that step settles or the sheet closes.
 * Its steps fill on tint as each is observed (the dot's own transition).
 */
export function ChangeTrail({
  userId,
  projectId,
  roleKey,
  act,
  sent,
  at,
  className = "",
}: {
  userId: string;
  projectId: string;
  roleKey: string;
  act: ChangeAct;
  /** The act's own answer: Zitadel took it, and nothing waits. */
  sent: boolean;
  /** When the act answered, in ms: a fetch older than this says nothing about it. */
  at: number;
  className?: string;
}) {
  const access = useUserAccess(userId);
  // The Zitadel read the person's access carried as the result appeared: only a newer one is about this act.
  // Nothing cached then (undefined) leaves nothing to compare with, so no Zitadel read can settle a step.
  const [before] = useState(() => (access.data ? (access.data.observation?.read_at ?? null) : undefined));
  const after = access.data !== undefined && access.dataUpdatedAt >= at;
  const observation = after ? access.data?.observation : undefined;
  const readAt = observation?.current ? observation.read_at : undefined;
  const newer =
    readAt && before !== undefined && (before === null || Date.parse(readAt) > Date.parse(before)) ? { readAt } : undefined;
  const standing = after ? access.data?.projects?.find((p) => p.project_id === projectId)?.standings?.[roleKey] : undefined;
  const steps = trailSteps(act, sent, standing, newer);
  const open = steps[steps.length - 1].tone !== "healthy";

  // Read again while the last step is open, on the page's own query, never past the watch.
  const reread = access.refetch;
  useEffect(() => {
    if (!open) return;
    const until = at + TRAIL_WATCH_MS;
    const timer = setInterval(() => {
      if (Date.now() >= until) clearInterval(timer);
      else void reread();
    }, TRAIL_REREAD_MS);
    return () => clearInterval(timer);
  }, [open, reread, at]);

  return (
    <div className={className}>
      <CheckTrail label={TRAIL_LABEL} checks={steps} />
    </div>
  );
}
