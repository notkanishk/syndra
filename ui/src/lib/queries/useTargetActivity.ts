"use client";

import { useQuery } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

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
  readable: boolean;
  /** Absent entirely when `readable` is false — never an empty list standing in. */
  events?: TargetActivityEvent[];
  /**
   * Shares with auditing switched off. Without it a short list reads as a quiet
   * week when half the shares were never being watched.
   */
  unaudited_shares?: string[];
  detail?: string;
}

export interface TargetActivityEvent {
  at: string;
  event: string;
  share?: string;
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
export function useTargetActivity(target: string | null, subject: string, enabled = true) {
  return useQuery({
    queryKey: ["target-activity", target, subject],
    queryFn: () =>
      request<TargetActivity>(
        `/targets/${encodeURIComponent(target!)}/activity?subject=${encodeURIComponent(subject)}`,
      ),
    enabled: Boolean(target) && Boolean(subject) && enabled,
    // The target's log, not Syndra's record of it: it moves without Syndra
    // doing anything, so a long stale time would show a quiet screen for a busy
    // account.
    staleTime: 60_000,
  });
}
