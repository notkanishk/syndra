"use client";

import Link from "next/link";
import { useState } from "react";

import { EmptyState, ListStates } from "@/components/states";
import { HoldDialog } from "@/components/review/HoldDialog";
import { ServiceAccountTag, useAccountHref } from "@/components/system/ServiceAccountTag";
import { TakeAwayDialog } from "@/components/targets/TakeAwayDialog";
import { Button } from "@/components/ui/Button";
import { Card, CardColumns, CardHeader, CardRow } from "@/components/ui/Card";
import { LeavingRow, useLeavingRows } from "@/components/ui/LeavingRows";
import { PaginationRow, usePaging } from "@/components/ui/PaginationRow";
import { UserAvatar, UserName } from "@/components/names";
import { formatShortDate } from "@/lib/format";
import { targetLabel } from "@/lib/nav";
import { useNameResolver } from "@/lib/queries/useNameResolver";
import { useTargetHomes, useTargetInventory, type AccountHome, type BoundAccount } from "@/lib/queries/useTargets";
import { HomeLine, LimitDialog, limitVerb } from "@/components/targets/HomeLimit";

/**
 * Whose accounts are on this target (design §21's second question).
 *
 * The inventory beside this one answers which accounts Syndra does NOT manage.
 * This answers the half an operator actually acts on, and it had no surface at
 * all: the bindings were read on every inventory pass, counted, and thrown away.
 *
 * It is also where the two answers to one question live side by side — **pause
 * it or end it.** A hold withholds what a role grants and can be lifted; taking
 * access away composes a hold with a credential rotation and is dressed as the
 * revocation it is. Putting them on the same row is what makes the choice
 * visible; putting them on different screens would make the heavier one the
 * only one an operator remembers.
 */
export function PeopleOnTarget({ target }: { target: string }) {
  const inventory = useTargetInventory(target);
  const homes = useTargetHomes(target);
  const accounts = inventory.data?.accounts ?? [];
  const homeOf = new Map((homes.data?.accounts ?? []).map((h) => [h.subject_id, h]));
  const name = targetLabel(target);
  const paging = usePaging();
  // S5: somebody whose access has been revoked leaves on the next read, fading where they were and
  // closing the gap after, and not under the pointer.
  const leaving = useLeavingRows(accounts, (a) => a.subject_id, !inventory.isLoading && !inventory.error);
  const drawn = leaving.rows.slice(0, paging.limit);
  // A limit verb widens the actions seat (A-03 §3); the heads follow the rows.
  const seat = accounts.some((a) => limitVerb(homeOf.get(a.subject_id))) ? "tablet:w-[380px]" : "tablet:w-[260px]";

  return (
    <Card {...leaving.listProps}>
      <CardHeader
        title="Accounts Syndra manages"
        count={inventory.data?.bound}
        note="Put what a role gives them on hold, or revoke it (end their access)"
      />
      <ListStates
        isLoading={inventory.isLoading}
        error={inventory.error}
        isEmpty={leaving.rows.length === 0}
        onRetry={() => inventory.refetch()}
        errorTitle="The account list could not be read"
        empty={
          <EmptyState
            title="Nobody yet"
            guidance={`No role is mapped to ${name} yet, or the changes that would create accounts are still waiting for you to send from Pending changes.`}
          />
        }
      >
        <>
          {/* Name · description · numbers (8b): the person, the account they
              have there, and since when in the last seat. */}
          <CardColumns>
            <span className="w-[220px] flex-none">Person</span>
            <span className="min-w-0 flex-1">Account on {name}</span>
            <span className="w-[110px] flex-none text-right">Since</span>
            <span className={`flex-none ${seat}`} />
          </CardColumns>
          {drawn.map((account) => (
            <LeavingRow key={account.subject_id} {...leaving.props(account.subject_id)}>
              <PersonRow
                target={target}
                account={account}
                home={homeOf.get(account.subject_id)}
                seat={seat}
              />
            </LeavingRow>
          ))}
          <PaginationRow
            shown={Math.min(paging.limit, accounts.length)}
            total={accounts.length}
            onMore={paging.more}
            onAll={() => paging.all(accounts.length)}
            onBack={paging.back}
          />
        </>
      </ListStates>
    </Card>
  );
}

function PersonRow({
  target,
  account,
  home,
  seat,
}: {
  target: string;
  account: BoundAccount;
  /** The actions seat's width, shared with the column heads. */
  seat: string;
  /** Their home folder limit and where it comes from; absent until read. */
  home?: AccountHome;
}) {
  const [dialog, setDialog] = useState<"hold" | "takeaway" | "limit" | null>(null);
  const verb = limitVerb(home);
  // The name, because both dialogs speak to and about a person — and rung 3
  // asks the operator to type it. Through the same resolver every other surface
  // uses, so the name in the dialog is the name in the row: asking somebody to
  // type "Ada Rivera" beside a row that says "Ada" is a puzzle, not a check.
  const resolver = useNameResolver();
  const accountHref = useAccountHref();
  const name = resolver.resolveUser(account.subject_id).value?.display_name ?? account.subject_id;

  return (
    <>
      <CardRow data-account={account.subject_id} className="flex-wrap gap-y-1.5">
        <span className="flex w-[220px] flex-none items-center gap-2.5">
          <UserAvatar id={account.subject_id} size="row" />
          <span className="min-w-0">
            <Link
              href={accountHref(account.subject_id)}
              className="[overflow-wrap:anywhere] text-[14.5px] font-semibold text-ink motion-tint hover:text-accent-text"
            >
              <UserName id={account.subject_id} />
            </Link>
            <ServiceAccountTag id={account.subject_id} />
          </span>
        </span>
        <span className="min-w-0 flex-1 basis-[200px] [overflow-wrap:anywhere] font-mono text-[13.5px] text-ink">
          {account.username}
        </span>
        <span className="w-[110px] flex-none text-right text-[13.5px] text-muted">{formatShortDate(account.bound_at)}</span>
        <span className={`flex flex-none flex-wrap items-center justify-end gap-2 ${seat}`}>
          {/* Its home folder limit first, the one write place for it (A-03 §3);
              then pause it, or end it, the reversible one first. */}
          {verb && (
            <Button variant="outline" size="sm" onClick={() => setDialog("limit")}>
              {verb}
            </Button>
          )}
          <Button variant="outline" size="sm" onClick={() => setDialog("hold")}>
            Put on hold
          </Button>
          <Button variant="danger" size="sm" onClick={() => setDialog("takeaway")}>
            Revoke
          </Button>
        </span>
        {home && <HomeLine home={home} />}
      </CardRow>
      {dialog === "limit" && home && (
        <LimitDialog target={target} name={name} home={home} onClose={() => setDialog(null)} />
      )}

      {dialog === "hold" && (
        <HoldDialog
          subjectId={account.subject_id}
          subjectName={name}
          target={target}
          // The lifecycle field, which is what a hold on a whole target holds.
          // A hold on one share is authored from the share's own row on the
          // person's page; this row is about the account.
          //
          // `value` is what the resolver reads — a lifecycle denial is always
          // `enabled = true`, the state being refused — and `label` is what the
          // operator reads. Sending the label would be a malformed term the
          // backend refuses; showing the value would be a dialog saying "Hold
          // true for Ada".
          field="enabled"
          value="true"
          label={`access to ${targetLabel(target)}`}
          onClose={() => setDialog(null)}
        />
      )}
      {dialog === "takeaway" && (
        <TakeAwayDialog
          target={target}
          subjectId={account.subject_id}
          subjectName={name}
          onClose={() => setDialog(null)}
        />
      )}
    </>
  );
}
