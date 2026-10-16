"use client";

import { StatusDot } from "@/components/ui/Badge";
import { askLengthForMember, askLengthForOperator, formatShortDate } from "@/lib/format";
import type { AccessRequest } from "@/lib/queries/useRequests";
import { useRequestsMine } from "@/lib/queries/useRequests";
import { nothingSentYet } from "@/components/access/AccessSource";
import { useMyChanges } from "@/lib/queries/useMyChanges";
import { useUserAccess, useUserGrants } from "@/lib/queries/useUsers";

/**
 * What someone already has of a role they ask for, said where it decides
 * something.
 *
 * Found on production (15 Oct): a member asked five times for a role they held
 * for good, the queue showed all five, and approving one would have cut their
 * access to 64 days. A request is now only for a role someone lacks, one per
 * role (owner, 15 Oct), and the backend refuses the rest; these are the same
 * facts, said before anyone acts.
 */

/** Whether the requester already has the role: approving is then off. */
export function alreadyHeld(request: AccessRequest): boolean {
  const kind = request.holding?.kind;
  // Keeping a role longer is asked of a role with an end: only one with none is settled.
  if (request.kind === "extend") return kind === "for_good";
  return Boolean(kind && kind !== "none");
}

/**
 * The ask's duration line, for the person deciding: how long, and the date
 * approving it now would end it; or, for a request to keep a role, that it is.
 * A member reads `describeAskForMember`: the length only.
 */
export function describeAsk(request: AccessRequest): string {
  const days = request.duration_days ?? 0;
  if (request.kind !== "extend") return askLengthForOperator(days);
  return `Asks to keep it longer · ${
    days < 1 ? "with no end date" : `for ${days} more ${days === 1 ? "day" : "days"}`
  }`;
}

/** The same line for the person who asked: no date, because approving starts it then. */
export function describeAskForMember(request: AccessRequest): string {
  const days = request.duration_days ?? 0;
  if (request.kind !== "extend") return askLengthForMember(days);
  return describeAsk(request);
}

/**
 * For the person deciding. A request is for a role someone lacks (owner,
 * 15 Oct); if they have come to hold it since asking, approving is off, and the
 * row says why and what closes it.
 */
export function HoldingNote({ request }: { request: AccessRequest }) {
  const holding = request.holding;
  if (request.kind === "extend" && holding && holding.kind !== "for_good") {
    return (
      <span className="w-full text-[13px] text-muted">
        {holding.kind === "until"
          ? `Has it until ${formatShortDate(holding.until)}.`
          : "It ended while this waited. Approving gives it back for the time asked."}
      </span>
    );
  }
  if (!holding || holding.kind === "none") return null;
  return (
    <span className="flex w-full items-baseline gap-2 text-[13px] text-warn-text">
      <StatusDot tone="warn" />
      {holding.kind === "until"
        ? `Already has this, until ${formatShortDate(holding.until)}.`
        : "Already has this, with no end."}{" "}
      There is nothing to approve; decline it to close the request.
    </span>
  );
}

export type AskState =
  | { kind: "none" }
  /** Asked, and waiting for makerspace staff. */
  | { kind: "asked"; at?: string }
  | { kind: "for_good" }
  | { kind: "until"; until: string }
  /** Given, and not yet sent to Zitadel. */
  | { kind: "onway"; at?: string };

/**
 * What a person already has, or has asked for, of each role: the dialog's
 * reading of the same facts the backend checks. Only "none" can be asked for,
 * and "until" can be asked to keep.
 */
export function useAskState(userId: string) {
  const access = useUserAccess(userId);
  const grants = useUserGrants(userId);
  const requests = useRequestsMine();
  const changes = useMyChanges();

  return (projectId: string, roleKey: string): AskState => {
    if (!projectId || !roleKey) return { kind: "none" };
    const waiting = (requests.data ?? []).find(
      (r) => r.status === "pending" && r.requester_id === userId && r.project_id === projectId && r.role_key === roleKey,
    );
    if (waiting) return { kind: "asked", at: waiting.created_at };
    const grant = (grants.data ?? []).find((g) => g.project_id === projectId && g.role_key === roleKey);
    if (grant?.expires_at) return { kind: "until", until: grant.expires_at };
    const project = access.data?.projects?.find((p) => p.project_id === projectId);
    const role = [...(project?.source_roles ?? []), ...(project?.derived_roles ?? [])].find(
      (r) => r.role_key === roleKey,
    );
    if (role && nothingSentYet(role.reasons)) {
      // The newest thing that gave it: when the member was told.
      const at = (changes.data ?? [])
        .filter((c) => c.project_id === projectId && c.role_key === roleKey && (c.kind === "given" || c.waiting))
        .map((c) => c.at)
        .sort()
        .pop();
      return { kind: "onway", at };
    }
    const observed = project?.observed_role_keys?.includes(roleKey);
    if (grant || observed) return { kind: "for_good" };
    return { kind: "none" };
  };
}
