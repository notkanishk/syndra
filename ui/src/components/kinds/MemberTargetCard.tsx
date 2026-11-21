"use client";

import { MemberActions } from "@/components/actions/MemberAction";
import { MEMBER } from "@/components/results/copy";
import { Reads } from "@/components/results/ReadResult";
import { StorageMemberCard } from "@/components/kinds/storage/MyStorage";
import { ViewUsage } from "@/components/usage/UsageCard";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Card, CardHeader, CardRow } from "@/components/ui/Card";
import { NotReadLine } from "@/components/ui/ReadFreshness";
import { StandingLine } from "@/components/ui/StandingLine";
import { NOTHING_TO_DO } from "@/lib/outcome";
import type { MyTargetView } from "@/lib/queries/useMyStorage";

/**
 * A member's card for one system, chosen by the KIND the system's accepted
 * manifest declared. Storage has its own module (folders, addresses, a
 * password step); a kind with none (access, inventory, machine today) gets the
 * core card: the account's standing and its next step, which every kind can
 * say and none has to build.
 *
 * A system Syndra has not read yet never reaches a member: the member reads
 * (GET /me/systems, /me/targets) leave it out (owner, 11 Nov), so it is never
 * taken for one kind and never shown half-described. Operators see it on theirs.
 *
 * `untitled` leaves the card's own title off where the page's heading is
 * already the system's name (`/systems/{target}`).
 */
type CardProps = { view: MyTargetView; untitled?: boolean };

const KIND_CARD: Record<string, (props: CardProps) => React.ReactElement> = {
  storage: StorageMemberCard,
};

export function MemberTargetCard(props: CardProps) {
  const Module = KIND_CARD[props.view.kind ?? ""] ?? CoreMemberCard;
  // "What I have used" is every kind's, from the same descriptors, so it sits
  // beside the kind's card and not inside any of them.
  return (
    <div className="grid gap-4">
      <Module {...props} />
      <ViewUsage view={props.view} />
    </div>
  );
}

/** "Your account on Big laser is not made yet. Nothing for you to do. Syndra makes it." */
const SYNDRA_MAKES_IT = `${NOTHING_TO_DO} Syndra makes it.`;

export function CoreMemberCard({ view, untitled = false }: CardProps) {
  const name = view.name;
  const addon = view.read && view.read.state !== "none" ? view.read.data : undefined;
  return (
    <Card>
      {!untitled && <CardHeader title={name} />}
      {/* A system that can't be read says why, once, in the member's one sentence (design §13.1). */}
      {view.read?.state === "none" && <NotReadLine why={view.read.why} system={name} reader={MEMBER} className="mx-5 mt-[18px]" />}
      <CardRow first className="flex-wrap">
        {view.account ? (
          <span className="min-w-0 [overflow-wrap:anywhere] font-mono text-small">{view.account.username}</span>
        ) : (
          <span className="text-body text-muted">
            {view.entitled ? `Your account on ${name} is not made yet.` : `None of your roles reaches ${name}.`}
          </span>
        )}
        <span className="flex-1" />
        {addon?.standing ? (
          <StandingLine standing={addon.standing} system={name} audience="member" />
        ) : !view.entitled ? (
          <ArrowLink href="/requests?ask">Ask for access in Requests</ArrowLink>
        ) : (
          <span className="text-small text-muted">{view.account ? NOTHING_TO_DO : SYNDRA_MAKES_IT}</span>
        )}
      </CardRow>
      <Reads reads={addon?.reads} name={name} reader={MEMBER} />
      <MemberActions view={view} />
    </Card>
  );
}
