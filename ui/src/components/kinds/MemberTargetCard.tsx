"use client";

import { Card, CardHeader, CardRow } from "@/components/ui/Card";
import { StandingLine } from "@/components/ui/StandingLine";
import { StorageMemberCard } from "@/components/kinds/storage/MyStorage";
import { NOTHING_TO_DO } from "@/lib/outcome";
import { targetLabel } from "@/lib/nav";
import type { MyTargetView } from "@/lib/queries/useMyStorage";

/**
 * A member's card for one system, chosen by the KIND the system's accepted
 * manifest declared. Storage has its own module (folders, addresses, a
 * password step); a kind with none (access, inventory, machine today) gets the
 * core card: the account's standing and its next step, which every kind can
 * say and none has to build. A system whose manifest has not been accepted has
 * no kind at all, and says so: it is never taken for one.
 */
const KIND_CARD: Record<string, (props: { view: MyTargetView }) => React.ReactElement> = {
  storage: StorageMemberCard,
};

export function MemberTargetCard({ view }: { view: MyTargetView }) {
  if (!view.kind) return <NotReadCard view={view} />;
  const Module = KIND_CARD[view.kind];
  return Module ? <Module view={view} /> : <CoreMemberCard view={view} />;
}

/** Syndra has not read this system's description yet: no account can be shown, and none is guessed at. */
export function NotReadCard({ view }: { view: MyTargetView }) {
  const name = targetLabel(view.target);
  return (
    <Card>
      <CardHeader title={name} />
      <CardRow first className="flex-wrap">
        <span className="text-[14px] text-muted">Syndra has not read {name} yet, so it cannot show your account here.</span>
        <span className="flex-1" />
        <span className="text-[13px] text-muted">{NOTHING_TO_DO} Makerspace staff can see this, and it appears here once {name} answers.</span>
      </CardRow>
    </Card>
  );
}

export function CoreMemberCard({ view }: { view: MyTargetView }) {
  const name = targetLabel(view.target);
  return (
    <Card>
      <CardHeader title={name} />
      <CardRow first className="flex-wrap">
        {view.account ? (
          <span className="min-w-0 [overflow-wrap:anywhere] font-mono text-[13.5px]">{view.account.username}</span>
        ) : (
          <span className="text-[14px] text-muted">{view.entitled ? `Your account on ${name} is not made yet.` : `None of your roles reaches ${name}.`}</span>
        )}
        <span className="flex-1" />
        {view.standing ? (
          <StandingLine standing={view.standing} system={name} audience="member" />
        ) : (
          <span className="text-[13px] text-muted">{NOTHING_TO_DO}</span>
        )}
      </CardRow>
    </Card>
  );
}
