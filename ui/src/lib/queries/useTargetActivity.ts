"use client";

import { useQuery } from "@tanstack/react-query";

import { useReadAgain } from "@/lib/queries/fresh";
import { readFrom, type WireRead } from "@/lib/queries/read";

/** The activity log as a `Read`; "no account here" is a reading, not a failure. */
const readActivity = (path: string) => readFrom<TargetActivity>(path, { answeredBare: (b) => b.no_account === true });

/**
 * What a target's OWN audit log holds for one person.
 *
 * Not Syndra's audit feed, and deliberately not merged into it. Syndra's ledger
 * records what Syndra did; this records what the account did on the target,
 * including everything that happened with no involvement from Syndra at all.
 * Rendering them as one stream would make the second look like the first, which
 * is the whole reason a target is reconciled rather than trusted.
 *
 * `readable` is load-bearing. An empty `events` list from a log that could not
 * be read and an empty list from a log that recorded nothing are opposite
 * answers, and the surface has to be able to tell them apart.
 */
export interface TargetActivity {
  target: string;
  subject: string;
  /** Absent with `no_account`, the one reply that is a fact and not a read. */
  read?: WireRead;
  /**
   * This person has nothing bound on the target: a positive fact, read as
   * neutral, never as an error, and never as a read that failed.
   */
  no_account?: boolean;
  events?: TargetActivityEvent[];
  /**
   * Resources (shares) that were not recording this person. Without it a short
   * list reads as a quiet week when half the shares were never being watched.
   */
  uncovered?: string[];
  truncated?: boolean;
}

export interface TargetActivityEvent {
  at: string;
  event: string;
  /** Where on the target it happened (a share), named as `uncovered` names them. */
  resource?: string;
  success: boolean;
  /**
   * Where it came from. Measured on the live NAS: a week of SMB events was 553
   * rows, every one an authentication failure, spread across seven client
   * addresses — which is the difference between one person's saved password
   * being stale and somebody working through the building.
   */
  address?: string;
  /**
   * The target's own status token for the outcome, and only a token: the
   * add-on admits it by a check on its SHAPE, so an NTSTATUS passes and a
   * sentence does not. `NT_STATUS_NO_SUCH_USER` is what turns "Refused" into
   * something an operator can act on.
   */
  detail?: string;
}

/**
 * Operator-gated and subject-scoped. The member-facing read is `storage.status`,
 * which takes no subject at all — there is no shape in which a member asks this
 * about somebody else.
 */
const activityPath = (target: string | null, subject: string) =>
  `/targets/${encodeURIComponent(target!)}/activity?subject=${encodeURIComponent(subject)}`;

/** "Read again" on the log: the same GET, asking the target itself. */
export const useTargetActivityAgain = (target: string | null, subject: string) =>
  useReadAgain(["target-activity", target, subject], activityPath(target, subject), readActivity);

export function useTargetActivity(target: string | null, subject: string, enabled = true) {
  return useQuery({
    queryKey: ["target-activity", target, subject],
    queryFn: () => readActivity(activityPath(target, subject)),
    enabled: Boolean(target) && Boolean(subject) && enabled,
    // The target's log, not Syndra's record of it: it moves without Syndra
    // doing anything, so a long stale time would show a quiet screen for a busy
    // account.
    staleTime: 60_000,
  });
}
