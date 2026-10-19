"use client";

import { StatusDot } from "@/components/ui/Badge";
import { askLengthForMember, askLengthForOperator, formatShortDate } from "@/lib/format";
import type { AccessRequest } from "@/lib/queries/useRequests";
import { useRequestsMine } from "@/lib/queries/useRequests";
import { givenAt, nothingSentYet } from "@/components/access/AccessSource";
import { useMyChanges } from "@/lib/queries/useMyChanges";
import { useNameResolver } from "@/lib/queries/useNameResolver";
import type { Term } from "@/lib/queries/useTerms";
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

/** The same calendar day, in local time: a term's last day against an end counted in days. */
function sameDay(ymd: string, iso: string): boolean {
  const [y, m, d] = ymd.split("-").map(Number);
  const end = new Date(iso);
  return end.getFullYear() === y && end.getMonth() === m - 1 && end.getDate() === d;
}

/**
 * How much longer a request to keep a role asks for, in the words it was chosen
 * in: "Another month", "To the end of next term" (the new end is a term's last
 * day), else "N days".
 */
export function keepLength(request: AccessRequest, terms?: Term[]): string {
  const days = request.duration_days ?? 0;
  if (days < 1) return "With no end date";
  if (request.until && terms?.some((t) => sameDay(t.end, request.until!))) return "To the end of next term";
  if (days >= 28 && days <= 31) return "Another month";
  return `${days} ${days === 1 ? "day" : "days"}`;
}

/**
 * The ask's duration line, for the person deciding: how long, and the date
 * approving it now would end it; or, for a request to keep a role, how much
 * longer and where that lands. A member reads `describeAskForMember`.
 */
export function describeAsk(request: AccessRequest, terms?: Term[]): string {
  const days = request.duration_days ?? 0;
  if (request.kind !== "extend") return askLengthForOperator(days);
  const length = keepLength(request, terms);
  if (!request.until || !request.current_until) return length;
  return `${length} · to ${formatShortDate(request.until)} instead of ${formatShortDate(request.current_until)}`;
}

/**
 * The same line for the person who asked: no date for a new ask, because
 * approving starts it then. A request to keep counts from the current end, so
 * its dates are known and read the same.
 */
export function describeAskForMember(request: AccessRequest, terms?: Term[]): string {
  const days = request.duration_days ?? 0;
  if (request.kind !== "extend") return askLengthForMember(days);
  return describeAsk(request, terms);
}

/**
 * For the person deciding. A request is for a role someone lacks (owner,
 * 15 Oct); if they have come to hold it since asking, approving is off, and the
 * row says why and what closes it.
 */
export function HoldingNote({ request }: { request: AccessRequest }) {
  const holding = request.holding;
  const resolver = useNameResolver();
  if (request.kind === "extend" && holding && holding.kind !== "for_good") {
    // Never a guessed pronoun: the first name, or nothing that needs one.
    const first = resolver.resolveUser(request.requester_id).value?.display_name?.trim().split(/\s+/)[0];
    return (
      <span className="w-full text-[13px] text-muted">
        {holding.kind === "until"
          ? request.until
            ? `Approving moves ${first ? `${first}'s` : "their"} end date to ${formatShortDate(request.until)}. Nothing else changes.`
            : `Has it until ${formatShortDate(holding.until)}.`
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
      // When it was queued, from the access read; failing that, the newest
      // thing that gave it: when the member was told.
      const at = givenAt(role.reasons) ?? (changes.data ?? [])
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
