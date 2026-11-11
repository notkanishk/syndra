"use client";

import { StorageMemberCard } from "@/components/kinds/storage/MyStorage";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Card, CardHeader, CardRow } from "@/components/ui/Card";
import { Notice } from "@/components/ui/Notice";
import { StandingLine } from "@/components/ui/StandingLine";
import { formatClock } from "@/lib/format";
import { Unfold } from "@/lib/motion";
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
  return <Module {...props} />;
}

/** "Your account on Big laser is not made yet. Nothing for you to do. Syndra makes it." */
const SYNDRA_MAKES_IT = `${NOTHING_TO_DO} Syndra makes it.`;

export function CoreMemberCard({ view, untitled = false }: CardProps) {
  const name = view.name;
  const offline = view.health === "offline";
  return (
    <Card>
      {!untitled && <CardHeader title={name} />}
      {/* Every kind has an offline, so the core card says it the way the
          storage card does: what is safe, and that the page catches up. */}
      <Unfold open={offline}>
        <Notice
          tone="warn"
          title={`${name} is offline`}
          live
          className="mx-5 mt-[18px]"
          nothingToDo={`This page updates on its own when ${name} returns.`}
        >
          It stopped answering{view.reachable_since ? ` at ${formatClock(view.reachable_since)}` : ""}. Your
          account and access are safe.
        </Notice>
      </Unfold>
      <CardRow first className="flex-wrap">
        {view.account ? (
          <span className="min-w-0 [overflow-wrap:anywhere] font-mono text-[13.5px]">{view.account.username}</span>
        ) : (
          <span className="text-[14px] text-muted">
            {view.entitled ? `Your account on ${name} is not made yet.` : `None of your roles reaches ${name}.`}
          </span>
        )}
        <span className="flex-1" />
        {view.standing ? (
          <StandingLine standing={view.standing} system={name} audience="member" />
        ) : !view.entitled ? (
          <ArrowLink href="/requests?ask">Ask for access in Requests</ArrowLink>
        ) : (
          <span className="text-[13px] text-muted">{view.account ? NOTHING_TO_DO : SYNDRA_MAKES_IT}</span>
        )}
      </CardRow>
    </Card>
  );
}
