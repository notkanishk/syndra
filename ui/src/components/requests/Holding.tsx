"use client";

import { StatusDot } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { askLengthForMember, askLengthForOperator, formatShortDate } from "@/lib/format";
import { civilDay } from "@/lib/time";
import type { AccessRequest } from "@/lib/queries/useRequests";
import { useRequestsMine } from "@/lib/queries/useRequests";
import { onItsWay } from "@/lib/standing";
import { givenAt } from "@/components/access/AccessSource";
import { useMyChanges } from "@/lib/queries/useMyChanges";
import { useNameResolver } from "@/lib/queries/useNameResolver";
import { RuleConsentFailed } from "@/components/requests/RuleConsent";
import { useForetell } from "@/components/ui/CancelQueuedStep";
import { usePlanConsent } from "@/lib/usePlanConsent";
import type { Term } from "@/lib/queries/useTerms";
import { useUserAccess } from "@/lib/queries/useUsers";

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

/** The same makerspace day: a term's last day against an end counted in days. */
function sameDay(ymd: string, iso: string): boolean {
  return civilDay(new Date(iso)) === ymd;
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
  if (request.kind !== "extend") return askLengthForOperator(days, new Date(), request.asked_until);
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
  return (
    <>
      <HeldLine request={request} />
      <RulesLine request={request} />
    </>
  );
}

/**
 * What approving does, said where the approve button is, before the press
 * (A-21 §3.2): what the automatic rules also give (audit C2) and the waiting
 * change it would cancel, from the planner, the same Change the approval
 * writes. Nothing when nothing follows or there is nothing to approve.
 */
function RulesLine({ request }: { request: AccessRequest }) {
  const consent = useApprovePlan(request);
  const foretell = useForetell();
  // Final verification N5: a plan that can't be read is said, never silence
  // that reads as "nothing else follows" (the one consent rule).
  if (consent.failed) return <RuleConsentFailed consent={consent} />;
  const lines = [...consent.rules, foretell("Approving", consent.cancels)].filter(Boolean);
  if (lines.length === 0) return null;
  return <span className="w-full text-[13px] text-muted">If you approve: {lines.join(" ")}</span>;
}

/** The plan for approving this request: one read, shared by the line and the button. */
export function useApprovePlan(request: AccessRequest) {
  return usePlanConsent(
    alreadyHeld(request) ? null : { op: "approve", userId: request.requester_id, requestId: request.id },
    "approving",
  );
}

/**
 * Why approving is held, or undefined when it can go (N5). The consent is the
 * row: what approving does is part of what the person decides, so approve
 * waits for the plan, and a plan that failed holds it with the reason, rather
 * than approving a gift or a cancel nobody was shown. The same query as
 * RulesLine, so it is one read.
 */
export function useApproveHold(request: AccessRequest): string | undefined {
  const held = alreadyHeld(request);
  const consent = useApprovePlan(request);
  if (held) return "They already have this.";
  const plan = consent.plan;
  if (plan && plan.effect !== "apply") return [plan.detail, plan.consequence].filter(Boolean).join(" ");
  return consent.hold;
}

/** The Approve button, held while the rule preview is unread (N5). One for Home, Requests and a person's Requests tab. */
export function ApproveButton({
  request,
  onClick,
  className,
  isPending,
}: {
  request: AccessRequest;
  onClick: () => void;
  className?: string;
  isPending?: boolean;
}) {
  const hold = useApproveHold(request);
  return (
    <Button
      variant="accent"
      size="sm"
      className={className}
      isPending={isPending}
      disabled={hold !== undefined}
      reason={hold}
      onClick={onClick}
    >
      Approve
    </Button>
  );
}

function HeldLine({ request }: { request: AccessRequest }) {
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
    <span className="flex w-full items-baseline gap-2 text-[13px] text-muted">
      <StatusDot tone="neutral" />
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
  | { kind: "onway"; at?: string }
  /** A removal is on its way and Zitadel still shows it; it can be asked for again. */
  | { kind: "removing" };

/**
 * What a person already has, or has asked for, of each role: the dialog's
 * reading of the same facts the backend checks. Only "none" can be asked for,
 * and "until" can be asked to keep.
 */
export function useAskState(userId: string) {
  const access = useUserAccess(userId);
  const requests = useRequestsMine();
  const changes = useMyChanges();

  return (projectId: string, roleKey: string): AskState => {
    if (!projectId || !roleKey) return { kind: "none" };
    const waiting = (requests.data ?? []).find(
      (r) => r.status === "pending" && r.requester_id === userId && r.project_id === projectId && r.role_key === roleKey,
    );
    if (waiting) return { kind: "asked", at: waiting.created_at };
    const project = access.data?.projects?.find((p) => p.project_id === projectId);
    const role = [...(project?.source_roles ?? []), ...(project?.derived_roles ?? [])].find(
      (r) => r.role_key === roleKey,
    );
    // Whether they have it is the backend's answer (lib/standing.ts), only read here.
    const standing = project?.standings?.[roleKey];
    if (onItsWay(standing)) {
      // When it was queued, from the access read; failing that, the newest
      // thing that gave it: when the member was told.
      const at = givenAt(role?.reasons) ?? (changes.data ?? [])
        .filter((c) => c.project_id === projectId && c.role_key === roleKey && (c.kind === "given" || c.waiting))
        .map((c) => c.at)
        .sort()
        .pop();
      return { kind: "onway", at };
    }
    if (standing === "removing") return { kind: "removing" };
    // After "on its way": an end on a grant that hasn't reached Zitadel isn't
    // theirs to keep yet. Whether they already have it is the backend's rule
    // (holdings), the same one that refuses a second ask.
    const holding = project?.holdings?.[roleKey];
    if (holding?.kind === "until" && holding.until) return { kind: "until", until: holding.until };
    if (holding?.kind === "for_good" || holding?.kind === "until") return { kind: "for_good" };
    return { kind: "none" };
  };
}
