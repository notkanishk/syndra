"use client";

import { Card, CardHeader, CardRow } from "@/components/ui/Card";
import { StandingLine } from "@/components/ui/StandingLine";
import { StorageMemberCard } from "@/components/kinds/storage/MyStorage";
import { targetLabel } from "@/lib/nav";
import { useCachedRoster } from "@/lib/queries/useFieldSchema";
import type { MyTargetView } from "@/lib/queries/useMyStorage";

/**
 * A member's card for one system, chosen by the system's KIND. Storage has its
 * own module (folders, addresses, a password step); a kind with none (access,
 * inventory, machine today) gets the core card: the account's standing and its
 * next step, which every kind can say and none has to build.
 */
const KIND_CARD: Record<string, (props: { view: MyTargetView }) => React.ReactElement> = {
  storage: StorageMemberCard,
};

/** The roster's cache only: a member's proxy refuses it, and the member page reads another endpoint. */
export function useKind(target: string): string {
  const row = useCachedRoster()?.find((t) => t.target === target);
  // The backend does not send `kind` yet; the one kind that exists says storage.
  return row?.kind ?? "storage";
}

export function MemberTargetCard({ view, kind }: { view: MyTargetView; kind?: string }) {
  const own = useKind(view.target);
  const Card_ = KIND_CARD[kind ?? own];
  return Card_ ? <Card_ view={view} /> : <CoreMemberCard view={view} />;
}

export function CoreMemberCard({ view }: { view: MyTargetView }) {
  const name = targetLabel(view.target);
  return (
    <Card>
      <CardHeader title={name} />
      <CardRow first>
        {view.account ? (
          <span className="min-w-0 [overflow-wrap:anywhere] font-mono text-[13.5px]">{view.account.username}</span>
        ) : (
          <span className="text-[14px] text-muted">{view.entitled ? `Your account on ${name} is not made yet.` : `None of your roles reaches ${name}.`}</span>
        )}
        <span className="flex-1" />
        {view.standing ? (
          <StandingLine standing={view.standing} system={name} audience="member" />
        ) : (
          <span className="text-[13px] text-muted">Nothing for you to do.</span>
        )}
      </CardRow>
    </Card>
  );
}
